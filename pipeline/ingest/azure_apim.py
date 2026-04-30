"""Normalize Azure API Management (APIM) Retail Prices into api.gateway rows.

Tier mapping:
  Consumption → two-tier call pricing (first 1M calls free, then per-call).
  All other tiers → single unit_hour pricing.

terms.os reuses the api.gateway os/mode tokens added in M-δ Phase 0:
  apim-consumption, apim-developer, apim-basic, apim-standard,
  apim-premium, apim-isolated, apim-premium-v2.
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
from .azure_common import load_region_normalizer

logger = logging.getLogger(__name__)

_PROVIDER = "azure"
_SERVICE = "apim"
_KIND = "api.gateway"

# Map from skuName (lower) to (resource_name, os_token, mode)
_SKU_MAP: dict[str, tuple[str, str, str]] = {
    "consumption":   ("consumption",  "apim-consumption", "consumption"),
    "developer":     ("developer",    "apim-developer",   "provisioned"),
    "basic":         ("basic",        "apim-basic",       "provisioned"),
    "standard":      ("standard",     "apim-standard",    "provisioned"),
    "premium":       ("premium",      "apim-premium",     "provisioned"),
    "isolated":      ("isolated",     "apim-isolated",    "provisioned"),
    "premium v2":    ("premium-v2",   "apim-premium-v2",  "provisioned"),
}

# Excluded skuName patterns (case-insensitive substring match)
_EXCLUDE_KEYWORDS = [
    "gateway unit",
    "workspace gateway",
    "secondary unit",
    "self-hosted",
    "basic v2",
    "standard v2",
]


def _classify_sku(sku_name: str) -> tuple[str, str, str] | None:
    """Return (resource_name, os_token, mode) for a known skuName, or None."""
    lower = sku_name.lower().strip()
    # Check exclusions first.
    for excl in _EXCLUDE_KEYWORDS:
        if excl in lower:
            return None
    return _SKU_MAP.get(lower)


def ingest(*, prices_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with prices_path.open() as f:
        items = json.load(f).get("Items", [])

    # Group by (region, sku_lower) to prefer hourly over monthly when both exist.
    # Key: (armRegionName, sku_lower), Value: list of candidate items
    grouped: dict[tuple[str, str], list[dict[str, Any]]] = {}
    for item in items:
        service_name = item.get("serviceName", "")
        if service_name != "API Management":
            continue
        row_type = item.get("type", "Consumption")
        currency = item.get("currencyCode", "USD")
        if row_type != "Consumption" or currency != "USD":
            continue
        region = item.get("armRegionName", "")
        if not region:
            continue
        sku_name = item.get("skuName", "")
        sku_lower = sku_name.lower().strip()
        key = (region, sku_lower)
        grouped.setdefault(key, []).append(item)

    for (region, sku_lower), candidates in grouped.items():
        cls = _classify_sku(sku_lower)
        if cls is None:
            continue

        resource_name, os_token, mode = cls

        # Prefer hourly items; if none, fall back to monthly/other.
        hourly = [c for c in candidates if c.get("unitOfMeasure") == "1 Hour"]
        chosen = hourly if hourly else candidates

        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            logger.debug("azure_apim: unknown region %r — skipped", region)
            continue

        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "",
            "os": os_token,
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })

        if mode == "consumption":
            # Consumption pricing: derive from the per-10K call price.
            # The API reports price per 10K calls; convert to per-call.
            # Build two price tiers: free (0 → 1M) and paid (1M → ∞).
            per_call_items = [
                c for c in candidates
                if c.get("unitOfMeasure") == "10K"
            ]
            if not per_call_items:
                logger.warning(
                    "azure_apim: no 10K consumption item for region %r", region
                )
                continue
            item = per_call_items[0]
            usd_per_10k = float(item.get("retailPrice", 0))
            if usd_per_10k <= 0:
                continue
            per_call_price = usd_per_10k / 10_000.0
            sku_id = item.get("skuId") or f"APIM-{resource_name.upper()}-{region}"
            prices = [
                {
                    "dimension": "call",
                    "tier": "0",
                    "tier_upper": "1000000",
                    "amount": 0.0,
                    "unit": "request",
                },
                {
                    "dimension": "call",
                    "tier": "1000000",
                    "tier_upper": "",
                    "amount": per_call_price,
                    "unit": "request",
                },
            ]
        else:
            # Provisioned tiers: single unit_hour price entry.
            if not chosen:
                continue
            item = chosen[0]
            usd_per_hour = float(item.get("retailPrice", 0))
            if usd_per_hour <= 0:
                continue
            sku_id = item.get("skuId") or f"APIM-{resource_name.upper()}-{region}"
            prices = [
                {
                    "dimension": "unit_hour",
                    "tier": "0",
                    "tier_upper": "",
                    "amount": usd_per_hour,
                    "unit": "hr",
                },
            ]

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
                    "mode": mode,
                },
            },
            "terms": terms,
            "prices": prices,
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.azure_apim")
    ap.add_argument("--fixture", type=Path)
    ap.add_argument("--prices", type=Path)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--catalog-version", default=None)
    args = ap.parse_args()
    if args.fixture:
        prices_path = args.fixture / "prices.json" if args.fixture.is_dir() else args.fixture
    elif args.prices:
        prices_path = args.prices
    else:
        print("either --fixture or --prices required", file=sys.stderr)
        return 2
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with args.out.open("w") as fh:
        for row in ingest(prices_path=prices_path):
            fh.write(dumps(row) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
