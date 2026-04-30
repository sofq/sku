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


def test_ingest_serverless_collapses_to_one_row_per_region():
    # AWS publishes compute-OCU / indexing-OCU / storage-OCU as separate
    # offer SKUs; the ingest must collapse them into a single logical SKU
    # per region so `sku aws opensearch price --mode serverless` returns
    # one row with three price dimensions, not three rows.
    rows = list(ingest(offer_path=FIX / "offer.json"))
    sl = [r for r in rows if r["resource_attrs"]["extra"]["mode"] == "serverless"]
    assert len(sl) == 1, f"expected 1 serverless row, got {len(sl)}"
    row = sl[0]
    dims = {p["dimension"] for p in row["prices"]}
    assert dims == {"compute-ocu", "indexing-ocu", "storage"}


def test_ingest_serverless_storage_uses_gb_month_unit():
    rows = list(ingest(offer_path=FIX / "offer.json"))
    sl = [r for r in rows if r["resource_attrs"]["extra"]["mode"] == "serverless"]
    assert sl
    storage_prices = [p for p in sl[0]["prices"] if p["dimension"] == "storage"]
    assert storage_prices and storage_prices[0]["unit"] == "gb-month"
