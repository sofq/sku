"""Golden-row test: fixture Service Bus prices.json -> normalized NDJSON matches golden."""

from __future__ import annotations

import json
from pathlib import Path

from ingest.azure_service_bus_queues import ingest
from normalize.tier_tokens import parse_count_tier

FIXTURE = (
    Path(__file__).resolve().parent.parent
    / "testdata"
    / "azure_service_bus_queues"
    / "prices.json"
)
GOLDEN = (
    Path(__file__).resolve().parent.parent
    / "testdata"
    / "golden"
    / "azure_service_bus_queues_rows.jsonl"
)


def _load_rows():
    return list(ingest(prices_path=FIXTURE))


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = _load_rows()
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_messaging_queue_kind():
    rows = _load_rows()
    assert rows
    assert {r["kind"] for r in rows} == {"messaging.queue"}


def test_resource_names_are_standard_and_premium():
    rows = _load_rows()
    assert {r["resource_name"] for r in rows} == {"standard", "premium"}


def test_standard_rows_have_request_dimension():
    rows = _load_rows()
    std_rows = [r for r in rows if r["resource_name"] == "standard"]
    assert std_rows, "no standard rows found"
    for row in std_rows:
        dims = {p["dimension"] for p in row["prices"]}
        assert dims == {"request"}, f"row {row['sku_id']} has unexpected dims {dims}"


def test_premium_rows_have_mu_hour_dimension():
    rows = _load_rows()
    prem_rows = [r for r in rows if r["resource_name"] == "premium"]
    assert prem_rows, "no premium rows found"
    for row in prem_rows:
        dims = {p["dimension"] for p in row["prices"]}
        assert dims == {"mu_hour"}, f"row {row['sku_id']} has unexpected dims {dims}"


def test_standard_tiers_contiguous():
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


def test_standard_has_free_tier():
    rows = _load_rows()
    std_rows = [r for r in rows if r["resource_name"] == "standard"]
    assert std_rows, "no standard rows found"
    for row in std_rows:
        free_tiers = [p for p in row["prices"] if p["tier"] == "0" and p["amount"] == 0.0]
        assert free_tiers, f"row {row['sku_id']} missing free tier (tier=0, amount=0.0)"


def test_standard_amount_is_per_request():
    """Azure publishes Standard ops at $/M (unitOfMeasure '1M'); the ingestor
    must divide so amounts are stored as per-request rates. The largest tier
    fixture price is $0.80/M = 8e-7/request. Anything in the 0.1–10 range
    means the divisor was skipped (the B-1 bug)."""
    rows = _load_rows()
    std_rows = [r for r in rows if r["resource_name"] == "standard"]
    assert std_rows, "no standard rows found"
    for row in std_rows:
        for p in row["prices"]:
            if p["amount"] == 0.0:
                continue
            assert p["amount"] < 1e-3, (
                f"row {row['sku_id']} tier {p['tier']} amount={p['amount']} "
                f"looks like a $/M rate; ingestor must divide by unitOfMeasure"
            )


def test_premium_amount_is_hourly():
    rows = _load_rows()
    prem_rows = [r for r in rows if r["resource_name"] == "premium"]
    assert prem_rows, "no premium rows found"
    for row in prem_rows:
        for p in row["prices"]:
            assert p["unit"] == "hr", (
                f"row {row['sku_id']} premium price unit expected 'hr', got {p['unit']!r}"
            )
            assert p["amount"] == 0.928, (
                f"row {row['sku_id']} premium hourly price expected 0.928, got {p['amount']}"
            )
