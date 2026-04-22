"""Normalize AWS EBS rows out of the EC2 offer JSON (pure Python).

Spec §5 storage.block. EBS shares the AmazonEC2 offer with aws_ec2 — we use
the same stripped per-region files. Each row carries only the storage dimension
(GB-Mo); IOPS + throughput pricing -> m3a.3. One row per (volume_type, region);
volume_type comes from attributes.volumeApiName.
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
from .aws_common import (
    aws_regions_from_yaml,
    fetch_offer_regions_stripped,
    load_region_normalizer,
    shared_offer_basename,
)
from .aws_ec2 import _iter_product_prices

_PROVIDER = "aws"
_SERVICE = "ebs"
_KIND = "storage.block"

_ALLOWED_TYPES: set[str] = {"gp3", "gp2", "io2", "st1", "sc1"}


def ingest(*, offer_paths: Iterable[Path]) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    for offer_path in offer_paths:
        with offer_path.open() as fh:
            offer = json.load(fh)
        for sku_id, product, pd in _iter_product_prices(offer):
            if product.get("productFamily") != "Storage":
                continue
            attrs = product.get("attributes") or {}
            vol_type = attrs.get("volumeApiName")
            if vol_type not in _ALLOWED_TYPES:
                continue
            region = attrs.get("regionCode", "")
            region_normalized = normalizer.try_normalize(_PROVIDER, region)
            if region_normalized is None:
                continue

            unit = pd.get("unit") or ""
            usd_raw = (pd.get("pricePerUnit") or {}).get("USD")
            if usd_raw is None:
                continue
            try:
                usd = float(usd_raw)
            except (TypeError, ValueError):
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
                "sku_id": sku_id,
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": vol_type,
                "region": region,
                "region_normalized": region_normalized,
                "terms_hash": terms_hash(terms),
                "resource_attrs": {
                    "extra": {"volume_type": vol_type},
                },
                "terms": terms,
                "prices": [
                    {"dimension": "storage", "tier": "", "amount": usd, "unit": unit.lower()},
                ],
            }


def _resolve_paths(args: argparse.Namespace) -> list[Path]:
    if args.offer_dir:
        base = shared_offer_basename("aws_ebs")
        return sorted(
            p
            for p in args.offer_dir.glob(f"{base}-*.json")
            if p.is_file() and not p.name.endswith("-region_index.json")
        )
    if args.fixture:
        p = args.fixture
        return [p / "offer.json"] if p.is_dir() else [p]
    if args.offer:
        return [args.offer]
    return []


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_ebs")
    ap.add_argument("--fixture", type=Path)
    ap.add_argument("--offer", type=Path)
    ap.add_argument(
        "--offer-dir",
        type=Path,
        help="directory to fetch stripped per-region offers into (if empty, fetches).",
    )
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--catalog-version", default=None)
    args = ap.parse_args()

    if args.offer_dir is not None:
        base = shared_offer_basename("aws_ebs")
        have = any(
            p.is_file() and not p.name.endswith("-region_index.json")
            for p in args.offer_dir.glob(f"{base}-*.json")
        )
        if not have:
            args.offer_dir.mkdir(parents=True, exist_ok=True)
            fetch_offer_regions_stripped(
                "aws_ebs", args.offer_dir, regions=aws_regions_from_yaml()
            )

    paths = _resolve_paths(args)
    if not paths:
        print("either --fixture, --offer, or --offer-dir required", file=sys.stderr)
        return 2

    args.out.parent.mkdir(parents=True, exist_ok=True)
    with args.out.open("w") as fh:
        for row in ingest(offer_paths=paths):
            fh.write(dumps(row) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
