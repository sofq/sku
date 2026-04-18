"""Golden-row test: fixture EBS offer JSON -> normalized NDJSON matches golden."""

import json
from pathlib import Path

import pytest

from ingest.aws_ebs import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "aws_ebs" / "offer.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "aws_ebs_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(offer_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_storage_block_kind():
    rows = list(ingest(offer_path=FIXTURE))
    assert rows
    assert {r["kind"] for r in rows} == {"storage.block"}


def test_every_volume_type_present():
    rows = list(ingest(offer_path=FIXTURE))
    assert {r["resource_name"] for r in rows} == {"gp3", "gp2", "io2", "st1", "sc1"}


def test_each_row_has_only_storage_dim():
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        assert [p["dimension"] for p in r["prices"]] == ["storage"]


def test_unknown_region_rejected(tmp_path):
    bad = json.loads(FIXTURE.read_text())
    first_sku = next(iter(bad["products"]))
    bad["products"][first_sku]["attributes"]["regionCode"] = "ap-south-9"
    p = tmp_path / "bad.json"
    p.write_text(json.dumps(bad))
    with pytest.raises(KeyError, match="ap-south-9"):
        list(ingest(offer_path=p))
