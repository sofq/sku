"""Golden-row test: fixture Service Bus Topics prices.json -> normalized NDJSON matches golden."""

from __future__ import annotations

import json
from pathlib import Path

from ingest.azure_service_bus_topics import ingest
from normalize.tier_tokens import parse_count_tier

FIXTURE = (
    Path(__file__).resolve().parent.parent
    / "testdata"
    / "azure_service_bus_topics"
    / "prices.json"
)
GOLDEN = (
    Path(__file__).resolve().parent.parent
    / "testdata"
    / "golden"
    / "azure_service_bus_topics_rows.jsonl"
)


def _load_rows():
    return list(ingest(prices_path=FIXTURE))


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = _load_rows()
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_messaging_topic_kind():
    rows = _load_rows()
    assert rows
    assert {r["kind"] for r in rows} == {"messaging.topic"}


def test_standard_rows_have_tiered_request_prices():
    rows = _load_rows()
    std_rows = [r for r in rows if r["resource_name"] == "standard"]
    assert std_rows, "no standard rows found"
    for row in std_rows:
        dims = {p["dimension"] for p in row["prices"]}
        assert dims == {"request"}, f"row {row['sku_id']} has unexpected dims {dims}"
        # Must have more than 1 price entry (tiered)
        assert len(row["prices"]) > 1, f"row {row['sku_id']} expected tiered prices"


def test_premium_rows_have_mu_hour_price():
    rows = _load_rows()
    prem_rows = [r for r in rows if r["resource_name"] == "premium"]
    assert prem_rows, "no premium rows found"
    for row in prem_rows:
        dims = {p["dimension"] for p in row["prices"]}
        assert dims == {"mu_hour"}, f"row {row['sku_id']} has unexpected dims {dims}"
        for p in row["prices"]:
            assert p["unit"] == "hr", (
                f"row {row['sku_id']} premium price unit expected 'hr', got {p['unit']!r}"
            )
            assert p["amount"] == 0.928, (
                f"row {row['sku_id']} premium hourly price expected 0.928, got {p['amount']}"
            )


def test_regions_covered():
    rows = _load_rows()
    regions = {r["region"] for r in rows}
    assert "eastus" in regions, "eastus missing from rows"
    assert "westeurope" in regions, "westeurope missing from rows"


def test_tiers_contiguous():
    rows = _load_rows()
    std_rows = [r for r in rows if r["resource_name"] == "standard"]
    assert std_rows, "no standard rows found"
    for row in std_rows:
        prices = sorted(
            [p for p in row["prices"] if p["dimension"] == "request"],
            key=lambda p: parse_count_tier(p["tier"]),
        )
        assert prices, f"row {row['sku_id']} has no request prices"
        for i, p in enumerate(prices):
            if i < len(prices) - 1:
                assert p["tier_upper"] != "", (
                    f"row {row['sku_id']} tier[{i}].tier_upper is empty (non-last entry)"
                )
                assert p["tier_upper"] == prices[i + 1]["tier"], (
                    f"row {row['sku_id']} tier[{i}].tier_upper={p['tier_upper']!r} "
                    f"!= tier[{i+1}].tier={prices[i+1]['tier']!r} (not contiguous)"
                )
            else:
                assert p["tier_upper"] == "", (
                    f"row {row['sku_id']} last tier should have tier_upper='', "
                    f"got {p['tier_upper']!r}"
                )
