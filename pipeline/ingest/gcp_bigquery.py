"""Normalize GCP BigQuery Cloud Billing SKUs into warehouse.query rows.

Three pricing modes are distinguished by SKU description keywords:

  on-demand   — "Analysis" or "Flat-rate" rows with no edition keyword.
  capacity    — "Edition" rows: Enterprise / Enterprise Plus slots.
  storage     — "Storage" rows: Active / Long-term.

BigQuery exposes both Logical and Physical storage billing models with
distinct prices but the same `(resource_name, region)` shape. We ingest
Logical only — it is the default billing model and prevents duplicate rows
in `LookupWarehouseQuery`. Customers on the Physical model should price
manually until a separate resource_name (e.g. `storage-active-physical`) is
introduced.

BigQuery multi-region strings US / EU are mapped to bq-us / bq-eu.
All rows set terms.os = "on-demand" (placeholder for the mode discriminator);
the edition / storage tier lives in terms.support_tier.
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
from .gcp_common import load_region_normalizer

logger = logging.getLogger(__name__)

_PROVIDER = "gcp"
_SERVICE = "bigquery"
_KIND = "warehouse.query"

# BigQuery reports regions as either standard GCP region codes or multi-region
# shortcodes (US, EU). Map the shortcodes here; all others pass through
# load_region_normalizer() as normal GCP regions.
_MULTIREGION: dict[str, str] = {
    "us":  "bq-us",
    "eu":  "bq-eu",
    # Uppercase variants from Cloud Billing
    "US":  "bq-us",
    "EU":  "bq-eu",
}


def _hourly_usd(sku: dict) -> tuple[float, str]:
    """Return (amount, unit) from the first pricing tier of a SKU."""
    try:
        tiers = sku["pricingInfo"][0]["pricingExpression"]["tieredRates"]
        units = float(tiers[0]["unitPrice"]["units"])
        nanos = float(tiers[0]["unitPrice"]["nanos"]) / 1e9
        usage_unit = sku["pricingInfo"][0]["pricingExpression"]["usageUnit"]
        return units + nanos, usage_unit
    except (KeyError, IndexError):
        return 0.0, ""


def _parse_regions(sku: dict) -> list[str]:
    """Extract region or multi-region codes from a SKU's serviceRegions."""
    regions = sku.get("serviceRegions", [])
    if not regions:
        # Some BigQuery SKUs have no serviceRegions — treat as global.
        return []
    out = []
    for r in regions:
        r_lower = r.lower()
        mapped = _MULTIREGION.get(r) or _MULTIREGION.get(r_lower)
        if mapped:
            out.append(mapped)
        else:
            out.append(r)
    return out


def _classify(description: str) -> tuple[str, str, str] | None:
    """Return (resource_name, support_tier, dimension) or None to skip.

    resource_name maps 1:1 to the BigQuery edition name so the CLI's
    `--mode <name>` matches the ingested rows directly:

      on-demand                → on-demand analysis (per-TB)
      capacity-standard        → Standard edition slot commitment
      capacity-enterprise      → Enterprise edition slot commitment
      capacity-enterprise-plus → Enterprise Plus edition slot commitment
      storage-active           → active storage (per GB-month)
      storage-long-term        → long-term (cold) storage (per GB-month)
    """
    desc = description.lower()

    # Skip batch + BI Engine + legacy rows
    if any(kw in desc for kw in ("batch", "bi engine", "legacy", "flat rate", "streaming", "dml")):
        return None

    # Skip Physical-storage billing rows; Logical is the default model and
    # the two share the same (resource_name, region) shape, which would
    # otherwise produce duplicate rows in LookupWarehouseQuery.
    if "physical" in desc and "storage" in desc:
        return None

    if "enterprise plus" in desc or "enterprise_plus" in desc:
        if "slot" in desc:
            return "capacity-enterprise-plus", "enterprise-plus", "slot"
        return None

    if "enterprise" in desc:
        if "slot" in desc:
            return "capacity-enterprise", "enterprise", "slot"
        return None

    if "standard" in desc and "edition" in desc:
        if "slot" in desc:
            return "capacity-standard", "standard", "slot"
        return None

    if "long-term" in desc or "long term" in desc:
        if "storage" in desc:
            return "storage-long-term", "storage-long-term", "storage"
        return None

    if "active" in desc:
        if "storage" in desc:
            return "storage-active", "storage-active", "storage"
        return None

    if "analysis" in desc or "on-demand" in desc:
        return "on-demand", "", "query"

    return None


def ingest(*, skus_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    tracked_regions = {
        region: group
        for (provider, region), group in normalizer.table.items()
        if provider == _PROVIDER
    }

    with skus_path.open() as f:
        skus = json.load(f).get("skus", [])

    for sku in skus:
        description = sku.get("description", "")
        sku_id = sku.get("skuId", "")
        if not sku_id:
            continue

        classification = _classify(description)
        if classification is None:
            continue
        resource_name, support_tier, dimension = classification

        amount, usage_unit = _hourly_usd(sku)
        if amount <= 0:
            continue

        # Canonicalize unit
        if "TiBy" in usage_unit or "tebibyte" in usage_unit.lower():
            unit = "tb"
        elif "GiBy" in usage_unit or "mo" in usage_unit.lower():
            unit = "gb-month"
        elif dimension == "slot":
            unit = "slot-hour"
        else:
            unit = "hour"

        # Determine mode for extra.mode field
        if resource_name.startswith("capacity"):
            mode = "capacity"
            edition = resource_name.split("-", 1)[1] if "-" in resource_name else ""
            extra: dict[str, Any] = {"mode": mode, "edition": edition}
        elif resource_name.startswith("storage"):
            mode = "storage"
            storage_tier = "long-term" if "long" in resource_name else "active"
            extra = {"mode": mode, "storage_tier": storage_tier}
        else:
            mode = "on-demand"
            extra = {"mode": mode}

        regions = _parse_regions(sku)
        if not regions:
            # Global SKU — emit for known BigQuery regions
            regions = ["bq-us", "bq-eu"]

        for region in regions:
            # Map bq-us / bq-eu through normalizer (self-referential after regions.yaml update)
            if region in ("bq-us", "bq-eu"):
                region_normalized = region
            else:
                region_normalized = tracked_regions.get(region)
                if region_normalized is None:
                    continue

            terms = apply_kind_defaults(_KIND, {
                "commitment": "on_demand",
                "tenancy": "shared",
                "os": "on-demand",
                "support_tier": support_tier,
                "upfront": "",
                "payment_option": "",
            })
            row_sku_id = f"{sku_id}-{region}"
            yield {
                "sku_id": row_sku_id,
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
                    "extra": extra,
                },
                "terms": terms,
                "prices": [
                    {"dimension": dimension, "tier": "", "amount": amount, "unit": unit},
                ],
            }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.gcp_bigquery")
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
    with args.out.open("w") as fh:
        for row in ingest(skus_path=skus_path):
            fh.write(dumps(row) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
