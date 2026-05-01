"""Normalize GCP Cloud CDN SKU JSON into network.cdn rows.

Spec §5 network.cdn kind. Cloud CDN is billed under the Networking service
(E505-1604-58F8). Two resourceGroup values are relevant:

  CDNCacheEgress — geography-based egress from cache to clients (7 SKUs).
  Cdn            — cache lookup request count (1 SKU).

EdgeCacheEgress (Media CDN) is explicitly excluded.

Each CDNCacheEgress SKU has 4 tieredRates. We sort them by startUsageAmount
ascending and zip with canonical tokens ["0", "10TB", "150TB", "1PB"]. The exact
GiB boundaries differ from round powers of 1000 because Google bills in GiB
but defines tiers in TB (base-10). Sorting and zipping is more robust than
comparing float boundaries.

The Cdn cache-lookup SKU produces a single global request row.

NOTE: Both "North America" and "Other" map to us-east1. This means two
egress rows share region=us-east1 in the live shard (different sku_ids).
LookupCDN with region="us-east1" will return both. This is intentional —
the "Other" geography is a catch-all that happens to share US pricing.
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
from .gcp_common import load_region_normalizer, parse_unit_price

_PROVIDER = "gcp"
_SERVICE = "cloud-cdn"
_KIND = "network.cdn"

# Geography suffix → canonical GCP region
_CDN_GEO_MAP: dict[str, str] = {
    "North America": "us-east1",
    "Europe": "europe-west1",
    "Asia": "asia-northeast1",
    "Latin America": "southamerica-east1",
    "Oceania": "australia-southeast1",
    "Other": "us-east1",   # same price band as North America
    "China": "asia-east1",
}

# Description prefix shared by all CDNCacheEgress SKUs
_EGRESS_PREFIX = "Networking Cloud CDN Traffic Cache Data Transfer to "

# Tier tokens zipped with sorted tieredRates (ascending startUsageAmount).
# Real GCP CDN billing has 4 tiers: 0, ~10TB, ~150TB, ~1PB.
_EGRESS_TIER_TOKENS = ["0", "10TB", "150TB", "1PB"]


def _parse_egress_geo(description: str) -> str | None:
    """Extract geography name from a CDNCacheEgress SKU description.

    Returns the canonical key into _CDN_GEO_MAP, or None if the description
    doesn't match the expected prefix.
    """
    if not description.startswith(_EGRESS_PREFIX):
        return None
    return description[len(_EGRESS_PREFIX):]


def _build_egress_prices(tiered_rates: list[dict]) -> list[dict]:
    """Build a 3-tier contiguous egress prices list from GCP tieredRates.

    Sorts by startUsageAmount ascending and zips with ["0","10TB","150TB"].
    Price unit is always "gb" (API usageUnit is GiBy).
    """
    sorted_rates = sorted(tiered_rates, key=lambda r: float(r["startUsageAmount"]))
    if len(sorted_rates) != len(_EGRESS_TIER_TOKENS):
        raise ValueError(
            f"expected {len(_EGRESS_TIER_TOKENS)} tieredRates, got {len(sorted_rates)}"
        )
    prices = []
    n = len(_EGRESS_TIER_TOKENS)
    for i, (token, rate) in enumerate(zip(_EGRESS_TIER_TOKENS, sorted_rates)):
        unit_price = rate["unitPrice"]
        amount = parse_unit_price(
            units=unit_price["units"],
            nanos=int(unit_price["nanos"]),
        )
        tier_upper = _EGRESS_TIER_TOKENS[i + 1] if i < n - 1 else ""
        prices.append({
            "dimension": "egress",
            "tier": token,
            "tier_upper": tier_upper,
            "amount": amount,
            "unit": "gb",
        })
    return prices


def ingest(*, skus_path: Path) -> Iterable[dict[str, Any]]:
    with skus_path.open() as f:
        skus = json.load(f).get("skus", [])

    base_terms = apply_kind_defaults(_KIND, {
        "commitment": "on_demand",
        "tenancy": "",
        "os": "",
        "support_tier": "",
        "upfront": "",
        "payment_option": "",
    })
    th = terms_hash(base_terms)

    egress_rows: list[dict[str, Any]] = []
    request_row: dict[str, Any] | None = None

    for sku in skus:
        category = sku.get("category", {})
        resource_group = category.get("resourceGroup", "")

        # Skip EdgeCacheEgress (Media CDN) and anything else we don't handle
        if resource_group == "EdgeCacheEgress":
            continue
        if resource_group not in ("CDNCacheEgress", "Cdn"):
            continue

        usage_type = category.get("usageType", "OnDemand")
        if usage_type != "OnDemand":
            continue

        pricing_info = sku.get("pricingInfo", [])
        if not pricing_info:
            continue
        pricing_expr = pricing_info[0].get("pricingExpression", {})
        tiered_rates = pricing_expr.get("tieredRates", [])
        if not tiered_rates:
            continue

        description = sku.get("description", "")

        if resource_group == "CDNCacheEgress":
            geo = _parse_egress_geo(description)
            if geo is None or geo not in _CDN_GEO_MAP:
                continue
            region = _CDN_GEO_MAP[geo]
            sku_id = f"CLOUD-CDN-EGRESS-{geo.replace(' ', '-').upper()}"
            prices = _build_egress_prices(tiered_rates)
            egress_rows.append({
                "sku_id": sku_id,
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": "standard",
                "region": region,
                "region_normalized": "global",
                "terms_hash": th,
                "resource_attrs": {
                    "extra": {
                        "mode": "edge-egress",
                        "sku": "cloud-cdn-standard",
                    },
                },
                "terms": base_terms,
                "prices": prices,
            })

        elif resource_group == "Cdn":
            # Cache lookup requests — single global row
            rate = tiered_rates[0]
            unit_price = rate["unitPrice"]
            amount = parse_unit_price(
                units=unit_price["units"],
                nanos=int(unit_price["nanos"]),
            )
            request_row = {
                "sku_id": "CLOUD-CDN-REQUESTS-GLOBAL",
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": "standard",
                "region": "global",
                "region_normalized": "global",
                "terms_hash": th,
                "resource_attrs": {
                    "extra": {
                        "mode": "request",
                        "sku": "cloud-cdn-standard",
                    },
                },
                "terms": base_terms,
                "prices": [
                    {
                        "dimension": "request",
                        "tier": "0",
                        "tier_upper": "",
                        "amount": amount,
                        "unit": "request",
                    }
                ],
            }

    # Emit egress rows sorted by sku_id for deterministic golden output
    yield from sorted(egress_rows, key=lambda r: r["sku_id"])
    if request_row is not None:
        yield request_row


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.gcp_cloud_cdn")
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
    print(f"ingest.gcp_cloud_cdn: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
