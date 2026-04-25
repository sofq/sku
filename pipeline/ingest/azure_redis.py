"""Normalize Azure Cache for Redis Retail Prices into cache.kv rows.

Azure does not expose memory_gb in pricing JSON; we translate tier+size to
memory via a static map (Microsoft pricing page values).
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from collections.abc import Iterable
from pathlib import Path
from typing import Any

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import dumps
from .azure_common import load_region_normalizer, parse_unit_of_measure

_PROVIDER = "azure"
_SERVICE = "redis"
_KIND = "cache.kv"

_TIER_MAP = {
    "basic": "basic",
    "standard": "standard",
    "premium": "premium",
    "enterprise": "enterprise",
}

_MEMORY_GB = {
    ("basic", "C0"):        0.25,
    ("basic", "C1"):        1.0,
    ("standard", "C0"):     0.25,
    ("standard", "C1"):     1.0,
    ("standard", "C2"):     2.5,
    ("standard", "C3"):     6.0,
    ("standard", "C4"):     13.0,
    ("standard", "C5"):     26.0,
    ("standard", "C6"):     53.0,
    ("premium", "P1"):      6.0,
    ("premium", "P2"):      13.0,
    ("premium", "P3"):      26.0,
    ("premium", "P4"):      53.0,
    ("premium", "P5"):      120.0,
    ("enterprise", "E5"):   12.0,
    ("enterprise", "E10"):  25.0,
    ("enterprise", "E20"):  50.0,
    ("enterprise", "E50"):  100.0,
    ("enterprise", "E100"): 200.0,
}

_PARSE = re.compile(
    r"^(?P<tier>Basic|Standard|Premium|Enterprise)\s+(?P<size>[CPE]\d+)",
    re.IGNORECASE,
)


def _classify(meter_name: str) -> tuple[str, str] | None:
    m = _PARSE.match(meter_name.strip())
    if not m:
        return None
    tier = _TIER_MAP[m.group("tier").lower()]
    size = m.group("size").upper()
    return tier, size


def ingest(*, prices_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with prices_path.open() as f:
        items = json.load(f).get("Items", [])

    for item in items:
        meter_name = item.get("meterName", "")
        cls = _classify(meter_name)
        if cls is None:
            continue
        tier, size = cls
        region = item.get("armRegionName", "")
        usd = float(item.get("retailPrice", 0))
        uom = item.get("unitOfMeasure", "")
        row_type = item.get("type", "Consumption")
        currency = item.get("currencyCode", "USD")
        if row_type != "Consumption" or currency != "USD":
            continue
        if not region or usd <= 0:
            continue
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue
        try:
            divisor, unit = parse_unit_of_measure(uom)
        except ValueError:
            continue
        usd = usd / divisor

        memory_gb = _MEMORY_GB.get((tier, size))
        resource_name = f"{tier.capitalize()} {size}"

        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "redis",
            "os": tier,
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": item.get("skuId") or f"{_SERVICE}-{tier}-{size}-{region}",
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": resource_name,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "vcpu": None,
                "memory_gb": memory_gb,
                "extra": {
                    "engine": "redis",
                    "tier": tier,
                    "upstream_meter": meter_name,
                },
            },
            "terms": terms,
            "prices": [
                {"dimension": "compute", "tier": "", "amount": usd, "unit": unit},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.azure_redis")
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
    with args.out.open("w") as fh:
        for row in ingest(prices_path=prices_path):
            fh.write(dumps(row) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
