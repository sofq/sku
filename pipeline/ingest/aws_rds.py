"""Normalize AWS RDS offer JSON into sku row dicts via DuckDB.

Spec §5 db.relational kind. Engine and deployment-option ride the
terms.tenancy and terms.os slots — see enums.yaml comment for rationale.
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
_SERVICE = "rds"
_KIND = "db.relational"

_ENGINE_MAP = {"PostgreSQL": "postgres", "MySQL": "mysql", "MariaDB": "mariadb"}
_DEPL_MAP = {"Single-AZ": "single-az", "Multi-AZ": "multi-az"}

_SQL = """
WITH prod_keys AS (
  SELECT unnest(json_keys(products)) AS sku_id, products, terms FROM offer
),
products_flat AS (
  SELECT
    sku_id,
    json_extract_string(products, '$."' || sku_id || '".productFamily') AS family,
    json_extract_string(products, '$."' || sku_id || '".attributes.instanceType') AS instance_type,
    json_extract_string(products, '$."' || sku_id || '".attributes.regionCode') AS region,
    json_extract_string(products, '$."' || sku_id || '".attributes.databaseEngine') AS engine_raw,
    json_extract_string(products, '$."' || sku_id || '".attributes.deploymentOption') AS depl_raw,
    json_extract_string(products, '$."' || sku_id || '".attributes.licenseModel') AS license_model,
    json_extract_string(products, '$."' || sku_id || '".attributes.vcpu') AS vcpu,
    json_extract_string(products, '$."' || sku_id || '".attributes.memory') AS memory,
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
SELECT sku_id, instance_type, region, engine_raw, depl_raw, license_model, vcpu, memory,
  json_extract_string(terms,
    '$.OnDemand."' || sku_id || '"."' || term_key || '".priceDimensions."' || pd_key || '".unit') AS unit,
  CAST(json_extract_string(terms,
    '$.OnDemand."' || sku_id || '"."' || term_key || '".priceDimensions."' || pd_key || '".pricePerUnit.USD')
    AS DOUBLE) AS usd
FROM pd_keys
WHERE family = 'Database Instance'
"""


def _parse_memory(raw: str) -> float:
    return float(raw.split()[0])


def ingest(*, offer_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    con = open_conn()
    path_literal = str(offer_path).replace("'", "''")
    con.execute(
        f"CREATE VIEW offer AS SELECT * FROM read_json('{path_literal}', "
        "columns={products: 'JSON', terms: 'JSON'}, maximum_object_size=536870912)"
    )
    for sku_id, instance_type, region, engine_raw, depl_raw, license_model, vcpu_raw, memory_raw, unit, usd in (
        con.execute(_SQL).fetchall()
    ):
        if license_model and license_model != "No license required":
            continue
        if engine_raw not in _ENGINE_MAP or depl_raw not in _DEPL_MAP:
            continue
        region_normalized = normalizer.normalize(_PROVIDER, region)
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": _ENGINE_MAP[engine_raw],
            "os": _DEPL_MAP[depl_raw],
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": sku_id,
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": instance_type,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "vcpu": int(vcpu_raw) if vcpu_raw else None,
                "memory_gb": _parse_memory(memory_raw) if memory_raw else None,
                "extra": {
                    "engine": _ENGINE_MAP[engine_raw],
                    "deployment_option": _DEPL_MAP[depl_raw],
                },
            },
            "terms": terms,
            "prices": [
                {"dimension": "compute", "tier": "", "amount": usd, "unit": unit.lower()},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_rds")
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
