"""Golden-row test: fixture Cloud Billing JSON -> normalized NDJSON matches golden."""

import json
from pathlib import Path

from ingest.gcp_gce import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "gcp_gce" / "skus.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "gcp_gce_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(skus_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_compute_vm_kind():
    rows = list(ingest(skus_path=FIXTURE))
    assert rows, "fixture produced zero rows"
    assert {r["kind"] for r in rows} == {"compute.vm"}


def test_commitment_rows_filtered():
    """usageType='Commit1Yr' rows must not appear — on-demand only in m3b.3."""
    rows = list(ingest(skus_path=FIXTURE))
    out_ids = {r["sku_id"] for r in rows}
    assert "SKU-N1S2-USE1-CUD1Y" not in out_ids


def test_license_family_filtered():
    """resourceFamily='License' rows must not appear (they are surcharges, not SKUs)."""
    rows = list(ingest(skus_path=FIXTURE))
    out_ids = {r["sku_id"] for r in rows}
    assert "SKU-WIN-LIC-USE1" not in out_ids


def test_non_usd_filtered():
    """Currency guard — EUR-priced rows must not appear."""
    rows = list(ingest(skus_path=FIXTURE))
    out_ids = {r["sku_id"] for r in rows}
    assert "SKU-N1S2-EUW1-EUR" not in out_ids


def test_unknown_region_skipped(tmp_path):
    """A SKU in a region outside regions.yaml is silently dropped."""
    bad = json.loads(FIXTURE.read_text())
    for sku in bad["skus"]:
        rate = sku["pricingInfo"][0]["pricingExpression"]["tieredRates"][0]
        if (
            sku["category"]["usageType"] == "OnDemand"
            and rate["unitPrice"]["currencyCode"] == "USD"
        ):
            sku["serviceRegions"] = ["us-central1"]
            break
    p = tmp_path / "bad.json"
    p.write_text(json.dumps(bad))
    rows = list(ingest(skus_path=p))
    assert all(r["region"] != "us-central1" for r in rows)
