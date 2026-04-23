from __future__ import annotations

from pathlib import Path

from ingest.azure_mysql import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "azure_mysql" / "prices.json"


def test_ingest_emits_rows_for_mysql_skus():
    rows = list(ingest(prices_path=FIXTURE))
    assert len(rows) > 0
    for r in rows:
        assert r["provider"] == "azure"
        assert r["service"] == "mysql"
        assert r["kind"] == "db.relational"
        assert r["terms"]["tenancy"] == "azure-mysql"
        assert r["terms"]["os"] in {"single-az", "flexible-server", "managed-instance", "elastic-pool"}


def test_flexible_and_single_server_both_admitted():
    rows = list(ingest(prices_path=FIXTURE))
    deployments = {r["terms"]["os"] for r in rows}
    assert "flexible-server" in deployments
    assert "single-az" in deployments
