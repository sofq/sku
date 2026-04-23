"""Normalize GCP Cloud Billing Catalog JSON for Cloud Run (gen2) into sku row dicts.

Spec §5 kind=compute.function (pragmatic; Cloud Run is technically
compute.container, but shares the serverless three-dimension billing shape
with Cloud Functions — see plan taxonomy note). One row per (region) with
dimensions cpu-second / memory-gb-second / requests, architecture x86_64.

resource_name is the architecture slug ("x86_64") so LookupServerlessFunction
can point-lookup via --architecture; the service field distinguishes run
from functions.

Filters: serviceDisplayName='Cloud Run', usageType='OnDemand', currency USD,
region resolvable, resourceGroup='CloudRunV2' (drops gen1 CloudRun rows).
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
_SERVICE = "run"
_KIND = "compute.function"
# x86_64 is the only architecture exposed by Cloud Run as of the
# last catalog verification (see docs/coverage/gcp-arm-verification.md).
# Re-verify quarterly; if arm SKUs appear, switch to per-SKU architecture
# detection per docs/superpowers/plans/2026-04-22-gcp-arm64-serverless.md
# Phase B branch 1.
_ARCHITECTURE = "x86_64"
_SERVICE_DISPLAY = "Cloud Run"
# Live API uses resourceGroup="Compute" for all Cloud Run SKUs (gen1 and gen2).
# Gen1 rows are identified by "(1st Gen)" in the description.
_RESOURCE_GROUP = "Compute"

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
    if usage_unit == "count" or description == "Requests":
        return "requests"
    # Skip min-instance, worker-pool, jobs, commitment, and tier-2 variants to
    # avoid duplicate CPU/memory entries per region; only ingest the canonical
    # "Services CPU/Memory (Request-based billing)" SKUs.
    # Note: live API uses "Min-Instance" (hyphenated) and "Commitment" in description.
    if any(kw in description for kw in ("Min-Instance", "Min Instance", "Tier 2", "Commitment", "Worker Pools", "Jobs CPU", "Jobs Memory", "(1st Gen)")):
        return None
    if "cpu" in d or "core" in d:
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
    # Global price (region='global') for the Requests dimension — shared across
    # all regions.  Populated first; applied as a fan-out when emitting rows.
    global_requests: dict[str, Any] | None = None

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
        if resource_group != _RESOURCE_GROUP:
            continue
        dim = _dimension(description, usage_unit)
        if dim is None:
            continue
        if not service_regions:
            continue
        try:
            divisor, unit = parse_usage_unit(usage_unit)
        except ValueError:
            continue
        amount = parse_unit_price(units=price_units, nanos=int(price_nanos or 0)) / divisor
        # Zero-priced requests dimension has no information (it's a free-tier
        # SKU — the real Requests price is carried by a sibling non-zero SKU,
        # or the service genuinely doesn't charge per request in this shard).
        # Keeping a {"amount": 0.0} entry misleads `sku gcp run price` and
        # inflates `sku estimate`'s output with phantom $0 request lines.
        if dim == "requests" and amount == 0.0:
            continue
        price_entry = {"dimension": dim, "tier": "", "amount": amount, "unit": unit}

        region = service_regions[0]
        if region == "global" and dim == "requests":
            # One global Requests SKU shared across all regions.
            global_requests = price_entry
            continue

        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue
        bucket = grouped.setdefault(region, {
            "region_normalized": region_normalized,
            "prices": {},
        })
        if dim == "cpu-second":
            bucket["sku_id"] = sku_id
            bucket["description"] = description
        bucket["prices"][dim] = price_entry

    for region, bucket in grouped.items():
        # Fan out the global Requests price into every region bucket — but
        # only if it's non-zero (zero was already filtered above).
        if global_requests is not None and "requests" not in bucket["prices"]:
            bucket["prices"]["requests"] = global_requests
        if not {"cpu-second", "memory-gb-second"} <= bucket["prices"].keys():
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
        prices_out = [
            bucket["prices"]["cpu-second"],
            bucket["prices"]["memory-gb-second"],
        ]
        if "requests" in bucket["prices"]:
            prices_out.append(bucket["prices"]["requests"])
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
                    "resource_group": _RESOURCE_GROUP,
                },
            },
            "terms": terms,
            "prices": prices_out,
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.gcp_run")
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
    n = 0
    with args.out.open("w") as fh:
        for row in ingest(skus_path=skus_path):
            fh.write(dumps(row) + "\n")
            n += 1
    print(f"ingest.gcp_run: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
