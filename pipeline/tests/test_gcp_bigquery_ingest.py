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
        os_val = r["terms"]["os"]
        rn = r["resource_name"]
        mode = r["resource_attrs"]["extra"]["mode"]
        assert os_val == "on-demand", (
            f"expected terms.os='on-demand' for all rows, got {os_val!r} "
            f"(resource_name={rn!r}, extra.mode={mode!r})"
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
        # edition is the suffix of resource_name and matches the BQ edition name 1:1.
        assert r["resource_attrs"]["extra"]["edition"] in (
            "standard", "enterprise", "enterprise-plus",
        )


def test_ingest_capacity_resource_names_match_bq_edition_names():
    # resource_name must equal the BQ edition name so `--mode <name>` matches:
    #   Standard → capacity-standard
    #   Enterprise → capacity-enterprise
    #   Enterprise Plus → capacity-enterprise-plus
    rows = list(ingest(skus_path=FIX / "skus.json"))
    names = {r["resource_name"] for r in rows if r["resource_attrs"]["extra"]["mode"] == "capacity"}
    assert "capacity-standard" in names
    assert "capacity-enterprise" in names
    assert "capacity-enterprise-plus" in names


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


def test_ingest_skips_physical_storage_rows():
    # Logical and Physical storage share the same (resource_name, region)
    # shape — ingesting both produces duplicate rows in LookupWarehouseQuery.
    # We ingest Logical only; verify Physical SKUs are dropped.
    rows = list(ingest(skus_path=FIX / "skus.json"))
    sku_ids = {r["sku_id"].split("-")[-1] for r in rows}  # row_sku_id is "<skuId>-<region>"
    sku_id_full = {r["sku_id"] for r in rows}
    assert not any("PHYSICAL" in sid for sid in sku_id_full), (
        f"physical-storage rows must be skipped, got: {sku_id_full}"
    )

    # And there must be exactly one row per (resource_name, region) for storage.
    storage_keys = [
        (r["resource_name"], r["region"])
        for r in rows
        if r["resource_attrs"]["extra"]["mode"] == "storage"
    ]
    assert len(storage_keys) == len(set(storage_keys)), (
        f"duplicate (resource_name, region) for storage rows: {storage_keys}"
    )
    _ = sku_ids  # silence unused-name guard
