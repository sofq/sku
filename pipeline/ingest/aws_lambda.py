"""Normalize AWS Lambda offer JSON into sku row dicts.

Spec §5 compute.function kind. Lambda rows carry two price dimensions:
- requests (unit: requests) from group=AWS-Lambda-Requests
- duration (unit: second) from group=AWS-Lambda-Duration

One row per (architecture, region). Provisioned concurrency, SnapStart, and
ephemeral-storage surcharges are out of scope for m3a.2 (see non-goals).
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
_SERVICE = "lambda"
_KIND = "compute.function"

_ARCH_MAP: dict[str, str] = {"x86": "x86_64", "arm": "arm64"}
_GROUP_MAP: dict[str, str] = {
    "AWS-Lambda-Requests": "requests",
    "AWS-Lambda-Duration": "duration",
}


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

    grouped: dict[tuple[str, str], dict[str, dict[str, Any]]] = {}
    for sku_id, product in products.items():
        if product.get("productFamily") != "Serverless":
            continue
        attrs = product.get("attributes", {})
        region = attrs.get("regionCode", "")
        arch = _ARCH_MAP.get(attrs.get("archSupport", ""))
        dim = _GROUP_MAP.get(attrs.get("group", ""))
        if arch is None or dim is None:
            continue
        if normalizer.try_normalize(_PROVIDER, region) is None:
            continue
        pd = _first_pd(terms_od.get(sku_id) or {})
        if pd is None:
            continue
        usd = float(pd.get("pricePerUnit", {}).get("USD", "0"))
        unit = pd.get("unit", "")
        grouped.setdefault((arch, region), {})[dim] = {"sku": sku_id, "usd": usd, "unit": unit}

    for (arch, region), dims in sorted(grouped.items()):
        if {"requests", "duration"} - dims.keys():
            print(f"warn: dropping incomplete lambda row {arch}/{region}", file=sys.stderr)
            continue
        region_normalized = normalizer.normalize(_PROVIDER, region)
        sku_id = "::".join(sorted(dims[d]["sku"] for d in ("requests", "duration")))
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "",
            "os": "",
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": sku_id,
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": arch,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "architecture": arch,
                "extra": {},
            },
            "terms": terms,
            "prices": [
                {"dimension": "requests", "tier": "", "amount": dims["requests"]["usd"], "unit": dims["requests"]["unit"].lower()},
                {"dimension": "duration", "tier": "", "amount": dims["duration"]["usd"], "unit": dims["duration"]["unit"].lower()},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_lambda")
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
