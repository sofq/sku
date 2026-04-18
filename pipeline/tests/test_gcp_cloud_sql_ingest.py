"""Golden-row test + invariants for GCP Cloud SQL ingest."""

import json
from pathlib import Path

import pytest

from ingest.gcp_cloud_sql import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "gcp_cloud_sql" / "skus.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "gcp_cloud_sql_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(skus_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_db_relational_kind():
    rows = list(ingest(skus_path=FIXTURE))
    assert rows, "fixture produced zero rows"
    assert {r["kind"] for r in rows} == {"db.relational"}


def test_tenancy_matches_engine():
    """tenancy slot encodes the engine, like azure-sql / aws-rds do."""
    rows = list(ingest(skus_path=FIXTURE))
    engines = {r["terms"]["tenancy"] for r in rows}
    assert engines == {"cloud-sql-postgres", "cloud-sql-mysql"}


def test_os_slot_encodes_deployment():
    """os slot encodes zonal vs regional — the spec §5 deployment-option pattern."""
    rows = list(ingest(skus_path=FIXTURE))
    deployments = {r["terms"]["os"] for r in rows}
    assert deployments == {"zonal", "regional"}


def test_regional_costs_more_than_zonal_same_tier_region():
    """Regional HA should always be pricier than zonal for the same tier+region+engine."""
    rows = list(ingest(skus_path=FIXTURE))
    zonal = {
        (r["resource_name"], r["region"], r["terms"]["tenancy"]): r["prices"][0]["amount"]
        for r in rows if r["terms"]["os"] == "zonal"
    }
    regional = {
        (r["resource_name"], r["region"], r["terms"]["tenancy"]): r["prices"][0]["amount"]
        for r in rows if r["terms"]["os"] == "regional"
    }
    for key, z_price in zonal.items():
        assert regional[key] > z_price, f"regional must be > zonal for {key}"


def test_storage_family_filtered():
    rows = list(ingest(skus_path=FIXTURE))
    assert "SQL-STORAGE-SSD-USE1" not in {r["sku_id"] for r in rows}


def test_non_usd_filtered():
    rows = list(ingest(skus_path=FIXTURE))
    assert "SQL-PG-C2-EUW1-EUR" not in {r["sku_id"] for r in rows}
