"""Golden-row test: fixture S3 offer JSON -> normalized NDJSON matches golden."""

import json
from pathlib import Path

import pytest

from ingest.aws_s3 import ingest
from normalize.terms import terms_hash

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "aws_s3" / "offer.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "aws_s3_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(offer_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_storage_object_kind():
    rows = list(ingest(offer_path=FIXTURE))
    assert rows, "fixture produced zero rows"
    assert {r["kind"] for r in rows} == {"storage.object"}


def test_each_row_has_storage_plus_two_request_dims():
    """Every S3 row should carry exactly three price dimensions for m3a.2 scope."""
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        dims = {p["dimension"] for p in r["prices"]}
        assert dims == {"storage", "requests-put", "requests-get"}, \
            f"row {r['sku_id']} has dims {dims}"


def test_terms_hash_uses_empty_tenancy_os():
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        assert r["terms"]["tenancy"] == ""
        assert r["terms"]["os"] == ""
        assert r["terms_hash"] == terms_hash(r["terms"])


def test_unknown_region_skipped(tmp_path):
    bad = json.loads(FIXTURE.read_text())
    first_sku = next(iter(bad["products"]))
    bad["products"][first_sku]["attributes"]["regionCode"] = "ap-south-9"
    p = tmp_path / "bad.json"
    p.write_text(json.dumps(bad))
    rows = list(ingest(offer_path=p))
    assert all(r["region"] != "ap-south-9" for r in rows)
