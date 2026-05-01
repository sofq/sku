"""Tests for Azure API Management (APIM) ingest module."""

from __future__ import annotations

import json
from pathlib import Path

from ingest.azure_apim import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "azure_apim"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "azure_apim_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(prices_path=FIXTURE / "prices.json"))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_api_gateway_kind():
    rows = list(ingest(prices_path=FIXTURE / "prices.json"))
    assert rows, "fixture produced zero rows"
    assert all(r["kind"] == "api.gateway" for r in rows)


def test_consumption_rows_have_call_dimension():
    rows = list(ingest(prices_path=FIXTURE / "prices.json"))
    consumption = [r for r in rows if r["resource_name"] == "consumption"]
    assert consumption, "no consumption rows found"
    for r in consumption:
        dims = {p["dimension"] for p in r["prices"]}
        assert dims == {"call"}, f"consumption row {r['sku_id']} has unexpected dimensions: {dims}"
        # Should have exactly 2 price tiers: free (0→1M) and paid (1M→∞)
        assert len(r["prices"]) == 2, (
            f"consumption row {r['sku_id']} expected 2 price tiers, got {len(r['prices'])}"
        )
        free_tier = next((p for p in r["prices"] if p["tier"] == "0"), None)
        assert free_tier is not None, f"consumption row {r['sku_id']} missing free tier"
        assert free_tier["amount"] == 0.0, "first 1M calls should be free"
        assert free_tier["tier_upper"] == "1M", "free tier upper bound should be canonical '1M'"
        paid_tier = next((p for p in r["prices"] if p["tier"] == "1M"), None)
        assert paid_tier is not None, f"consumption row {r['sku_id']} missing paid tier"
        assert paid_tier["amount"] > 0.0, "paid tier should have positive price"
        assert paid_tier["tier_upper"] == "", "paid tier should be unbounded"


def test_provisioned_rows_have_unit_hour_dimension():
    rows = list(ingest(prices_path=FIXTURE / "prices.json"))
    provisioned = [r for r in rows if r["resource_name"] != "consumption"]
    assert provisioned, "no provisioned rows found"
    for r in provisioned:
        dims = {p["dimension"] for p in r["prices"]}
        assert dims == {"unit_hour"}, (
            f"provisioned row {r['sku_id']} has unexpected dimensions: {dims}"
        )
        assert len(r["prices"]) == 1, (
            f"provisioned row {r['sku_id']} expected 1 price entry, got {len(r['prices'])}"
        )
        price = r["prices"][0]
        assert price["unit"] == "hr", f"provisioned row unit should be 'hr', got {price['unit']!r}"
        assert price["amount"] > 0.0, "provisioned row should have positive hourly price"


def test_mode_values():
    rows = list(ingest(prices_path=FIXTURE / "prices.json"))
    for r in rows:
        mode = r["resource_attrs"]["extra"]["mode"]
        if r["resource_name"] == "consumption":
            assert mode == "consumption", (
                f"row {r['sku_id']} expected mode=consumption, got {mode!r}"
            )
        else:
            assert mode == "provisioned", (
                f"row {r['sku_id']} expected mode=provisioned, got {mode!r}"
            )


def test_skips_non_api_management_rows(tmp_path):
    prices = tmp_path / "prices.json"
    prices.write_text(json.dumps({
        "Items": [
            {
                "skuId": "KEEP",
                "serviceName": "API Management",
                "productName": "API Management",
                "skuName": "Standard",
                "meterName": "Standard Unit",
                "armRegionName": "eastus",
                "retailPrice": 0.9407,
                "unitOfMeasure": "1 Hour",
                "type": "Consumption",
                "currencyCode": "USD",
            },
            {
                "skuId": "SKIP",
                "serviceName": "Virtual Machines",
                "productName": "Virtual Machines",
                "skuName": "Standard_D2_v3",
                "meterName": "D2 v3",
                "armRegionName": "eastus",
                "retailPrice": 0.096,
                "unitOfMeasure": "1 Hour",
                "type": "Consumption",
                "currencyCode": "USD",
            },
        ],
    }))
    rows = list(ingest(prices_path=prices))
    assert [r["sku_id"] for r in rows] == ["KEEP"]


def test_skips_reservation_and_non_usd(tmp_path):
    prices = tmp_path / "prices.json"
    prices.write_text(json.dumps({
        "Items": [
            {
                "skuId": "KEEP",
                "serviceName": "API Management",
                "skuName": "Standard",
                "meterName": "Standard Unit",
                "armRegionName": "eastus",
                "retailPrice": 0.9407,
                "unitOfMeasure": "1 Hour",
                "type": "Consumption",
                "currencyCode": "USD",
            },
            {
                "skuId": "SKIP-RESV",
                "serviceName": "API Management",
                "skuName": "Standard",
                "meterName": "Standard Unit",
                "armRegionName": "eastus",
                "retailPrice": 0.8,
                "unitOfMeasure": "1 Hour",
                "type": "Reservation",
                "currencyCode": "USD",
            },
            {
                "skuId": "SKIP-EUR",
                "serviceName": "API Management",
                "skuName": "Standard",
                "meterName": "Standard Unit",
                "armRegionName": "eastus",
                "retailPrice": 0.85,
                "unitOfMeasure": "1 Hour",
                "type": "Consumption",
                "currencyCode": "EUR",
            },
        ],
    }))
    rows = list(ingest(prices_path=prices))
    assert [r["sku_id"] for r in rows] == ["KEEP"]


def test_empty_prices_returns_no_rows(tmp_path):
    empty = tmp_path / "prices.json"
    empty.write_text('{"Items": []}')
    assert list(ingest(prices_path=empty)) == []
