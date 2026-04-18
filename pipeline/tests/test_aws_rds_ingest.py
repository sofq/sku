"""Golden-row test: fixture RDS offer JSON -> normalized NDJSON matches golden."""

import json
from pathlib import Path

import pytest

from ingest.aws_rds import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "aws_rds" / "offer.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "aws_rds_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(offer_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_engine_and_deployment_encoded_in_terms():
    """RDS reuses terms.tenancy for engine and terms.os for deployment-option."""
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        assert r["kind"] == "db.relational"
        assert r["terms"]["tenancy"] in {"postgres", "mysql", "mariadb"}
        assert r["terms"]["os"] in {"single-az", "multi-az"}


def test_multi_az_price_doubles_single_az():
    """Same instance + engine + region: multi-az amount == 2 * single-az amount in the fixture."""
    rows = list(ingest(offer_path=FIXTURE))
    by_key = {
        (r["resource_name"], r["region"], r["terms"]["tenancy"], r["terms"]["os"]): r
        for r in rows
    }
    for (name, region, engine, depl), row in by_key.items():
        if depl == "multi-az":
            single = by_key[(name, region, engine, "single-az")]
            assert row["prices"][0]["amount"] == pytest.approx(
                single["prices"][0]["amount"] * 2, rel=1e-9,
            )
