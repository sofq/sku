"""Normalize AWS SQS offer JSON into sku row dicts via DuckDB.

Spec §5 messaging.queue kind. SQS has two queue types (Standard and FIFO)
with tiered per-request pricing. Each row is keyed by (queue_type, region)
and carries three price dimension entries (one per tier) in prices[].

Price tiers (by beginRange):
  tier 0:   0 – 100B requests
  tier 1:   100B – 200B requests
  tier 2:   200B+ (Inf)

All three tiers are grouped into a single row's prices[] list so lookups
and compare queries see the full pricing schedule in one record.
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
_SERVICE = "sqs"
_KIND = "messaging.queue"

# Map raw SQS tier boundary integers (as strings) to canonical count-domain tokens.
_SQS_TIER_MAP: dict[str, str] = {
    "0":             "0",
    "100000000000":  "100B",
    "200000000000":  "200B",
}


def _to_count_token(raw: str) -> str:
    """Convert a raw SQS beginRange/endRange string to a canonical tier token."""
    return _SQS_TIER_MAP.get(str(int(float(raw))) if raw not in ("", "Inf") else raw, raw)

# TODO(M-ε): AWS introduced Fair queues (queueType="Fair") in late 2024.
# Live ingest currently logs "warn: unknown queueType 'Fair', skipping" for
# every Fair-priced SKU. Treat as a follow-up coverage gap, not a bug.
_QUEUE_TYPE_MAP: dict[str, str] = {
    "Standard": "standard",
    "FIFO (first-in, first-out)": "fifo",
}

# DuckDB SQL: extract one row per (sku_id, priceDimension) using json_keys iteration.
# We unnest priceDimensions so each tier is a separate result row.
_SQL = """
WITH prod_keys AS (
  SELECT unnest(json_keys(products)) AS sku_id, products, terms FROM offer
),
products_flat AS (
  SELECT
    sku_id,
    json_extract_string(products, '$."' || sku_id || '".productFamily') AS family,
    json_extract_string(products, '$."' || sku_id || '".attributes.regionCode') AS region,
    json_extract_string(products, '$."' || sku_id || '".attributes.queueType') AS queue_type,
    terms
  FROM prod_keys
),
term_keys AS (
  SELECT *,
    json_keys(json_extract(terms, '$.OnDemand."' || sku_id || '"'))[1] AS term_key
  FROM products_flat
  WHERE family = 'API Request' AND queue_type IS NOT NULL
),
pd_unnested AS (
  SELECT tk.*,
    unnest(json_keys(json_extract(terms,
      '$.OnDemand."' || tk.sku_id || '"."' || tk.term_key || '".priceDimensions'))) AS pd_key
  FROM term_keys tk
)
SELECT
  sku_id, region, queue_type,
  json_extract_string(terms,
    '$.OnDemand."' || sku_id || '"."' || term_key || '".priceDimensions."' || pd_key || '".beginRange') AS begin_range,
  json_extract_string(terms,
    '$.OnDemand."' || sku_id || '"."' || term_key || '".priceDimensions."' || pd_key || '".endRange') AS end_range,
  CAST(json_extract_string(terms,
    '$.OnDemand."' || sku_id || '"."' || term_key || '".priceDimensions."' || pd_key || '".pricePerUnit.USD')
    AS DOUBLE) AS usd,
  json_extract_string(terms,
    '$.OnDemand."' || sku_id || '"."' || term_key || '".priceDimensions."' || pd_key || '".unit') AS unit
FROM pd_unnested
ORDER BY sku_id, begin_range
"""


def ingest(*, offer_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    con = open_conn()
    path_literal = str(offer_path).replace("'", "''")
    con.execute(
        f"CREATE VIEW offer AS SELECT * FROM read_json('{path_literal}', "
        "columns={products: 'JSON', terms: 'JSON'})"
    )

    # Group price dimension rows by (queue_type_canonical, region).
    # Value: list of tier dicts sorted by begin_range.
    grouped: dict[tuple[str, str], dict[str, Any]] = {}

    for sku_id, region, queue_type_raw, begin_range, end_range, usd, unit in (
        con.execute(_SQL).fetchall()
    ):
        if region is None or queue_type_raw is None:
            continue
        resource_name = _QUEUE_TYPE_MAP.get(queue_type_raw)
        if resource_name is None:
            print(f"warn: unknown queueType {queue_type_raw!r}, skipping", file=sys.stderr)
            continue
        if normalizer.try_normalize(_PROVIDER, region) is None:
            continue

        key = (resource_name, region)
        if key not in grouped:
            grouped[key] = {"sku_id": sku_id, "tiers": []}
        tier_upper = "" if (end_range is None or end_range == "Inf") else _to_count_token(end_range)
        grouped[key]["tiers"].append({
            "begin_range": _to_count_token(begin_range or "0"),
            "tier_upper": tier_upper,
            "usd": usd,
            "unit": unit or "requests",
        })

    for (resource_name, region), entry in sorted(grouped.items()):
        tiers = entry["tiers"]
        if not tiers:
            print(f"warn: no tiers for sqs {resource_name}/{region}", file=sys.stderr)
            continue
        region_normalized = normalizer.normalize(_PROVIDER, region)
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "",
            "os": "",
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        prices = [
            {
                "dimension": "request",
                "tier": t["begin_range"],
                "tier_upper": t["tier_upper"],
                "amount": t["usd"],
                "unit": "request",
            }
            for t in tiers
        ]
        yield {
            "sku_id": entry["sku_id"],
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": resource_name,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "extra": {"mode": resource_name},
            },
            "terms": terms,
            "prices": prices,
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_sqs")
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
    n = 0
    with args.out.open("w") as fh:
        for row in ingest(offer_path=offer_path):
            fh.write(dumps(row) + "\n")
            n += 1
    print(f"ingest.aws_sqs: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
