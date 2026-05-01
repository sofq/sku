"""Normalize GCP Cloud DNS SKU JSON into dns.zone rows.

Spec §5 dns.zone kind. Cloud DNS is globally priced — all rows use
region="global" / region_normalized="global". Only the "public" zone type
is emitted (no private-zone distinction in the upstream billing API).

Zone pricing (ManagedZone SKU):
  tier  0–25   zones : $0.20/zone/month
  tier  25–10K zones : $0.10/zone/month
  tier  10K+   zones : $0.03/zone/month

Query pricing (DNS Query SKU):
  tier  0–1B   queries: $0.0000004/query
  tier  1B+    queries: $0.0000002/query
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
from .gcp_common import parse_unit_price

_PROVIDER = "gcp"
_SERVICE = "cloud-dns"
_KIND = "dns.zone"
_REGION = "global"
_REGION_NORMALIZED = "global"
_SKU_ID = "GCP-DNS-PUBLIC-GLOBAL"

# Tier token pairs: (lower, upper). Empty upper = unbounded last tier.
_ZONE_TIERS = [("0", "25"), ("25", "10K"), ("10K", "")]
_QUERY_TIERS = [("0", "1B"), ("1B", "")]


def _build_tiered_prices(
    tiered_rates: list[dict],
    tier_mapping: list[tuple[str, str]],
    dimension: str,
    unit: str,
) -> list[dict]:
    """Convert tieredRates list into canonical price dicts using a tier mapping.

    Args:
        tiered_rates: GCP billing tieredRates list (each with startUsageAmount + unitPrice).
        tier_mapping: List of (tier, tier_upper) string pairs to assign to each rate.
        dimension: The price dimension name.
        unit: The canonical unit string.

    Returns:
        List of price dicts sorted by startUsageAmount ascending.
    """
    sorted_rates = sorted(tiered_rates, key=lambda r: r.get("startUsageAmount", 0))
    if len(sorted_rates) != len(tier_mapping):
        raise ValueError(
            f"Expected {len(tier_mapping)} tieredRates for dimension={dimension!r}, "
            f"got {len(sorted_rates)}"
        )
    prices = []
    for rate, (tier, tier_upper) in zip(sorted_rates, tier_mapping):
        up = rate["unitPrice"]
        amount = parse_unit_price(units=up.get("units", "0"), nanos=up.get("nanos", 0))
        prices.append({
            "dimension": dimension,
            "tier": tier,
            "tier_upper": tier_upper,
            "amount": amount,
            "unit": unit,
        })
    return prices


def ingest(*, skus_path: Path) -> Iterable[dict[str, Any]]:
    with skus_path.open() as f:
        skus = json.load(f).get("skus", [])

    zone_tiered_rates: list[dict] | None = None
    query_tiered_rates: list[dict] | None = None

    for sku in skus:
        # Filter to DNS resource group only
        if sku.get("category", {}).get("resourceGroup") != "DNS":
            continue

        description = sku.get("description", "")
        try:
            tiered_rates = sku["pricingInfo"][0]["pricingExpression"]["tieredRates"]
        except (KeyError, IndexError):
            continue
        if not tiered_rates:
            continue

        # Identify SKU by description
        desc_lower = description.lower()
        if "managedzone" in desc_lower or "managed zone" in desc_lower:
            if zone_tiered_rates is None:
                zone_tiered_rates = tiered_rates
        elif "dns query" in desc_lower:
            if query_tiered_rates is None:
                query_tiered_rates = tiered_rates

    if zone_tiered_rates is None:
        return

    base_terms = apply_kind_defaults(_KIND, {
        "commitment": "on_demand",
        "tenancy": "",
        "os": "dns-public",
        "support_tier": "",
        "upfront": "",
        "payment_option": "",
    })
    th = terms_hash(base_terms)

    zone_prices = _build_tiered_prices(
        zone_tiered_rates, _ZONE_TIERS, "hosted_zone", "mo"
    )
    query_prices: list[dict] = []
    if query_tiered_rates is not None:
        query_prices = _build_tiered_prices(
            query_tiered_rates, _QUERY_TIERS, "query", "request"
        )

    yield {
        "sku_id": _SKU_ID,
        "provider": _PROVIDER,
        "service": _SERVICE,
        "kind": _KIND,
        "resource_name": "public",
        "region": _REGION,
        "region_normalized": _REGION_NORMALIZED,
        "terms_hash": th,
        "resource_attrs": {
            "extra": {
                "mode": "public",
            },
        },
        "terms": base_terms,
        "prices": zone_prices + query_prices,
    }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.gcp_cloud_dns")
    ap.add_argument("--fixture", type=Path)
    ap.add_argument("--skus", type=Path)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--catalog-version", default=None)
    args = ap.parse_args()
    if args.fixture:
        skus_path = args.fixture / "skus.json" if args.fixture.is_dir() else args.fixture
    elif args.skus:
        skus_path = args.skus
    else:
        print("either --fixture or --skus required", file=sys.stderr)
        return 2
    args.out.parent.mkdir(parents=True, exist_ok=True)
    n = 0
    with args.out.open("w") as fh:
        for row in ingest(skus_path=skus_path):
            fh.write(dumps(row) + "\n")
            n += 1
    print(f"ingest.gcp_cloud_dns: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
