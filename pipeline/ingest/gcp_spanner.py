"""Normalize GCP Cloud Billing Catalog JSON for Cloud Spanner into sku row dicts.

Spec §5 kind=db.relational. Edition slot is encoded in the tenancy field
(spanner-standard / spanner-enterprise / spanner-enterprise-plus). Prices
are expressed per Processing Unit (PU) per hour; node_hour_usd = pu_hour_usd * 1000.
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

from ._duckdb import dumps
from .gcp_common import load_region_normalizer

_PROVIDER = "gcp"
_SERVICE = "spanner"
_KIND = "db.relational"

# Pattern for storage SKUs: must contain both "ssd" and "storage" (case-insensitive).
_STORAGE_RE = re.compile(r"ssd", re.IGNORECASE)
_STORAGE_WORD_RE = re.compile(r"storage", re.IGNORECASE)

# Edition detection — enterprise-plus MUST be checked before enterprise.
_EDITION_HINTS: tuple[tuple[str, str], ...] = (
    ("enterprise plus", "enterprise-plus"),
    ("enterprise",      "enterprise"),
    ("standard",        "standard"),
)


def _detect_edition(description: str) -> str | None:
    lower = description.lower()
    for needle, edition in _EDITION_HINTS:
        if needle in lower:
            return edition
    return None


def _is_storage(description: str) -> bool:
    return bool(_STORAGE_RE.search(description) and _STORAGE_WORD_RE.search(description))


def _parse_price(sku: dict) -> float:
    pricing = sku["pricingInfo"][0]["pricingExpression"]["tieredRates"][0]["unitPrice"]
    units = float(pricing["units"])
    nanos = float(pricing["nanos"]) / 1e9
    return units + nanos


def ingest(*, skus_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    with open(skus_path) as fh:
        data = json.load(fh)

    for sku in data.get("skus", []):
        description = sku.get("description", "")
        service_regions = sku.get("serviceRegions", [])
        if not service_regions:
            continue

        region = service_regions[0]
        region_normalized = normalizer.try_normalize(_PROVIDER, region)
        if region_normalized is None:
            continue

        price = _parse_price(sku)

        if _is_storage(description):
            # Storage SKU
            resource_name = "spanner-storage"
            tenancy = "spanner-storage"
            terms = apply_kind_defaults(_KIND, {
                "commitment": "on_demand",
                "tenancy": tenancy,
                "os": "",
                "support_tier": "",
                "upfront": "",
                "payment_option": "",
            })
            yield {
                "sku_id": sku["skuId"],
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": resource_name,
                "region": region,
                "region_normalized": region_normalized,
                "terms_hash": terms_hash(terms),
                "resource_attrs": {
                    "extra": {
                        "description": description,
                        "kind": "storage",
                        "ssd_gb_month_usd": price,
                    },
                },
                "terms": terms,
                "prices": [
                    {"dimension": "storage", "tier": "", "amount": price, "unit": "gb-month"},
                ],
            }
        else:
            # Compute SKU — detect edition
            edition = _detect_edition(description)
            if edition is None:
                continue
            resource_name = f"spanner-{edition}"
            tenancy = f"spanner-{edition}"
            pu_hour_usd = price
            node_hour_usd = pu_hour_usd * 1000
            terms = apply_kind_defaults(_KIND, {
                "commitment": "on_demand",
                "tenancy": tenancy,
                "os": "",
                "support_tier": "",
                "upfront": "",
                "payment_option": "",
            })
            yield {
                "sku_id": sku["skuId"],
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": resource_name,
                "region": region,
                "region_normalized": region_normalized,
                "terms_hash": terms_hash(terms),
                "resource_attrs": {
                    "extra": {
                        "description": description,
                        "edition": edition,
                        "pu_hour_usd": pu_hour_usd,
                        "node_hour_usd": node_hour_usd,
                    },
                },
                "terms": terms,
                "prices": [
                    {"dimension": "compute", "tier": "", "amount": pu_hour_usd, "unit": "pu-hour"},
                ],
            }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.gcp_spanner")
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
    print(f"ingest.gcp_spanner: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
