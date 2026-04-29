from pathlib import Path

from ingest.aws_opensearch import ingest

FIX = Path(__file__).parent / "fixtures" / "aws_opensearch"


def test_ingest_emits_search_engine_kind():
    rows = list(ingest(offer_path=FIX / "offer.json"))
    assert rows
    assert all(r["kind"] == "search.engine" for r in rows)


def test_ingest_managed_cluster_terms_os_and_extra_mode_agree():
    rows = list(ingest(offer_path=FIX / "offer.json"))
    mc = [r for r in rows if r["resource_attrs"]["extra"]["mode"] == "managed-cluster"]
    assert mc, "expected at least one managed-cluster row"
    for r in mc:
        assert r["terms"]["os"] == "managed-cluster"


def test_ingest_serverless_terms_os_and_extra_mode_agree():
    rows = list(ingest(offer_path=FIX / "offer.json"))
    sl = [r for r in rows if r["resource_attrs"]["extra"]["mode"] == "serverless"]
    assert sl, "expected at least one serverless row"
    for r in sl:
        assert r["terms"]["os"] == "serverless"


def test_ingest_managed_cluster_has_instance_specs():
    rows = list(ingest(offer_path=FIX / "offer.json"))
    mc = [r for r in rows if r["resource_attrs"]["extra"]["mode"] == "managed-cluster"]
    for r in mc:
        assert r["resource_attrs"]["vcpu"] is not None
        assert r["resource_attrs"]["memory_gb"] is not None


def test_ingest_skips_unknown_region():
    rows = list(ingest(offer_path=FIX / "offer.json"))
    regions = {r["region"] for r in rows}
    assert "xx-fake-1" not in regions


def test_ingest_empty_offer_returns_no_rows(tmp_path):
    empty = tmp_path / "offer.json"
    empty.write_text('{"products": {}, "terms": {"OnDemand": {}}}')
    assert list(ingest(offer_path=empty)) == []
