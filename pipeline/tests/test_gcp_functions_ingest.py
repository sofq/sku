"""Golden-row and invariant tests for the Cloud Functions ingester."""

from __future__ import annotations

import json
from pathlib import Path

from ingest.gcp_functions import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "gcp_functions" / "skus.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "gcp_functions_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["region"])


def test_fixture_matches_golden():
    rows = list(ingest(skus_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_compute_function_kind():
    rows = list(ingest(skus_path=FIXTURE))
    assert rows, "fixture produced zero rows"
    assert {r["kind"] for r in rows} == {"compute.function"}


def test_gen1_rows_filtered():
    ids = {r["sku_id"] for r in ingest(skus_path=FIXTURE)}
    assert "FN-USEAST1-CPU-GEN1" not in ids


def test_non_usd_rows_filtered():
    ids = {r["sku_id"] for r in ingest(skus_path=FIXTURE)}
    assert "FN-USEAST1-CPU-EUR" not in ids


def test_commitment_rows_filtered():
    for row in ingest(skus_path=FIXTURE):
        assert row["terms"]["commitment"] == "on_demand"


def test_three_dimensions_per_row():
    expected = ["cpu-second", "memory-gb-second", "requests"]
    for row in ingest(skus_path=FIXTURE):
        assert sorted(p["dimension"] for p in row["prices"]) == expected


def test_architecture_attr():
    for row in ingest(skus_path=FIXTURE):
        assert row["resource_attrs"]["architecture"] == "x86_64"
        assert row["resource_name"] == "x86_64"


def test_zero_priced_requests_dimension_dropped(tmp_path):
    """A free-tier Invocations SKU (units=0, nanos=0) must not surface as a
    $0 dimension. Rows still emit with cpu + memory dimensions only."""
    bad = json.loads(FIXTURE.read_text())
    zeroed_any = False
    for sku in bad["skus"]:
        desc = sku.get("description", "")
        cat = sku.get("category") or {}
        if (
            cat.get("resourceGroup") == "Compute"
            and (sku["pricingInfo"][0]["pricingExpression"]["usageUnit"] == "count"
                 or "invocation" in desc.lower())
        ):
            rate = sku["pricingInfo"][0]["pricingExpression"]["tieredRates"][0]
            rate["unitPrice"]["units"] = "0"
            rate["unitPrice"]["nanos"] = 0
            zeroed_any = True
    assert zeroed_any, "fixture should have at least one Invocations SKU"
    p = tmp_path / "bad.json"
    p.write_text(json.dumps(bad))
    rows = list(ingest(skus_path=p))
    assert rows, "rows should still emit without a Requests dimension"
    for row in rows:
        dims = [pr["dimension"] for pr in row["prices"]]
        assert "requests" not in dims, row


def test_unknown_region_skipped(tmp_path):
    bad = json.loads(FIXTURE.read_text())
    for sku in bad["skus"]:
        cat = sku["category"]
        rate = sku["pricingInfo"][0]["pricingExpression"]["tieredRates"][0]
        if (
            cat["resourceGroup"] == "Compute"
            and cat["usageType"] == "OnDemand"
            and rate["unitPrice"]["currencyCode"] == "USD"
            and sku.get("serviceRegions", ["global"]) != ["global"]
        ):
            sku["serviceRegions"] = ["mars-1"]
    p = tmp_path / "bad.json"
    p.write_text(json.dumps(bad))
    rows = list(ingest(skus_path=p))
    assert all(r["region"] != "mars-1" for r in rows)
