"""Normalize GCP GKE (Kubernetes Engine) SKU JSON into container.orchestration rows."""

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
from .gcp_common import load_region_normalizer

logger = logging.getLogger(__name__)

_PROVIDER = "gcp"
_SERVICE = "gke"
_KIND = "container.orchestration"

# SKU descriptions to skip (exact match)
_SKIP_ZONAL = "Zonal Kubernetes Clusters"
_SKIP_EXTENDED = "Extended Period Kubernetes Clusters"

# Keywords to skip if present in description
_SKIP_SPOT = "Spot"
_SKIP_COMMITTED = "Committed"

# SKU id for the Regional Kubernetes Clusters control-plane price
_REGIONAL_SKU_ID = "B561-BFBD-1264"
# Last-resort default if upstream omits _REGIONAL_SKU_ID. GKE has charged
# $0.10/cluster-hour since 2020. We warn whenever we fall back so daily-data
# CI surfaces the upstream gap rather than silently shipping a stale price.
_REGIONAL_SKU_DEFAULT_PRICE = 0.10


def _hourly_usd(sku: dict) -> float:
    try:
        tiers = sku["pricingInfo"][0]["pricingExpression"]["tieredRates"]
        units = float(tiers[0]["unitPrice"]["units"])
        nanos = float(tiers[0]["unitPrice"]["nanos"]) / 1e9
        return units + nanos
    except (KeyError, IndexError):
        return 0.0


def ingest(*, skus_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with skus_path.open() as f:
        skus = json.load(f).get("skus", [])

    tracked_regions = {
        region: group
        for (provider, region), group in normalizer.table.items()
        if provider == _PROVIDER
    }

    # Pass 1: collect Autopilot regions and per-region prices
    # Key = region, value = dict with keys: mcpu, memory, storage
    autopilot_regions: dict[str, dict[str, Any]] = {}

    # Also collect the Regional control-plane price (global SKU, single price).
    # `regional_sku_seen` lets us distinguish "we found the SKU, here's the
    # live price" from "we never saw the SKU, falling back to the default".
    regional_cluster_price: float = _REGIONAL_SKU_DEFAULT_PRICE
    regional_sku_seen = False

    for sku in skus:
        description = sku.get("description", "")

        # Find the Regional Kubernetes Clusters price
        if sku.get("skuId") == _REGIONAL_SKU_ID:
            p = _hourly_usd(sku)
            if p > 0:
                regional_cluster_price = p
                regional_sku_seen = True
            continue

        # Skip unwanted cluster-level SKUs
        if description == _SKIP_ZONAL or description == _SKIP_EXTENDED:
            continue
        if _SKIP_SPOT in description:
            continue
        if _SKIP_COMMITTED in description or "Commitment" in description:
            continue

        # Only process Autopilot Pod mCPU Requests (not Spot, not Arm)
        if "Autopilot Pod mCPU Requests" not in description:
            continue
        if "Spot" in description or "Arm" in description:
            continue

        regions = sku.get("serviceRegions", [])
        for region in regions:
            if region == "global":
                continue
            region_normalized = normalizer.try_normalize(_PROVIDER, region)
            if region_normalized is None:
                continue
            if region not in autopilot_regions:
                autopilot_regions[region] = {}
            # Store the normalized region for later use
            autopilot_regions[region]["_region_normalized"] = region_normalized
            autopilot_regions[region]["mcpu"] = _hourly_usd(sku)

    # Second sub-pass: collect memory and storage prices for the discovered regions
    for sku in skus:
        description = sku.get("description", "")

        # Autopilot Pod Memory Requests (not Spot, not Arm)
        if "Autopilot Pod Memory Requests" in description:
            if "Spot" in description or "Arm" in description:
                continue
            regions = sku.get("serviceRegions", [])
            for region in regions:
                if region not in autopilot_regions:
                    continue
                autopilot_regions[region]["memory"] = _hourly_usd(sku)

        # Autopilot Pod Ephemeral Storage Requests (not Spot, not Arm)
        elif "Autopilot Pod Ephemeral Storage Requests" in description:
            if "Spot" in description or "Arm" in description:
                continue
            regions = sku.get("serviceRegions", [])
            for region in regions:
                if region not in autopilot_regions:
                    continue
                autopilot_regions[region]["storage"] = _hourly_usd(sku)

    # Pass 2: emit rows

    if not autopilot_regions:
        logger.warning(
            "ingest.gcp_gke: no Autopilot mCPU regions discovered; "
            "Autopilot rows will be empty",
        )

    if not regional_sku_seen:
        logger.warning(
            "ingest.gcp_gke: regional control-plane SKU %s not present in "
            "upstream response; falling back to $%.2f/cluster-hour for all "
            "%d regions",
            _REGIONAL_SKU_ID,
            _REGIONAL_SKU_DEFAULT_PRICE,
            len(autopilot_regions),
        )

    # Standard control-plane rows — one per tracked GCP region. The regional
    # control-plane SKU is global, so these rows must not depend on Autopilot
    # per-region SKU coverage.
    for region, region_normalized in sorted(tracked_regions.items()):
        terms = apply_kind_defaults(
            _KIND,
            {
                "commitment": "on_demand",
                "tenancy": "kubernetes",
                "os": "standard",
                "support_tier": "",
                "upfront": "",
                "payment_option": "",
            },
        )
        yield {
            "sku_id": f"{_REGIONAL_SKU_ID}-{region}",
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": "gke-standard",
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "vcpu": None,
                "memory_gb": None,
                "extra": {"tier": "standard", "mode": "control-plane"},
            },
            "terms": terms,
            "prices": [
                {"dimension": "cluster", "tier": "", "amount": regional_cluster_price, "unit": "hr"},
            ],
        }

    # Autopilot rows — one per region where we have all 3 prices
    for region, data in autopilot_regions.items():
        region_normalized = data.get("_region_normalized")
        if region_normalized is None:
            continue
        mcpu_price = data.get("mcpu")
        memory_price = data.get("memory")
        storage_price = data.get("storage")
        if mcpu_price is None or memory_price is None or storage_price is None:
            continue
        terms = apply_kind_defaults(
            _KIND,
            {
                "commitment": "on_demand",
                "tenancy": "kubernetes",
                "os": "autopilot",
                "support_tier": "",
                "upfront": "",
                "payment_option": "",
            },
        )
        yield {
            "sku_id": f"gke-autopilot-{region}",
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": "gke-autopilot",
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "vcpu": None,
                "memory_gb": None,
                "extra": {"tier": "autopilot", "mode": "autopilot"},
            },
            "terms": terms,
            "prices": [
                {"dimension": "vcpu", "tier": "", "amount": mcpu_price, "unit": "milliCPU-hr"},
                {"dimension": "memory", "tier": "", "amount": memory_price, "unit": "GiBy-hr"},
                {"dimension": "storage", "tier": "", "amount": storage_price, "unit": "GiBy-hr"},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.gcp_gke")
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
    print(f"ingest.gcp_gke: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
