"""Golden-row test: fixture DynamoDB offer JSON -> normalized NDJSON matches golden."""

import json
from pathlib import Path

import pytest

from ingest.aws_dynamodb import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "aws_dynamodb" / "offer.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "aws_dynamodb_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(offer_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_db_nosql_kind():
    rows = list(ingest(offer_path=FIXTURE))
    assert rows
    assert {r["kind"] for r in rows} == {"db.nosql"}


def test_every_row_has_three_dimensions():
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        dims = {p["dimension"] for p in r["prices"]}
        assert dims == {"storage", "read_request_units", "write_request_units"}, \
            f"row {r['sku_id']} has dims {dims}"


def test_both_table_classes_present():
    rows = list(ingest(offer_path=FIXTURE))
    assert {r["resource_name"] for r in rows} == {"standard", "standard-ia"}


def test_standard_ia_storage_cheaper_than_standard_same_region():
    rows = list(ingest(offer_path=FIXTURE))
    by_key = {(r["region"], r["resource_name"]): r for r in rows}
    for region in {r["region"] for r in rows}:
        std_storage = next(p["amount"] for p in by_key[(region, "standard")]["prices"]
                           if p["dimension"] == "storage")
        ia_storage = next(p["amount"] for p in by_key[(region, "standard-ia")]["prices"]
                           if p["dimension"] == "storage")
        assert ia_storage < std_storage, \
            f"standard-ia storage {ia_storage} should be < standard {std_storage} in {region}"


def test_unknown_region_skipped(tmp_path):
    bad = json.loads(FIXTURE.read_text())
    first_sku = next(iter(bad["products"]))
    bad["products"][first_sku]["attributes"]["regionCode"] = "ap-south-9"
    p = tmp_path / "bad.json"
    p.write_text(json.dumps(bad))
    rows = list(ingest(offer_path=p))
    assert all(r["region"] != "ap-south-9" for r in rows)
