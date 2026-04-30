"""Golden-row and behavioral tests for the gcp_firestore ingest module."""

import json
from pathlib import Path

from ingest.gcp_firestore import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "gcp_firestore"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "gcp_firestore_rows.jsonl"


def _canonical(rows: list[dict]) -> list[dict]:
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_db_nosql_kind():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    assert rows
    assert all(r["kind"] == "db.nosql" for r in rows)


def test_resource_name_is_native():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    assert rows
    assert all(r["resource_name"] == "native" for r in rows)


def test_all_rows_have_four_dimensions():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    expected_dims = {"storage", "document_read", "document_write", "document_delete"}
    for r in rows:
        dims = {p["dimension"] for p in r["prices"]}
        assert dims == expected_dims, (
            f"row {r['sku_id']!r} has dims {dims}, expected {expected_dims}"
        )


def test_tenancy_is_firestore_native():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    assert rows
    for r in rows:
        assert r["terms"]["tenancy"] == "firestore-native", (
            f"row {r['sku_id']!r} has tenancy {r['terms']['tenancy']!r}"
        )


def test_regions_covered():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    regions = {r["region"] for r in rows}
    assert "us-east1" in regions
    assert "europe-west1" in regions


def test_datastore_skus_excluded():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    sku_ids = {r["sku_id"] for r in rows}
    assert not any("DATASTORE" in sid for sid in sku_ids), (
        "Datastore-mode SKUs must be excluded"
    )


def test_small_ops_excluded():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    # SmallOps SKUs have $0 price — verify no $0 document prices
    for r in rows:
        for p in r["prices"]:
            if p["dimension"] != "storage":
                assert p["amount"] > 0 or p["dimension"] not in (
                    "document_read", "document_write", "document_delete"
                ), f"Unexpected $0 price for {p['dimension']} in row {r['sku_id']!r}"


def test_empty_skus_returns_no_rows(tmp_path):
    empty = tmp_path / "skus.json"
    empty.write_text('{"skus": []}')
    assert list(ingest(skus_path=empty)) == []


def test_extra_mode_is_native():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    assert rows
    for r in rows:
        assert r["resource_attrs"]["extra"]["mode"] == "native", (
            f"row {r['sku_id']!r} has extra.mode {r['resource_attrs']['extra'].get('mode')!r}"
        )
