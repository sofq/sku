"""Normalize AWS SNS offer JSON into sku row dicts.

Spec §5 messaging.topic kind. SNS rows are keyed by region; resource_name is
always "standard" (SNS has one tier of topic). We ingest from the
productFamily='API Request' family to capture per-publish-request pricing.

Each region emits two price tier entries:
- tier "0": first 1 million requests free (amount = 0.0, tier_upper = "1M")
- tier "1M": paid per-request (amount = price_per_million / 1_000_000, tier_upper = "")

The Message Delivery family (per-endpoint-type) is out of scope for this shard.
"""

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
from normalize.tier_tokens import TIER_TOKENS_COUNT

from ._duckdb import dumps
from .aws_common import AWS_LOCATION_TO_REGION, load_region_normalizer


def _canonicalize_count_tier(numeric: str) -> str:
    """Convert a raw numeric AWS tier boundary (e.g. "1000000") to a canonical
    tier token from TIER_TOKENS_COUNT (e.g. "1M"). Returns "" for empty/"Inf".
    Raises ValueError if the resulting token is not in the vocabulary."""
    if numeric in ("0", "", "Inf"):
        return numeric if numeric != "Inf" else ""
    n = int(numeric)
    if n >= 1_000_000_000 and n % 1_000_000_000 == 0:
        token = f"{n // 1_000_000_000}B"
    elif n >= 1_000_000 and n % 1_000_000 == 0:
        token = f"{n // 1_000_000}M"
    elif n >= 1_000 and n % 1_000 == 0:
        token = f"{n // 1_000}K"
    else:
        token = numeric
    if token not in TIER_TOKENS_COUNT:
        raise ValueError(
            f"aws_sns: tier {numeric!r} -> {token!r} not in TIER_TOKENS_COUNT; "
            f"add it to pipeline/normalize/tier_tokens.py"
        )
    return token

# AWS SNS pricing-API descriptions consistently include the per-N-million rate,
# e.g. "$0.50 per 1 million SNS Requests after first 1 million". Match a
# leading "$<n> per <m> million" pattern so we can decide whether pricePerUnit
# is already per-request (matches the stated rate at scale) or per-million
# (fixture-style, where pricePerUnit.USD == "0.50").
_PER_MILLION_RE = re.compile(
    r"\$([0-9]*\.?[0-9]+)\s+per\s+([0-9]+)\s+million",
    re.IGNORECASE,
)


def _detect_per_million_divisor(description: str, price_per_unit_usd: float) -> float:
    """Return 1_000_000 if description says "$X per N million" AND
    price_per_unit_usd matches X (i.e. the pricePerUnit was stored at the
    advertised per-N-million rate, not already converted to per-request).
    Returns 1.0 otherwise."""
    if not description:
        return 1.0
    m = _PER_MILLION_RE.search(description)
    if not m:
        return 1.0
    advertised_rate = float(m.group(1)) / float(m.group(2))
    # Tolerance to account for floating-point representation.
    if abs(price_per_unit_usd - advertised_rate) <= advertised_rate * 1e-6:
        return 1_000_000.0
    return 1.0

_PROVIDER = "aws"
_SERVICE = "sns"
_KIND = "messaging.topic"

# AWS SNS offer uses a human-readable "location" attribute (not "regionCode").
# The shared map in aws_common handles both "EU (...)" and "Europe (...)"
# spellings; unknown locations are skipped silently.
_LOCATION_TO_REGION = AWS_LOCATION_TO_REGION


def ingest(*, offer_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with offer_path.open() as f:
        offer = json.load(f)
    products = offer.get("products", {})
    terms_od = offer.get("terms", {}).get("OnDemand", {})

    # Two passes: collect free tier and paid tier SKUs per region, then pair them.
    # free_tier[region] = {sku_id, begin_range="0", end_range}
    # paid_tier[region] = {sku_id, begin_range, end_range, usd_per_million}
    free_tiers: dict[str, dict[str, Any]] = {}
    paid_tiers: dict[str, dict[str, Any]] = {}

    for sku_id, product in products.items():
        if product.get("productFamily") != "API Request":
            continue
        attrs = product.get("attributes", {})
        # SNS uses a human-readable location string.
        location_raw = attrs.get("location", "")
        region = _LOCATION_TO_REGION.get(location_raw)
        if region is None:
            continue
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue

        term_data = terms_od.get(sku_id)
        if not term_data:
            continue
        term = next(iter(term_data.values()), None)
        if not term:
            continue

        # Iterate all priceDimensions — there may be one per tier per SKU,
        # or both tiers inside the same term. SNS fixture has one dimension
        # per SKU; the real upstream may vary.
        for pd in term.get("priceDimensions", {}).values():
            usd = float(pd.get("pricePerUnit", {}).get("USD", "0"))
            unit = pd.get("unit", "Requests")
            begin_raw = pd.get("beginRange", "0")
            end_raw = pd.get("endRange", "Inf")

            entry = {
                "sku_id": sku_id,
                "region": region,
                "region_normalized": region_normalized,
                "usd": usd,
                "unit": unit,
                "begin_range": begin_raw,
                "end_range": end_raw,
            }

            if begin_raw == "0":
                # Free tier (first 1M).
                free_tiers[region] = entry
            else:
                # Paid tier (1M+). Use the priceDimension `description` to detect
                # whether pricePerUnit is per-request (live AWS offer files) or
                # per-million (some test fixtures). AWS descriptions consistently
                # spell out the rate, e.g. "$0.50 per 1 million SNS Requests".
                description = pd.get("description", "")
                divisor = _detect_per_million_divisor(description, usd)
                if divisor != 1.0:
                    usd = usd / divisor
                paid_tiers[region] = {**entry, "usd": usd}

    for region, free in sorted(free_tiers.items()):
        paid = paid_tiers.get(region)
        if paid is None:
            print(f"warn: aws_sns: no paid tier for {region}, skipping", file=sys.stderr)
            continue

        region_normalized = free["region_normalized"]
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "",
            "os": "",
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        sku_id = f"{free['sku_id']}::{paid['sku_id']}"
        tier_upper_free = _canonicalize_count_tier(free["end_range"])
        paid_tier_lower = _canonicalize_count_tier(paid["begin_range"])
        yield {
            "sku_id": sku_id,
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": "standard",
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "extra": {"mode": "publish"},
            },
            "terms": terms,
            "prices": [
                {
                    "dimension": "request",
                    "tier": "0",
                    "tier_upper": tier_upper_free,
                    "amount": 0.0,
                    "unit": free["unit"].lower(),
                },
                {
                    "dimension": "request",
                    "tier": paid_tier_lower,
                    "tier_upper": "",
                    "amount": paid["usd"],
                    "unit": paid["unit"].lower(),
                },
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_sns")
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
    n = 0
    with args.out.open("w") as fh:
        for row in ingest(offer_path=offer_path):
            fh.write(dumps(row) + "\n")
            n += 1
    print(f"ingest.aws_sns: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
