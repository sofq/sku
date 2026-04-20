"""Golden-row test: fixture Lambda offer JSON -> normalized NDJSON matches golden."""

import json
from pathlib import Path

from ingest.aws_lambda import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "aws_lambda" / "offer.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "aws_lambda_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(offer_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_compute_function_kind():
    rows = list(ingest(offer_path=FIXTURE))
    assert rows
    assert {r["kind"] for r in rows} == {"compute.function"}


def test_each_row_has_requests_and_duration():
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        dims = {p["dimension"] for p in r["prices"]}
        assert dims == {"requests", "duration"}, f"row {r['sku_id']} has dims {dims}"


def test_arm64_price_lower_than_x86_64_same_region():
    rows = list(ingest(offer_path=FIXTURE))
    by_region_arch = {(r["region"], r["resource_name"]): r for r in rows}
    for (region, arch), row in by_region_arch.items():
        if arch == "arm64":
            x86 = by_region_arch[(region, "x86_64")]
            arm_dur = next(p["amount"] for p in row["prices"] if p["dimension"] == "duration")
            x86_dur = next(p["amount"] for p in x86["prices"] if p["dimension"] == "duration")
            assert arm_dur < x86_dur, f"arm64 duration {arm_dur} should be < x86_64 {x86_dur}"


def test_unknown_region_skipped(tmp_path):
    bad = json.loads(FIXTURE.read_text())
    first_sku = next(iter(bad["products"]))
    bad["products"][first_sku]["attributes"]["regionCode"] = "ap-south-9"
    p = tmp_path / "bad.json"
    p.write_text(json.dumps(bad))
    rows = list(ingest(offer_path=p))
    assert all(r["region"] != "ap-south-9" for r in rows)
