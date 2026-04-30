from pathlib import Path

from ingest.azure_appservice import ingest

FIX = Path(__file__).parent / "fixtures" / "azure_appservice"


def test_ingest_emits_paas_app_kind():
    rows = list(ingest(prices_path=FIX / "prices.json"))
    assert rows
    assert all(r["kind"] == "paas.app" for r in rows)


def test_ingest_terms_os_matches_extra_os():
    rows = list(ingest(prices_path=FIX / "prices.json"))
    for r in rows:
        assert r["terms"]["os"] == r["resource_attrs"]["extra"]["os"]


def test_ingest_detects_linux_and_windows():
    rows = list(ingest(prices_path=FIX / "prices.json"))
    os_values = {r["terms"]["os"] for r in rows}
    assert "linux" in os_values
    assert "windows" in os_values


def test_ingest_support_tier_matches_extra_tier():
    rows = list(ingest(prices_path=FIX / "prices.json"))
    for r in rows:
        assert r["terms"]["support_tier"] == r["resource_attrs"]["extra"]["tier"]


def test_ingest_known_skus_have_vcpu_and_memory():
    rows = list(ingest(prices_path=FIX / "prices.json"))
    known = [r for r in rows if not r["resource_attrs"]["extra"].get("unknown_sku")]
    assert known
    for r in known:
        assert r["resource_attrs"]["vcpu"] is not None
        assert r["resource_attrs"]["memory_gb"] is not None


def test_ingest_skips_zero_price():
    rows = list(ingest(prices_path=FIX / "prices.json"))
    # F1 is free (retailPrice=0.0) — should be skipped
    names = {r["resource_name"] for r in rows}
    assert "F1" not in names


def test_ingest_empty_prices_returns_no_rows(tmp_path):
    empty = tmp_path / "prices.json"
    empty.write_text('{"Items": []}')
    assert list(ingest(prices_path=empty)) == []


def test_ingest_isolated_v2_sku_canonicalizes_to_lowercase_v():
    # Regression: prior canonicalization only normalized P*v* to lowercase v,
    # leaving I*v2 as "I1V2" — which mismatched _PLAN_SKU_SPECS keys
    # (lowercase v) and _APP_SERVICE_SKUS in validate/azure.py.
    rows = list(ingest(prices_path=FIX / "prices.json"))
    iso = [r for r in rows if r["resource_name"].startswith("I")]
    assert iso, "expected at least one Isolated-v2 row"
    for r in iso:
        assert r["resource_name"] == "I1v2", r["resource_name"]
        assert not r["resource_attrs"]["extra"].get("unknown_sku")
        assert r["resource_attrs"]["vcpu"] == 2
        assert r["resource_attrs"]["memory_gb"] == 8.0


def test_ingest_accepts_live_spaced_premium_v3_names(tmp_path):
    prices = tmp_path / "prices.json"
    prices.write_text(
        """{"Items": [{
          "skuId": "DZH318Z0DCR6/01FQ",
          "productName": "Azure App Service Premium v3 Plan - Linux",
          "meterName": "P1 v3 App",
          "skuName": "P1 v3",
          "armRegionName": "eastus",
          "retailPrice": 0.169,
          "unitOfMeasure": "1 Hour",
          "type": "Consumption",
          "currencyCode": "USD"
        }]}"""
    )
    rows = list(ingest(prices_path=prices))
    assert len(rows) == 1
    row = rows[0]
    assert row["resource_name"] == "P1v3"
    assert row["terms"]["support_tier"] == "premiumv3"
    assert row["terms"]["os"] == "linux"
    assert row["resource_attrs"]["vcpu"] == 2
    assert row["resource_attrs"]["memory_gb"] == 8.0
