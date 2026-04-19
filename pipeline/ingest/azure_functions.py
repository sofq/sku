"""Normalize Azure retail-prices JSON for Functions (Consumption plan) into sku row dicts.

Spec §5 compute.function kind. Each row carries two dimensions:
- executions (requests) from meterName='Total Executions'
- duration   (gb-seconds) from meterName='Execution Time'
One row per (arch, region); m3b.2 ingests x86_64 Consumption only.
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Iterable
from pathlib import Path
from typing import Any

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import dumps, open_conn
from .azure_common import load_region_normalizer, parse_request_uom

_PROVIDER = "azure"
_SERVICE = "functions"
_KIND = "compute.function"

_DIM_MAP: dict[str, str] = {
    "Total Executions": "executions",
    "Execution Time":   "duration",
}

_DURATION_UNIT = "gb-seconds"

_SQL = """
WITH items AS (
  SELECT UNNEST(Items, recursive := true)
  FROM read_json_auto('{path}', maximum_object_size=33554432)
)
SELECT
  meterId       AS sku_id,
  armSkuName    AS sku_name,
  armRegionName AS region,
  meterName     AS meter_name,
  retailPrice   AS price,
  unitOfMeasure AS uom,
  currencyCode  AS currency,
  type          AS row_type,
  serviceName   AS service_name
FROM items
"""


def ingest(*, prices_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    con = open_conn()
    path_literal = str(prices_path).replace("'", "''")
    sql = _SQL.replace("{path}", path_literal)

    grouped: dict[tuple[str, str], dict[str, dict[str, Any]]] = {}
    for (
        sku_id, sku_name, region, meter_name, price, uom, currency, row_type, service_name,
    ) in con.execute(sql).fetchall():
        if service_name != "Functions":
            continue
        if row_type != "Consumption":
            continue
        if currency != "USD":
            continue
        if sku_name != "Consumption":  # m3b.2: exclude Premium / Dedicated plans
            continue
        dim = _DIM_MAP.get(meter_name)
        if dim is None:
            continue
        if normalizer.try_normalize(_PROVIDER, region) is None:
            continue  # skip regions outside our coverage map
        if dim == "executions":
            divisor, unit = parse_request_uom(uom)
            amount = price / divisor
        else:
            # Duration meter publishes retailPrice in USD per GB-second already;
            # unitOfMeasure = "1" (per gb-second). We keep the raw price and label.
            amount = price
            unit = _DURATION_UNIT
        key = ("x86_64", region)
        grouped.setdefault(key, {})[dim] = {"sku": sku_id, "amount": amount, "unit": unit}

    required = {"executions", "duration"}
    for (arch, region), dims in sorted(grouped.items()):
        if required - dims.keys():
            print(f"warn: dropping incomplete functions row {arch}/{region}", file=sys.stderr)
            continue
        region_normalized = normalizer.normalize(_PROVIDER, region)
        sku_id = "::".join(sorted(dims[d]["sku"] for d in required))
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "",
            "os": "",
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": sku_id,
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": arch,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "architecture": arch,
                "extra": {"plan": "consumption"},
            },
            "terms": terms,
            "prices": [
                {"dimension": "executions", "tier": "", "amount": dims["executions"]["amount"], "unit": dims["executions"]["unit"]},
                {"dimension": "duration",   "tier": "", "amount": dims["duration"]["amount"],   "unit": dims["duration"]["unit"]},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.azure_functions")
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
