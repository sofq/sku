"""Normalize GCP Cloud Billing Catalog JSON for Cloud SQL into sku row dicts.

Spec §5 kind=db.relational. Tenancy slot encodes engine (cloud-sql-postgres /
cloud-sql-mysql); os slot encodes deployment (zonal / regional). Both are
lifted from `description` keyword matching because the real Catalog API
collapses them into a single resourceGroup.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path
from typing import Any, Iterable

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import dumps, open_conn
from .gcp_common import load_region_normalizer, parse_unit_price, parse_usage_unit

_PROVIDER = "gcp"
_SERVICE = "cloud-sql"
_KIND = "db.relational"

_ENGINE_HINTS: tuple[tuple[str, str], ...] = (
    ("PostgreSQL", "cloud-sql-postgres"),
    ("MySQL",      "cloud-sql-mysql"),
)

_DEPLOYMENT_HINTS: tuple[tuple[str, str], ...] = (
    ("Regional", "regional"),
    ("Zonal",    "zonal"),
)

# Extract the `db-custom-<vcpu>-<mb>` slug from the description.
_TIER_RE = re.compile(r"\b(db-custom-\d+-\d+)\b")

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
  usageType                                         AS usage_type,
  serviceRegions                                    AS service_regions,
  pricingInfo[1].pricingExpression.usageUnit        AS usage_unit,
  pricingInfo[1].pricingExpression.tieredRates[1].unitPrice.currencyCode AS currency,
  pricingInfo[1].pricingExpression.tieredRates[1].unitPrice.units        AS price_units,
  pricingInfo[1].pricingExpression.tieredRates[1].unitPrice.nanos        AS price_nanos
FROM entries
"""


def _classify(description: str, hints: tuple[tuple[str, str], ...]) -> str | None:
    for needle, value in hints:
        if needle in description:
            return value
    return None


def ingest(*, skus_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    con = open_conn()
    path_literal = str(skus_path).replace("'", "''")
    sql = _SQL.replace("{path}", path_literal)
    for (
        sku_id, description, svc_name, resource_family, usage_type,
        service_regions, usage_unit, currency, price_units, price_nanos,
    ) in con.execute(sql).fetchall():
        if svc_name != "Cloud SQL":
            continue
        if resource_family != "Compute":
            continue
        if usage_type != "OnDemand":
            continue
        if currency != "USD":
            continue
        engine = _classify(description, _ENGINE_HINTS)
        deployment = _classify(description, _DEPLOYMENT_HINTS)
        if engine is None or deployment is None:
            continue
        tier_match = _TIER_RE.search(description)
        if tier_match is None:
            continue
        tier = tier_match.group(1)
        if not service_regions:
            continue
        region = service_regions[0]
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue
        divisor, unit = parse_usage_unit(usage_unit)
        amount = parse_unit_price(units=price_units, nanos=int(price_nanos)) / divisor
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": engine,
            "os": deployment,
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": sku_id,
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": tier,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "extra": {
                    "description": description,
                    "engine": engine,
                    "deployment_option": deployment,
                },
            },
            "terms": terms,
            "prices": [
                {"dimension": "compute", "tier": "", "amount": amount, "unit": unit},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.gcp_cloud_sql")
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
