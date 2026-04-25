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
