"""Tests for pipeline.validate.azure — Azure retail pricing revalidator."""

from __future__ import annotations

import pytest
import requests_mock as requests_mock_module

from validate.azure import revalidate
from validate.sampler import Sample

_AZURE_PRICES_URL = "https://prices.azure.com/api/retail/prices"

_SAMPLE = Sample(
    sku_id="azure-vm/Standard_D2_v3/eastus",
    region="eastus",
    resource_name="Standard_D2_v3",
    price_amount=0.096,
    price_currency="USD",
    dimension="on-demand",
)


def _api_response(unit_price: float) -> dict:
    return {
        "Items": [
            {
                "meterName": "Standard_D2_v3",
                "armRegionName": "eastus",
                "unitPrice": unit_price,
                "currencyCode": "USD",
                "retailPrice": unit_price,
                "skuName": "D2 v3",
                "productName": "Virtual Machines Dv3 Series",
                "type": "Consumption",
            }
        ],
        "NextPageLink": None,
        "Count": 1,
    }


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_azure_no_drift(requests_mock: requests_mock_module.Mocker) -> None:
    """Catalog within 1% -> no drift record."""
    requests_mock.get(_AZURE_PRICES_URL, json=_api_response(0.096))
    drift, missing = revalidate([_SAMPLE])
    assert drift == []
    assert missing == []


def test_azure_drift_detected(requests_mock: requests_mock_module.Mocker) -> None:
    """5% upstream price higher -> drift record."""
    upstream = 0.096 * 1.05
    requests_mock.get(_AZURE_PRICES_URL, json=_api_response(upstream))
    drift, missing = revalidate([_SAMPLE])
    assert len(drift) == 1
    rec = drift[0]
    assert rec.sku_id == _SAMPLE.sku_id
    assert rec.catalog_amount == pytest.approx(0.096)
    assert rec.upstream_amount == pytest.approx(upstream)
    assert rec.source == "azure"


def test_azure_missing_upstream(requests_mock: requests_mock_module.Mocker) -> None:
    """Empty Items response -> missing list."""
    requests_mock.get(_AZURE_PRICES_URL, json={"Items": [], "NextPageLink": None, "Count": 0})
    drift, missing = revalidate([_SAMPLE])
    assert drift == []
    assert len(missing) == 1
    assert missing[0] == _SAMPLE.sku_id


def test_azure_multiple_samples(requests_mock: requests_mock_module.Mocker) -> None:
    """Two samples: one match, one drift."""
    s1 = Sample(
        sku_id="azure-vm/Standard_D2_v3/eastus",
        region="eastus",
        resource_name="Standard_D2_v3",
        price_amount=0.096,
        price_currency="USD",
        dimension="on-demand",
    )
    s2 = Sample(
        sku_id="azure-vm/Standard_F4s_v2/westus",
        region="westus",
        resource_name="Standard_F4s_v2",
        price_amount=0.200,
        price_currency="USD",
        dimension="on-demand",
    )
    # Respond in call order (two separate GET calls)
    requests_mock.get(
        _AZURE_PRICES_URL,
        [
            {"json": _api_response(0.096)},   # s1: match
            {"json": _api_response(0.230)},   # s2: drift ~15%
        ],
    )
    drift, missing = revalidate([s1, s2])
    assert len(drift) == 1
    assert drift[0].sku_id == s2.sku_id
    assert missing == []


def test_azure_aks_standard_filters_by_upstream_meter_name(
    requests_mock: requests_mock_module.Mocker,
) -> None:
    """AKS validation maps aks-standard to the real Azure meter name."""
    sample = Sample(
        sku_id="azure-aks-standard-eastus",
        region="eastus",
        resource_name="aks-standard",
        price_amount=0.10,
        price_currency="USD",
        dimension="cluster",
    )
    requests_mock.get(_AZURE_PRICES_URL, json=_api_response(0.10))
    drift, missing = revalidate([sample])
    assert drift == []
    assert missing == []
    filt = requests_mock.last_request.qs["$filter"][0]
    assert "metername eq 'standard uptime sla'" in filt
    assert "servicename eq 'azure kubernetes service'" in filt


def test_azure_aks_free_synthetic_zero_does_not_require_upstream_call(
    requests_mock: requests_mock_module.Mocker,
) -> None:
    """AKS free rows are synthetic zero-price rows and validate without API lookup."""
    sample = Sample(
        sku_id="azure-aks-free-eastus",
        region="eastus",
        resource_name="aks-free",
        price_amount=0.0,
        price_currency="USD",
        dimension="cluster",
    )
    drift, missing = revalidate([sample])
    assert drift == []
    assert missing == []
    assert requests_mock.call_count == 0


def test_azure_ambiguous_multi_item_response_is_missing_not_drift(
    requests_mock: requests_mock_module.Mocker,
) -> None:
    """When the meterName+region filter returns multiple Items with disagreeing
    positive unitPrices, treat as missing rather than guessing items[0].
    Otherwise we file false-positive drift records (e.g. Azure SQL where one
    meter spans compute, vCore-overage and storage line-items).
    """
    requests_mock.get(
        _AZURE_PRICES_URL,
        json={
            "Items": [
                {
                    "meterName": "vCore",
                    "armRegionName": "eastus",
                    "unitPrice": 0.5,
                    "currencyCode": "USD",
                    "skuName": "GP_Gen5",
                    "productName": "SQL Database Single General Purpose - Compute Gen5",
                    "type": "Consumption",
                    "unitOfMeasure": "1 Hour",
                },
                {
                    "meterName": "vCore",
                    "armRegionName": "eastus",
                    "unitPrice": 0.25,
                    "currencyCode": "USD",
                    "skuName": "GP_Gen5",
                    "productName": "SQL Database Elastic Pool General Purpose - Compute Gen5",
                    "type": "Consumption",
                    "unitOfMeasure": "1 Hour",
                },
            ],
            "NextPageLink": None,
            "Count": 2,
        },
    )
    sample = Sample(
        sku_id="azure-sql/GP_Gen5_2/eastus",
        region="eastus",
        resource_name="vCore",
        price_amount=0.5,
        price_currency="USD",
        dimension="on-demand",
    )
    drift, missing = revalidate([sample])
    assert drift == []
    assert missing == [sample.sku_id]


def test_azure_aks_virtual_nodes_filters_by_container_instance_dimension(
    requests_mock: requests_mock_module.Mocker,
) -> None:
    """AKS virtual-node validation maps memory samples to ACI memory pricing."""
    sample = Sample(
        sku_id="azure-aks-vn-linux-eastus",
        region="eastus",
        resource_name="aks-virtual-nodes-linux",
        price_amount=0.00445,
        price_currency="USD",
        dimension="memory",
    )
    requests_mock.get(_AZURE_PRICES_URL, json=_api_response(0.00445))
    drift, missing = revalidate([sample])
    assert drift == []
    assert missing == []
    filt = requests_mock.last_request.qs["$filter"][0]
    assert "servicename eq 'container instances'" in filt
    assert "metername eq 'standard memory duration'" in filt
    assert "skuname eq 'standard'" in filt
