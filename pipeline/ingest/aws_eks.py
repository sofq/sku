"""Normalize AWS EKS offer JSON into container.orchestration rows."""

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
_SERVICE = "eks"
_KIND = "container.orchestration"


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

    # Fargate accumulates vcpu + memory prices before yielding one row per region
    fargate_buckets: dict[str, dict] = {}  # key = region

    for sku_id, product in products.items():
        attrs = product.get("attributes", {})
        op = attrs.get("operation", "")
        usage = attrs.get("usagetype", "")
        eks_type = attrs.get("eksproducttype", "")
        region = attrs.get("regionCode", "") or attrs.get("location", "")
        if not region:
            continue
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue

        # Skip AutoMode, hybrid nodes, and other addon capabilities
        if eks_type in ("AutoMode", "HybridNodes"):
            continue
        if op in (
            "ACKUsage",
            "ArgoCDUsage",
            "KROUsage",
            "HybridNodesUsage",
            "ProvisionedControlPlaneUsage",
            "EKSAutoUsage",
        ):
            continue

        pd = _first_pd(terms_od.get(sku_id) or {})
        if pd is None:
            continue
        usd = float(pd.get("pricePerUnit", {}).get("USD", "0"))
        unit = pd.get("unit", "").lower()

        # Control plane standard (skip Outposts variant)
        if op == "CreateOperation" and "Outposts" not in usage:
            tier = "standard"
            terms = apply_kind_defaults(_KIND, {
                "commitment": "on_demand",
                "tenancy": "kubernetes",
                "os": tier,
            })
            yield {
                "sku_id": sku_id,
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": "eks-standard",
                "region": region,
                "region_normalized": region_normalized,
                "terms_hash": terms_hash(terms),
                "resource_attrs": {
                    "vcpu": None,
                    "memory_gb": None,
                    "extra": {"mode": "control-plane", "tier": tier},
                },
                "terms": terms,
                "prices": [{"dimension": "cluster", "tier": "", "amount": usd, "unit": unit}],
            }

        # Control plane extended support
        elif op == "ExtendedSupport":
            tier = "extended-support"
            terms = apply_kind_defaults(_KIND, {
                "commitment": "on_demand",
                "tenancy": "kubernetes",
                "os": tier,
            })
            yield {
                "sku_id": sku_id,
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": "eks-extended-support",
                "region": region,
                "region_normalized": region_normalized,
                "terms_hash": terms_hash(terms),
                "resource_attrs": {
                    "vcpu": None,
                    "memory_gb": None,
                    "extra": {"mode": "control-plane", "tier": tier},
                },
                "terms": terms,
                "prices": [{"dimension": "cluster", "tier": "", "amount": usd, "unit": unit}],
            }

        # Fargate vCPU
        elif "Fargate-vCPU-Hours" in usage:
            if region not in fargate_buckets:
                ft = apply_kind_defaults(_KIND, {
                    "commitment": "on_demand",
                    "tenancy": "kubernetes",
                    "os": "fargate",
                })
                fargate_buckets[region] = {
                    "sku_id": f"aws-eks-fargate-{region}",
                    "provider": _PROVIDER,
                    "service": _SERVICE,
                    "kind": _KIND,
                    "resource_name": "eks-fargate",
                    "region": region,
                    "region_normalized": region_normalized,
                    "terms_hash": terms_hash(ft),
                    "resource_attrs": {
                        "vcpu": None,
                        "memory_gb": None,
                        "extra": {"mode": "fargate"},
                    },
                    "terms": ft,
                    "prices": [],
                }
            fargate_buckets[region]["prices"].append(
                {"dimension": "vcpu", "tier": "", "amount": usd, "unit": unit}
            )

        # Fargate memory (not EphemeralStorage)
        elif "Fargate-GB-Hours" in usage and "Ephemeral" not in usage:
            if region not in fargate_buckets:
                ft = apply_kind_defaults(_KIND, {
                    "commitment": "on_demand",
                    "tenancy": "kubernetes",
                    "os": "fargate",
                })
                fargate_buckets[region] = {
                    "sku_id": f"aws-eks-fargate-{region}",
                    "provider": _PROVIDER,
                    "service": _SERVICE,
                    "kind": _KIND,
                    "resource_name": "eks-fargate",
                    "region": region,
                    "region_normalized": region_normalized,
                    "terms_hash": terms_hash(ft),
                    "resource_attrs": {
                        "vcpu": None,
                        "memory_gb": None,
                        "extra": {"mode": "fargate"},
                    },
                    "terms": ft,
                    "prices": [],
                }
            fargate_buckets[region]["prices"].append(
                {"dimension": "memory", "tier": "", "amount": usd, "unit": unit}
            )

    # Emit Fargate rows only when both vcpu and memory prices are present
    for row in fargate_buckets.values():
        dims = {p["dimension"] for p in row["prices"]}
        if {"vcpu", "memory"}.issubset(dims):
            yield row


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_eks")
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
