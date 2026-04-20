"""Unit tests for pipeline.discover.{aws,azure,gcp,openrouter}."""

from __future__ import annotations

import pytest
import requests_mock

from discover import aws as aws_disc
from discover import azure as azure_disc
from discover import gcp as gcp_disc
from discover import openrouter as or_disc
from ingest.aws_common import _AWS_OFFER_BASE
from ingest.azure_common import _AZURE_RETAIL_BASE
from ingest.gcp_common import _GCP_BILLING_BASE, _GCP_SERVICE_IDS

# -----------------------------------------------------------------------------
# AWS discover
# -----------------------------------------------------------------------------


def test_aws_discover_returns_publicationDate():
    with requests_mock.Mocker() as m:
        m.get(
            f"{_AWS_OFFER_BASE}/AmazonEC2/index.json",
            json={"publicationDate": "2026-04-18T00:00:00Z"},
        )
        m.get(
            f"{_AWS_OFFER_BASE}/AmazonRDS/index.json",
            json={"publicationDate": "2026-04-17T00:00:00Z"},
        )
        result = aws_disc.discover(["aws_ec2", "aws_rds"])
    assert result == {
        "aws_ec2": "2026-04-18T00:00:00Z",
        "aws_rds": "2026-04-17T00:00:00Z",
    }


def test_aws_discover_ebs_maps_to_ec2_endpoint():
    with requests_mock.Mocker() as m:
        m.get(
            f"{_AWS_OFFER_BASE}/AmazonEC2/index.json",
            json={"publicationDate": "2026-04-18T00:00:00Z"},
        )
        result = aws_disc.discover(["aws_ebs"])
    assert result == {"aws_ebs": "2026-04-18T00:00:00Z"}


def test_aws_discover_http_failure_raises_runtimeerror():
    with requests_mock.Mocker() as m:
        m.get(f"{_AWS_OFFER_BASE}/AmazonEC2/index.json", status_code=500)
        with pytest.raises(RuntimeError, match="aws_discover_http_500"):
            aws_disc.discover(["aws_ec2"])


def test_aws_discover_indicator_changes_when_upstream_changes():
    with requests_mock.Mocker() as m:
        m.get(
            f"{_AWS_OFFER_BASE}/AmazonEC2/index.json",
            json={"publicationDate": "2026-04-18T00:00:00Z"},
        )
        a = aws_disc.discover(["aws_ec2"])
    with requests_mock.Mocker() as m:
        m.get(
            f"{_AWS_OFFER_BASE}/AmazonEC2/index.json",
            json={"publicationDate": "2026-04-19T00:00:00Z"},
        )
        b = aws_disc.discover(["aws_ec2"])
    assert a["aws_ec2"] != b["aws_ec2"]


def test_aws_discover_unknown_shard_raises_keyerror():
    with pytest.raises(KeyError):
        aws_disc.discover(["aws_nope"])


# -----------------------------------------------------------------------------
# Azure discover
# -----------------------------------------------------------------------------


def test_azure_discover_hashes_top_one_page():
    with requests_mock.Mocker() as m:
        m.get(_AZURE_RETAIL_BASE, json={"Items": [{"skuId": "abc", "retailPrice": 0.1}]})
        result = azure_disc.discover(["azure_vm"])
    assert set(result) == {"azure_vm"}
    assert len(result["azure_vm"]) == 64  # sha256 hex length


def test_azure_discover_indicator_changes_when_upstream_changes():
    with requests_mock.Mocker() as m:
        m.get(_AZURE_RETAIL_BASE, json={"Items": [{"skuId": "a"}]})
        a = azure_disc.discover(["azure_vm"])
    with requests_mock.Mocker() as m:
        m.get(_AZURE_RETAIL_BASE, json={"Items": [{"skuId": "b"}]})
        b = azure_disc.discover(["azure_vm"])
    assert a["azure_vm"] != b["azure_vm"]


def test_azure_discover_http_failure_raises():
    with requests_mock.Mocker() as m:
        m.get(_AZURE_RETAIL_BASE, status_code=503)
        with pytest.raises(RuntimeError, match="azure_discover_http_503"):
            azure_disc.discover(["azure_vm"])


def test_azure_discover_unknown_shard_raises_keyerror():
    with pytest.raises(KeyError):
        azure_disc.discover(["azure_nope"])


def test_azure_discover_uses_top_one_param():
    with requests_mock.Mocker() as m:
        m.get(_AZURE_RETAIL_BASE, json={"Items": []})
        azure_disc.discover(["azure_vm"])
        qs = m.request_history[0].qs
    assert qs.get("$top") == ["1"]


# -----------------------------------------------------------------------------
# GCP discover
# -----------------------------------------------------------------------------


