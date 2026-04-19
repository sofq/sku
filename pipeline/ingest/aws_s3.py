"""Normalize AWS S3 offer JSON into sku row dicts via DuckDB.

Spec §5 storage.object kind. S3 rows carry three price dimensions in m3a.2:
- storage (GB-Mo) from productFamily=Storage
- requests-put (requests) from productFamily=API Request, group=S3-API-Tier1
- requests-get (requests) from productFamily=API Request, group=S3-API-Tier2

We group by (storage_class, region) so each row has one entry per dimension.
A row missing any of the three dimensions is dropped with a stderr warning;
the fixture and the upstream AWS offer both publish all three for every
storage class + region in m3a.2's scope.
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
_SERVICE = "s3"
_KIND = "storage.object"

# Map the volumeType attribute string to our canonical storage-class slug.
# Classes outside this map are skipped (see m3a.2 non-goals: Glacier
# Flexible / Deep Archive / RRS / Outposts -> m3a.3).
_CLASS_MAP: dict[str, str] = {
    "Standard": "standard",
    "Standard - Infrequent Access": "standard-ia",
    "One Zone - Infrequent Access": "one-zone-ia",
    "Intelligent-Tiering Frequent Access": "intelligent-tiering",
    "Amazon Glacier Instant Retrieval": "glacier-instant",
}

# Durability + availability tier per class. Values from AWS public docs,
# encoded here to populate resource_attrs.
_CLASS_ATTRS: dict[str, dict[str, Any]] = {
    "standard":            {"durability_nines": 11, "availability_tier": "standard"},
    "standard-ia":         {"durability_nines": 11, "availability_tier": "infrequent"},
    "one-zone-ia":         {"durability_nines": 11, "availability_tier": "one-zone"},
    "intelligent-tiering": {"durability_nines": 11, "availability_tier": "standard"},
    "glacier-instant":     {"durability_nines": 11, "availability_tier": "archive"},
}

# Tier1 = PUT/COPY/POST/LIST, Tier2 = GET/SELECT. Map the `group` attribute
# to our price dimension slug.
_REQUEST_GROUP_MAP: dict[str, str] = {
    "S3-API-Tier1": "requests-put",
    "S3-API-Tier2": "requests-get",
}

# Pulls every SKU's attributes + its single on-demand priceDimension. S3's
# on-demand term has one non-tiered priceDimension per SKU for the m3a.2
# scope (first tier only for storage); we take the first pd_key.
_SQL = """
WITH products_flat AS (
  SELECT
    p.key AS sku_id,
    json_extract_string(p.value, '$.productFamily') AS family,
    json_extract_string(p.value, '$.attributes.regionCode') AS region,
    json_extract_string(p.value, '$.attributes.volumeType') AS volume_type,
    json_extract_string(p.value, '$.attributes.group') AS group_raw
  FROM offer, json_each(offer.products) AS p(key, value)
  WHERE json_extract_string(p.value, '$.productFamily') IN ('Storage', 'API Request')
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
SELECT pf.sku_id, pf.family, pf.region, pf.volume_type, pf.group_raw,
  json_extract_string(pk.term_obj, '$."' || pk.term_key || '".priceDimensions."' || pk.pd_key || '".unit') AS unit,
  CAST(json_extract_string(pk.term_obj, '$."' || pk.term_key || '".priceDimensions."' || pk.pd_key || '".pricePerUnit.USD') AS DOUBLE) AS usd,
  json_extract_string(pk.term_obj, '$."' || pk.term_key || '".priceDimensions."' || pk.pd_key || '".beginRange') AS begin_range
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

    # Collect: (storage_class, region) -> {"storage": {sku,usd,unit,volume_type}, ...}
    grouped: dict[tuple[str, str], dict[str, dict[str, Any]]] = {}

    for sku_id, family, region, volume_type, group_raw, unit, usd, begin_range in (
        con.execute(_SQL).fetchall()
    ):
        klass = _CLASS_MAP.get(volume_type)
        if klass is None:
            continue
        if family == "Storage":
            # Keep first tier only (beginRange = '0' or absent).
            if begin_range not in (None, "0"):
                continue
            dim = "storage"
        elif family == "API Request":
            dim = _REQUEST_GROUP_MAP.get(group_raw)
            if dim is None:
                continue
        else:
            continue
        normalizer.normalize(_PROVIDER, region)  # early reject on unknown region
        key = (klass, region)
        grouped.setdefault(key, {})[dim] = {
            "sku": sku_id, "usd": usd, "unit": unit, "volume_type": volume_type,
        }

    for (klass, region), dims in sorted(grouped.items()):
        required = {"storage", "requests-put", "requests-get"}
        if required - dims.keys():
            print(f"warn: dropping incomplete s3 row {klass}/{region}", file=sys.stderr)
            continue
        region_normalized = normalizer.normalize(_PROVIDER, region)
        # Deterministic synthetic sku_id: join the three upstream SKUs in a
        # stable order so the identifier is reproducible across builds.
        sku_id = "::".join(sorted(dims[d]["sku"] for d in required))
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "",
            "os": "",
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        attrs = _CLASS_ATTRS[klass]
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
                "durability_nines": attrs["durability_nines"],
                "availability_tier": attrs["availability_tier"],
                "extra": {"volume_type": dims["storage"]["volume_type"]},
            },
            "terms": terms,
            "prices": [
                {"dimension": "storage",      "tier": "", "amount": dims["storage"]["usd"],      "unit": dims["storage"]["unit"].lower()},
                {"dimension": "requests-put", "tier": "", "amount": dims["requests-put"]["usd"], "unit": dims["requests-put"]["unit"].lower()},
                {"dimension": "requests-get", "tier": "", "amount": dims["requests-get"]["usd"], "unit": dims["requests-get"]["unit"].lower()},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_s3")
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
