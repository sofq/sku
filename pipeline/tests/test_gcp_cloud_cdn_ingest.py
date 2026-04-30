"""Tests for gcp_cloud_cdn ingest module."""

import json
from pathlib import Path

from ingest.gcp_cloud_cdn import ingest

_DATA = Path(__file__).resolve().parent.parent / "testdata"
FIXTURE = _DATA / "gcp_cloud_cdn" / "skus.json"
GOLDEN = _DATA / "golden" / "gcp_cloud_cdn_rows.jsonl"

_EGRESS_TIER_TOKENS = ["0", "10TB", "150TB"]


def _canonical(rows: list[dict]) -> list[dict]:
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(skus_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_network_cdn_kind():
    rows = list(ingest(skus_path=FIXTURE))
    assert rows
    assert {r["kind"] for r in rows} == {"network.cdn"}


def test_egress_rows_have_three_tiers():
    rows = list(ingest(skus_path=FIXTURE))
    egress_rows = [r for r in rows if r["resource_attrs"]["extra"]["mode"] == "edge-egress"]
    assert egress_rows, "expected at least one egress row"
    for row in egress_rows:
        prices = row["prices"]
        assert len(prices) == 3, f"expected 3 tiers, got {len(prices)} for {row['sku_id']}"
        tiers = [p["tier"] for p in prices]
        assert tiers == _EGRESS_TIER_TOKENS, f"tier tokens mismatch: {tiers}"
        units = {p["unit"] for p in prices}
        assert units == {"gb"}, f"unexpected units: {units}"
        dimensions = {p["dimension"] for p in prices}
        assert dimensions == {"egress"}, f"unexpected dimensions: {dimensions}"


def test_egress_tiers_contiguous():
    rows = list(ingest(skus_path=FIXTURE))
    for row in rows:
        if row["resource_attrs"]["extra"]["mode"] != "edge-egress":
            continue
        prices = sorted(row["prices"], key=lambda p: _EGRESS_TIER_TOKENS.index(p["tier"]))
        n = len(prices)
        for i, p in enumerate(prices):
            if i < n - 1:
                assert p["tier_upper"] != "", (
                    f"{row['sku_id']} tier[{i}] tier_upper must not be empty"
                )
                assert p["tier_upper"] == prices[i + 1]["tier"], (
                    f"{row['sku_id']} tier[{i}].tier_upper={p['tier_upper']!r} "
                    f"!= tier[{i+1}].tier={prices[i+1]['tier']!r}"
                )
            else:
                assert p["tier_upper"] == "", (
                    f"{row['sku_id']} last tier tier_upper must be '' (unbounded)"
                )


def test_request_row_is_global():
    rows = list(ingest(skus_path=FIXTURE))
    request_rows = [r for r in rows if r["resource_attrs"]["extra"]["mode"] == "request"]
    assert len(request_rows) == 1, f"expected exactly 1 request row, got {len(request_rows)}"
    req = request_rows[0]
    assert req["region"] == "global"
    assert req["region_normalized"] == "global"
    assert req["sku_id"] == "CLOUD-CDN-REQUESTS-GLOBAL"
    assert len(req["prices"]) == 1
    price = req["prices"][0]
    assert price["dimension"] == "request"
    assert price["unit"] == "request"
    assert price["tier"] == "0"
    assert price["tier_upper"] == ""
    assert price["amount"] > 0
