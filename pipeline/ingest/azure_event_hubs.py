"""Normalize Azure Event Hubs Retail Prices into messaging.queue rows.

Two tiers are supported:
- Standard: one row per region with two price dimensions (tu_hour, event).
  The TU-hour meter gives the per-throughput-unit hourly cost; the event
  meter gives the per-million-event ingress cost.
- Premium: one row per region with one price dimension (ppu_hour).
  PPU = Premium Processing Unit, billed hourly.

Excluded: Dedicated Capacity Units, Kafka Endpoint hours, Capture hours,
Geo Replication meters, Extended Retention meters.
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
_SERVICE = "event-hubs"
_KIND = "messaging.queue"

# Standard tier meter names
_STANDARD_TU_METER = "Standard Throughput Unit"
_STANDARD_EVENT_METER = "Standard Ingress Events"

# Premium tier meter name
_PREMIUM_PPU_METER = "Premium Processing Unit"


def _is_standard(item: dict[str, Any]) -> bool:
    sku_name = item.get("skuName", "")
    return "Standard" in sku_name


def _is_premium(item: dict[str, Any]) -> bool:
    sku_name = item.get("skuName", "")
    return "Premium" in sku_name


def ingest(*, prices_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with prices_path.open() as f:
        items = json.load(f).get("Items", [])

    # Group by (tier_label, region): standard rows collect (tu_price, event_price),
    # premium rows collect ppu_price.
    # standard_groups: region -> {"tu": float|None, "event": float|None}
    standard_groups: dict[str, dict[str, float | None]] = defaultdict(
        lambda: {"tu": None, "event": None}
    )
    # premium_groups: region -> float (ppu_hour price)
    premium_groups: dict[str, float] = {}

    for item in items:
        service_name = item.get("serviceName", "")
        if service_name != "Event Hubs":
            continue
        row_type = item.get("type", "Consumption")
        currency = item.get("currencyCode", "USD")
        if row_type != "Consumption" or currency != "USD":
            continue

        region = item.get("armRegionName", "")
        if not region:
            continue
        usd = float(item.get("retailPrice", 0))
        if usd <= 0:
            continue

        meter_name = item.get("meterName", "")

        if _is_standard(item):
            if meter_name == _STANDARD_TU_METER:
                uom = item.get("unitOfMeasure", "")
                try:
                    divisor, _ = parse_unit_of_measure(uom)
                except ValueError:
                    continue
                standard_groups[region]["tu"] = usd / divisor
            elif meter_name == _STANDARD_EVENT_METER:
                # Azure publishes ingress events with UoM "1M" — divide so
                # downstream consumers see a per-event rate consistent with
                # how the messaging.queue estimator multiplies by ops.
                uom = item.get("unitOfMeasure", "")
                try:
                    divisor, _ = parse_request_uom(uom)
                except ValueError:
                    continue
                standard_groups[region]["event"] = usd / divisor
        elif _is_premium(item):
            if meter_name == _PREMIUM_PPU_METER:
                uom = item.get("unitOfMeasure", "")
                try:
                    divisor, _ = parse_unit_of_measure(uom)
                except ValueError:
                    continue
                premium_groups[region] = usd / divisor

    # Emit standard rows — only when both dimensions are present.
    for region in sorted(standard_groups):
        entry = standard_groups[region]
        tu_price = entry["tu"]
        event_price = entry["event"]
        if tu_price is None or event_price is None:
            print(
                f"warn: azure_event_hubs standard region {region!r} missing meters "
                f"(tu={tu_price}, event={event_price}), skipping",
                file=sys.stderr,
            )
            continue
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "",
            "os": "",
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": f"EH-STD-{region}",
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
            "prices": [
                {
                    "dimension": "tu_hour",
                    "tier": "0",
                    "tier_upper": "",
                    "amount": tu_price,
                    "unit": "hr",
                },
                {
                    "dimension": "event",
                    "tier": "0",
                    "tier_upper": "",
                    "amount": event_price,
                    "unit": "request",
                },
            ],
        }

    # Emit premium rows.
    for region in sorted(premium_groups):
        ppu_price = premium_groups[region]
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "",
            "os": "",
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": f"EH-PREM-{region}",
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
                    "dimension": "ppu_hour",
                    "tier": "0",
                    "tier_upper": "",
                    "amount": ppu_price,
                    "unit": "hr",
                },
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.azure_event_hubs")
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
    print(f"ingest.azure_event_hubs: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
