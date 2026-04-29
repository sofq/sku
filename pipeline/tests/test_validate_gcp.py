"""Tests for pipeline.validate.gcp — GCP Cloud Billing revalidator."""

from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest
import requests_mock as requests_mock_module

from validate.gcp import revalidate
from validate.sampler import Sample

_BASE_URL = "https://cloudbilling.googleapis.com/v1/services"

_SAMPLE = Sample(
    sku_id="some-sku-id:n1-standard-2",
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


def _gke_billing_response() -> dict:
    """Return GKE SKUs where the first positive regional SKU is not the match."""
    return {
        "skus": [
            {
                "skuId": "6B92-A835-08AB",
                "description": "Zonal Kubernetes Clusters",
                "serviceRegions": ["global"],
                "pricingInfo": [
                    {"pricingExpression": {"tieredRates": [{"unitPrice": {"units": "0", "nanos": 100_000_000}}]}}
                ],
            },
            {
                "skuId": "B561-BFBD-1264",
                "description": "Regional Kubernetes Clusters",
                "serviceRegions": ["global"],
                "pricingInfo": [
                    {"pricingExpression": {"tieredRates": [{"unitPrice": {"units": "0", "nanos": 100_000_000}}]}}
                ],
            },
            {
                "skuId": "CA45-51A5-8C74",
                "description": "Autopilot Pod mCPU Requests (us-east1)",
                "serviceRegions": ["us-east1"],
                "pricingInfo": [
                    {"pricingExpression": {"tieredRates": [{"unitPrice": {"units": "0", "nanos": 44_500}}]}}
                ],
            },
            {
                "skuId": "9FF0-AB0A-36E8",
                "description": "Autopilot Pod Memory Requests (us-east1)",
                "serviceRegions": ["us-east1"],
                "pricingInfo": [
                    {"pricingExpression": {"tieredRates": [{"unitPrice": {"units": "0", "nanos": 4_922_500}}]}}
                ],
            },
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


def test_gcp_gke_standard_matches_regional_cluster_sku_even_when_global(
    requests_mock: requests_mock_module.Mocker,
) -> None:
    """GKE standard validation must select Regional Kubernetes Clusters."""
    service_id = "CCD8-9BF1-090E"
    sample = Sample(
        sku_id="B561-BFBD-1264-us-east1",
        region="us-east1",
        resource_name="gke-standard",
        price_amount=0.10,
        price_currency="USD",
        dimension="cluster",
    )
    requests_mock.get(f"{_BASE_URL}/{service_id}/skus", json=_gke_billing_response())
    with patch("google.auth.default", return_value=_mock_auth()):
        drift, missing = revalidate([sample], service_id=service_id)
    assert drift == []
    assert missing == []


def test_gcp_unmatched_sku_is_missing_not_drift(
    requests_mock: requests_mock_module.Mocker,
) -> None:
    """When the catalog sku_id has no exact match in the queried service, mark
    as missing — never fall back to the first SKU that happens to be in the
    same region (that produces wildly wrong drift records, e.g. comparing a
    Spanner PU-hour price to an unrelated GCE compute SKU).
    """
    service_id = "CC63-0873-48FD"  # Spanner
    # Catalog sku_id won't match any upstream skuId in this response.
    sample = Sample(
        sku_id="0E4C-7EAD-157A",
        region="us-east1",
        resource_name="spanner-standard",
        price_amount=0.41,
        price_currency="USD",
        dimension="compute",
    )
    requests_mock.get(
        f"{_BASE_URL}/{service_id}/skus",
        json={
            "skus": [
                {
                    "skuId": "DIFFERENT-SKU-ID",
                    "description": "Some other Spanner SKU",
                    "serviceRegions": ["us-east1"],
                    "pricingInfo": [
                        {"pricingExpression": {"tieredRates": [{"unitPrice": {"units": "0", "nanos": 80_000_000}}]}}
                    ],
                }
            ],
            "nextPageToken": "",
        },
    )
    with patch("google.auth.default", return_value=_mock_auth()):
        drift, missing = revalidate([sample], service_id=service_id)
    assert drift == []
    assert missing == [sample.sku_id]


def test_gcp_region_suffixed_catalog_id_matches_upstream(
    requests_mock: requests_mock_module.Mocker,
) -> None:
    """Memorystore stores ``{skuId}-{region}`` for multi-region SKUs (per
    pipeline/ingest/gcp_memorystore.py). The validator must recognise this
    suffix scheme and match against the bare upstream skuId.
    """
    service_id = "5AF5-2C11-D467"  # Memorystore Redis
    sample = Sample(
        sku_id="4ADD-4226-A7A7-us-east1",
        region="us-east1",
        resource_name="memorystore-redis-standard-5gb",
        price_amount=0.054,
        price_currency="USD",
        dimension="compute",
    )
    requests_mock.get(
        f"{_BASE_URL}/{service_id}/skus",
        json={
            "skus": [
                {
                    "skuId": "0000-AAAA-BBBB",
                    "description": "Some Memorystore SKU we don't want",
                    "serviceRegions": ["us-east1"],
                    "pricingInfo": [
                        {"pricingExpression": {"tieredRates": [{"unitPrice": {"units": "9", "nanos": 0}}]}}
                    ],
                },
                {
                    "skuId": "4ADD-4226-A7A7",
                    "description": "Memorystore Redis Standard 5GB Capacity",
                    "serviceRegions": ["us-east1", "us-east4"],
                    "pricingInfo": [
                        {"pricingExpression": {"tieredRates": [{"unitPrice": {"units": "0", "nanos": 54_000_000}}]}}
                    ],
                },
            ],
            "nextPageToken": "",
        },
    )
    with patch("google.auth.default", return_value=_mock_auth()):
        drift, missing = revalidate([sample], service_id=service_id)
    assert drift == []
    assert missing == []


def test_gcp_gke_autopilot_matches_requested_dimension(
    requests_mock: requests_mock_module.Mocker,
) -> None:
    """GKE Autopilot memory validation must not use the first regional price."""
    service_id = "CCD8-9BF1-090E"
    sample = Sample(
        sku_id="gke-autopilot-us-east1",
        region="us-east1",
        resource_name="gke-autopilot",
        price_amount=0.0049225,
        price_currency="USD",
        dimension="memory",
    )
    requests_mock.get(f"{_BASE_URL}/{service_id}/skus", json=_gke_billing_response())
    with patch("google.auth.default", return_value=_mock_auth()):
        drift, missing = revalidate([sample], service_id=service_id)
    assert drift == []
    assert missing == []
