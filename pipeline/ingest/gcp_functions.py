"""Normalize GCP Cloud Billing Catalog JSON for Cloud Functions (gen2) into sku row dicts.

Spec §5 kind=compute.function. One row per region, dimensions cpu-second /
memory-gb-second / requests, architecture x86_64. gen1 rows (resourceGroup=
CloudFunctions) are filtered; only gen2 (CloudFunctionsV2) emits.

resource_name is the architecture slug ("x86_64") so LookupServerlessFunction
can point-lookup via --architecture; the service field distinguishes
functions from run.
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
from .gcp_common import load_region_normalizer, parse_unit_price, parse_usage_unit

_PROVIDER = "gcp"
_SERVICE = "functions"
_KIND = "compute.function"
_ARCHITECTURE = "x86_64"
_SERVICE_DISPLAY = "Cloud Functions"
_RESOURCE_GROUP_GEN2 = "CloudFunctionsV2"

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


def _dimension(description: str, usage_unit: str) -> str | None:
    d = description.lower()
    if usage_unit == "count" or "invocation" in d or "request" in d:
        return "requests"
    if "cpu" in d:
        return "cpu-second"
    if "memory" in d:
        return "memory-gb-second"
    return None


def ingest(*, skus_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    con = open_conn()
    path_literal = str(skus_path).replace("'", "''")
    sql = _SQL.replace("{path}", path_literal)

    grouped: dict[str, dict[str, Any]] = {}
    for (
        sku_id, description, svc_name, _resource_family, resource_group, usage_type,
        service_regions, usage_unit, currency, price_units, price_nanos,
    ) in con.execute(sql).fetchall():
        if svc_name != _SERVICE_DISPLAY:
            continue
        if usage_type != "OnDemand":
            continue
        if currency != "USD":
            continue
        if resource_group != _RESOURCE_GROUP_GEN2:
            continue  # drops gen1 CloudFunctions
        dim = _dimension(description, usage_unit)
        if dim is None:
            continue
        if not service_regions:
            continue
        region = service_regions[0]
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue
        try:
            divisor, unit = parse_usage_unit(usage_unit)
        except ValueError:
            continue
        amount = parse_unit_price(units=price_units, nanos=int(price_nanos or 0)) / divisor
        bucket = grouped.setdefault(region, {
            "region_normalized": region_normalized,
            "prices": {},
        })
        if dim == "cpu-second":
            bucket["sku_id"] = sku_id
            bucket["description"] = description
        bucket["prices"][dim] = {"dimension": dim, "tier": "", "amount": amount, "unit": unit}

    for region, bucket in grouped.items():
        if set(bucket["prices"].keys()) != {"cpu-second", "memory-gb-second", "requests"}:
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
            "resource_name": _ARCHITECTURE,
            "region": region,
            "region_normalized": bucket["region_normalized"],
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "architecture": _ARCHITECTURE,
                "extra": {
                    "description": bucket["description"],
                    "resource_group": _RESOURCE_GROUP_GEN2,
                },
            },
            "terms": terms,
            "prices": [
                bucket["prices"]["cpu-second"],
                bucket["prices"]["memory-gb-second"],
                bucket["prices"]["requests"],
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.gcp_functions")
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
