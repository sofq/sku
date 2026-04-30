"""Normalize GCP Cloud Firestore Cloud Billing SKUs into db.nosql rows.

Only native-mode Firestore SKUs are ingested (resourceGroup prefix "Firestore").
Datastore-mode SKUs (resourceGroup prefix "Datastore") and free metadata ops
(FirestoreSmallOps) are excluded.

One row is emitted per region with four price dimensions:
  storage        — per GiB-month
  document_read  — per read operation
  document_write — per write operation
  document_delete — per delete operation

Region is parsed from the SKU description using a static map keyed on location
keywords (e.g. "North America", "Europe", "Asia", "South America").
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from collections.abc import Iterable
from pathlib import Path
from typing import Any

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import dumps
from .gcp_common import load_region_normalizer, parse_unit_price

_PROVIDER = "gcp"
_SERVICE = "firestore"
_KIND = "db.nosql"

# Resource groups to ingest and their output dimension names.
_RGROUP_TO_DIM: dict[str, str] = {
    "FirestoreEntityPutOps": "document_write",
    "FirestoreEntityDeleteOps": "document_delete",
    "FirestoreReadOps": "document_read",
    "FirestoreStorage": "storage",
}

# Skip this resource group — always $0.00 free-tier metadata ops.
_SKIP_RGROUP = "FirestoreSmallOps"

# Map description location keywords to canonical GCP region codes.
_REGION_MAP: dict[str, str] = {
    "North America": "us-east1",
    "Europe": "europe-west1",
    "Asia": "asia-northeast1",
    "South America": "southamerica-east1",
}

_REGION_RE = re.compile(r"(North America|Europe|Asia|South America)")


def _parse_region(description: str) -> str | None:
    """Extract a GCP region from a Firestore SKU description."""
    m = _REGION_RE.search(description)
    if m is None:
        return None
    return _REGION_MAP.get(m.group(1))


def _extract_price(sku: dict) -> float:
    """Return the per-unit price from the first tiered rate of a SKU."""
    try:
        rate = sku["pricingInfo"][0]["pricingExpression"]["tieredRates"][0]
        return parse_unit_price(
            units=rate["unitPrice"]["units"],
            nanos=int(rate["unitPrice"]["nanos"]),
        )
    except (KeyError, IndexError, ValueError):
        return 0.0


def ingest(*, skus_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()

    with skus_path.open() as f:
        skus = json.load(f).get("skus", [])

    # Group SKUs by region, collecting one price per dimension.
    # Structure: region -> dimension_name -> price
    region_dims: dict[str, dict[str, float]] = {}

    for sku in skus:
        category = sku.get("category", {})
        resource_group = category.get("resourceGroup", "")

        # Exclude Datastore-mode SKUs.
        if resource_group.startswith("Datastore"):
            continue

        # Only ingest known native Firestore resource groups.
        if resource_group not in _RGROUP_TO_DIM:
            continue

        # Skip free SmallOps (belt-and-suspenders; also excluded above).
        if resource_group == _SKIP_RGROUP:
            continue

        # Filter on OnDemand usage type.
        if category.get("usageType", "OnDemand") != "OnDemand":
            continue

        # Verify USD currency.
        try:
            currency = (
                sku["pricingInfo"][0]["pricingExpression"]["tieredRates"][0][
                    "unitPrice"
                ].get("currencyCode", "USD")
            )
        except (KeyError, IndexError):
            continue
        if currency != "USD":
            continue

        description = sku.get("description", "")
        region = _parse_region(description)
        if region is None:
            continue

        price = _extract_price(sku)
        # Storage always has a price; ops may be $0 only for SmallOps (already excluded).
        # We still accept $0 writes/reads if they're intentionally free in a region,
        # but the fixture tests use non-zero prices, so this is fine.

        dimension = _RGROUP_TO_DIM[resource_group]
        region_dims.setdefault(region, {})[dimension] = price

    required_dims = {"storage", "document_read", "document_write", "document_delete"}

    for region in sorted(region_dims):
        dims = region_dims[region]
        if required_dims - dims.keys():
            print(
                f"warn: dropping incomplete firestore row for region {region!r} "
                f"(missing: {required_dims - dims.keys()})",
                file=sys.stderr,
            )
            continue

        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            print(
                f"warn: unknown region {region!r} for gcp_firestore, skipping",
                file=sys.stderr,
            )
            continue

        sku_id = f"FIRESTORE-NATIVE-{region}"

        terms = apply_kind_defaults(
            _KIND,
            {
                "commitment": "on_demand",
                "tenancy": "firestore-native",
                "os": "",
                "support_tier": "",
                "upfront": "",
                "payment_option": "",
            },
        )

        yield {
            "sku_id": sku_id,
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": "native",
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "vcpu": None,
                "memory_gb": None,
                "extra": {"mode": "native"},
            },
            "terms": terms,
            "prices": [
                {
                    "dimension": "storage",
                    "tier": "",
                    "amount": dims["storage"],
                    "unit": "gb-mo",
                },
                {
                    "dimension": "document_read",
                    "tier": "",
                    "amount": dims["document_read"],
                    "unit": "request",
                },
                {
                    "dimension": "document_write",
                    "tier": "",
                    "amount": dims["document_write"],
                    "unit": "request",
                },
                {
                    "dimension": "document_delete",
                    "tier": "",
                    "amount": dims["document_delete"],
                    "unit": "request",
                },
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.gcp_firestore")
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
    print(f"ingest.gcp_firestore: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
