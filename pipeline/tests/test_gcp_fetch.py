"""Tests for fetch_skus() in ingest.gcp_common.

Uses requests_mock to simulate GCP Cloud Billing Catalog API responses.
"""

from __future__ import annotations

import hashlib
import json
from urllib.parse import parse_qs, urlparse

import pytest

from ingest.gcp_common import fetch_skus

_GCE_SERVICE_ID = "6F81-5844-456A"
_GCE_BASE_URL = f"https://cloudbilling.googleapis.com/v1/services/{_GCE_SERVICE_ID}/skus"

_SKU_A = {"skuId": "AAA-111", "description": "sku-a"}
_SKU_B = {"skuId": "BBB-222", "description": "sku-b"}
_SKU_C = {"skuId": "CCC-333", "description": "sku-c"}


def _build_sorted_skus(items: list[dict]) -> list[dict]:
    return sorted(items, key=lambda x: x.get("skuId", ""))


def _compute_hash(sorted_skus: list[dict]) -> str:
    serialized = json.dumps(sorted_skus, separators=(",", ":"), sort_keys=True).encode()
    return hashlib.sha256(serialized).hexdigest()


def test_three_page_pagination(requests_mock, tmp_path):
    """Three pages via nextPageToken — all 3 skus written, sorted by skuId."""
    requests_mock.get(
        _GCE_BASE_URL,
        [
            {"json": {"skus": [_SKU_B], "nextPageToken": "tok1"}},
            {"json": {"skus": [_SKU_A], "nextPageToken": "tok2"}},
            {"json": {"skus": [_SKU_C]}},
        ],
    )
    target = tmp_path / "gcp_gce_raw.json"
    fetch_skus("gcp_gce", target)

    assert target.exists()
    data = json.loads(target.read_text())
    assert list(data.keys()) == ["skus"]
    assert data["skus"] == _build_sorted_skus([_SKU_B, _SKU_A, _SKU_C])
    assert requests_mock.call_count == 3


def test_no_api_key_in_querystring(requests_mock, tmp_path):
    """fetch_skus must not send `?key=<...>` — auth goes via Bearer header."""
    requests_mock.get(
        _GCE_BASE_URL,
        [
            {"json": {"skus": [_SKU_A], "nextPageToken": "tok1"}},
            {"json": {"skus": [_SKU_B]}},
        ],
    )
    target = tmp_path / "out.json"
    fetch_skus("gcp_gce", target)

    assert requests_mock.call_count == 2
    for req in requests_mock.request_history:
        qs = parse_qs(urlparse(req.url).query)
        assert "key" not in qs, f"unexpected key= in querystring: {req.url}"


def test_gcp_gce_service_id_in_url(requests_mock, tmp_path):
    """`fetch_skus('gcp_gce', ...)` hits a URL containing the correct service id."""
    requests_mock.get(_GCE_BASE_URL, json={"skus": [_SKU_A]})
    target = tmp_path / "out.json"
    fetch_skus("gcp_gce", target)

    assert requests_mock.call_count == 1
    assert f"/services/{_GCE_SERVICE_ID}/skus" in requests_mock.request_history[0].url


def test_403_raises_gcp_forbidden_no_retry(requests_mock, tmp_path):
    """403 response raises RuntimeError('gcp_forbidden') immediately, no retries."""
    requests_mock.get(_GCE_BASE_URL, status_code=403)
    target = tmp_path / "out.json"

    with pytest.raises(RuntimeError) as exc_info:
        fetch_skus("gcp_gce", target)

    err_msg = str(exc_info.value)
    assert "gcp_forbidden" in err_msg
    assert _GCE_SERVICE_ID in err_msg
    assert requests_mock.call_count == 1


def test_500_then_200_succeeds_with_retry(requests_mock, tmp_path):
    """A single 500 on the first page is retried and succeeds on the second attempt."""
    requests_mock.get(
        _GCE_BASE_URL,
        [
            {"status_code": 500},
            {"json": {"skus": [_SKU_A]}},
        ],
    )
    target = tmp_path / "out.json"
    fetch_skus("gcp_gce", target, retries=3)

    assert target.exists()
    data = json.loads(target.read_text())
    assert data["skus"] == [_SKU_A]
    # 1 failed attempt + 1 successful attempt = 2 calls
    assert requests_mock.call_count == 2


def test_hash_stability(requests_mock, tmp_path):
    """Running twice with identical mock data returns the identical digest."""
    requests_mock.get(_GCE_BASE_URL, json={"skus": [_SKU_B, _SKU_A]})
    target1 = tmp_path / "out1.json"
    target2 = tmp_path / "out2.json"

    digest1 = fetch_skus("gcp_gce", target1)
    digest2 = fetch_skus("gcp_gce", target2)

    assert digest1 == digest2
    assert len(digest1) == 64  # SHA256 hex


def test_unknown_shard_raises_key_error(tmp_path):
    """Unknown shard propagates KeyError from the dict lookup (no request made)."""
    target = tmp_path / "out.json"
    with pytest.raises(KeyError):
        fetch_skus("gcp_unknown", target)


def test_user_agent_on_every_request(requests_mock, tmp_path):
    """User-Agent header matching sku-pipeline/... is present on every request."""
    requests_mock.get(
        _GCE_BASE_URL,
        [
            {"json": {"skus": [_SKU_A], "nextPageToken": "tok1"}},
            {"json": {"skus": [_SKU_B]}},
        ],
    )
    target = tmp_path / "out.json"
    fetch_skus("gcp_gce", target)

    assert requests_mock.call_count == 2
    for req in requests_mock.request_history:
        ua = req.headers.get("User-Agent", "")
        assert ua.startswith("sku-pipeline/"), f"unexpected User-Agent: {ua!r}"
