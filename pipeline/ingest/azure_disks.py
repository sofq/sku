"""Normalize Azure retail-prices JSON for Managed Disks into sku row dicts.

Spec §5 storage.block kind. In m3b.2 each row carries only the storage
dimension (1/Month unit); IOPS + throughput pricing -> m3b.3. One row per
(disk_type, region); disk_type in {standard-hdd, standard-ssd, premium-ssd}.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path
from typing import Any, Iterable

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import dumps, open_conn
from .azure_common import load_region_normalizer, parse_storage_uom

_PROVIDER = "azure"
_SERVICE = "disks"
_KIND = "storage.block"

_TYPE_MAP: dict[str, str] = {
    "Standard_LRS":    "standard-hdd",
    "StandardSSD_LRS": "standard-ssd",
    "Premium_LRS":     "premium-ssd",
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

    for (
        sku_id, sku_name, region, price, uom, currency, row_type, service_name,
    ) in con.execute(sql).fetchall():
        if service_name != "Storage":
            continue
        if row_type != "Consumption":
            continue
        if currency != "USD":
            continue
        disk_type = _TYPE_MAP.get(sku_name)
        if disk_type is None:
            continue  # Ultra + reserved + others skipped
        divisor, unit = parse_storage_uom(uom)
        region_normalized = normalizer.normalize(_PROVIDER, region)
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
            "resource_name": disk_type,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "extra": {"arm_sku_name": sku_name, "redundancy": "lrs"},
            },
            "terms": terms,
            "prices": [
                {"dimension": "storage", "tier": "", "amount": price / divisor, "unit": unit},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.azure_disks")
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
