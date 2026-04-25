"""Normalize AWS Aurora offer JSON into sku row dicts.

Aurora rows split into two capacity_mode variants:
- provisioned: instance-type-priced (db.r6g.large etc).
- serverless-v2: ACU-hour-priced; vcpu/memory null.

Storage (per GB-month) and I/O (per million requests) are extracted once per
region and attached to every instance row's extra.
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
_SERVICE = "aurora"
_KIND = "db.relational"

_ENGINE_MAP = {
    "Aurora MySQL":      "aurora-mysql",
    "Aurora PostgreSQL": "aurora-postgres",
}


def _parse_memory(raw: str) -> float:
    return float(raw.split()[0])


def _first_pd(term_data: dict) -> dict | None:
    term = next(iter(term_data.values()), None)
    if not term:
        return None
    return next(iter(term.get("priceDimensions", {}).values()), None)


def _storage_class_of(attrs: dict) -> str:
    if "I/O-Optimized" in attrs.get("usagetype", ""):
        return "io-optimized"
    return "standard"


def _collect_region_extras(products: dict, terms_od: dict) -> dict[tuple[str, str], dict]:
    extras: dict[tuple[str, str], dict] = {}
    io_per_region: dict[str, float] = {}

    for sku_id, product in products.items():
        family = product.get("productFamily", "")
        attrs = product.get("attributes", {})
        region = attrs.get("regionCode", "")
        if not region:
            continue
        if family == "Database Storage" and "Aurora" in attrs.get("storageMedia", ""):
            pd = _first_pd(terms_od.get(sku_id) or {})
            if pd is None:
                continue
            usd = float(pd.get("pricePerUnit", {}).get("USD", "0"))
            cls = _storage_class_of(attrs)
            extras[(region, cls)] = {
                "storage_gb_month_usd": usd,
                "io_per_million_usd": None,
                "storage_class": cls,
            }
        elif family == "System Operation" and attrs.get("group") == "Aurora-IO-Operation":
            pd = _first_pd(terms_od.get(sku_id) or {})
            if pd is None:
                continue
            usd = float(pd.get("pricePerUnit", {}).get("USD", "0"))
            io_per_region[region] = usd * 1_000_000

    for (region, _cls), bucket in extras.items():
        if region in io_per_region:
            bucket["io_per_million_usd"] = io_per_region[region]
    return extras


def _pick_canonical_storage(region_extras: dict[tuple[str, str], dict], region: str) -> dict:
    if (region, "standard") in region_extras:
        return dict(region_extras[(region, "standard")])
    if (region, "io-optimized") in region_extras:
        return dict(region_extras[(region, "io-optimized")])
    return {}


def ingest(*, offer_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with offer_path.open() as f:
        offer = json.load(f)
    products = offer.get("products", {})
    terms_od = offer.get("terms", {}).get("OnDemand", {})
    region_extras = _collect_region_extras(products, terms_od)

    for sku_id, product in products.items():
        family = product.get("productFamily", "")
        if family not in {"Database Instance", "ServerlessV2"}:
            continue
        attrs = product.get("attributes", {})
        engine_raw = attrs.get("databaseEngine", "")
        if engine_raw not in _ENGINE_MAP:
            continue
        engine = _ENGINE_MAP[engine_raw]
        region = attrs.get("regionCode", "")
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue
        pd = _first_pd(terms_od.get(sku_id) or {})
        if pd is None:
            continue
        usd = float(pd.get("pricePerUnit", {}).get("USD", "0"))
        unit = pd.get("unit", "").lower()
        rextra = _pick_canonical_storage(region_extras, region)

        if family == "Database Instance":
            instance_type = attrs.get("instanceType", "")
            vcpu_raw = attrs.get("vcpu", "")
            memory_raw = attrs.get("memory", "")
            extra = {**rextra, "engine": engine, "capacity_mode": "provisioned"}
            terms = apply_kind_defaults(_KIND, {
                "commitment": "on_demand",
                "tenancy": engine,
                "os": "single-az",
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
                    "extra": extra,
                },
                "terms": terms,
                "prices": [{"dimension": "compute", "tier": "", "amount": usd, "unit": unit}],
            }

        elif family == "ServerlessV2":
            extra = {**rextra, "engine": engine, "capacity_mode": "serverless-v2", "acu_hour_usd": usd}
            terms = apply_kind_defaults(_KIND, {
                "commitment": "on_demand",
                "tenancy": engine,
                "os": "single-az",
                "support_tier": "",
                "upfront": "",
                "payment_option": "",
            })
            yield {
                "sku_id": sku_id,
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": "aurora-serverless-v2",
                "region": region,
                "region_normalized": region_normalized,
                "terms_hash": terms_hash(terms),
                "resource_attrs": {"vcpu": None, "memory_gb": None, "extra": extra},
                "terms": terms,
                "prices": [{"dimension": "compute", "tier": "", "amount": usd, "unit": unit}],
            }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_aurora")
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
