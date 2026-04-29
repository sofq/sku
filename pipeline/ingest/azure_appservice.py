"""Normalize Azure App Service Retail Prices into paas.app rows.

Tier mapping uses a static plan-spec table (vcpu, memory_gb) keyed by SKU name
(e.g. "P1v3", "F1"). Unknown SKUs emit a row with null vcpu/memory and a
warning so downstream validation can surface coverage gaps.
"""

from __future__ import annotations

import argparse
import json
import logging
import re
import sys
from collections.abc import Iterable
from pathlib import Path
from typing import Any

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import dumps
from .azure_common import load_region_normalizer, parse_unit_of_measure

logger = logging.getLogger(__name__)

_PROVIDER = "azure"
_SERVICE = "appservice"
_KIND = "paas.app"

# Static plan specs: (vcpu, memory_gb) keyed by canonical SKU name.
# Source: https://azure.microsoft.com/en-us/pricing/details/app-service/linux/
_PLAN_SKU_SPECS: dict[str, tuple[int, float]] = {
    # Free / Shared
    "F1":   (1, 1.0),
    "D1":   (1, 1.75),
    # Basic
    "B1":   (1, 1.75),
    "B2":   (2, 3.5),
    "B3":   (4, 7.0),
    # Standard
    "S1":   (1, 1.75),
    "S2":   (2, 3.5),
    "S3":   (4, 7.0),
    # Premium v2
    "P1v2": (1, 3.5),
    "P2v2": (2, 7.0),
    "P3v2": (4, 14.0),
    # Premium v3
    "P0v3": (1, 4.0),
    "P1v3": (2, 8.0),
    "P2v3": (4, 16.0),
    "P3v3": (8, 32.0),
    # Isolated v2
    "I1v2": (2, 8.0),
    "I2v2": (4, 16.0),
    "I3v2": (8, 32.0),
    "I4v2": (16, 64.0),
    "I5v2": (32, 128.0),
    "I6v2": (64, 256.0),
    # Isolated
    "I1":   (2, 3.5),
    "I2":   (4, 7.0),
    "I3":   (8, 14.0),
}

# Map from productName keyword → support_tier
_TIER_KEYWORDS: list[tuple[str, str]] = [
    ("Isolated v2", "isolatedv2"),
    ("Isolated",    "isolated"),
    ("PremiumV3",   "premiumv3"),
    ("PremiumV2",   "premium"),
    ("Premium",     "premium"),
    ("Standard",    "standard"),
    ("Basic",       "basic"),
    ("Shared",      "shared"),
    ("Free",        "free"),
]

_SKU_RE = re.compile(r"\b(F1|D1|B[123]|S[123]|P[0-3]v[23]?|I[1-6]v?2?)\b", re.IGNORECASE)
_OS_RE  = re.compile(r"\b(Linux|Windows)\b", re.IGNORECASE)


def _classify_tier(product_name: str) -> str:
    for keyword, tier in _TIER_KEYWORDS:
        if keyword.lower() in product_name.lower():
            return tier
    return ""


def _classify_os(product_name: str, meter_name: str) -> str:
    for text in (product_name, meter_name):
        m = _OS_RE.search(text)
        if m:
            return m.group(1).lower()
    return "linux"  # default


def ingest(*, prices_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with prices_path.open() as f:
        items = json.load(f).get("Items", [])

    for item in items:
        product_name = item.get("productName", "")
        meter_name = item.get("meterName", "")
        sku_name_raw = item.get("skuName", "")
        region = item.get("armRegionName", "")
        usd = float(item.get("retailPrice", 0))
        uom = item.get("unitOfMeasure", "")
        row_type = item.get("type", "Consumption")
        currency = item.get("currencyCode", "USD")

        if row_type != "Consumption" or currency != "USD":
            continue
        if not region:
            continue

        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue

        try:
            divisor, unit = parse_unit_of_measure(uom)
        except ValueError:
            continue

        usd = usd / divisor
        if usd <= 0:
            continue

        # Extract canonical SKU name from skuName or meterName
        sku_match = _SKU_RE.search(sku_name_raw) or _SKU_RE.search(meter_name)
        if not sku_match:
            continue
        sku = sku_match.group(1).upper()
        # Normalize Pxv3 pattern to P*v3
        sku = re.sub(r"(?i)P(\d)V(\d)", lambda m: f"P{m.group(1)}v{m.group(2)}", sku)

        tier = _classify_tier(product_name)
        if not tier:
            continue

        os_val = _classify_os(product_name, meter_name)
        specs = _PLAN_SKU_SPECS.get(sku)
        if specs is None:
            logger.warning("azure_appservice: unknown SKU %r in %r", sku, product_name)

        vcpu = specs[0] if specs else None
        memory_gb = specs[1] if specs else None

        sku_id = item.get("skuId") or f"{_SERVICE}-{sku}-{os_val}-{region}"

        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "dedicated",
            "os": os_val,
            "support_tier": tier,
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": sku_id,
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": sku,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "vcpu": vcpu,
                "memory_gb": memory_gb,
                "extra": {
                    "tier": tier,
                    "os": os_val,
                    **({"unknown_sku": True} if specs is None else {}),
                },
            },
            "terms": terms,
            "prices": [
                {"dimension": "instance", "tier": "", "amount": usd, "unit": unit},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.azure_appservice")
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
