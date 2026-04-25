"""Normalize Azure AKS Retail Prices into container.orchestration rows.

Upstream: Azure Retail Prices API, serviceName='Azure Kubernetes Service'.
Optional: Container Instances API for virtual-nodes (ACI) pricing.

Out of scope for M-γ.2:
- AKS Automatic product (separate pricing model, per compute profile)
- Windows virtual-nodes (ACI Windows adds a per-second software charge on top
  of Linux compute rates; no separate Windows vCPU price exists in the API)
- Spot pricing for ACI
- GPU / Confidential ACI containers
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
from .azure_common import load_region_normalizer

_PROVIDER = "azure"
_SERVICE = "aks"
_KIND = "container.orchestration"

# Meter names to tier mapping from AKS API
_METER_TO_TIER = {
    "Standard Uptime SLA": "standard",
    "Standard Long Term Support": "premium",  # LTS add-on treated as premium tier
}


def ingest(*, prices_path: Path, aci_prices_path: Path | None = None) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with prices_path.open() as f:
        items = json.load(f).get("Items", [])

    # Track regions where we saw a Standard entry to synthesize Free tier
    standard_regions: dict[str, str] = {}  # region -> region_normalized

    for item in items:
        if item.get("type") != "Consumption" or item.get("currencyCode") != "USD":
            continue
        product_name = item.get("productName", "")
        meter_name = item.get("meterName", "")
        region = item.get("armRegionName", "")
        if not region:
            continue
        # Only process plain AKS (not Automatic)
        if product_name != "Azure Kubernetes Service":
            continue
        tier = _METER_TO_TIER.get(meter_name)
        if tier is None:
            continue
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue

        usd = float(item.get("retailPrice", 0))
        unit = "hour"
        resource_name = f"aks-{tier}"
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "kubernetes",
            "os": tier,
        })
        yield {
            "sku_id": f"azure-aks-{tier}-{region}",
            "provider": _PROVIDER, "service": _SERVICE, "kind": _KIND,
            "resource_name": resource_name,
            "region": region, "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "vcpu": None, "memory_gb": None,
                "extra": {"mode": "control-plane", "tier": tier},
            },
            "terms": terms,
            "prices": [{"dimension": "cluster", "tier": "", "amount": usd, "unit": unit}],
        }

        # Track standard regions for Free tier synthesis
        if tier == "standard":
            standard_regions[region] = region_normalized

    # Synthesize Free tier rows ($0) for regions with a Standard entry
    for region, region_normalized in standard_regions.items():
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "kubernetes",
            "os": "free",
        })
        yield {
            "sku_id": f"azure-aks-free-{region}",
            "provider": _PROVIDER, "service": _SERVICE, "kind": _KIND,
            "resource_name": "aks-free",
            "region": region, "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "vcpu": None, "memory_gb": None,
                "extra": {"mode": "control-plane", "tier": "free"},
            },
            "terms": terms,
            "prices": [{"dimension": "cluster", "tier": "", "amount": 0.0, "unit": "hour"}],
        }

    # Virtual-nodes (ACI) — Linux only in M-γ.2
    if aci_prices_path is not None:
        with aci_prices_path.open() as f:
            aci_items = json.load(f).get("Items", [])

        # Accumulate vcpu + memory per region
        vn_buckets: dict[str, dict] = {}

        for item in aci_items:
            if item.get("type", "Consumption") != "Consumption" or item.get("currencyCode") != "USD":
                continue
            product_name = item.get("productName", "")
            meter_name = item.get("meterName", "")
            sku_name = item.get("skuName", "")
            region = item.get("armRegionName", "")
            if not region or product_name != "Container Instances" or sku_name != "Standard":
                continue
            region_normalized = normalizer.try_normalize(_PROVIDER, region)
            if region_normalized is None:
                continue

            usd = float(item.get("retailPrice", 0))
            uom = item.get("unitOfMeasure", "")
            # Normalize unit: "1 Hour" -> "hour", "1 GB Hour" -> "gb-hour"
            unit = "gb-hour" if "GB" in uom else "hour"

            if "vCPU" in meter_name and "Memory" not in meter_name:
                dim = "vcpu"
            elif "Memory" in meter_name:
                dim = "memory"
            else:
                continue

            if region not in vn_buckets:
                vn_terms = apply_kind_defaults(_KIND, {
                    "commitment": "on_demand",
                    "tenancy": "kubernetes",
                    "os": "virtual-nodes",
                })
                vn_buckets[region] = {
                    "sku_id": f"azure-aks-vn-linux-{region}",
                    "provider": _PROVIDER, "service": _SERVICE, "kind": _KIND,
                    "resource_name": "aks-virtual-nodes-linux",
                    "region": region, "region_normalized": region_normalized,
                    "terms_hash": terms_hash(vn_terms),
                    "resource_attrs": {
                        "vcpu": None, "memory_gb": None,
                        "extra": {"mode": "virtual-nodes", "aci_os": "linux"},
                    },
                    "terms": vn_terms,
                    "prices": [],
                }
            vn_buckets[region]["prices"].append(
                {"dimension": dim, "tier": "", "amount": usd, "unit": unit}
            )

        for row in vn_buckets.values():
            dims = {p["dimension"] for p in row["prices"]}
            if {"vcpu", "memory"}.issubset(dims):
                yield row


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.azure_aks")
    ap.add_argument("--fixture", type=Path)
    ap.add_argument("--prices", type=Path)
    ap.add_argument("--aci-prices", type=Path)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--catalog-version", default=None)
    args = ap.parse_args()
    if args.fixture:
        prices_path = args.fixture / "prices.json" if args.fixture.is_dir() else args.fixture
        aci_path = None
    elif args.prices:
        prices_path = args.prices
        aci_path = args.aci_prices
    else:
        print("either --fixture or --prices required", file=sys.stderr)
        return 2
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with args.out.open("w") as fh:
        for row in ingest(prices_path=prices_path, aci_prices_path=aci_path):
            fh.write(dumps(row) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
