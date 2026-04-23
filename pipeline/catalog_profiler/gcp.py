"""Scan GCP Cloud Billing Catalog JSON and emit CoverageRow per (serviceDisplayName, resourceGroup).

Schema is uniform across all SKUs. attribute_keys fingerprints the top-level
fields plus category sub-keys (prefixed "category.") so feed schema additions
are visible in report diffs. The fingerprint is the same for every CoverageRow.
"""

from __future__ import annotations

import json
from pathlib import Path

from ingest._duckdb import open_conn

_SHARD_BY_BUCKET: dict[tuple[str, str], str] = {
    ("Compute Engine",        "N1Standard"):      "gcp_gce",
    ("Compute Engine",        "N2Standard"):      "gcp_gce",
    ("Compute Engine",        "E2Instance"):      "gcp_gce",
    ("Compute Engine",        "Compute"):         "gcp_gce",
    ("Cloud SQL",             "SQLGen2InstancesN1Standard"): "gcp_cloud_sql",
    ("Cloud SQL",             "ApplicationServices"):         "gcp_cloud_sql",
    ("Cloud Storage",         "RegionalStorage"): "gcp_gcs",
    ("Cloud Run",             "Compute"):         "gcp_run",
    ("Cloud Run Functions",   "Compute"):         "gcp_functions",
}


def scan_catalog(*, skus_path: Path) -> list:
    from .types import CoverageRow

    # Fingerprint the feed schema from a sample; include category sub-keys
    # prefixed "category." so they're identifiable in diffs.
    with skus_path.open() as fh:
        raw = json.load(fh)
    sample_keys: set[str] = set()
    for sku in (raw.get("skus") or [])[:50]:
        sample_keys.update(sku.keys())
        for k in (sku.get("category") or {}):
            sample_keys.add(f"category.{k}")
    feed_attrs = tuple(sorted(sample_keys))

    con = open_conn()
    path_literal = str(skus_path).replace("'", "''")
    rows = con.execute(f"""
        WITH entries AS (
          SELECT UNNEST(skus, recursive := true)
          FROM read_json_auto('{path_literal}', maximum_object_size=33554432)
        )
        SELECT
          serviceDisplayName,
          resourceGroup,
          COUNT(*)                         AS sku_count,
          ARRAY_AGG(DISTINCT skuId)[1:4]   AS samples
        FROM entries
        GROUP BY serviceDisplayName, resourceGroup
    """).fetchall()

    out = []
    for svc, group, sku_count, samples in rows:
        key = (svc or "", group or "")
        shard = _SHARD_BY_BUCKET.get(key)
        out.append(CoverageRow(
            bucket_key=f"{svc}/{group}",
            bucket_label=f"{svc} / {group}",
            sku_count=int(sku_count),
            attribute_keys=feed_attrs,
            sample_sku_ids=tuple((samples or [])[:3]),
            covered_by_shard=shard,
            coverage_ratio=1.0 if shard else 0.0,
        ))
    return out
