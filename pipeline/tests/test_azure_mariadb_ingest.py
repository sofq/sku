from __future__ import annotations

from pathlib import Path

from ingest.azure_mariadb import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "azure_mariadb" / "prices.json"


def test_ingest_emits_rows_for_mariadb_skus():
    rows = list(ingest(prices_path=FIXTURE))
    assert len(rows) > 0
    for r in rows:
        assert r["provider"] == "azure"
        assert r["service"] == "mariadb"
        assert r["kind"] == "db.relational"
        assert r["terms"]["tenancy"] == "azure-mariadb"
        assert r["terms"]["os"] == "single-az"
