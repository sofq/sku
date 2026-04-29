from pathlib import Path

from ingest.gcp_bigquery import ingest

FIX = Path(__file__).parent / "fixtures" / "gcp_bigquery"


def test_ingest_emits_warehouse_query_kind():
    rows = list(ingest(skus_path=FIX / "skus.json"))
    assert rows
    assert all(r["kind"] == "warehouse.query" for r in rows)


def test_ingest_terms_os_is_always_on_demand():
    # BigQuery: terms.os is always "on-demand" regardless of mode.
    # The actual pricing mode lives in extra.mode.
    rows = list(ingest(skus_path=FIX / "skus.json"))
    for r in rows:
        assert r["terms"]["os"] == "on-demand", (
            f"expected terms.os='on-demand' for all rows, got {r['terms']['os']!r} "
            f"(resource_name={r['resource_name']!r}, extra.mode={r['resource_attrs']['extra']['mode']!r})"
        )


def test_ingest_extra_mode_discriminates_pricing_shape():
    rows = list(ingest(skus_path=FIX / "skus.json"))
    modes = {r["resource_attrs"]["extra"]["mode"] for r in rows}
    assert "on-demand" in modes
    assert "capacity" in modes
    assert "storage" in modes


def test_ingest_skips_batch_rows():
    rows = list(ingest(skus_path=FIX / "skus.json"))
    names = {r["resource_name"] for r in rows}
    # The fixture has a "Batch Analysis" SKU that must be skipped
    assert "BQ-SKIP-BATCH" not in names


def test_ingest_multiregion_us_maps_to_bq_us():
    rows = list(ingest(skus_path=FIX / "skus.json"))
    regions = {r["region"] for r in rows}
    assert "bq-us" in regions


def test_ingest_multiregion_eu_maps_to_bq_eu():
    rows = list(ingest(skus_path=FIX / "skus.json"))
    regions = {r["region"] for r in rows}
    assert "bq-eu" in regions


def test_ingest_capacity_rows_carry_edition():
    rows = list(ingest(skus_path=FIX / "skus.json"))
    cap = [r for r in rows if r["resource_attrs"]["extra"]["mode"] == "capacity"]
    assert cap
    for r in cap:
        assert "edition" in r["resource_attrs"]["extra"]
        # edition is the suffix of resource_name: "capacity-standard" → "standard",
        # "capacity-enterprise" → "enterprise"
        assert r["resource_attrs"]["extra"]["edition"] in ("standard", "enterprise")


def test_ingest_storage_rows_carry_storage_tier():
    rows = list(ingest(skus_path=FIX / "skus.json"))
    storage = [r for r in rows if r["resource_attrs"]["extra"]["mode"] == "storage"]
    assert storage
    for r in storage:
        assert "storage_tier" in r["resource_attrs"]["extra"]
        assert r["resource_attrs"]["extra"]["storage_tier"] in ("active", "long-term")


def test_ingest_empty_skus_returns_no_rows(tmp_path):
    empty = tmp_path / "skus.json"
    empty.write_text('{"skus": []}')
    assert list(ingest(skus_path=empty)) == []
