from pathlib import Path

from ingest.gcp_spanner import ingest

FIXTURE = Path(__file__).parent.parent / "testdata" / "gcp_spanner"


def test_ingest_emits_kind_db_relational():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    assert rows
    assert all(r["kind"] == "db.relational" for r in rows)


def test_ingest_compute_rows_have_pu_and_node_hourly():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    compute = [r for r in rows if r["resource_attrs"]["extra"].get("kind") != "storage"]
    for r in compute:
        e = r["resource_attrs"]["extra"]
        assert "pu_hour_usd" in e
        assert "node_hour_usd" in e
        assert abs(e["node_hour_usd"] - e["pu_hour_usd"] * 1000) < 1e-6


def test_ingest_storage_rows_carry_extra_kind_storage():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    storage = [r for r in rows if r["resource_attrs"]["extra"].get("kind") == "storage"]
    assert storage, "fixture should include at least one storage SKU"
    for r in storage:
        assert "ssd_gb_month_usd" in r["resource_attrs"]["extra"]


def test_ingest_emits_three_editions():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    eds = {r["resource_attrs"]["extra"].get("edition") for r in rows
           if r["resource_attrs"]["extra"].get("edition")}
    assert {"standard", "enterprise", "enterprise-plus"}.issubset(eds)
