"""Normalize AWS EC2 offer JSON into sku row dicts (pure Python).

The AWS AmazonEC2 offer is ~8 GB combined. `aws_common.fetch_offer_regions_stripped`
downloads per-region index files, stream-strips each via ijson, and hands us
small JSON files (~30 MB). Here we iterate those files with `json.load` and
emit NDJSON rows for our coverage surface (Compute Instance, Linux/Windows/RHEL,
Shared/Dedicated, On-Demand).
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

_PROVIDER = "aws"
_SERVICE = "ec2"
_KIND = "compute.vm"

_OS_MAP: dict[str, str] = {"Linux": "linux", "Windows": "windows", "RHEL": "rhel"}
_TENANCY_MAP: dict[str, str] = {"Shared": "shared", "Dedicated": "dedicated"}


def _parse_memory(raw: str) -> float:
    return float(raw.split()[0])


def _iter_product_prices(offer: dict[str, Any]) -> Iterable[tuple[str, dict, dict]]:
    """Yield (sku_id, product, price_dimension) for every On-Demand priced product.

    Works on both the full AWS EC2 offer shape and the stripped shape — both keep
    `products.<sku>.{productFamily,attributes}` and
    `terms.OnDemand.<sku>.<termKey>.priceDimensions.<pdKey>.{unit, pricePerUnit}`.
    """
    products = offer.get("products") or {}
    terms_od = (offer.get("terms") or {}).get("OnDemand") or {}
    for sku_id, product in products.items():
        term_data = terms_od.get(sku_id)
        if not term_data:
            continue
        term_obj = next(iter(term_data.values()), None) or {}
        pd = next(iter((term_obj.get("priceDimensions") or {}).values()), None)
        if pd is None:
            continue
        yield sku_id, product, pd


def ingest(*, offer_paths: Iterable[Path]) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    for offer_path in offer_paths:
        with offer_path.open() as fh:
            offer = json.load(fh)
        for sku_id, product, pd in _iter_product_prices(offer):
            if product.get("productFamily") != "Compute Instance":
                continue
            attrs = product.get("attributes") or {}
            if attrs.get("preInstalledSw") != "NA":
                continue
            if attrs.get("capacitystatus") != "Used":
                continue
            os_raw = attrs.get("operatingSystem")
            tenancy_raw = attrs.get("tenancy")
            if os_raw not in _OS_MAP or tenancy_raw not in _TENANCY_MAP:
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
                "tenancy": _TENANCY_MAP[tenancy_raw],
                "os": _OS_MAP[os_raw],
                "support_tier": "",
                "upfront": "",
                "payment_option": "",
            })
            yield {
                "sku_id": sku_id,
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": attrs.get("instanceType"),
                "region": region,
                "region_normalized": region_normalized,
                "terms_hash": terms_hash(terms),
                "resource_attrs": {
                    "vcpu": int(attrs.get("vcpu")),
                    "memory_gb": _parse_memory(attrs.get("memory", "0")),
                    "architecture": "x86_64",
                    "extra": {
                        "physical_processor": attrs.get("physicalProcessor"),
                        "network_performance": attrs.get("networkPerformance"),
                    },
                },
                "terms": terms,
                "prices": [
                    {"dimension": "compute", "tier": "", "amount": usd, "unit": unit.lower()},
                ],
            }


def _resolve_paths(args: argparse.Namespace) -> list[Path]:
    """Resolve CLI args → a list of stripped offer-JSON paths ready to feed ingest."""
    if args.offer_dir:
        base = shared_offer_basename("aws_ec2")
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
    ap = argparse.ArgumentParser(prog="ingest.aws_ec2")
    ap.add_argument("--fixture", type=Path, help="path to a trimmed offer.json (tests)")
    ap.add_argument("--offer", type=Path, help="single stripped offer.json")
    ap.add_argument(
        "--offer-dir",
        type=Path,
        help="directory to fetch stripped per-region offers into (if empty, fetches).",
    )
    ap.add_argument("--out", type=Path, required=True)
    # --catalog-version is consumed by the packager; accept and ignore here.
    ap.add_argument("--catalog-version", default=None)
    args = ap.parse_args()

    # If offer-dir is given but empty of shared-basename stripped files,
    # drive the fetch ourselves. fetch_offer_regions_stripped short-circuits
    # per-region on an already-stripped file so a sibling `aws_ebs` run can
    # skip the download+strip entirely.
    if args.offer_dir is not None:
        base = shared_offer_basename("aws_ec2")
        have = any(
            p.is_file() and not p.name.endswith("-region_index.json")
            for p in args.offer_dir.glob(f"{base}-*.json")
        )
        if not have:
            args.offer_dir.mkdir(parents=True, exist_ok=True)
            fetch_offer_regions_stripped(
                "aws_ec2", args.offer_dir, regions=aws_regions_from_yaml()
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
