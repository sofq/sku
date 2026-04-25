from pathlib import Path

from ingest.gcp_memorystore import ingest

FIXTURE = Path(__file__).parent.parent / "testdata" / "gcp_memorystore"


def test_ingest_emits_cache_kv():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    assert rows
    assert all(r["kind"] == "cache.kv" for r in rows)


def test_ingest_includes_both_engines():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    engines = {r["resource_attrs"]["extra"]["engine"] for r in rows}
    assert engines == {"redis", "memcached"}


def test_ingest_memory_gb_populated():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    for r in rows:
        assert r["resource_attrs"]["memory_gb"] is not None


def test_ingest_resource_name_includes_engine():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    for r in rows:
        engine = r["resource_attrs"]["extra"]["engine"]
        assert r["resource_name"].startswith(f"memorystore-{engine}")
