"""Normalize Azure retail-prices JSON for Blob Storage into sku row dicts via DuckDB.

Spec §5 storage.object kind. Each row carries three price dimensions:
- storage   (gb-mo) from meterName like '*LRS Data Stored'
- read-ops  (requests) from '*LRS Read Operations'
- write-ops (requests) from '*LRS Write Operations'
One row per (tier, region); tier = hot | cool | archive on LRS.
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
from .azure_common import load_region_normalizer, parse_request_uom, parse_storage_uom

_PROVIDER = "azure"
_SERVICE = "blob"
_KIND = "storage.object"

# Map armSkuName to our canonical tier slug. LRS variants only (m3b.2 scope).
_TIER_MAP: dict[str, str] = {
    "Hot LRS": "hot",
    "Cool LRS": "cool",
    "Archive LRS": "archive",
}

# durability / availability per tier — matches aws_s3.py shape.
_TIER_ATTRS: dict[str, dict[str, Any]] = {
    "hot":     {"durability_nines": 11, "availability_tier": "standard"},
    "cool":    {"durability_nines": 11, "availability_tier": "infrequent"},
    "archive": {"durability_nines": 11, "availability_tier": "archive"},
}

# meterName suffix → our dimension slug.
_DIM_BY_METER_SUFFIX: dict[str, str] = {
    "Data Stored":      "storage",
    "Read Operations":  "read-ops",
    "Write Operations": "write-ops",
}

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


def _dim_for(meter_name: str) -> str | None:
    for suffix, dim in _DIM_BY_METER_SUFFIX.items():
        if meter_name.endswith(suffix):
            return dim
    return None


def ingest(*, prices_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    con = open_conn()
    path_literal = str(prices_path).replace("'", "''")
    sql = _SQL.replace("{path}", path_literal)

    grouped: dict[tuple[str, str], dict[str, dict[str, Any]]] = {}
    for (
        sku_id, sku_name, region, meter_name, price, uom, currency, row_type, service_name,
    ) in con.execute(sql).fetchall():
        if service_name != "Storage":
            continue
        if row_type != "Consumption":
            continue
        if currency != "USD":
            continue
        tier = _TIER_MAP.get(sku_name)
        if tier is None:
            continue
        dim = _dim_for(meter_name)
        if dim is None:
            continue
        if normalizer.try_normalize(_PROVIDER, region) is None:
            continue  # skip regions outside our coverage map
        if dim == "storage":
            divisor, unit = parse_storage_uom(uom)
        else:
            divisor, unit = parse_request_uom(uom)
        key = (tier, region)
        grouped.setdefault(key, {})[dim] = {
            "sku": sku_id, "amount": price / divisor, "unit": unit,
        }

    required = {"storage", "read-ops", "write-ops"}
    for (tier, region), dims in sorted(grouped.items()):
        if required - dims.keys():
            print(f"warn: dropping incomplete blob row {tier}/{region}", file=sys.stderr)
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
        attrs = _TIER_ATTRS[tier]
        yield {
            "sku_id": sku_id,
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": tier,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "durability_nines": attrs["durability_nines"],
                "availability_tier": attrs["availability_tier"],
                "extra": {"redundancy": "lrs"},
            },
            "terms": terms,
            "prices": [
                {"dimension": "storage",    "tier": "", "amount": dims["storage"]["amount"],    "unit": dims["storage"]["unit"]},
                {"dimension": "read-ops",   "tier": "", "amount": dims["read-ops"]["amount"],   "unit": dims["read-ops"]["unit"]},
                {"dimension": "write-ops",  "tier": "", "amount": dims["write-ops"]["amount"],  "unit": dims["write-ops"]["unit"]},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.azure_blob")
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
