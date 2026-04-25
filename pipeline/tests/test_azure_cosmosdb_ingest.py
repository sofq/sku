from pathlib import Path

from ingest.azure_cosmosdb import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "azure_cosmosdb"


def test_ingest_emits_kind_db_nosql():
    rows = list(ingest(prices_path=FIXTURE / "prices.json"))
    assert rows
    assert all(r["kind"] == "db.nosql" for r in rows)


def test_ingest_splits_by_capacity_mode():
    rows = list(ingest(prices_path=FIXTURE / "prices.json"))
    modes = {r["resource_attrs"]["extra"]["capacity_mode"] for r in rows}
    assert {"provisioned", "serverless", "storage"}.issubset(modes)


def test_ingest_carries_api_in_extra():
    rows = list(ingest(prices_path=FIXTURE / "prices.json"))
    apis = {r["resource_attrs"]["extra"].get("api") for r in rows if r["resource_attrs"]["extra"].get("api")}
    assert "sql" in apis
    assert "mongo" in apis


def test_ingest_provisioned_has_ru_per_sec_hour_usd():
    rows = list(ingest(prices_path=FIXTURE / "prices.json"))
    prov = [r for r in rows if r["resource_attrs"]["extra"].get("capacity_mode") == "provisioned"]
    assert prov
    for r in prov:
        assert "ru_per_sec_hour_usd" in r["resource_attrs"]["extra"]


def test_ingest_serverless_has_ru_million_usd():
    rows = list(ingest(prices_path=FIXTURE / "prices.json"))
    sv = [r for r in rows if r["resource_attrs"]["extra"].get("capacity_mode") == "serverless"]
    assert sv
    for r in sv:
        assert "ru_million_usd" in r["resource_attrs"]["extra"]
