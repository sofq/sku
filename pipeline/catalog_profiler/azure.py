"""Scan Azure retail-prices JSON and emit CoverageRow per (serviceName, meterCategory).

Retail-prices is a flat list of Items — we read once via DuckDB for speed
and bucket in Python. Schema is uniform across all Azure items, so
attribute_keys is the same for every CoverageRow and reflects the full
feed schema. This detects when Azure adds or removes a top-level field.
"""

from __future__ import annotations

import json
from pathlib import Path

from ingest._duckdb import open_conn

_SHARD_BY_BUCKET: dict[tuple[str, str], str] = {
    ("Virtual Machines", "Virtual Machines"): "azure_vm",
    ("SQL Database", "SQL Database"): "azure_sql",
    ("Azure Database for PostgreSQL", "Azure Database for PostgreSQL"): "azure_postgres",
    ("Azure Database for MySQL", "Azure Database for MySQL"): "azure_mysql",
    ("Azure Database for MariaDB", "Azure Database for MariaDB"): "azure_mariadb",
    ("Storage", "Storage"): "azure_blob",
    ("Azure App Service", "Azure App Service"): "azure_functions",
    ("Functions", "Azure Functions"): "azure_functions",
    ("Storage", "Managed Disks"): "azure_disks",
}


def scan_prices(*, prices_path: Path) -> list:
    from .types import CoverageRow

    # Fingerprint the feed schema from a sample (schema is uniform across all Items).
    with prices_path.open() as fh:
        raw = json.load(fh)
    sample_keys: set[str] = set()
    for item in (raw.get("Items") or [])[:50]:
        sample_keys.update(item.keys())
    feed_attrs = tuple(sorted(sample_keys))

    con = open_conn()
    path_literal = str(prices_path).replace("'", "''")
    # Probe which columns the feed actually exposes — meterCategory is absent
    # in VM-only fixture snapshots (and some narrow feeds). When missing we fall
    # back to serviceName so the _SHARD_BY_BUCKET lookup still resolves.
    col_rows = con.execute(f"""
        WITH items AS (
          SELECT UNNEST(Items, recursive := true)
          FROM read_json_auto('{path_literal}', maximum_object_size=33554432)
        )
        SELECT * FROM items LIMIT 0
    """).description
    avail_cols = {d[0] for d in (col_rows or [])}
    meter_cat_expr = "meterCategory" if "meterCategory" in avail_cols else "serviceName"
    sku_example_expr = "armSkuName" if "armSkuName" in avail_cols else "skuName"
    sku_examples_sql = (
        f"ARRAY_AGG(DISTINCT {sku_example_expr}) FILTER (WHERE {sku_example_expr} IS NOT NULL)"
    )

    rows = con.execute(f"""
        WITH items AS (
          SELECT UNNEST(Items, recursive := true)
          FROM read_json_auto('{path_literal}', maximum_object_size=33554432)
        )
        SELECT
          serviceName,
          COALESCE({meter_cat_expr}, serviceName)        AS meter_category,
          COUNT(*)                                       AS sku_count,
          {sku_examples_sql}                             AS sku_examples
        FROM items
        GROUP BY serviceName, COALESCE({meter_cat_expr}, serviceName)
    """).fetchall()

    out = []
    for service_name, meter_category, sku_count, examples in rows:
        key = (service_name or "", meter_category or "")
        shard = _SHARD_BY_BUCKET.get(key)
        out.append(
            CoverageRow(
                bucket_key=f"{service_name}/{meter_category}",
                bucket_label=f"{service_name} / {meter_category}",
                sku_count=int(sku_count),
                attribute_keys=feed_attrs,
                sample_sku_ids=tuple((examples or [])[:3]),
                covered_by_shard=shard,
                coverage_ratio=1.0 if shard else 0.0,
            )
        )
    return out
