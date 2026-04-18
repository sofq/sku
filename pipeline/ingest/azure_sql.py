"""Normalize Azure retail-prices JSON for SQL Database into sku row dicts.

Spec §5 db.relational kind. Engine slot is set to 'azure-sql' (single value
distinguishes from AWS RDS rows in cross-provider compare). Deployment
option rides the terms.os slot — Single Database -> 'single-az',
Managed Instance -> 'managed-instance', Elastic Pool -> 'elastic-pool'.
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
_SERVICE = "sql"
_KIND = "db.relational"

_DEPLOYMENT_HINTS: tuple[tuple[str, str], ...] = (
    ("Managed Instance", "managed-instance"),
    ("Elastic Pool",     "elastic-pool"),
    ("Single Database",  "single-az"),
)

_SQL = """
WITH items AS (
  SELECT UNNEST(Items, recursive := true)
  FROM read_json_auto('{path}', maximum_object_size=33554432)
)
SELECT
  meterId       AS sku_id,
  skuName       AS resource_name,
  armRegionName AS region,
  productName   AS product_name,
  retailPrice   AS price,
  unitOfMeasure AS uom,
  currencyCode  AS currency,
  type          AS row_type,
  serviceName   AS service_name
FROM items
"""


def _classify_deployment(product_name: str) -> str | None:
    for hint, value in _DEPLOYMENT_HINTS:
        if hint in product_name:
            return value
    return None


def ingest(*, prices_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    con = open_conn()
    path_literal = str(prices_path).replace("'", "''")
    sql = _SQL.replace("{path}", path_literal)
    for (
        sku_id, sku_name, region, product, price, uom, currency, row_type, service_name,
    ) in con.execute(sql).fetchall():
        if service_name != "SQL Database":
            continue
        if row_type != "Consumption":
            continue
        if currency != "USD":
            continue
        deployment = _classify_deployment(product)
        if deployment is None:
            continue
        region_normalized = normalizer.normalize(_PROVIDER, region)
        divisor, unit = parse_unit_of_measure(uom)
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "azure-sql",
            "os": deployment,
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": sku_id,
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": sku_name,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "extra": {
                    "product_name": product,
                    "deployment_option": deployment,
                },
            },
            "terms": terms,
            "prices": [
                {"dimension": "compute", "tier": "", "amount": price / divisor, "unit": unit},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.azure_sql")
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
