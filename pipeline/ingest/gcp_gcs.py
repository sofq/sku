"""Normalize GCP Cloud Billing Catalog JSON for Cloud Storage into sku row dicts.

Spec §5 kind=storage.object. One row per (storage_class, region) with three
price dimensions: storage (gb-mo), read-ops (requests), write-ops (requests).

resourceGroup -> canonical storage-class slug (StandardStorage -> standard).
Meter-description keyword -> dimension slug ("Class A Operations" -> read-ops,
"Class B Operations" -> write-ops, else -> storage).

Filters applied:
  - category.serviceDisplayName == 'Cloud Storage'
  - category.usageType == 'OnDemand'
  - pricingInfo[0] currencyCode == 'USD'
  - serviceRegions[0] resolvable via the shared GCP region normalizer
    (drops multi/dual region SKUs — their region strings like "us" are absent)
  - category.resourceGroup in the canonical class set (drops
    MultiRegionalStorage, DualRegionalStorage, and any future class).

A (class, region) pair emits ONLY when all three meter dimensions are present.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path
from typing import Any, Iterable

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import dumps, open_conn
from .gcp_common import load_region_normalizer, parse_unit_price, parse_usage_unit

_PROVIDER = "gcp"
_SERVICE = "gcs"
_KIND = "storage.object"

_CLASS_MAP: dict[str, str] = {
    "StandardStorage": "standard",
    "NearlineStorage": "nearline",
    "ColdlineStorage": "coldline",
    "ArchiveStorage": "archive",
}

_CLASS_ATTRS: dict[str, dict[str, Any]] = {
    "standard": {"durability_nines": 11, "availability_tier": "standard"},
    "nearline": {"durability_nines": 11, "availability_tier": "infrequent"},
    "coldline": {"durability_nines": 11, "availability_tier": "cold"},
    "archive":  {"durability_nines": 11, "availability_tier": "archive"},
}

_SQL = """
WITH entries AS (
  SELECT UNNEST(skus, recursive := true)
  FROM read_json_auto('{path}', maximum_object_size=33554432)
)
SELECT
  skuId                                             AS sku_id,
  description                                       AS description,
  serviceDisplayName                                AS service_display_name,
  resourceFamily                                    AS resource_family,
  resourceGroup                                     AS resource_group,
  usageType                                         AS usage_type,
  serviceRegions                                    AS service_regions,
  pricingInfo[1].pricingExpression.usageUnit        AS usage_unit,
  pricingInfo[1].pricingExpression.tieredRates[1].unitPrice.currencyCode AS currency,
  pricingInfo[1].pricingExpression.tieredRates[1].unitPrice.units        AS price_units,
  pricingInfo[1].pricingExpression.tieredRates[1].unitPrice.nanos        AS price_nanos
FROM entries
"""


def _dimension(description: str) -> str:
    d = description.lower()
    if "class a operations" in d:
        return "read-ops"
    if "class b operations" in d:
        return "write-ops"
    return "storage"


def ingest(*, skus_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    con = open_conn()
    path_literal = str(skus_path).replace("'", "''")
    sql = _SQL.replace("{path}", path_literal)

    # (class, region) -> {sku_id for storage meter, description, resource_group, region_normalized, prices:{dim: {...}}}
    grouped: dict[tuple[str, str], dict[str, Any]] = {}
    for (
        sku_id, description, svc_name, resource_family, resource_group, usage_type,
        service_regions, usage_unit, currency, price_units, price_nanos,
    ) in con.execute(sql).fetchall():
        if svc_name != "Cloud Storage":
            continue
        if usage_type != "OnDemand":
            continue
        if currency != "USD":
            continue
        storage_class = _CLASS_MAP.get(resource_group)
        if storage_class is None:
            continue  # drops MultiRegional, DualRegional, and any future class
        if not service_regions:
            continue
        region = service_regions[0]
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue
        divisor, unit = parse_usage_unit(usage_unit)
        amount = parse_unit_price(units=price_units, nanos=int(price_nanos or 0)) / divisor
        dim = _dimension(description)
        key = (storage_class, region)
        bucket = grouped.setdefault(key, {
            "region_normalized": region_normalized,
            "prices": {},
        })
        # Keep the storage-meter SKU's identity as the row anchor.
        if dim == "storage":
            bucket["sku_id"] = sku_id
            bucket["description"] = description
            bucket["resource_group"] = resource_group
        bucket["prices"][dim] = {"dimension": dim, "tier": "", "amount": amount, "unit": unit}

    for (storage_class, region), bucket in grouped.items():
        if set(bucket["prices"].keys()) != {"storage", "read-ops", "write-ops"}:
            continue
        if "sku_id" not in bucket:
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
            "sku_id": bucket["sku_id"],
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": storage_class,
            "region": region,
            "region_normalized": bucket["region_normalized"],
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                **_CLASS_ATTRS[storage_class],
                "extra": {
                    "description": bucket["description"],
                    "resource_group": bucket["resource_group"],
                },
            },
            "terms": terms,
            "prices": [
                bucket["prices"]["storage"],
                bucket["prices"]["read-ops"],
                bucket["prices"]["write-ops"],
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.gcp_gcs")
    ap.add_argument("--fixture", type=Path)
    ap.add_argument("--skus", type=Path)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--catalog-version", default=None)
    args = ap.parse_args()
    if args.fixture:
        skus_path = args.fixture / "skus.json" if args.fixture.is_dir() else args.fixture
    elif args.skus:
        skus_path = args.skus
    else:
        print("either --fixture or --skus required", file=sys.stderr)
        return 2
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with args.out.open("w") as fh:
        for row in ingest(skus_path=skus_path):
            fh.write(dumps(row) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
