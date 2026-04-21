"""Tests for pipeline.validate.gcp — GCP Cloud Billing revalidator."""

from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest
import requests_mock as requests_mock_module

from validate.gcp import revalidate
from validate.sampler import Sample

_BASE_URL = "https://cloudbilling.googleapis.com/v1/services"

_SAMPLE = Sample(
    sku_id="gcp-gce/n1-standard-2/us-east1",
    region="us-east1",
    resource_name="n1-standard-2",
    price_amount=0.095,
    price_currency="USD",
    dimension="on-demand",
)


def _billing_response(nanos: int, units: int = 0) -> dict:
    """Return a Cloud Billing SKU list response with one item."""
    return {
        "skus": [
            {
                "skuId": "some-sku-id",
                "description": "N1 Standard Instance Core running in Americas",
                "serviceRegions": ["us-east1"],
                "pricingInfo": [
                    {
                        "pricingExpression": {
                            "tieredRates": [
                                {
                                    "startUsageAmount": 0,
                                    "unitPrice": {
                                        "currencyCode": "USD",
                                        "units": str(units),
                                        "nanos": nanos,
                                    },
                                }
                            ]
                        }
                    }
                ],
            }
        ],
        "nextPageToken": "",
    }


def _mock_auth() -> tuple[MagicMock, MagicMock]:
    """Return mock (credentials, project) for google.auth.default."""
    creds = MagicMock()
    creds.token = "mock-token"
    creds.valid = True
    creds.refresh = MagicMock()
    return creds, "mock-project"


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_gcp_no_drift(requests_mock: requests_mock_module.Mocker) -> None:
    """Catalog within 1% -> no drift record."""
    # 95_000_000 nanos = $0.095
    service_id = "6F81-5844-456A"  # GCE compute service
    requests_mock.get(
        f"{_BASE_URL}/{service_id}/skus",
        json=_billing_response(nanos=95_000_000),
    )
    with patch("google.auth.default", return_value=_mock_auth()):
        drift, missing = revalidate([_SAMPLE], service_id=service_id)
    assert drift == []
    assert missing == []


def test_gcp_drift_detected(requests_mock: requests_mock_module.Mocker) -> None:
    """Upstream 10% higher -> drift record."""
    service_id = "6F81-5844-456A"
    # 0.095 * 1.10 = 0.1045 -> 104_500_000 nanos
    upstream_nanos = 104_500_000
    requests_mock.get(
        f"{_BASE_URL}/{service_id}/skus",
        json=_billing_response(nanos=upstream_nanos),
    )
    with patch("google.auth.default", return_value=_mock_auth()):
        drift, missing = revalidate([_SAMPLE], service_id=service_id)
    assert len(drift) == 1
    rec = drift[0]
    assert rec.sku_id == _SAMPLE.sku_id
    assert rec.source == "gcp"
    upstream_expected = upstream_nanos / 1e9
    assert rec.upstream_amount == pytest.approx(upstream_expected)


def test_gcp_missing_upstream(requests_mock: requests_mock_module.Mocker) -> None:
    """Empty skus list -> missing."""
    service_id = "6F81-5844-456A"
    requests_mock.get(
        f"{_BASE_URL}/{service_id}/skus",
        json={"skus": [], "nextPageToken": ""},
    )
    with patch("google.auth.default", return_value=_mock_auth()):
        drift, missing = revalidate([_SAMPLE], service_id=service_id)
    assert drift == []
    assert len(missing) == 1
    assert missing[0] == _SAMPLE.sku_id
