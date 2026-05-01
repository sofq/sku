"""Normalize AWS API Gateway offer JSON into sku row dicts.

Spec §5 api.gateway kind. API Gateway rows carry tiered request pricing:
- REST API: 4 tiers (333M / 1B / 20B / Inf)
- HTTP API: 2 tiers (300M / Inf)

One row per (api_type, region). WebSocket API calls are out of scope.

The upstream offer uses the `operation` attribute to distinguish API types:
- REST:      operation = "ApiGatewayRequest"
- HTTP:      operation = "ApiGatewayHttpApi"
- WebSocket: operation = "ApiGatewayWebSocket" — skipped

The `location` attribute holds display names like "US East (N. Virginia)";
we map these to canonical region codes via _LOCATION_MAP.

Each SKU has a single OnDemand term with multiple priceDimensions (one per
tier). We iterate all priceDimensions and sort by beginRange to produce
ordered tier entries.
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
from normalize.tier_tokens import TIER_TOKENS_COUNT

from ._duckdb import dumps
from .aws_common import AWS_LOCATION_TO_REGION, load_region_normalizer


def _canonicalize_count_tier(numeric: str) -> str:
    """Convert a raw numeric AWS tier boundary (e.g. "333000000") to a
    canonical tier token (e.g. "333M"). Returns the input unchanged for
    "0" or "Inf". Raises ValueError if the resulting token is not in the
    shared TIER_TOKENS_COUNT vocabulary — this surfaces unknown breakpoints
    instead of letting them silently bypass the canonical set."""
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
        token = numeric  # fall back; will fail vocabulary check below
    if token not in TIER_TOKENS_COUNT:
        raise ValueError(
            f"aws_api_gateway: tier {numeric!r} -> {token!r} not in TIER_TOKENS_COUNT; "
            f"add it to pipeline/normalize/tier_tokens.py and rerun "
            f"`make generate-go-tier-tokens`"
        )
    return token

_PROVIDER = "aws"
_SERVICE = "api-gateway"
_KIND = "api.gateway"

# AWS API Gateway uses the same display-location convention as other AWS
# services. The shared map in aws_common handles both "EU (...)" and
# "Europe (...)" spellings; unknown locations are skipped with a stderr
# warning so the daily ingest doesn't hard-fail when AWS adds a region.
_LOCATION_MAP = AWS_LOCATION_TO_REGION

# Map operation attribute → (resource_name, os token, mode)
# os token is "" — resource_name alone distinguishes rest vs http within
# the shard, and the Go CLI's lookup uses Terms{Commitment:"on_demand"}.
_OPERATION_MAP: dict[str, tuple[str, str, str]] = {
    "ApiGatewayRequest": ("rest", "", "rest"),
    "ApiGatewayHttpApi": ("http", "", "http"),
}


def ingest(*, offer_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with offer_path.open() as f:
        offer = json.load(f)
    products = offer.get("products", {})
    terms_od = offer.get("terms", {}).get("OnDemand", {})

    # Group tiers: key = (resource_name, region), value = list of tier dicts
    grouped: dict[tuple[str, str], dict[str, Any]] = {}

    for sku_id, product in products.items():
        if product.get("productFamily") != "API Calls":
            continue
        attrs = product.get("attributes", {})
        operation = attrs.get("operation", "")
        op_info = _OPERATION_MAP.get(operation)
        if op_info is None:
            # WebSocket or unknown — skip
            continue

        resource_name, os_token, mode = op_info
        location = attrs.get("location", "")
        region = _LOCATION_MAP.get(location)
        if region is None:
            print(
                f"warn: ingest.aws_api_gateway: unknown location {location!r}, skipping",
                file=sys.stderr,
            )
            continue
        if normalizer.try_normalize(_PROVIDER, region) is None:
            continue

        # Each SKU has a single term with multiple priceDimensions (one per tier).
        term_data = terms_od.get(sku_id) or {}
        term = next(iter(term_data.values()), None)
        if term is None:
            continue
        pds = term.get("priceDimensions", {})
        if not pds:
            continue

        # Collect all tier entries for this SKU
        tiers = []
        for pd in pds.values():
            begin_range = pd.get("beginRange", "0")
            end_range = pd.get("endRange", "Inf")
            unit = pd.get("unit", "Requests")
            usd = float(pd.get("pricePerUnit", {}).get("USD", "0"))
            tiers.append({
                "begin_range": begin_range,
                "end_range": end_range,
                "unit": unit,
                "usd": usd,
            })

        key = (resource_name, region)
        grouped[key] = {
            "sku_id": sku_id,
            "os_token": os_token,
            "mode": mode,
            "tiers": tiers,
        }

    for (resource_name, region), entry in sorted(grouped.items()):
        region_normalized = normalizer.normalize(_PROVIDER, region)
        os_token = entry["os_token"]
        mode = entry["mode"]

        # Sort tiers by beginRange numerically
        tiers = sorted(entry["tiers"], key=lambda t: int(t["begin_range"]))

        # Build price list with canonical tier tokens.
        prices = []
        for i, tier in enumerate(tiers):
            begin_tok = _canonicalize_count_tier(tier["begin_range"])
            end_tok = _canonicalize_count_tier(tier["end_range"])
            prices.append({
                "dimension": "request",
                "tier": begin_tok,
                "tier_upper": end_tok,
                "amount": tier["usd"],
                "unit": "request",
            })

        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": "",
            "os": os_token,
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": entry["sku_id"],
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": resource_name,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "extra": {"mode": mode},
            },
            "terms": terms,
            "prices": prices,
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_api_gateway")
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
    print(f"ingest.aws_api_gateway: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
