"""Normalize GCP Cloud Pub/Sub SKU JSON into messaging.topic rows.

Spec §5 messaging.topic kind. Pub/Sub delivery pricing is global-only.
One row is emitted per "Message Delivery Basic" SKU representing the
paid throughput tier (the free first 10 GiB is not surfaced separately).

Price conversion: the API returns a per-TiB rate; we convert to per-GiB
by dividing by 1024 (1 TiB = 1024 GiB).

GCP Pub/Sub topics and queues share the same underlying pricing;
both filter the same "Message Delivery Basic" SKU from the Cloud Billing API.
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
from .gcp_common import parse_unit_price

_PROVIDER = "gcp"
_SERVICE = "pubsub-topics"
_KIND = "messaging.topic"
_REGION = "global"
_RESOURCE_NAME = "throughput"


def _is_basic_delivery(sku: dict) -> bool:
    """Return True for the 'Message Delivery Basic' SKU we want to ingest."""
    category = sku.get("category", {})
    if category.get("resourceGroup") != "Message":
        return False
    description = sku.get("description", "")
    if "Message Delivery Basic" not in description:
        return False
    return True


def _paid_tier_price_per_tib(tiered_rates: list[dict]) -> float | None:
    """Return the USD price per TiB from the highest-startUsageAmount tier.

    The free tier has startUsageAmount=0 and unitPrice units="0", nanos=0.
    The paid tier has startUsageAmount>0. We sort by startUsageAmount and
    take the last entry (highest tier = paid rate).
    """
    if not tiered_rates:
        return None
    sorted_rates = sorted(tiered_rates, key=lambda r: float(r.get("startUsageAmount", 0)))
    last = sorted_rates[-1]
    unit_price = last.get("unitPrice", {})
    usd = parse_unit_price(
        units=unit_price.get("units", "0"),
        nanos=int(unit_price.get("nanos", 0)),
    )
    return usd


def ingest(*, skus_path: Path) -> Iterable[dict[str, Any]]:
    with skus_path.open() as f:
        skus = json.load(f).get("skus", [])

    for sku in skus:
        # Filter: only OnDemand usage type
        usage_type = sku.get("category", {}).get("usageType", "OnDemand")
        if usage_type != "OnDemand":
            continue

        # Currency guard
        try:
            currency = sku["pricingInfo"][0]["pricingExpression"]["tieredRates"][0][
                "unitPrice"
            ].get("currencyCode", "USD")
        except (KeyError, IndexError):
            continue
        if currency != "USD":
            continue

        if not _is_basic_delivery(sku):
            continue

        # Extract tiered rates
        try:
            tiered_rates = sku["pricingInfo"][0]["pricingExpression"]["tieredRates"]
        except (KeyError, IndexError):
            continue

        price_per_tib = _paid_tier_price_per_tib(tiered_rates)
        if price_per_tib is None or price_per_tib <= 0:
            continue

        # Convert per-TiB to per-GiB
        price_per_gib = price_per_tib / 1024.0

        terms = apply_kind_defaults(
            _KIND,
            {
                "commitment": "on_demand",
                "tenancy": "",
                "os": "",
                "support_tier": "",
                "upfront": "",
                "payment_option": "",
            },
        )

        yield {
            "sku_id": "PUBSUB-BASIC-GLOBAL",
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": _RESOURCE_NAME,
            "region": _REGION,
            "region_normalized": _REGION,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "extra": {"mode": "throughput"},
            },
            "terms": terms,
            "prices": [
                {
                    "dimension": "throughput",
                    "tier": "0",
                    "tier_upper": "",
                    "amount": price_per_gib,
                    "unit": "gb-mo",
                }
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.gcp_pubsub_topics")
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
    print(f"ingest.gcp_pubsub_topics: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
