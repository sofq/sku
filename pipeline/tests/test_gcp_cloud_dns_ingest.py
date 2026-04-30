"""Golden-row test: fixture Cloud DNS skus.json -> normalized NDJSON matches golden."""

import json
from pathlib import Path

import pytest

from ingest.gcp_cloud_dns import ingest
from ._tier_contiguity import assert_tiers_contiguous

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "gcp_cloud_dns" / "skus.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "gcp_cloud_dns_rows.jsonl"


def _rows() -> list[dict]:
    return list(ingest(skus_path=FIXTURE))


def test_fixture_matches_golden():
    rows = _rows()
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert rows == expected


def test_single_global_row():
    """Cloud DNS is globally priced — expect exactly one row with region='global'."""
    rows = _rows()
    assert len(rows) == 1
    row = rows[0]
    assert row["region"] == "global"
    assert row["region_normalized"] == "global"
    assert row["resource_name"] == "public"


def test_kind_is_dns_zone():
    rows = _rows()
    assert rows, "expected at least one row"
    assert all(r["kind"] == "dns.zone" for r in rows)


def test_has_zone_and_query_dimensions():
    """The single row must carry both hosted_zone and query dimensions."""
    rows = _rows()
    assert rows
    dims = {p["dimension"] for p in rows[0]["prices"]}
    assert "hosted_zone" in dims, "missing hosted_zone dimension"
    assert "query" in dims, "missing query dimension"


def test_zone_tiers_contiguous():
    """hosted_zone tiers must be contiguous (0 -> 25 -> 10K -> unbounded)."""
    rows = _rows()
    zone_prices = [p for p in rows[0]["prices"] if p["dimension"] == "hosted_zone"]
    assert len(zone_prices) == 3
    assert zone_prices[0] == {"dimension": "hosted_zone", "tier": "0", "tier_upper": "25", "amount": pytest.approx(0.20), "unit": "mo"}
    assert zone_prices[1] == {"dimension": "hosted_zone", "tier": "25", "tier_upper": "10K", "amount": pytest.approx(0.10), "unit": "mo"}
    assert zone_prices[2] == {"dimension": "hosted_zone", "tier": "10K", "tier_upper": "", "amount": pytest.approx(0.03), "unit": "mo"}
    assert_tiers_contiguous(rows, "dns.zone", "count")


def test_query_tiers_contiguous():
    """query tiers must be contiguous (0 -> 1B -> unbounded)."""
    rows = _rows()
    query_prices = [p for p in rows[0]["prices"] if p["dimension"] == "query"]
    assert len(query_prices) == 2
    assert query_prices[0] == {"dimension": "query", "tier": "0", "tier_upper": "1B", "amount": pytest.approx(4e-7), "unit": "request"}
    assert query_prices[1] == {"dimension": "query", "tier": "1B", "tier_upper": "", "amount": pytest.approx(2e-7), "unit": "request"}
    assert_tiers_contiguous(rows, "dns.zone", "count")
