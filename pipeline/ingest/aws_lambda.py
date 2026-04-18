"""Normalize AWS Lambda offer JSON into sku row dicts via DuckDB.

Spec §5 compute.function kind. Lambda rows carry two price dimensions:
- requests (unit: requests) from group=AWS-Lambda-Requests
- duration (unit: second) from group=AWS-Lambda-Duration

One row per (architecture, region). Provisioned concurrency, SnapStart, and
ephemeral-storage surcharges are out of scope for m3a.2 (see non-goals).
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
_SERVICE = "lambda"
_KIND = "compute.function"

_ARCH_MAP: dict[str, str] = {"x86": "x86_64", "arm": "arm64"}
_GROUP_MAP: dict[str, str] = {
    "AWS-Lambda-Requests": "requests",
    "AWS-Lambda-Duration": "duration",
}

_SQL = """
WITH prod_keys AS (
  SELECT unnest(json_keys(products)) AS sku_id, products, terms FROM offer
),
products_flat AS (
  SELECT
    sku_id,
    json_extract_string(products, '$."' || sku_id || '".productFamily') AS family,
    json_extract_string(products, '$."' || sku_id || '".attributes.regionCode') AS region,
    json_extract_string(products, '$."' || sku_id || '".attributes.archSupport') AS arch_raw,
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
SELECT sku_id, region, arch_raw, group_raw,
  json_extract_string(terms,
    '$.OnDemand."' || sku_id || '"."' || term_key || '".priceDimensions."' || pd_key || '".unit') AS unit,
  CAST(json_extract_string(terms,
    '$.OnDemand."' || sku_id || '"."' || term_key || '".priceDimensions."' || pd_key || '".pricePerUnit.USD')
    AS DOUBLE) AS usd
FROM pd_keys
WHERE family = 'Serverless'
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
    for sku_id, region, arch_raw, group_raw, unit, usd in con.execute(_SQL).fetchall():
        arch = _ARCH_MAP.get(arch_raw)
        dim = _GROUP_MAP.get(group_raw)
        if arch is None or dim is None:
            continue
        normalizer.normalize(_PROVIDER, region)  # early reject on unknown region
        key = (arch, region)
        grouped.setdefault(key, {})[dim] = {"sku": sku_id, "usd": usd, "unit": unit}

    for (arch, region), dims in sorted(grouped.items()):
        if {"requests", "duration"} - dims.keys():
            print(f"warn: dropping incomplete lambda row {arch}/{region}", file=sys.stderr)
            continue
        region_normalized = normalizer.normalize(_PROVIDER, region)
        sku_id = "::".join(sorted(dims[d]["sku"] for d in ("requests", "duration")))
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
            "resource_name": arch,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "architecture": arch,
                "extra": {},
            },
            "terms": terms,
            "prices": [
                {"dimension": "requests", "tier": "", "amount": dims["requests"]["usd"], "unit": dims["requests"]["unit"].lower()},
                {"dimension": "duration", "tier": "", "amount": dims["duration"]["usd"], "unit": dims["duration"]["unit"].lower()},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_lambda")
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
