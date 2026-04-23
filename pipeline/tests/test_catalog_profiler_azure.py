from __future__ import annotations

from pathlib import Path

from catalog_profiler.azure import scan_prices


def test_scan_azure_vm_fixture():
    prices = Path(__file__).resolve().parent.parent / "testdata" / "azure_vm" / "prices.json"
    rows = scan_prices(prices_path=prices)
    labels = {r.bucket_label for r in rows}
    assert any("Virtual Machines" in s for s in labels)
    vm = next(r for r in rows if "Virtual Machines" in r.bucket_label)
    assert vm.sku_count > 0
    assert vm.covered_by_shard == "azure_vm"
    assert "armSkuName" in vm.attribute_keys
    assert "retailPrice" in vm.attribute_keys


def test_scan_azure_hosted_db_fixtures_are_covered():
    base = Path(__file__).resolve().parent.parent / "testdata"
    cases = {
        "azure_postgres": "azure_postgres",
        "azure_mysql": "azure_mysql",
        "azure_mariadb": "azure_mariadb",
    }
    for fixture_dir, shard in cases.items():
        rows = scan_prices(prices_path=base / fixture_dir / "prices.json")
        assert rows
        assert {r.covered_by_shard for r in rows} == {shard}
