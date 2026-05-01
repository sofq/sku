"""Normalize Azure Service Bus Retail Prices into messaging.queue rows.

Azure Service Bus exposes two queue-relevant tiers:
  - Standard: tiered per-million-operations pricing (4 tiers, first is free).
  - Premium:  hourly per-Messaging-Unit pricing.

The upstream Azure Retail Prices API returns all 3 paid tier prices under a
single meterName "Standard Messaging Operations" (one item per price point per
region). We sort by price descending and zip with the hardcoded tier bounds.
The free first tier (0-13M) is hardcoded as _STD_FREE_TIER.
Premium returns a single hourly item.

Relevant meterNames:
  Standard:
    "Standard Messaging Operations"  -> all paid tiers ($0.80, $0.50, $0.20/M)
  Premium:
    "Premium Messaging Unit"         -> hourly per MU
"""

from __future__ import annotations

import argparse
import json
import sys
from collections import defaultdict
from collections.abc import Iterable
from pathlib import Path
from typing import Any

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import dumps
from .azure_common import (
    load_region_normalizer,
    parse_request_uom,
    parse_unit_of_measure,
)

_PROVIDER = "azure"
_SERVICE = "service-bus-queues"
_KIND = "messaging.queue"

# Canonical tier boundary tokens (count domain) for Standard operations.
# The free first tier (0-13M) is implicit in the API; we hardcode it.
_STD_FREE_TIER = {
    "dimension": "request",
    "tier": "0",
    "tier_upper": "13M",
    "amount": 0.0,
    "unit": "request",
}

# Ordered paid tier boundaries for Standard operations.
# The API returns all paid prices under a single meterName "Standard Messaging Operations".
# We sort by price descending and zip with these bounds.
_STD_TIER_BOUNDS: list[tuple[str, str]] = [
    ("13M",   "100M"),
    ("100M",  "2500M"),
    ("2500M", ""),
]

_STD_METER_NAME = "Standard Messaging Operations"
_VALID_SKU_NAMES = {"Standard", "Premium"}
_VALID_METER_NAMES = frozenset({_STD_METER_NAME, "Premium Messaging Unit"})


def ingest(*, prices_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with prices_path.open() as f:
        items = json.load(f).get("Items", [])

    # Collect Standard paid prices grouped by region (per-request USD amounts).
    std_raw: dict[str, list[float]] = defaultdict(list)
    # Collect Premium tier items grouped by region.
    # region -> (sku_id, hourly_price, region_normalized)
    prem_items: dict[str, tuple[str, float, str]] = {}

    for item in items:
        service_name = item.get("serviceName", "")
        sku_name = item.get("skuName", "")
        meter_name = item.get("meterName", "")
        region = item.get("armRegionName", "")
        usd = float(item.get("retailPrice", 0))
        row_type = item.get("type", "Consumption")
        currency = item.get("currencyCode", "USD")

        if service_name != "Service Bus":
            continue
        if sku_name not in _VALID_SKU_NAMES:
            continue
        if meter_name not in _VALID_METER_NAMES:
            continue
        if row_type != "Consumption" or currency != "USD":
            continue
        if not region:
            continue

        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue

        sku_id = item.get("skuId") or f"SB-{sku_name.upper()[:4]}-{region}"

        if sku_name == "Standard":
            if meter_name != _STD_METER_NAME:
                continue
            if usd <= 0:
                continue  # skip free-tier entry
            uom = item.get("unitOfMeasure", "")
            try:
                divisor, _unit = parse_request_uom(uom)
            except ValueError:
                continue
            std_raw[region].append(usd / divisor)

        elif sku_name == "Premium":
            if meter_name != "Premium Messaging Unit":
                continue
            uom = item.get("unitOfMeasure", "")
            try:
                divisor, _unit = parse_unit_of_measure(uom)
            except ValueError:
                continue
            hourly = usd / divisor
            if region not in prem_items:
                prem_items[region] = (sku_id, hourly, region_normalized)

    # Emit Standard rows.
    for region, raw_prices in sorted(std_raw.items()):
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue
        sorted_prices = sorted(raw_prices, reverse=True)
        if len(sorted_prices) != len(_STD_TIER_BOUNDS):
            print(
                f"warn: azure_service_bus_queues region {region!r} expected "
                f"{len(_STD_TIER_BOUNDS)} paid tiers, got {len(sorted_prices)}, skipping",
                file=sys.stderr,
            )
            continue
        prices: list[dict[str, Any]] = [_STD_FREE_TIER.copy()]
        for (tier, tier_upper), amount in zip(_STD_TIER_BOUNDS, sorted_prices, strict=True):
            prices.append({
                "dimension": "request",
                "tier": tier,
                "tier_upper": tier_upper,
                "amount": amount,
                "unit": "request",
            })
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "",
            "os": "",
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": f"SB-STD-{region}",
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": "standard",
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "extra": {"mode": "standard"},
            },
            "terms": terms,
            "prices": prices,
        }

    # Emit Premium rows.
    for region, (_sku_id, hourly, region_normalized) in sorted(prem_items.items()):
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "",
            "os": "",
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": f"SB-PREM-{region}",
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": "premium",
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "extra": {"mode": "premium"},
            },
            "terms": terms,
            "prices": [
                {
                    "dimension": "mu_hour",
                    "tier": "0",
                    "tier_upper": "",
                    "amount": hourly,
                    "unit": "hr",
                }
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.azure_service_bus_queues")
    ap.add_argument("--fixture", type=Path)
    ap.add_argument("--prices", type=Path)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--catalog-version", default=None)
    args = ap.parse_args()
    if args.fixture:
        prices_path = (
            args.fixture / "prices.json" if args.fixture.is_dir() else args.fixture
        )
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
    print(f"ingest.azure_service_bus_queues: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
