"""Golden-row test + invariants for the Azure blob ingest."""

import json
from pathlib import Path

from ingest.azure_blob import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "azure_blob" / "prices.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "azure_blob_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(prices_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_storage_object_kind():
    rows = list(ingest(prices_path=FIXTURE))
    assert rows, "fixture produced zero rows"
    assert {r["kind"] for r in rows} == {"storage.object"}


def test_each_row_carries_three_dims():
    rows = list(ingest(prices_path=FIXTURE))
    for r in rows:
        dims = {p["dimension"] for p in r["prices"]}
        assert dims == {"storage", "read-ops", "write-ops"}, \
            f"row {r['sku_id']} missing dims: {dims}"


def test_reservation_rows_filtered():
    rows = list(ingest(prices_path=FIXTURE))
    raw = json.loads(FIXTURE.read_text())
    reservation_ids = {it["meterId"] for it in raw["Items"] if it["type"] == "Reservation"}
    assert reservation_ids, "fixture should contain at least one Reservation row"
    for r in rows:
        parts = set(r["sku_id"].split("::"))
        assert reservation_ids.isdisjoint(parts)


def test_non_usd_rows_filtered():
    rows = list(ingest(prices_path=FIXTURE))
    raw = json.loads(FIXTURE.read_text())
    eur_ids = {it["meterId"] for it in raw["Items"] if it["currencyCode"] == "EUR"}
    assert eur_ids, "fixture should contain at least one EUR row"
    for r in rows:
        parts = set(r["sku_id"].split("::"))
        assert eur_ids.isdisjoint(parts)


def test_unknown_region_skipped(tmp_path):
    bad = json.loads(FIXTURE.read_text())
    for it in bad["Items"]:
        if it["type"] == "Consumption" and it["currencyCode"] == "USD":
            it["armRegionName"] = "centralus"
            break
    p = tmp_path / "bad.json"
    p.write_text(json.dumps(bad))
    rows = list(ingest(prices_path=p))
    assert all(r["region"] != "centralus" for r in rows)
