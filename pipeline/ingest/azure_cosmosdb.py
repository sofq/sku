"""Normalize Azure Cosmos DB Retail Prices into db.nosql rows."""

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
from .azure_common import load_region_normalizer

_PROVIDER = "azure"
_SERVICE = "cosmosdb"
_KIND = "db.nosql"


def _api_from_product(product_name: str) -> str:
    pn = product_name.lower()
    if "mongo" in pn:
        return "mongo"
    if "cassandra" in pn:
        return "cassandra"
    if "table" in pn:
        return "table"
    if "gremlin" in pn or "graph" in pn:
        return "gremlin"
    return "sql"


def _classify(meter_name: str) -> tuple[str, str] | None:
    m = meter_name.lower()
    if "serverless" in m and "request" in m:
        return ("serverless", "ru_million_usd")
    if "storage" in m:
        return ("storage", "gb_month_usd")
    if "ru/s" in m:
        return ("provisioned", "ru_per_sec_hour_usd")
    return None


def ingest(*, prices_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with prices_path.open() as f:
        items = json.load(f).get("Items", [])

    for item in items:
        meter_name = item.get("meterName", "")
        product_name = item.get("productName", "")
        region = item.get("armRegionName", "")
        usd = float(item.get("retailPrice", 0))
        unit_raw = item.get("unitOfMeasure", "")
        row_type = item.get("type", "Consumption")
        currency = item.get("currencyCode", "USD")
        if row_type != "Consumption" or currency != "USD":
            continue
        if not region or usd <= 0:
            continue
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue
        cls = _classify(meter_name)
        if cls is None:
            continue
        capacity_mode, extra_key = cls
        api = _api_from_product(product_name)

        if capacity_mode == "storage":
            extra: dict[str, Any] = {
                "capacity_mode": "storage",
                "kind": "storage",
                "gb_month_usd": usd,
                "upstream_meter": meter_name,
            }
            resource_name = "cosmos-storage"
        elif capacity_mode == "serverless":
            extra = {
                "capacity_mode": "serverless",
                "api": api,
                "ru_million_usd": usd,
                "upstream_meter": meter_name,
            }
            resource_name = "cosmos-serverless"
        else:  # provisioned
            per_ru_hour = usd / 100.0 if "100 ru/s" in meter_name.lower() else usd
            usd = per_ru_hour
            extra = {
                "capacity_mode": "provisioned",
                "api": api,
                "ru_per_sec_hour_usd": per_ru_hour,
                "upstream_meter": meter_name,
            }
            resource_name = "cosmos-provisioned"

        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": api,
            "os": capacity_mode,
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": item.get("skuId") or f"{_SERVICE}-{capacity_mode}-{api}-{region}",
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": resource_name,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "vcpu": None,
                "memory_gb": None,
                "extra": extra,
            },
            "terms": terms,
            "prices": [
                {
                    "dimension": capacity_mode if capacity_mode != "storage" else "storage",
                    "tier": "",
                    "amount": usd,
                    "unit": unit_raw.lower(),
                },
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.azure_cosmosdb")
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
