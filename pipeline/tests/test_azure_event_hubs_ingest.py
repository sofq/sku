"""Golden-row test: fixture Event Hubs prices.json -> normalized NDJSON matches golden."""

from __future__ import annotations

import json
from pathlib import Path

from ingest.azure_event_hubs import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "azure_event_hubs" / "prices.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "azure_event_hubs_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(prices_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_messaging_queue_kind():
    rows = list(ingest(prices_path=FIXTURE))
    assert rows
    assert {r["kind"] for r in rows} == {"messaging.queue"}


def test_resource_names_are_standard_and_premium():
    rows = list(ingest(prices_path=FIXTURE))
    assert {r["resource_name"] for r in rows} == {"standard", "premium"}


def test_standard_rows_have_both_dimensions():
    rows = list(ingest(prices_path=FIXTURE))
    standard_rows = [r for r in rows if r["resource_name"] == "standard"]
    assert standard_rows, "expected at least one standard row"
    for row in standard_rows:
        dims = {p["dimension"] for p in row["prices"]}
        assert "tu_hour" in dims, f"row {row['sku_id']} missing tu_hour dimension"
        assert "event" in dims, f"row {row['sku_id']} missing event dimension"


def test_premium_rows_have_ppu_hour_dimension():
    rows = list(ingest(prices_path=FIXTURE))
    premium_rows = [r for r in rows if r["resource_name"] == "premium"]
    assert premium_rows, "expected at least one premium row"
    for row in premium_rows:
        dims = {p["dimension"] for p in row["prices"]}
        assert dims == {"ppu_hour"}, (
            f"row {row['sku_id']} has unexpected dims {dims}"
        )
