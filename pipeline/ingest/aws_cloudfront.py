"""Normalize AWS CloudFront offer JSON into sku row dicts via DuckDB.

Spec §5 network.cdn kind. CloudFront is a global service whose pricing
varies by edge-region (attributes.location), not by AWS compute region.
For m3a.3 scope we ingest only the data-transfer-out dimension (first
tier, `GB`), mapped to canonical AWS region codes per LOCATION_MAP. HTTPS
requests, Lambda@Edge, field-level encryption, origin shield, and
real-time logs are out of scope (see plan non-goals).

resource_name = "standard" — CloudFront's only public offering today.
Future security-edition SKUs would land as a second resource_name.
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
_SERVICE = "cloudfront"
_KIND = "network.cdn"

# Upstream edge-location strings → canonical AWS region code.
# Every entry's value must also exist in pipeline/normalize/regions.yaml;
# the test_location_map_exhaustiveness test enforces this. Additional
# edge regions land when regions.yaml grows (spec §3 future regions).
LOCATION_MAP: dict[str, str] = {
    "United States, Mexico, & Canada":         "us-east-1",
    "Europe, Israel":                          "eu-west-1",
    "Asia Pacific (including Japan & Taiwan)": "ap-northeast-1",
}

_SQL = """
WITH prod_keys AS (
  SELECT unnest(json_keys(products)) AS sku_id, products, terms FROM offer
),
products_flat AS (
  SELECT
    sku_id,
    json_extract_string(products, '$."' || sku_id || '".productFamily') AS family,
    json_extract_string(products, '$."' || sku_id || '".attributes.location') AS location_raw,
    json_extract_string(products, '$."' || sku_id || '".attributes.transferType') AS transfer_type,
    terms
  FROM prod_keys
),
term_keys AS (
  SELECT *,
    json_keys(json_extract(terms, '$.OnDemand."' || sku_id || '"'))[1] AS term_key
  FROM products_flat
  WHERE family = 'Data Transfer' AND transfer_type = 'CloudFront Outbound'
),
pd_keys AS (
  SELECT *,
    json_keys(json_extract(terms,
      '$.OnDemand."' || sku_id || '"."' || term_key || '".priceDimensions'))[1] AS pd_key
  FROM term_keys
)
SELECT sku_id, location_raw,
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

    grouped: dict[tuple[str, str], dict[str, Any]] = {}

    for sku_id, location_raw, unit, usd, begin_range in con.execute(_SQL).fetchall():
        if location_raw not in LOCATION_MAP:
            raise KeyError(location_raw)
        region = LOCATION_MAP[location_raw]
        if begin_range not in (None, "0"):
            continue
        normalizer.normalize(_PROVIDER, region)
        key = ("standard", region)
        grouped[key] = {"sku": sku_id, "usd": usd, "unit": unit}

    for (resource_name, region), entry in sorted(grouped.items()):
        region_normalized = normalizer.normalize(_PROVIDER, region)
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand", "tenancy": "", "os": "",
            "support_tier": "", "upfront": "", "payment_option": "",
        })
        yield {
            "sku_id": entry["sku"],
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": resource_name,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "extra": {"tier": "PriceClass_All"},
            },
            "terms": terms,
            "prices": [
                {"dimension": "data_transfer_out", "tier": "",
                 "amount": entry["usd"], "unit": entry["unit"].lower()},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_cloudfront")
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
