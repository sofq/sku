"""Normalize GCP Memorystore (Redis + Memcached) SKU JSON into cache.kv rows."""

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
from .gcp_common import load_region_normalizer

_PROVIDER = "gcp"
_SERVICE = "memorystore"
_KIND = "cache.kv"

_MEMORY_RE = re.compile(r"(\d+(?:\.\d+)?)\s*GB", re.IGNORECASE)
_M_TIER_RE = re.compile(r"\bM([1-5])\b", re.IGNORECASE)
_TIER_PAT = re.compile(r"\b(basic|standard)\b", re.IGNORECASE)
_M_TIER_MEMORY_GB = {
    "1": 5.0,
    "2": 10.0,
    "3": 16.0,
    "4": 64.0,
    "5": 128.0,
}


def _engine_of(sku: dict) -> str | None:
    name = sku.get("category", {}).get("serviceDisplayName", "").lower()
    if "redis" in name:
        return "redis"
    if "memcached" in name:
        return "memcached"
    return None


def _hourly_usd(sku: dict) -> float:
    try:
        tiers = sku["pricingInfo"][0]["pricingExpression"]["tieredRates"]
        units = float(tiers[0]["unitPrice"]["units"])
        nanos = float(tiers[0]["unitPrice"]["nanos"]) / 1e9
        return units + nanos
    except (KeyError, IndexError):
        return 0.0


def _memory_gb(description: str) -> float | None:
    m = _MEMORY_RE.search(description)
    if m:
        return float(m.group(1))
    if re.search(r"\bCustom\s+Core\b", description, re.IGNORECASE):
        return None
    if re.search(r"\bCustom\s+RAM\b", description, re.IGNORECASE):
        return 1.0
    m_tier = _M_TIER_RE.search(description)
    if m_tier:
        return _M_TIER_MEMORY_GB[m_tier.group(1)]
    return None


def _tier(description: str) -> str | None:
    m = _TIER_PAT.search(description)
    return m.group(1).lower() if m else None


def ingest(*, skus_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with skus_path.open() as f:
        skus = json.load(f).get("skus", [])

    for sku in skus:
        # Filter usageType and currency
        usage_type = sku.get("category", {}).get("usageType", "OnDemand")
        if usage_type != "OnDemand":
            continue
        try:
            currency = sku["pricingInfo"][0]["pricingExpression"]["tieredRates"][0]["unitPrice"].get(
                "currencyCode", "USD"
            )
        except (KeyError, IndexError):
            continue
        if currency != "USD":
            continue

        engine = _engine_of(sku)
        if engine is None:
            continue
        description = sku.get("description", "")
        memory_gb = _memory_gb(description)
        if memory_gb is None:
            continue
        tier = _tier(description) or "standard"
        regions = sku.get("serviceRegions", [])
        usd = _hourly_usd(sku)
        if usd <= 0:
            continue

        for region in regions:
            region_normalized = normalizer.try_normalize(_PROVIDER, region)
            if region_normalized is None:
                continue
            resource_name = f"memorystore-{engine}-{tier}-{int(memory_gb)}gb"
            terms = apply_kind_defaults(
                _KIND,
                {
                    "commitment": "on_demand",
                    "tenancy": engine,
                    "os": "",
                    "support_tier": "",
                    "upfront": "",
                    "payment_option": "",
                },
            )
            yield {
                "sku_id": sku.get("skuId") or f"{resource_name}-{region}",
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": resource_name,
                "region": region,
                "region_normalized": region_normalized,
                "terms_hash": terms_hash(terms),
                "resource_attrs": {
                    "vcpu": None,
                    "memory_gb": memory_gb,
                    "extra": {"engine": engine, "tier": tier},
                },
                "terms": terms,
                "prices": [
                    {"dimension": "compute", "tier": "", "amount": usd, "unit": "hour"},
                ],
            }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.gcp_memorystore")
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
    print(f"ingest.gcp_memorystore: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
