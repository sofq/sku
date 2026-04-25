"""Normalize AWS RDS offer JSON into sku row dicts.

Spec §5 db.relational kind. Engine and deployment-option ride the
terms.tenancy and terms.os slots — see enums.yaml comment for rationale.
"""

from __future__ import annotations

import argparse
import json
import sys
from collections.abc import Iterable
from pathlib import Path
from typing import Any

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import dumps
from .aws_common import load_region_normalizer

_PROVIDER = "aws"
_SERVICE = "rds"
_KIND = "db.relational"

_ENGINE_MAP = {
    "PostgreSQL":        "postgres",
    "MySQL":             "mysql",
    "MariaDB":           "mariadb",
    "Oracle":            "oracle",
    "SQL Server":        "sqlserver",
}
_DEPL_MAP = {
    "Single-AZ":                    "single-az",
    "Multi-AZ":                     "multi-az",
    "Multi-AZ (readable standbys)": "multi-az-cluster",
}


def _parse_memory(raw: str) -> float:
    return float(raw.split()[0])


def _first_pd(term_data: dict) -> dict | None:
    term = next(iter(term_data.values()), None)
    if not term:
        return None
    return next(iter(term.get("priceDimensions", {}).values()), None)


def ingest(*, offer_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with offer_path.open() as f:
        offer = json.load(f)
    products = offer.get("products", {})
    terms_od = offer.get("terms", {}).get("OnDemand", {})

    for sku_id, product in products.items():
        if product.get("productFamily") != "Database Instance":
            continue
        attrs = product.get("attributes", {})
        instance_type = attrs.get("instanceType", "")
        region = attrs.get("regionCode", "")
        engine_raw = attrs.get("databaseEngine", "")
        depl_raw = attrs.get("deploymentOption", "")
        license_model = attrs.get("licenseModel", "")
        vcpu_raw = attrs.get("vcpu", "")
        memory_raw = attrs.get("memory", "")

        if engine_raw not in _ENGINE_MAP or depl_raw not in _DEPL_MAP:
            continue
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue
        pd = _first_pd(terms_od.get(sku_id) or {})
        if pd is None:
            continue
        usd = float(pd.get("pricePerUnit", {}).get("USD", "0"))
        unit = pd.get("unit", "")

        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": _ENGINE_MAP[engine_raw],
            "os": _DEPL_MAP[depl_raw],
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
                "vcpu": int(vcpu_raw) if vcpu_raw else None,
                "memory_gb": _parse_memory(memory_raw) if memory_raw else None,
                "extra": {
                    "engine": _ENGINE_MAP[engine_raw],
                    "deployment_option": _DEPL_MAP[depl_raw],
                    "license_model": license_model,
                },
            },
            "terms": terms,
            "prices": [
                {"dimension": "compute", "tier": "", "amount": usd, "unit": unit.lower()},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_rds")
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
