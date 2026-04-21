"""Normalize GCP Cloud Billing Catalog JSON for Compute Engine into sku row dicts.

Spec §5 kind=compute.vm. The live Cloud Billing Catalog API exposes per-component
pricing — one SKU per vCPU-hour and one per GiB-hour — rather than whole-machine-
type bundles. This module composes them into per-(machine_type, region) rows by
multiplying component prices by the machine spec (vcpu count and RAM GiB).

Machine types are enumerated in _MACHINE_SPECS below; the CPU/RAM description
prefix identifies which component SKU belongs to which machine family.
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Iterable
from pathlib import Path
from typing import Any

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import dumps, open_conn
from .gcp_common import load_region_normalizer, parse_unit_price, parse_usage_unit

_PROVIDER = "gcp"
_SERVICE = "gce"
_KIND = "compute.vm"

# (vcpu, ram_gib, cpu_desc_prefix, ram_desc_prefix)
# cpu/ram_desc_prefix must match the start of the description field in the
# Cloud Billing SKU so we can identify the right component SKU.
_MACHINE_SPECS: dict[str, tuple[int, float, str, str]] = {
    "n1-standard-2": (2, 7.5,  "N1 Predefined Instance Core", "N1 Predefined Instance Ram"),
    "n1-standard-4": (4, 15.0, "N1 Predefined Instance Core", "N1 Predefined Instance Ram"),
    "e2-standard-2": (2, 8.0,  "E2 Instance Core",            "E2 Instance Ram"),
}

_SQL = """
WITH entries AS (
  SELECT UNNEST(skus, recursive := true)
  FROM read_json_auto('{path}', maximum_object_size=33554432)
)
SELECT
  skuId                                             AS sku_id,
  description                                       AS description,
  serviceDisplayName                                AS service_display_name,
  resourceFamily                                    AS resource_family,
  resourceGroup                                     AS resource_group,
  usageType                                         AS usage_type,
  serviceRegions                                    AS service_regions,
  pricingInfo[1].pricingExpression.usageUnit        AS usage_unit,
  pricingInfo[1].pricingExpression.tieredRates[1].unitPrice.currencyCode AS currency,
  pricingInfo[1].pricingExpression.tieredRates[1].unitPrice.units        AS price_units,
  pricingInfo[1].pricingExpression.tieredRates[1].unitPrice.nanos        AS price_nanos
FROM entries
"""

# Build reverse-lookup: cpu_prefix -> [machine_type], ram_prefix -> [machine_type]
_CPU_PREFIX_TO_MACHINES: dict[str, list[str]] = {}
_RAM_PREFIX_TO_MACHINES: dict[str, list[str]] = {}
for _mt, (_vcpu, _ram, _cpu_pfx, _ram_pfx) in _MACHINE_SPECS.items():
    _CPU_PREFIX_TO_MACHINES.setdefault(_cpu_pfx, []).append(_mt)
    _RAM_PREFIX_TO_MACHINES.setdefault(_ram_pfx, []).append(_mt)


def ingest(*, skus_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    con = open_conn()
    path_literal = str(skus_path).replace("'", "''")
    sql = _SQL.replace("{path}", path_literal)

    # region -> cpu_prefix -> (sku_id, price_per_vcpu_hr)
    cpu_prices: dict[str, dict[str, tuple[str, float]]] = {}
    # region -> ram_prefix -> price_per_gib_hr
    ram_prices: dict[str, dict[str, float]] = {}

    for (
        sku_id, description, svc_name, resource_family, _resource_group, usage_type,
        service_regions, usage_unit, currency, price_units, price_nanos,
    ) in con.execute(sql).fetchall():
        if svc_name != "Compute Engine":
            continue
        if resource_family != "Compute":
            continue
        if usage_type != "OnDemand":
            continue
        if currency != "USD":
            continue
        if not service_regions:
            continue
        region = service_regions[0]
        try:
            divisor, unit = parse_usage_unit(usage_unit)
        except ValueError:
            continue
        price = parse_unit_price(units=price_units, nanos=int(price_nanos)) / divisor

        # Match CPU component SKUs.
        for cpu_pfx in _CPU_PREFIX_TO_MACHINES:
            if description.startswith(cpu_pfx) and unit == "hrs":
                cpu_prices.setdefault(region, {})[cpu_pfx] = (sku_id, price)
                break
        # Match RAM component SKUs.
        for ram_pfx in _RAM_PREFIX_TO_MACHINES:
            if description.startswith(ram_pfx) and unit == "gb-hr":
                ram_prices.setdefault(region, {})[ram_pfx] = price
                break

    # Compose per-machine-type rows.
    terms = apply_kind_defaults(_KIND, {
        "commitment": "on_demand",
        "tenancy": "shared",
        "os": "linux",
        "support_tier": "",
        "upfront": "",
        "payment_option": "",
    })
    for machine_type, (vcpu, ram_gib, cpu_pfx, ram_pfx) in _MACHINE_SPECS.items():
        for region in sorted(set(cpu_prices) | set(ram_prices)):
            region_cpu = cpu_prices.get(region, {})
            region_ram = ram_prices.get(region, {})
            if cpu_pfx not in region_cpu or ram_pfx not in region_ram:
                continue
            sku_id, cpu_price = region_cpu[cpu_pfx]
            ram_price = region_ram[ram_pfx]
            region_normalized = normalizer.try_normalize(_PROVIDER, region)
            if region_normalized is None:
                continue
            amount = vcpu * cpu_price + ram_gib * ram_price
            yield {
                # Compose a unique sku_id since multiple machine types share
                # the same CPU component SKU per region.
                "sku_id": f"{sku_id}:{machine_type}",
                "provider": _PROVIDER,
                "service": _SERVICE,
                "kind": _KIND,
                "resource_name": machine_type,
                "region": region,
                "region_normalized": region_normalized,
                "terms_hash": terms_hash(terms),
                "resource_attrs": {
                    "architecture": "x86_64",
                    "extra": {
                        "vcpu": vcpu,
                        "ram_gib": ram_gib,
                        "cpu_desc_prefix": cpu_pfx,
                    },
                },
                "terms": terms,
                "prices": [
                    {"dimension": "compute", "tier": "", "amount": amount, "unit": "hrs"},
                ],
            }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.gcp_gce")
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
    print(f"ingest.gcp_gce: wrote {n} rows", file=sys.stderr)
    if n == 0:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
