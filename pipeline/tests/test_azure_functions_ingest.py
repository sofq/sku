"""Golden-row test + invariants for the Azure functions ingest."""

import json
from pathlib import Path

from ingest.azure_functions import ingest

_DATA = Path(__file__).resolve().parent.parent / "testdata"
FIXTURE = _DATA / "azure_functions" / "prices.json"
GOLDEN = _DATA / "golden" / "azure_functions_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(prices_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_compute_function_kind():
    rows = list(ingest(prices_path=FIXTURE))
    assert rows, "fixture produced zero rows"
    assert {r["kind"] for r in rows} == {"compute.function"}


def test_each_row_carries_two_dims():
    rows = list(ingest(prices_path=FIXTURE))
    for r in rows:
        dims = {p["dimension"] for p in r["prices"]}
        assert dims == {"executions", "duration"}, f"row {r['sku_id']} missing dims: {dims}"


def test_premium_plan_filtered():
    rows = list(ingest(prices_path=FIXTURE))
    for r in rows:
        parts = set(r["sku_id"].split("::"))
        assert "fn-premium-eastus" not in parts
