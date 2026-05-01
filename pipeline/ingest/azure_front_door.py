"""Normalize Azure Front Door Retail Prices into network.cdn rows.

Azure Front Door has two current SKUs: Standard and Premium.
Each SKU emits three row types:
  - base-fee: a per-month base charge (region="global")
  - edge-egress: data transfer out, tiered by volume
  - request: per-request pricing (price per 10K in API, converted to per-request)

We filter to productName == "Azure Front Door" (not "Azure Front Door Service"
which is the deprecated classic tier).

The API returns zone names ("Zone 1" .. "Zone 8") as armRegionName, not ARM
region codes.  _ZONE_REGION_MAP translates each zone to a representative ARM
region for normalization.  Government zones ("US Gov Zone 1", "DE Gov Zone 2")
are absent from the map and are silently skipped.

Egress tiers are identified by the `tierMinimumUnits` field in the API response,
which maps directly to the canonical byte-domain tier tokens in _EGRESS_TIERS.
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
from .azure_common import load_region_normalizer, parse_request_uom, parse_storage_uom

_PROVIDER = "azure"
_SERVICE = "front-door"
_KIND = "network.cdn"

# Canonical byte-domain tier sequence for egress (from tier_tokens.py).
# The API returns one item per tier; we sort by price desc (tier-0 is most
# expensive) and zip with this sequence. The last emitted tier always gets
# tier_upper="" regardless of how many tiers the API returned.
_EGRESS_TIERS = [
    ("0", "10TB"),
    ("10TB", "50TB"),
    ("50TB", "150TB"),
    ("150TB", "500TB"),
    ("500TB", "1PB"),
    ("1PB", "5PB"),
    ("5PB", ""),
]

# Zone 1=North America, Zone 2=Asia Pacific (incl. Japan), Zone 3=South America,
# Zone 4=Australia, Zone 5=India, Zone 6=Europe, Zone 7=Middle East & Africa,
# Zone 8=Korea.  Government zones are omitted — skip them during ingest.
_ZONE_REGION_MAP: dict[str, str] = {
    "Zone 1": "eastus",
    "Zone 2": "japaneast",
    "Zone 3": "brazilsouth",
    "Zone 4": "australiaeast",
    "Zone 5": "centralindia",
    "Zone 6": "westeurope",
    "Zone 7": "southafricanorth",
    "Zone 8": "koreacentral",
}

_SKU_SLUG = {
    "Standard": "front-door-standard",
    "Premium": "front-door-premium",
}

_RESOURCE_NAME = {
    "Standard": "standard",
    "Premium": "premium",
}


def _mode_for_meter(meter_name: str) -> str | None:
    """Map meterName to internal mode token. Returns None to skip."""
    ml = meter_name.lower()
    if "base fees" in ml or "base fee" in ml:
        return "base-fee"
    if "data transfer out" in ml:
        return "edge-egress"
    if "requests" in ml or "request" in ml:
        return "request"
    return None


def ingest(*, prices_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with prices_path.open() as f:
        items = json.load(f).get("Items", [])

    # Bucket items by (sku_name, mode).
    # For base-fee: key is (sku_name, "base-fee", "global")
    # For egress/request: key is (sku_name, mode, arm_region_name)
    # Each bucket entry is a list of raw items.
    buckets: dict[tuple[str, str, str], list[dict[str, Any]]] = {}

    for item in items:
        product_name = item.get("productName", "").strip()
        if product_name != "Azure Front Door":
            continue
        sku_name = item.get("skuName", "").strip()
        if sku_name not in _SKU_SLUG:
            continue
        if item.get("type", "Consumption") != "Consumption":
            continue
        if item.get("currencyCode", "USD") != "USD":
            continue

        meter_name = item.get("meterName", "")
        mode = _mode_for_meter(meter_name)
        if mode is None:
            continue

        retail_price = float(item.get("retailPrice", 0))
        if retail_price <= 0:
            continue

        arm_region = item.get("armRegionName", "") or ""
        if mode == "base-fee":
            # Base-fee items come with no meaningful armRegionName
            region_key = "global"
        else:
            if not arm_region:
                continue
            region_key = arm_region

        key = (sku_name, mode, region_key)
        buckets.setdefault(key, []).append(item)

    # Now emit one row per bucket.
    for (sku_name, mode, region_key) in sorted(buckets.keys()):
        bucket_items = buckets[(sku_name, mode, region_key)]
        sku_slug = _SKU_SLUG[sku_name]
        resource_name = _RESOURCE_NAME[sku_name]

        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "",
            "os": "",
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })

        if mode == "base-fee":
            # Single item expected; emit with region="global"
            item = bucket_items[0]
            uom = item.get("unitOfMeasure", "")
            try:
                divisor, unit = parse_storage_uom(uom)
            except ValueError:
                divisor, unit = 1.0, "month"
            usd = float(item.get("retailPrice", 0)) / divisor
            sku_id = item.get("skuId") or f"afd-{sku_name.lower()}-base-fee"
            prices = [
                {
                    "dimension": "fee",
                    "tier": "0",
                    "tier_upper": "",
                    "amount": usd,
                    "unit": unit,
                },
            ]
            yield {
                "sku_id": sku_id,
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": resource_name,
                "region": "global",
                "region_normalized": "global",
                "terms_hash": terms_hash(terms),
                "resource_attrs": {
                    "extra": {
                        "sku": sku_slug,
                        "mode": mode,
                    },
                },
                "terms": terms,
                "prices": prices,
            }

        elif mode == "edge-egress":
            arm_region = _ZONE_REGION_MAP.get(region_key)
            if arm_region is None:
                continue  # government or unknown zone
            region_normalized = normalizer.try_normalize(_PROVIDER, arm_region)
            if region_normalized is None:
                continue

            # Sort by tierMinimumUnits ascending: tier 0 first.
            # Using tierMinimumUnits (not price) handles zones where high-volume
            # tiers share the same unit price (e.g., Zone 8).
            sorted_items = sorted(
                bucket_items, key=lambda x: float(x.get("tierMinimumUnits", 0))
            )

            prices = []
            for idx in range(min(len(sorted_items), len(_EGRESS_TIERS))):
                item = sorted_items[idx]
                tier_tok, tier_upper_tok = _EGRESS_TIERS[idx]
                usd = float(item.get("retailPrice", 0))
                is_last = idx == min(len(sorted_items), len(_EGRESS_TIERS)) - 1
                actual_tier_upper = "" if is_last else tier_upper_tok
                prices.append({
                    "dimension": "egress",
                    "tier": tier_tok,
                    "tier_upper": actual_tier_upper,
                    "amount": usd,
                    "unit": "gb",
                })

            sku_id = (
                sorted_items[0].get("skuId")
                or f"afd-{sku_name.lower()}-egress-{region_key}"
            )
            yield {
                "sku_id": sku_id,
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": resource_name,
                "region": region_key,
                "region_normalized": region_normalized,
                "terms_hash": terms_hash(terms),
                "resource_attrs": {
                    "extra": {
                        "sku": sku_slug,
                        "mode": mode,
                    },
                },
                "terms": terms,
                "prices": prices,
            }

        elif mode == "request":
            arm_region = _ZONE_REGION_MAP.get(region_key)
            if arm_region is None:
                continue  # government or unknown zone
            region_normalized = normalizer.try_normalize(_PROVIDER, arm_region)
            if region_normalized is None:
                continue

            # Take the tier-0 (lowest tierMinimumUnits) item as the base per-request price.
            item = min(bucket_items, key=lambda x: float(x.get("tierMinimumUnits", 0)))
            uom = item.get("unitOfMeasure", "")
            try:
                divisor, unit = parse_request_uom(uom)
            except ValueError:
                divisor, unit = 10_000.0, "requests"
            usd = float(item.get("retailPrice", 0)) / divisor
            sku_id = item.get("skuId") or f"afd-{sku_name.lower()}-request-{region_key}"
            prices = [
                {
                    "dimension": "request",
                    "tier": "0",
                    "tier_upper": "",
                    "amount": usd,
                    "unit": unit,
                },
            ]
            yield {
                "sku_id": sku_id,
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": resource_name,
                "region": region_key,
                "region_normalized": region_normalized,
                "terms_hash": terms_hash(terms),
                "resource_attrs": {
                    "extra": {
                        "sku": sku_slug,
                        "mode": mode,
                    },
                },
                "terms": terms,
                "prices": prices,
            }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.azure_front_door")
    ap.add_argument("--fixture", type=Path)
    ap.add_argument("--prices", type=Path)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--catalog-version", default=None)
    args = ap.parse_args()
    if args.fixture:
        prices_path = args.fixture / "prices.json" if args.fixture.is_dir() else args.fixture
    elif args.prices:
        prices_path = args.prices
    else:
        print("either --fixture or --prices required", file=sys.stderr)
        return 2
    args.out.parent.mkdir(parents=True, exist_ok=True)
    n = 0
    with args.out.open("w") as fh:
        for row in ingest(prices_path=prices_path):
            fh.write(dumps(row) + "\n")
            n += 1
    print(f"ingest.azure_front_door: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
