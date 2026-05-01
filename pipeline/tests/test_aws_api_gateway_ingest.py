"""Tests for AWS API Gateway ingest module."""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from ingest.aws_api_gateway import ingest
from normalize.tier_tokens import parse_count_tier

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "aws_api_gateway" / "offer.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "aws_api_gateway_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(offer_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_api_gateway_kind():
    rows = list(ingest(offer_path=FIXTURE))
    assert rows, "fixture produced zero rows"
    assert {r["kind"] for r in rows} == {"api.gateway"}


def test_resource_names_are_rest_and_http():
    rows = list(ingest(offer_path=FIXTURE))
    resource_names = {r["resource_name"] for r in rows}
    assert resource_names == {"rest", "http"}


def test_every_row_has_request_dimension():
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        dims = {p["dimension"] for p in r["prices"]}
        assert "request" in dims, f"row {r['sku_id']} missing 'request' dimension, has {dims}"
        assert dims == {"request"}, f"row {r['sku_id']} has unexpected dimensions: {dims}"


def test_regions_covered():
    rows = list(ingest(offer_path=FIXTURE))
    regions = {r["region"] for r in rows}
    assert "us-east-1" in regions, "us-east-1 not found in regions"
    assert "eu-west-1" in regions, "eu-west-1 not found in regions"


def test_rest_has_4_tiers():
    rows = list(ingest(offer_path=FIXTURE))
    rest_rows = [r for r in rows if r["resource_name"] == "rest"]
    assert rest_rows, "no REST rows found"
    for r in rest_rows:
        assert len(r["prices"]) == 4, (
            f"REST row {r['sku_id']} has {len(r['prices'])} tiers, expected 4"
        )


def test_http_has_2_tiers():
    rows = list(ingest(offer_path=FIXTURE))
    http_rows = [r for r in rows if r["resource_name"] == "http"]
    assert http_rows, "no HTTP rows found"
    for r in http_rows:
        assert len(r["prices"]) == 2, (
            f"HTTP row {r['sku_id']} has {len(r['prices'])} tiers, expected 2"
        )


def test_unknown_location_rejected(tmp_path):
    bad = json.loads(FIXTURE.read_text())
    # Set an unknown location on one of the REST products
    bad["products"]["APIGW-REST-USE1"]["attributes"]["location"] = "Unknown Location (Nowhere)"
    p = tmp_path / "bad.json"
    p.write_text(json.dumps(bad))
    with pytest.raises(KeyError, match="Unknown API Gateway location"):
        list(ingest(offer_path=p))


def test_tiers_contiguous():
    """Verify each row's price tiers are contiguous: tier[i].tier_upper == tier[i+1].tier,
    and the last tier has tier_upper == ''."""
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        prices = sorted(r["prices"], key=lambda p: parse_count_tier(p["tier"]))
        for i, price in enumerate(prices):
            if i < len(prices) - 1:
                assert price["tier_upper"] == prices[i + 1]["tier"], (
                    f"Row {r['sku_id']}: tier[{i}].tier_upper={price['tier_upper']!r} "
                    f"!= tier[{i+1}].tier={prices[i+1]['tier']!r} (not contiguous)"
                )
                assert price["tier_upper"] != "", (
                    f"Row {r['sku_id']}: non-last tier[{i}] has empty tier_upper"
                )
            else:
                assert price["tier_upper"] == "", (
                    f"Row {r['sku_id']}: last tier tier_upper should be '' (unbounded), "
                    f"got {price['tier_upper']!r}"
                )


def test_websocket_skipped():
    """WebSocket operations must not appear in output."""
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        mode = r.get("resource_attrs", {}).get("extra", {}).get("mode", "")
        assert mode != "websocket", f"Row {r['sku_id']} has unexpected mode=websocket"


def test_terms_os_tokens():
    """REST rows should have os=rest-api, HTTP rows should have os=http-api."""
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        if r["resource_name"] == "rest":
            assert r["terms"]["os"] == "rest-api", (
                f"REST row {r['sku_id']} has os={r['terms']['os']!r}, expected 'rest-api'"
            )
        elif r["resource_name"] == "http":
            assert r["terms"]["os"] == "http-api", (
                f"HTTP row {r['sku_id']} has os={r['terms']['os']!r}, expected 'http-api'"
            )
