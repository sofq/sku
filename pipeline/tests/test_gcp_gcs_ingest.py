"""Golden-row and invariant tests for the GCS ingester."""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from ingest.gcp_gcs import ingest


FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "gcp_gcs" / "skus.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "gcp_gcs_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: (r["resource_name"], r["region"]))


def test_fixture_matches_golden():
    rows = list(ingest(skus_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_storage_object_kind():
    rows = list(ingest(skus_path=FIXTURE))
    assert rows, "fixture produced zero rows"
    assert {r["kind"] for r in rows} == {"storage.object"}


def test_storage_class_values_are_canonical():
    allowed = {"standard", "nearline", "coldline", "archive"}
    for row in ingest(skus_path=FIXTURE):
        assert row["resource_name"] in allowed, row["resource_name"]


def test_multi_region_rows_filtered():
    for row in ingest(skus_path=FIXTURE):
        assert row["region"] in {"us-east1", "europe-west1"}, row["region"]


def test_non_usd_rows_filtered():
    ids = {r["sku_id"] for r in ingest(skus_path=FIXTURE)}
    assert "STD-USEAST1-STORAGE-EUR" not in ids


def test_commitment_rows_filtered():
    for row in ingest(skus_path=FIXTURE):
        assert row["terms"]["commitment"] == "on_demand", row


def test_three_dimensions_per_row():
    expected = ["read-ops", "storage", "write-ops"]
    for row in ingest(skus_path=FIXTURE):
        assert sorted(p["dimension"] for p in row["prices"]) == expected


def test_unknown_region_rejected(tmp_path):
    bad = json.loads(FIXTURE.read_text())
    # Point the standard-storage / read-ops / write-ops meters at an unknown region.
    for sku in bad["skus"]:
        if sku["category"]["resourceGroup"] == "StandardStorage" and sku["category"]["usageType"] == "OnDemand":
            if sku["pricingInfo"][0]["pricingExpression"]["tieredRates"][0]["unitPrice"]["currencyCode"] == "USD":
                sku["serviceRegions"] = ["mars-1"]
    p = tmp_path / "bad.json"
    p.write_text(json.dumps(bad))
    with pytest.raises(KeyError, match="gcp/mars-1"):
        list(ingest(skus_path=p))
