"""Normalize GCP Cloud Billing Catalog JSON for Compute Engine into sku row dicts.

Spec §5 kind=compute.vm. For m3b.3 we ingest "whole-machine-type" SKUs —
`category.resourceGroup` values like `N1Standard2Linux` that bundle vCPU+RAM
into one SKU per region. True core+RAM decomposition for custom shapes is
deferred past v1.0.
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
_SERVICE = "gce"
_KIND = "compute.vm"

# resourceGroup -> canonical machine-type. Enumerated here (not regex-parsed)
# because the real Catalog API uses PascalCase-digit-SuffixOS strings with no
# documented grammar; keeping the mapping explicit prevents silent drift.
_RESOURCE_GROUP_TO_MACHINE: dict[str, str] = {
    "N1Standard2Linux": "n1-standard-2",
    "N1Standard4Linux": "n1-standard-4",
    "E2Standard2Linux": "e2-standard-2",
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


def ingest(*, skus_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    con = open_conn()
    path_literal = str(skus_path).replace("'", "''")
    sql = _SQL.replace("{path}", path_literal)
    for (
        sku_id, description, svc_name, resource_family, resource_group, usage_type,
        service_regions, usage_unit, currency, price_units, price_nanos,
    ) in con.execute(sql).fetchall():
        if svc_name != "Compute Engine":
            continue
        if resource_family != "Compute":
            continue
        if usage_type != "OnDemand":
            continue
        if currency != "USD":
            continue
        machine_type = _RESOURCE_GROUP_TO_MACHINE.get(resource_group)
        if machine_type is None:
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
        amount = parse_unit_price(units=price_units, nanos=int(price_nanos)) / divisor
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "shared",
            "os": "linux",
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": sku_id,
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": machine_type,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "architecture": "x86_64",
                "extra": {
                    "description": description,
                    "resource_group": resource_group,
                },
            },
            "terms": terms,
            "prices": [
                {"dimension": "compute", "tier": "", "amount": amount, "unit": unit},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.gcp_gce")
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
