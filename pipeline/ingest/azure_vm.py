"""Normalize Azure retail-prices JSON for Virtual Machines into sku row dicts via DuckDB.

Spec §3 (format stack): DuckDB read_json_auto lifts the flat `Items` array
into a per-row column set; one SQL pass filters to Consumption + USD, and
Python applies the OS / Spot / region-canonicalisation logic before
yielding NDJSON.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path
from typing import Any, Iterable

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import dumps, open_conn
from .azure_common import load_region_normalizer, parse_unit_of_measure

_PROVIDER = "azure"
_SERVICE = "vm"
_KIND = "compute.vm"

_SQL = """
WITH items AS (
  SELECT UNNEST(Items, recursive := true)
  FROM read_json_auto('{path}', maximum_object_size=33554432)
)
SELECT
  meterId       AS sku_id,
  armSkuName    AS resource_name,
  armRegionName AS region,
  productName   AS product_name,
  retailPrice   AS price,
  unitOfMeasure AS uom,
  currencyCode  AS currency,
  type          AS row_type,
  serviceName   AS service_name
FROM items
"""

# A note on filtering: prices.azure.com sometimes includes 'Spot' or 'Low
# Priority' variants. We detect them by substring in productName because
# the API has no dedicated `priority` column.
_SPOT_HINTS = ("Spot", "Low Priority")


def ingest(*, prices_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    con = open_conn()
    path_literal = str(prices_path).replace("'", "''")
    sql = _SQL.replace("{path}", path_literal)
    for (
        sku_id, arm_sku, region, product, price, uom, currency, row_type, service_name,
    ) in con.execute(sql).fetchall():
        if service_name != "Virtual Machines":
            continue
        if row_type != "Consumption":
            continue
        if currency != "USD":
            continue
        if any(hint in product for hint in _SPOT_HINTS):
            continue
        # OS detection: productName contains "Windows" for Windows VMs;
        # everything else is Linux (the only two surfaces we ship in m3b.1).
        os_value = "windows" if "Windows" in product else "linux"
        region_normalized = normalizer.normalize(_PROVIDER, region)
        divisor, unit = parse_unit_of_measure(uom)
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "shared",
            "os": os_value,
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": sku_id,
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": arm_sku,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "architecture": "x86_64",
                "extra": {
                    "product_name": product,
                },
            },
            "terms": terms,
            "prices": [
                {"dimension": "compute", "tier": "", "amount": price / divisor, "unit": unit},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.azure_vm")
    ap.add_argument("--fixture", type=Path)
    ap.add_argument("--prices", type=Path)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--catalog-version", default=None)  # consumed by packager
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
