"""Golden-row test + invariants for the Azure disks ingest."""

import json
from pathlib import Path

from ingest.azure_disks import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "azure_disks" / "prices.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "azure_disks_rows.jsonl"

_ALLOWED_TYPES = {"standard-hdd", "standard-ssd", "premium-ssd"}


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(prices_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_storage_block_kind():
    rows = list(ingest(prices_path=FIXTURE))
    assert rows, "fixture produced zero rows"
    assert {r["kind"] for r in rows} == {"storage.block"}


def test_ultra_rejected():
    rows = list(ingest(prices_path=FIXTURE))
    raw = json.loads(FIXTURE.read_text())
    ultra_ids = {it["meterId"] for it in raw["Items"] if it["armSkuName"] == "UltraSSD_LRS"}
    assert ultra_ids, "fixture should contain at least one Ultra row"
    out_ids = {r["sku_id"] for r in rows}
    assert ultra_ids.isdisjoint(out_ids)


def test_resource_name_in_allowed_types():
    rows = list(ingest(prices_path=FIXTURE))
    for r in rows:
        assert r["resource_name"] in _ALLOWED_TYPES, f"unexpected {r['resource_name']}"


def test_single_dim_per_row():
    rows = list(ingest(prices_path=FIXTURE))
    for r in rows:
        dims = [p["dimension"] for p in r["prices"]]
        assert dims == ["storage"], f"row {r['sku_id']} has dims {dims}"
