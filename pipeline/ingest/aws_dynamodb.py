"""Normalize AWS DynamoDB offer JSON into sku row dicts via DuckDB.

Spec §5 db.nosql kind. On-demand only in m3a.3 scope. Each row is keyed by
(table_class, region) and carries three price dimensions:

- storage             (GB-Mo) from productFamily=Database Storage, group=DDB-StorageUsage
- read_request_units  (ReadRequestUnits)  from group=DDB-ReadUnits
- write_request_units (WriteRequestUnits) from group=DDB-WriteUnits

Provisioned capacity, reserved capacity, Global Tables, backups, streams,
DAX, and export-to-S3 are all out of scope for m3a.3 (see plan non-goals).
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path
from typing import Any, Iterable

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import dumps, open_conn
from .aws_common import load_region_normalizer

_PROVIDER = "aws"
_SERVICE = "dynamodb"
_KIND = "db.nosql"

_CLASS_MAP: dict[str, str] = {
    "Standard": "standard",
    "Standard - Infrequent Access": "standard-ia",
}

_GROUP_MAP: dict[str, str] = {
    "DDB-StorageUsage": "storage",
    "DDB-ReadUnits":    "read_request_units",
    "DDB-WriteUnits":   "write_request_units",
}

_SQL = """
WITH prod_keys AS (
  SELECT unnest(json_keys(products)) AS sku_id, products, terms FROM offer
),
products_flat AS (
  SELECT
    sku_id,
    json_extract_string(products, '$."' || sku_id || '".attributes.regionCode') AS region,
    json_extract_string(products, '$."' || sku_id || '".attributes.storageClass') AS klass_raw,
    json_extract_string(products, '$."' || sku_id || '".attributes.group') AS group_raw,
    terms
  FROM prod_keys
),
term_keys AS (
  SELECT *,
    json_keys(json_extract(terms, '$.OnDemand."' || sku_id || '"'))[1] AS term_key
  FROM products_flat
),
pd_keys AS (
  SELECT *,
    json_keys(json_extract(terms,
      '$.OnDemand."' || sku_id || '"."' || term_key || '".priceDimensions'))[1] AS pd_key
  FROM term_keys
)
SELECT sku_id, region, klass_raw, group_raw,
  json_extract_string(terms,
    '$.OnDemand."' || sku_id || '"."' || term_key || '".priceDimensions."' || pd_key || '".unit') AS unit,
  CAST(json_extract_string(terms,
    '$.OnDemand."' || sku_id || '"."' || term_key || '".priceDimensions."' || pd_key || '".pricePerUnit.USD')
    AS DOUBLE) AS usd,
  json_extract_string(terms,
    '$.OnDemand."' || sku_id || '"."' || term_key || '".priceDimensions."' || pd_key || '".beginRange') AS begin_range
FROM pd_keys
"""


def ingest(*, offer_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    con = open_conn()
    path_literal = str(offer_path).replace("'", "''")
    con.execute(
        f"CREATE VIEW offer AS SELECT * FROM read_json('{path_literal}', "
        "columns={products: 'JSON', terms: 'JSON'})"
    )

    grouped: dict[tuple[str, str], dict[str, dict[str, Any]]] = {}

    for sku_id, region, klass_raw, group_raw, unit, usd, begin_range in (
        con.execute(_SQL).fetchall()
    ):
        klass = _CLASS_MAP.get(klass_raw)
        dim = _GROUP_MAP.get(group_raw)
        if klass is None or dim is None:
            continue
        if dim == "storage" and begin_range not in (None, "0"):
            continue
        normalizer.normalize(_PROVIDER, region)
        key = (klass, region)
        grouped.setdefault(key, {})[dim] = {"sku": sku_id, "usd": usd, "unit": unit}

    required = {"storage", "read_request_units", "write_request_units"}
    for (klass, region), dims in sorted(grouped.items()):
        if required - dims.keys():
            print(f"warn: dropping incomplete dynamodb row {klass}/{region}", file=sys.stderr)
            continue
        region_normalized = normalizer.normalize(_PROVIDER, region)
        sku_id = "::".join(sorted(dims[d]["sku"] for d in required))
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand", "tenancy": "", "os": "",
            "support_tier": "", "upfront": "", "payment_option": "",
        })
        yield {
            "sku_id": sku_id,
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": klass,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "durability_nines": 11,
                "availability_tier": "standard" if klass == "standard" else "infrequent",
                "extra": {"table_class": klass},
            },
            "terms": terms,
            "prices": [
                {"dimension": "storage",              "tier": "", "amount": dims["storage"]["usd"],              "unit": dims["storage"]["unit"].lower()},
                {"dimension": "read_request_units",   "tier": "", "amount": dims["read_request_units"]["usd"],   "unit": dims["read_request_units"]["unit"].lower()},
                {"dimension": "write_request_units",  "tier": "", "amount": dims["write_request_units"]["usd"],  "unit": dims["write_request_units"]["unit"].lower()},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_dynamodb")
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
