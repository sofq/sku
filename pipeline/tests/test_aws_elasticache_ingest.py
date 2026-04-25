from pathlib import Path

from ingest.aws_elasticache import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "aws_elasticache"


def test_ingest_emits_kind_cache_kv():
    rows = list(ingest(offer_path=FIXTURE / "offer.json"))
    assert rows
    assert all(r["kind"] == "cache.kv" for r in rows)


def test_ingest_splits_by_engine():
    rows = list(ingest(offer_path=FIXTURE / "offer.json"))
    engines = {r["resource_attrs"]["extra"]["engine"] for r in rows}
    assert engines == {"redis", "memcached"}


def test_ingest_populates_memory_gb():
    rows = list(ingest(offer_path=FIXTURE / "offer.json"))
    for r in rows:
        assert r["resource_attrs"]["memory_gb"] is not None
        assert r["resource_attrs"]["memory_gb"] > 0


def test_ingest_populates_vcpu():
    rows = list(ingest(offer_path=FIXTURE / "offer.json"))
    for r in rows:
        assert r["resource_attrs"]["vcpu"] is not None


def test_ingest_empty_offer_returns_no_rows(tmp_path):
    empty = tmp_path / "empty.json"
    empty.write_text('{"products":{}, "terms":{"OnDemand":{}}}')
    assert list(ingest(offer_path=empty)) == []
