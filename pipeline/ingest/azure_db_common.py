"""Shared DuckDB scanner for Azure hosted-DB retail-prices ingests.

azure_sql, azure_postgres, azure_mysql, azure_mariadb all differ only in
(service_name, tenancy_slug, service). Everything else is identical.

_classify_deployment returns None when no hint matches — rows with unknown
deployment types are skipped. Callers that need a fallback should add an
explicit hint rather than relying on a default.

Usage:
    from .azure_db_common import ingest_hosted_db

    def ingest(*, prices_path):
        yield from ingest_hosted_db(
            prices_path=prices_path,
            service_name="Azure Database for PostgreSQL",
            tenancy_slug="azure-postgres",
            service="postgres",
        )
"""

from __future__ import annotations

from collections.abc import Iterable
from pathlib import Path
from typing import Any

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import open_conn
from .azure_common import load_region_normalizer, parse_unit_of_measure


_DEPLOYMENT_HINTS: tuple[tuple[str, str], ...] = (
    ("Managed Instance", "managed-instance"),
    ("Elastic Pool",     "elastic-pool"),
    ("Single Database",  "single-az"),
    ("Single Server",    "single-az"),    # legacy Azure DB for MySQL/PG
    ("Flexible Server",  "flexible-server"),
)


def _classify_deployment(product_name: str) -> str | None:
    for hint, value in _DEPLOYMENT_HINTS:
        if hint in product_name:
            return value
    return None  # unknown deployment — caller skips row


_SQL = """
WITH items AS (
  SELECT UNNEST(Items, recursive := true)
  FROM read_json_auto('{path}', maximum_object_size=33554432)
)
SELECT
  CAST(meterId AS VARCHAR) AS sku_id,
  skuName       AS resource_name,
  armRegionName AS region,
  productName   AS product_name,
  retailPrice   AS price,
  unitOfMeasure AS uom
FROM items
WHERE serviceName = '{service_name}'
  AND type        = 'Consumption'
  AND currencyCode = 'USD'
"""


def ingest_hosted_db(
    *,
    prices_path: Path,
    service_name: str,
    tenancy_slug: str,
    service: str,
    kind: str = "db.relational",
) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    con = open_conn()
    path_literal = str(prices_path).replace("'", "''")
    service_name_literal = service_name.replace("'", "''")
    sql = _SQL.replace("{path}", path_literal).replace("{service_name}", service_name_literal)
    seen: set[str] = set()
    for (
        sku_id, sku_name, region, product, price, uom,
    ) in con.execute(sql).fetchall():
        deployment = _classify_deployment(product)
        if deployment is None:
            continue
        region_normalized = normalizer.try_normalize("azure", region)
        if region_normalized is None:
            continue
        try:
            divisor, unit = parse_unit_of_measure(uom)
        except ValueError:
            continue
        if sku_id in seen:
            continue
        seen.add(sku_id)
        terms = apply_kind_defaults(kind, {
            "commitment": "on_demand",
            "tenancy": tenancy_slug,
            "os": deployment,
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        amount = float(price) / divisor
        yield {
            "sku_id": sku_id,
            "provider": "azure",
            "service": service,
            "kind": kind,
            "resource_name": sku_name,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {"extra": {"product_name": product, "deployment_option": deployment}},
            "terms": terms,
            "prices": [{"dimension": "compute", "tier": "", "amount": amount, "unit": unit}],
        }
