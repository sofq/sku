"""Normalize AWS Route53 offer JSON into sku row dicts.

Spec §5 dns.zone kind. Route53 is a global service — all rows use
region="global" / region_normalized="global". Two zone types are emitted:
  - "public"  (resource_name="public")  with hosted_zone + query price tiers
  - "private" (resource_name="private") with hosted_zone price tiers only

Zone pricing (HostedZone group, productFamily='DNS Zone'):
  tier  0-25  zones  : $0.50/zone/month
  tier 25+    zones  : $0.10/zone/month

Query pricing (usagetype ending '-DNS-Queries', productFamily='DNS Query'):
  tier  0-1B  queries: $0.0000004/query
  tier 1B+    queries: $0.0000002/query

Firewall queries, geo-based, and latency-based query SKUs are excluded.
"""

from __future__ import annotations

import argparse
import json
import sys
from collections.abc import Iterable
from pathlib import Path
from typing import Any

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import dumps

_PROVIDER = "aws"
_SERVICE = "route53"
_KIND = "dns.zone"
_REGION = "global"
_REGION_NORMALIZED = "global"


def _collect_tiered_prices(
    pricedimensions: dict,
    dimension: str,
    unit: str,
) -> list[dict]:
    """Convert AWS priceDimensions dict into sorted contiguous tier list.

    Args:
        pricedimensions: The priceDimensions dict from the offer.
        dimension: The dimension name to assign each price row.
        unit: The unit string to use for each price row.

    Returns:
        A sorted list of price dicts with tier/tier_upper pairs.
    """
    entries = []
    for pd in pricedimensions.values():
        begin = pd.get("beginRange", "")
        end = pd.get("endRange", "")
        usd_str = pd.get("pricePerUnit", {}).get("USD", "0")
        usd = float(usd_str)
        entries.append((begin, end, usd))

    # Sort by numeric begin range
    def _begin_int(e: tuple) -> int:
        try:
            return int(e[0])
        except (ValueError, TypeError):
            return 0

    entries.sort(key=_begin_int)

    # Map begin/end to tier tokens
    def _to_tier_token(raw: str) -> str:
        """Map raw begin/endRange to canonical tier token."""
        if raw in ("", "Inf"):
            return ""
        try:
            val = int(raw)
        except (ValueError, TypeError):
            return raw
        if val == 0:
            return "0"
        if val == 25:
            return "25"
        if val >= 1_000_000_000:
            return "1B"
        return str(val)

    prices = []
    n = len(entries)
    for i, (begin, end, usd) in enumerate(entries):
        tier = _to_tier_token(begin)
        # tier_upper: use next entry's begin if not last; empty string for last
        if i < n - 1:
            tier_upper = _to_tier_token(entries[i + 1][0])
        else:
            tier_upper = ""
        prices.append({
            "dimension": dimension,
            "tier": tier,
            "tier_upper": tier_upper,
            "amount": usd,
            "unit": unit,
        })
    return prices


def ingest(*, offer_path: Path) -> Iterable[dict[str, Any]]:
    with offer_path.open() as f:
        offer = json.load(f)

    products = offer.get("products", {})
    terms_od = offer.get("terms", {}).get("OnDemand", {})

    # ---- Collect zone prices (productFamily=DNS Zone, group=HostedZone) ----
    zone_prices: list[dict] | None = None

    for sku_id, product in products.items():
        if product.get("productFamily") != "DNS Zone":
            continue
        attrs = product.get("attributes", {})
        # Real offer uses usagetype=HostedZone; fixture uses group=HostedZone
        if attrs.get("group") != "HostedZone" and attrs.get("usagetype") != "HostedZone":
            continue
        # Get price dimensions for this SKU
        sku_terms = terms_od.get(sku_id, {})
        if not sku_terms:
            continue
        term = next(iter(sku_terms.values()), None)
        if term is None:
            continue
        pds = term.get("priceDimensions", {})
        zone_prices = _collect_tiered_prices(pds, "hosted_zone", "mo")
        break  # Only one HostedZone SKU expected

    # ---- Collect query prices (productFamily=DNS Query, usagetype *-DNS-Queries) ----
    query_prices: list[dict] | None = None

    for sku_id, product in products.items():
        if product.get("productFamily") != "DNS Query":
            continue
        attrs = product.get("attributes", {})
        usagetype = attrs.get("usagetype", "")
        # Only standard DNS queries — exclude Firewall, Geo, Proximity, Latency, etc.
        if not (usagetype == "DNS-Queries" or usagetype.endswith("-DNS-Queries")):
            continue
        # Exclude special query types
        lowered = usagetype.lower()
        if any(x in lowered for x in ("firewall", "geo", "proximity", "latency", "routing")):
            continue
        sku_terms = terms_od.get(sku_id, {})
        if not sku_terms:
            continue
        term = next(iter(sku_terms.values()), None)
        if term is None:
            continue
        pds = term.get("priceDimensions", {})
        query_prices = _collect_tiered_prices(pds, "query", "query")
        break  # Take first matching query SKU

    if zone_prices is None:
        return

    base_terms = apply_kind_defaults(_KIND, {
        "commitment": "on_demand",
        "tenancy": "",
        "os": "",
        "support_tier": "",
        "upfront": "",
        "payment_option": "",
    })
    th = terms_hash(base_terms)

    for zone_type in ("public", "private"):
        # Public zones get both zone + query prices; private gets only zone prices
        if zone_type == "public" and query_prices is not None:
            prices = zone_prices + query_prices
        else:
            prices = list(zone_prices)

        yield {
            "sku_id": f"R53-ZONE-001-{zone_type}",
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": zone_type,
            "region": _REGION,
            "region_normalized": _REGION_NORMALIZED,
            "terms_hash": th,
            "resource_attrs": {
                "extra": {
                    "mode": zone_type,
                },
            },
            "terms": base_terms,
            "prices": prices,
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_route53")
    ap.add_argument("--fixture", type=Path)
    ap.add_argument("--offer", type=Path)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--catalog-version", default=None)
    args = ap.parse_args()
    if args.fixture:
        offer_path = args.fixture / "offer.json" if args.fixture.is_dir() else args.fixture
    elif args.offer:
        offer_path = args.offer
    else:
        print("either --fixture or --offer required", file=sys.stderr)
        return 2
    args.out.parent.mkdir(parents=True, exist_ok=True)
    n = 0
    with args.out.open("w") as fh:
        for row in ingest(offer_path=offer_path):
            fh.write(dumps(row) + "\n")
            n += 1
    print(f"ingest.aws_route53: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
