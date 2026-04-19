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
WITH products_flat AS (
  SELECT
    p.key AS sku_id,
    json_extract_string(p.value, '$.productFamily') AS family,
    json_extract_string(p.value, '$.attributes.regionCode') AS region,
    json_extract_string(p.value, '$.attributes.archSupport') AS arch_raw,
    json_extract_string(p.value, '$.attributes.group') AS group_raw
  FROM offer, json_each(offer.products) AS p(key, value)
  WHERE json_extract_string(p.value, '$.productFamily') = 'Serverless'
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
SELECT pf.sku_id, pf.region, pf.arch_raw, pf.group_raw,
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

    grouped: dict[tuple[str, str], dict[str, dict[str, Any]]] = {}
    for sku_id, region, arch_raw, group_raw, unit, usd in con.execute(_SQL).fetchall():
        arch = _ARCH_MAP.get(arch_raw)
        dim = _GROUP_MAP.get(group_raw)
        if arch is None or dim is None:
            continue
        if normalizer.try_normalize(_PROVIDER, region) is None:
            continue  # skip regions outside our coverage map
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
