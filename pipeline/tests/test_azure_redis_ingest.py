from pathlib import Path
import json

from ingest.azure_redis import ingest

FIXTURE = Path(__file__).parent.parent / "testdata" / "azure_redis"


def test_ingest_emits_kind_cache_kv():
    rows = list(ingest(prices_path=FIXTURE / "prices.json"))
    assert rows
    assert all(r["kind"] == "cache.kv" for r in rows)


def test_ingest_engine_is_always_redis():
    rows = list(ingest(prices_path=FIXTURE / "prices.json"))
    for r in rows:
        assert r["resource_attrs"]["extra"]["engine"] == "redis"


def test_ingest_carries_tier_in_extra():
    rows = list(ingest(prices_path=FIXTURE / "prices.json"))
    tiers = {r["resource_attrs"]["extra"].get("tier") for r in rows}
    assert {"basic", "standard", "premium", "enterprise"}.issubset(tiers)


def test_ingest_memory_gb_populated():
    rows = list(ingest(prices_path=FIXTURE / "prices.json"))
    for r in rows:
        assert r["resource_attrs"]["memory_gb"] is not None
        assert r["resource_attrs"]["memory_gb"] > 0


def test_ingest_preserves_upstream_meter():
    rows = list(ingest(prices_path=FIXTURE / "prices.json"))
    for r in rows:
        assert "upstream_meter" in r["resource_attrs"]["extra"]


def test_ingest_skips_non_consumption_and_non_usd_rows(tmp_path):
    prices = tmp_path / "prices.json"
    prices.write_text(json.dumps({
        "Items": [
            {
                "skuId": "keep",
                "meterName": "Standard C1",
                "armRegionName": "eastus",
                "retailPrice": 0.05,
                "unitOfMeasure": "1 Hour",
                "type": "Consumption",
                "currencyCode": "USD",
            },
            {
                "skuId": "reservation",
                "meterName": "Standard C1",
                "armRegionName": "eastus",
                "retailPrice": 0.04,
                "unitOfMeasure": "1 Hour",
                "type": "Reservation",
                "currencyCode": "USD",
            },
            {
                "skuId": "eur",
                "meterName": "Standard C1",
                "armRegionName": "eastus",
                "retailPrice": 0.045,
                "unitOfMeasure": "1 Hour",
                "type": "Consumption",
                "currencyCode": "EUR",
            },
        ],
    }))
    rows = list(ingest(prices_path=prices))
    assert [r["sku_id"] for r in rows] == ["keep"]


def test_ingest_handles_live_azure_redis_cache_product_shape(tmp_path):
    prices = tmp_path / "prices.json"
    prices.write_text(json.dumps({
        "Items": [
            {
                "skuId": "standard-c3-eastus",
                "productName": "Azure Redis Cache Standard",
                "skuName": "C3",
                "meterName": "C3 Cache Instance",
                "armRegionName": "eastus",
                "retailPrice": 0.281,
                "unitOfMeasure": "1 Hour",
                "type": "Consumption",
                "currencyCode": "USD",
            },
            {
                "skuId": "managed-b20-eastus",
                "productName": "Azure Managed Redis - Balanced",
                "skuName": "B20",
                "meterName": "B20 Cache Instance",
                "armRegionName": "eastus",
                "retailPrice": 0.692,
                "unitOfMeasure": "1 Hour",
                "type": "Consumption",
                "currencyCode": "USD",
            },
        ],
    }))

    rows = list(ingest(prices_path=prices))

    assert len(rows) == 1
    assert rows[0]["sku_id"] == "standard-c3-eastus"
    assert rows[0]["resource_name"] == "Standard C3"
    assert rows[0]["resource_attrs"]["memory_gb"] == 6.0
    assert rows[0]["resource_attrs"]["extra"]["tier"] == "standard"