def test_gcp_discover_hashes_first_sku():
    service_id = _GCP_SERVICE_IDS["gcp_gce"]
    with requests_mock.Mocker() as m:
        m.get(
            f"{_GCP_BILLING_BASE}/services/{service_id}/skus",
            json={"skus": [{"skuId": "one"}]},
        )
        result = gcp_disc.discover(["gcp_gce"], api_key="k")
    assert result["gcp_gce"].startswith("sha256:")
    assert len(result["gcp_gce"]) == len("sha256:") + 64


def test_gcp_discover_hash_is_deterministic():
    service_id = _GCP_SERVICE_IDS["gcp_gce"]
    with requests_mock.Mocker() as m:
        m.get(
            f"{_GCP_BILLING_BASE}/services/{service_id}/skus",
            json={"skus": [{"skuId": "one"}]},
        )
        first = gcp_disc.discover(["gcp_gce"], api_key="k")
        second = gcp_disc.discover(["gcp_gce"], api_key="k")
    assert first == second


def test_gcp_discover_hash_changes_when_first_sku_changes():
    service_id = _GCP_SERVICE_IDS["gcp_gce"]
    with requests_mock.Mocker() as m:
        m.get(
            f"{_GCP_BILLING_BASE}/services/{service_id}/skus",
            json={"skus": [{"skuId": "one"}]},
        )
        before = gcp_disc.discover(["gcp_gce"], api_key="k")
    with requests_mock.Mocker() as m:
        m.get(
            f"{_GCP_BILLING_BASE}/services/{service_id}/skus",
            json={"skus": [{"skuId": "two"}]},
        )
        after = gcp_disc.discover(["gcp_gce"], api_key="k")
    assert before["gcp_gce"] != after["gcp_gce"]


def test_gcp_discover_sends_api_key_and_pagesize():
    service_id = _GCP_SERVICE_IDS["gcp_gce"]
    with requests_mock.Mocker() as m:
        m.get(
            f"{_GCP_BILLING_BASE}/services/{service_id}/skus",
            json={"skus": []},
        )
        gcp_disc.discover(["gcp_gce"], api_key="secret-key")
        qs = m.request_history[0].qs
    assert qs.get("key") == ["secret-key"]
    assert qs.get("pagesize") == ["1"]


def test_gcp_discover_http_failure_raises():
    service_id = _GCP_SERVICE_IDS["gcp_gce"]
    with requests_mock.Mocker() as m:
        m.get(f"{_GCP_BILLING_BASE}/services/{service_id}/skus", status_code=403)
        with pytest.raises(RuntimeError, match="gcp_discover_http_403"):
            gcp_disc.discover(["gcp_gce"], api_key="k")


def test_gcp_discover_unknown_shard_raises_keyerror():
    with pytest.raises(KeyError):
        gcp_disc.discover(["gcp_nope"], api_key="k")


# -----------------------------------------------------------------------------
# OpenRouter discover
# -----------------------------------------------------------------------------


def test_openrouter_discover_hashes_sorted_model_ids():
    with requests_mock.Mocker() as m:
        m.get(
            "https://openrouter.ai/api/v1/models",
            json={"data": [{"id": "b/two"}, {"id": "a/one"}, {"id": "c/three"}]},
        )
        result = or_disc.discover(["openrouter"])
    assert set(result) == {"openrouter"}
    assert len(result["openrouter"]) == 64


def test_openrouter_discover_stable_and_order_independent():
    with requests_mock.Mocker() as m:
        m.get(
            "https://openrouter.ai/api/v1/models",
            json={"data": [{"id": "a"}, {"id": "b"}]},
        )
        a = or_disc.discover(["openrouter"])
    with requests_mock.Mocker() as m:
        m.get(
            "https://openrouter.ai/api/v1/models",
            json={"data": [{"id": "b"}, {"id": "a"}]},
        )
        b = or_disc.discover(["openrouter"])
    assert a == b


def test_openrouter_discover_indicator_changes_when_models_change():
    with requests_mock.Mocker() as m:
        m.get("https://openrouter.ai/api/v1/models", json={"data": [{"id": "a"}]})
        a = or_disc.discover(["openrouter"])
    with requests_mock.Mocker() as m:
        m.get(
            "https://openrouter.ai/api/v1/models",
            json={"data": [{"id": "a"}, {"id": "b"}]},
        )
        b = or_disc.discover(["openrouter"])
    assert a["openrouter"] != b["openrouter"]


def test_openrouter_discover_unknown_shard_raises_keyerror():
    with pytest.raises(KeyError):
        or_disc.discover(["not-openrouter"])


def test_openrouter_discover_empty_shards_returns_empty():
    result = or_disc.discover([])
    assert result == {}
