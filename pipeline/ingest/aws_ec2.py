"""Normalize AWS EC2 offer JSON into sku row dicts via DuckDB.

Spec §3 (format stack): read the offer's `products` and `terms.OnDemand`
as JSON columns and navigate them with json_extract — the nested shape
uses SKU IDs as object keys, so DuckDB infers them as STRUCTs unless we
force a JSON column type. One SQL pass joins each product with its
on-demand term + priceDimension entry; Python then filters to the
supported OS/tenancy surface and emits NDJSON rows.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path
from typing import Any, Iterable

from normalize.enums import apply_kind_defaults
from normalize.terms import terms_hash

from ._duckdb import dumps, open_conn
from .aws_common import load_region_normalizer

_PROVIDER = "aws"
_SERVICE = "ec2"
_KIND = "compute.vm"

# Keep the OS mapping narrow — only variants we ship in v1.
_OS_MAP: dict[str, str] = {"Linux": "linux", "Windows": "windows", "RHEL": "rhel"}
# Tenancy comes through as Shared / Dedicated / Host; we drop Host for m3a.1.
_TENANCY_MAP: dict[str, str] = {"Shared": "shared", "Dedicated": "dedicated"}

_SQL = """
WITH products_flat AS (
  SELECT
    p.key AS sku_id,
    json_extract_string(p.value, '$.productFamily') AS family,
    json_extract_string(p.value, '$.attributes.instanceType') AS instance_type,
    json_extract_string(p.value, '$.attributes.regionCode') AS region,
    json_extract_string(p.value, '$.attributes.operatingSystem') AS os_raw,
    json_extract_string(p.value, '$.attributes.tenancy') AS tenancy_raw,
    json_extract_string(p.value, '$.attributes.preInstalledSw') AS pre_sw,
    json_extract_string(p.value, '$.attributes.capacitystatus') AS cap,
    json_extract_string(p.value, '$.attributes.vcpu') AS vcpu,
    json_extract_string(p.value, '$.attributes.memory') AS memory,
    json_extract_string(p.value, '$.attributes.physicalProcessor') AS cpu,
    json_extract_string(p.value, '$.attributes.networkPerformance') AS net
  FROM offer, json_each(offer.products) AS p(key, value)
  WHERE json_extract_string(p.value, '$.productFamily') = 'Compute Instance'
    AND json_extract_string(p.value, '$.attributes.preInstalledSw') = 'NA'
    AND json_extract_string(p.value, '$.attributes.capacitystatus') = 'Used'
),
terms_flat AS (
  SELECT
    t.key AS sku_id,
    (json_keys(t.value))[1] AS term_key,
    t.value AS term_obj
  FROM offer, json_each(json_extract(offer.terms, '$.OnDemand')) AS t(key, value)
),
pd_keys AS (
  SELECT tf.sku_id, tf.term_key, tf.term_obj,
    (json_keys(json_extract(tf.term_obj, '$."' || tf.term_key || '".priceDimensions')))[1] AS pd_key
  FROM terms_flat tf
)
SELECT pf.sku_id, pf.instance_type, pf.region, pf.os_raw, pf.tenancy_raw, pf.vcpu, pf.memory, pf.cpu, pf.net,
  json_extract_string(pk.term_obj, '$."' || pk.term_key || '".priceDimensions."' || pk.pd_key || '".unit') AS unit,
  CAST(json_extract_string(pk.term_obj, '$."' || pk.term_key || '".priceDimensions."' || pk.pd_key || '".pricePerUnit.USD') AS DOUBLE) AS usd
FROM products_flat pf
JOIN pd_keys pk ON pf.sku_id = pk.sku_id
"""


def _parse_memory(raw: str) -> float:
    return float(raw.split()[0])


def ingest(*, offer_path: Path) -> Iterable[dict[str, Any]]:
    normalizer = load_region_normalizer()
    con = open_conn()
    path_literal = str(offer_path).replace("'", "''")
    con.execute(
        f"CREATE VIEW offer AS SELECT * FROM read_json('{path_literal}', "
        "columns={products: 'JSON', terms: 'JSON'}, maximum_object_size=134217728)"
    )
    for sku_id, instance_type, region, os_raw, tenancy_raw, vcpu_raw, memory_raw, cpu, net, unit, usd in (
        con.execute(_SQL).fetchall()
    ):
        if os_raw not in _OS_MAP or tenancy_raw not in _TENANCY_MAP:
            continue
        region_normalized = normalizer.normalize(_PROVIDER, region)
        terms = apply_kind_defaults(_KIND, {
            "commitment": "on_demand",
            "tenancy": _TENANCY_MAP[tenancy_raw],
            "os": _OS_MAP[os_raw],
            "support_tier": "",
            "upfront": "",
            "payment_option": "",
        })
        yield {
            "sku_id": sku_id,
            "provider": _PROVIDER,
            "service": _SERVICE,
            "kind": _KIND,
            "resource_name": instance_type,
            "region": region,
            "region_normalized": region_normalized,
            "terms_hash": terms_hash(terms),
            "resource_attrs": {
                "vcpu": int(vcpu_raw),
                "memory_gb": _parse_memory(memory_raw),
                "architecture": "x86_64",
                "extra": {
                    "physical_processor": cpu,
                    "network_performance": net,
                },
            },
            "terms": terms,
            "prices": [
                {"dimension": "compute", "tier": "", "amount": usd, "unit": unit.lower()},
            ],
        }


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.aws_ec2")
    ap.add_argument("--fixture", type=Path, help="path to a trimmed offer.json (tests)")
    ap.add_argument("--offer", type=Path, help="path to live offer.json")
    ap.add_argument("--out", type=Path, required=True)
    # --catalog-version is consumed by the packager; accept and ignore here.
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
