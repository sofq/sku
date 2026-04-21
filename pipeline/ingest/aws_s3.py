"""Normalize AWS S3 offer JSON into sku row dicts via DuckDB.

Spec §5 storage.object kind. S3 rows carry three price dimensions in m3a.2:
- storage (GB-Mo) from productFamily=Storage
- requests-put (requests) from productFamily=API Request, class-specific Tier1 group
- requests-get (requests) from productFamily=API Request, class-specific Tier2 group

We group by (storage_class, region) so each row has one entry per dimension.
A row missing any of the three dimensions is dropped with a stderr warning.

API Request products in the upstream offer do not carry a volumeType attribute;
instead the storage class is encoded in the group name (S3-API-Tier1 for
standard/intelligent-tiering, S3-API-SIA-Tier1 for standard-ia, etc.).
The fixture uses volumeType on API Request rows for backward compatibility —
both shapes are handled by checking volumeType first, falling back to group.
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
    # Old offer shape used "Amazon Glacier Instant Retrieval"; current uses "Glacier Instant Retrieval".
    "Amazon Glacier Instant Retrieval": "glacier-instant",
    "Glacier Instant Retrieval": "glacier-instant",
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

# Map API Request group name -> (class_slug, dim).
# Standard and Intelligent-Tiering share Tier1/Tier2; each other class has its own prefix.
# These are used when the API Request product has no volumeType (live upstream shape).
_REQUEST_GROUP_CLASS_DIM: dict[str, tuple[str, str]] = {
    "S3-API-Tier1":     ("standard",            "requests-put"),
    "S3-API-Tier2":     ("standard",            "requests-get"),
    "S3-API-SIA-Tier1": ("standard-ia",         "requests-put"),
    "S3-API-SIA-Tier2": ("standard-ia",         "requests-get"),
    "S3-API-ZIA-Tier1": ("one-zone-ia",         "requests-put"),
    "S3-API-ZIA-Tier2": ("one-zone-ia",         "requests-get"),
    "S3-API-GIR-Tier1": ("glacier-instant",     "requests-put"),
    "S3-API-GIR-Tier2": ("glacier-instant",     "requests-get"),
    "S3-API-INT-Tier1": ("intelligent-tiering", "requests-put"),
    "S3-API-INT-Tier2": ("intelligent-tiering", "requests-get"),
}

# Tier1/Tier2 groups are also used for intelligent-tiering in the live offer.
# We record them separately so we can emit them for both standard AND intelligent-tiering.
_TIER1_TIER2_ALSO_INT = {"S3-API-Tier1", "S3-API-Tier2"}

# For the fixture shape: map the Tier1/Tier2 group to dimension when class comes from volumeType.
_REQUEST_GROUP_DIM: dict[str, str] = {
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
        if normalizer.try_normalize(_PROVIDER, region) is None:
            continue  # skip regions outside our coverage map

        if family == "Storage":
            klass = _CLASS_MAP.get(volume_type)
            if klass is None:
                continue
            # Keep first tier only (beginRange = '0' or absent).
            if begin_range not in (None, "0"):
                continue
            key = (klass, region)
            grouped.setdefault(key, {})["storage"] = {
                "sku": sku_id, "usd": usd, "unit": unit, "volume_type": volume_type,
            }
        elif family == "API Request":
            if volume_type is not None:
                # Fixture shape: volumeType on API Request encodes the class directly.
                klass = _CLASS_MAP.get(volume_type)
                dim = _REQUEST_GROUP_DIM.get(group_raw)
                if klass is None or dim is None:
                    continue
                key = (klass, region)
                grouped.setdefault(key, {})[dim] = {
                    "sku": sku_id, "usd": usd, "unit": unit, "volume_type": volume_type,
                }
            else:
                # Live shape: no volumeType; class and dim encoded in group name.
                result = _REQUEST_GROUP_CLASS_DIM.get(group_raw)
                if result is None:
                    continue
                klass, dim = result
                entry = {"sku": sku_id, "usd": usd, "unit": unit, "volume_type": None}
                grouped.setdefault((klass, region), {})[dim] = entry
                # S3-API-Tier1/Tier2 are shared by standard AND intelligent-tiering.
                if group_raw in _TIER1_TIER2_ALSO_INT:
                    int_dim = "requests-put" if group_raw == "S3-API-Tier1" else "requests-get"
                    grouped.setdefault(("intelligent-tiering", region), {})[int_dim] = entry

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
    n = 0
    with args.out.open("w") as fh:
        for row in ingest(offer_path=offer_path):
            fh.write(dumps(row) + "\n")
            n += 1
    print(f"ingest.aws_s3: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
