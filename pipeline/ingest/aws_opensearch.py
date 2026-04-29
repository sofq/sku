"""Normalize AWS OpenSearch Service offer JSON into search.engine rows.

Two modes:
  managed-cluster  — Instance-based rows (productFamily = "Amazon OpenSearch Service").
  serverless       — OCU + storage rows (productFamily = "Amazon OpenSearch Service Serverless").
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
from collections.abc import Iterable
from pathlib import Path
from typing import Any

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import dumps
from .aws_common import load_region_normalizer

logger = logging.getLogger(__name__)

_PROVIDER = "aws"
_SERVICE = "opensearch"
_KIND = "search.engine"

# Instance families with known vcpu + memory specs (from AWS docs).
_INSTANCE_SPECS: dict[str, tuple[int, float]] = {
    "t3.small.search":    (2, 2.0),
    "t3.medium.search":   (2, 4.0),
    "m5.large.search":    (2, 8.0),
    "m5.xlarge.search":   (4, 16.0),
    "m5.2xlarge.search":  (8, 32.0),
    "m5.4xlarge.search":  (16, 64.0),
    "m6g.large.search":   (2, 8.0),
    "m6g.xlarge.search":  (4, 16.0),
    "m6g.2xlarge.search": (8, 32.0),
    "m6g.4xlarge.search": (16, 64.0),
    "r5.large.search":    (2, 16.0),
    "r5.xlarge.search":   (4, 32.0),
    "r5.2xlarge.search":  (8, 64.0),
    "r5.4xlarge.search":  (16, 128.0),
    "r6g.large.search":   (2, 16.0),
    "r6g.xlarge.search":  (4, 32.0),
    "r6g.2xlarge.search": (8, 64.0),
    "r6g.4xlarge.search": (16, 128.0),
    "c5.large.search":    (2, 4.0),
    "c5.xlarge.search":   (4, 8.0),
    "c5.2xlarge.search":  (8, 16.0),
    "i3.large.search":    (2, 15.25),
    "i3.xlarge.search":   (4, 30.5),
    "i3.2xlarge.search":  (8, 61.0),
    "i3.4xlarge.search":  (16, 122.0),
}


def _first_pd(term_data: dict) -> dict | None:
    term = next(iter(term_data.values()), None)
    if not term:
        return None
    return next(iter(term.get("priceDimensions", {}).values()), None)


def _classify_managed_cluster(attrs: dict) -> tuple[str, str] | None:
    """Return (instance_type, instance_family) or None to skip."""
    instance_type = attrs.get("instanceType", "")
    if not instance_type.endswith(".search"):
        return None
    family = instance_type.split(".")[0]
    return instance_type, family


def _classify_serverless(attrs: dict) -> str | None:
    """Return dimension name or None to skip."""
    operation = attrs.get("operation", "")
    if "OpenSearchComputeOCU" in operation:
        return "ocu"
    if "OpenSearchStorageOCU" in operation or "OpenSearchIndexingOCU" in operation:
        return "storage"
    return None


def ingest(*, offer_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with offer_path.open() as f:
        offer = json.load(f)
    products = offer.get("products", {})
    terms_od = offer.get("terms", {}).get("OnDemand", {})

    for sku_id, product in products.items():
        family = product.get("productFamily", "")
        attrs = product.get("attributes", {})
        region = attrs.get("regionCode", "")
        if not region:
            continue
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue
        pd = _first_pd(terms_od.get(sku_id) or {})
        if pd is None:
            continue
        usd = float(pd.get("pricePerUnit", {}).get("USD", "0"))
        if usd <= 0:
            continue
        unit_raw = pd.get("unit", "Hrs").lower()
        unit = "hour" if "hr" in unit_raw else unit_raw

        if family == "Amazon OpenSearch Service":
            result = _classify_managed_cluster(attrs)
            if result is None:
                continue
            instance_type, instance_family = result
            vcpu, memory_gb = _INSTANCE_SPECS.get(instance_type, (None, None))
            terms = apply_kind_defaults(_KIND, {
                "commitment": "on_demand",
                "tenancy": "shared",
                "os": "managed-cluster",
                "support_tier": "",
                "upfront": "",
                "payment_option": "",
            })
            yield {
                "sku_id": sku_id,
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": instance_type,
                "region": region,
                "region_normalized": region_normalized,
                "terms_hash": terms_hash(terms),
                "resource_attrs": {
                    "vcpu": vcpu,
                    "memory_gb": memory_gb,
                    "extra": {
                        "mode": "managed-cluster",
                        "instance_family": instance_family,
                    },
                },
                "terms": terms,
                "prices": [
                    {"dimension": "instance", "tier": "", "amount": usd, "unit": unit},
                ],
            }

        elif family in ("Amazon OpenSearch Serverless", "Amazon OpenSearch Service Serverless"):
            dimension = _classify_serverless(attrs)
            if dimension is None:
                continue
            # Serverless uses a single resource_name for all OCU/storage rows.
            resource_name = "opensearch-serverless"
            billed_unit = "gb-month" if dimension == "storage" else "hour"
            terms = apply_kind_defaults(_KIND, {
                "commitment": "on_demand",
                "tenancy": "shared",
                "os": "serverless",
                "support_tier": "",
                "upfront": "",
                "payment_option": "",
            })
            yield {
                "sku_id": sku_id,
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": resource_name,
                "region": region,
                "region_normalized": region_normalized,
                "terms_hash": terms_hash(terms),
                "resource_attrs": {
                    "vcpu": None,
                    "memory_gb": None,
                    "extra": {
                        "mode": "serverless",
                    },
                },
                "terms": terms,
                "prices": [
                    {"dimension": dimension, "tier": "", "amount": usd, "unit": billed_unit},
                ],
            }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_opensearch")
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
