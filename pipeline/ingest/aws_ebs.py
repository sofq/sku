"""Normalize AWS EBS offer JSON into sku row dicts via DuckDB.

Spec §5 storage.block kind. In m3a.2 each row carries only the storage
dimension (GB-Mo); IOPS + throughput pricing -> m3a.3. One row per
(volume_type, region); volume_type comes from attributes.volumeApiName.
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
from .aws_common import load_region_normalizer

_PROVIDER = "aws"
_SERVICE = "ebs"
_KIND = "storage.block"

_ALLOWED_TYPES: set[str] = {"gp3", "gp2", "io2", "st1", "sc1"}

_SQL = """
WITH products_flat AS (
  SELECT
    p.key AS sku_id,
    json_extract_string(p.value, '$.productFamily') AS family,
    json_extract_string(p.value, '$.attributes.regionCode') AS region,
    json_extract_string(p.value, '$.attributes.volumeApiName') AS vol_type
  FROM offer, json_each(offer.products) AS p(key, value)
  WHERE json_extract_string(p.value, '$.productFamily') = 'Storage'
),
terms_flat AS (
  SELECT
    t.key AS sku_id,
    (json_keys(t.value))[1] AS term_key,
    t.value AS term_obj
  FROM offer, json_each(json_extract(offer.terms, '$.OnDemand')) AS t(key, value)
),
pd_keys AS (
  SELECT tf.sku_id, tf.term_key, tf.term_obj,
    (json_keys(json_extract(tf.term_obj, '$."' || tf.term_key || '".priceDimensions')))[1] AS pd_key
  FROM terms_flat tf
)
SELECT pf.sku_id, pf.region, pf.vol_type,
  json_extract_string(pk.term_obj, '$."' || pk.term_key || '".priceDimensions."' || pk.pd_key || '".unit') AS unit,
  CAST(json_extract_string(pk.term_obj, '$."' || pk.term_key || '".priceDimensions."' || pk.pd_key || '".pricePerUnit.USD') AS DOUBLE) AS usd
FROM products_flat pf
JOIN pd_keys pk ON pf.sku_id = pk.sku_id
"""


def ingest(*, offer_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    con = open_conn()
    path_literal = str(offer_path).replace("'", "''")
    con.execute(
        f"CREATE VIEW offer AS SELECT * FROM read_json('{path_literal}', "
        "columns={products: 'JSON', terms: 'JSON'}, maximum_object_size=134217728)"
    )

    rows = list(con.execute(_SQL).fetchall())
    # Filter first to allowed types so any non-EBS "Storage" family rows in
    # the same offer file (e.g. RDS storage SKUs when the source is shared)
    # don't trip the region validator.
    rows = [r for r in rows if r[2] in _ALLOWED_TYPES]

    for sku_id, region, vol_type, unit, usd in rows:
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue
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
            "resource_name": vol_type,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "extra": {"volume_type": vol_type},
            },
            "terms": terms,
            "prices": [
                {"dimension": "storage", "tier": "", "amount": usd, "unit": unit.lower()},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_ebs")
    ap.add_argument("--fixture", type=Path)
    ap.add_argument("--offer", type=Path)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--catalog-version", default=None)
    args = ap.parse_args()
    if args.fixture:
        offer_path = args.fixture / "offer.json" if args.fixture.is_dir() else args.fixture
    elif args.offer:
        offer_path = args.offer
    else:
        print("either --fixture or --offer required", file=sys.stderr)
        return 2
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with args.out.open("w") as fh:
        for row in ingest(offer_path=offer_path):
            fh.write(dumps(row) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
