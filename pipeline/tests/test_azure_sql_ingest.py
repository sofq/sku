"""Golden-row test: fixture Azure SQL retail-prices JSON -> normalized NDJSON matches golden."""

import json
from pathlib import Path

import pytest

from ingest.azure_sql import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "azure_sql" / "prices.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "azure_sql_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(prices_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_kind_and_terms_shape():
    rows = list(ingest(prices_path=FIXTURE))
    assert rows
    for r in rows:
        assert r["kind"] == "db.relational"
        assert r["terms"]["tenancy"] == "azure-sql"
        assert r["terms"]["os"] in {"single-az", "managed-instance", "elastic-pool"}


def test_business_critical_more_expensive_than_general_purpose():
    """Sanity invariant: BC tier costs strictly more than GP at same SKU + region + deployment."""
    rows = list(ingest(prices_path=FIXTURE))
    by_key = {(r["resource_name"], r["region"], r["terms"]["os"]): r["prices"][0]["amount"]
              for r in rows}
    for (sku, region, depl), price in by_key.items():
        if sku.startswith("BC_"):
            gp_sku = "GP_" + sku[3:]
            if (gp_sku, region, depl) in by_key:
                assert price > by_key[(gp_sku, region, depl)]


def test_unit_of_measure_divisor_applied():
    """The one '100 Hours' row in the fixture must come out at the same per-hour rate as its '1 Hour' twin."""
    rows = list(ingest(prices_path=FIXTURE))
    # Group by (sku, region, deployment); we expect equal-or-very-close per-hour amounts
    # for the row priced as '100 Hours' and any sibling priced as '1 Hour'. Since the
    # fixture seeds the BC_Gen5_4 ManagedInstance westeurope row at 100x the hourly
    # baseline, post-divisor it must equal the eastus same-SKU same-deployment rate * 1.05.
    rate = {(r["resource_name"], r["region"], r["terms"]["os"]): r["prices"][0]["amount"]
            for r in rows}
    east = rate.get(("BC_Gen5_4", "eastus", "managed-instance"))
    west = rate.get(("BC_Gen5_4", "westeurope", "managed-instance"))
    if east and west:
        assert west == pytest.approx(east * 1.05, rel=1e-9)


def test_unknown_uom_rejected(tmp_path):
    """A row with a non-time unitOfMeasure must fail the build."""
    bad = json.loads(FIXTURE.read_text())
    for it in bad["Items"]:
        if it["serviceName"] == "SQL Database" and it["currencyCode"] == "USD":
            it["unitOfMeasure"] = "1 GB/Month"
            break
    p = tmp_path / "bad.json"
    p.write_text(json.dumps(bad))
    with pytest.raises(ValueError, match="GB/Month"):
        list(ingest(prices_path=p))
