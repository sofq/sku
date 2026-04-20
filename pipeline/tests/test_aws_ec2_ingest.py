"""Golden-row test: fixture EC2 offer JSON -> normalized NDJSON matches golden."""

import json
from pathlib import Path

from ingest.aws_ec2 import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "aws_ec2" / "offer.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "aws_ec2_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(offer_paths=[FIXTURE]))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_compute_vm_kind():
    rows = list(ingest(offer_paths=[FIXTURE]))
    assert rows, "fixture produced zero rows"
    assert {r["kind"] for r in rows} == {"compute.vm"}


def test_terms_hash_matches_terms_content():
    """For every row, recompute terms_hash from the row's terms and assert equality."""
    from normalize.terms import terms_hash
    rows = list(ingest(offer_paths=[FIXTURE]))
    for r in rows:
        assert r["terms_hash"] == terms_hash(r["terms"])


def test_unknown_region_skipped(tmp_path):
    """A product in a region outside regions.yaml is silently dropped."""
    bad = json.loads(FIXTURE.read_text())
    first_sku = next(iter(bad["products"]))
    bad["products"][first_sku]["attributes"]["regionCode"] = "ap-south-9"
    p = tmp_path / "bad.json"
    p.write_text(json.dumps(bad))
    rows = list(ingest(offer_paths=[p]))
    assert all(r["region"] != "ap-south-9" for r in rows)
