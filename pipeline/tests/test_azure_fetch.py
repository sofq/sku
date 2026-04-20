"""Tests for fetch_prices() in ingest.azure_common (TDD, Task 3 m3a.4.1)."""

from __future__ import annotations

import hashlib
import json

import pytest
import requests_mock as req_mock

import ingest.azure_common as azure_common
from ingest.azure_common import _AZURE_RETAIL_BASE, fetch_prices

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_PAGE2_URL = "https://mocked/page2"

_ITEM1 = {"skuId": "sku-A", "retailPrice": 0.01}
_ITEM2 = {"skuId": "sku-B", "retailPrice": 0.02}
_ITEM3 = {"skuId": "sku-C", "retailPrice": 0.03}
_ITEM4 = {"skuId": "sku-D", "retailPrice": 0.04}
_ITEM5 = {"skuId": "sku-E", "retailPrice": 0.05}

_PAGE1_RESP = {"Items": [_ITEM3, _ITEM1, _ITEM2], "NextPageLink": _PAGE2_URL}
_PAGE2_RESP = {"Items": [_ITEM5, _ITEM4], "NextPageLink": None}


def _sorted_items(items: list[dict]) -> list[dict]:
    return sorted(items, key=lambda x: x.get("skuId", ""))


def _hash_items(items: list[dict]) -> str:
    sorted_items = _sorted_items(items)
    data = json.dumps(sorted_items, separators=(",", ":"), sort_keys=True).encode()
    return hashlib.sha256(data).hexdigest()


# ---------------------------------------------------------------------------
# Test 1: Two-page pagination + stable hash
# ---------------------------------------------------------------------------


def test_two_page_pagination(tmp_path):
    target = tmp_path / "out.json"
    with req_mock.Mocker() as m:
        m.get(_AZURE_RETAIL_BASE, json=_PAGE1_RESP)
        m.get(_PAGE2_URL, json=_PAGE2_RESP)

        digest = fetch_prices("serviceName eq 'Virtual Machines'", target)

    body = json.loads(target.read_text())
    assert list(body.keys()) == ["Items"]
    assert len(body["Items"]) == 5

    expected_order = _sorted_items([_ITEM1, _ITEM2, _ITEM3, _ITEM4, _ITEM5])
    assert body["Items"] == expected_order

    expected_digest = _hash_items([_ITEM1, _ITEM2, _ITEM3, _ITEM4, _ITEM5])
    assert digest == expected_digest


def test_two_page_pagination_hash_stable(tmp_path):
    """Hash is deterministic: two calls with identical data produce same digest."""
    target1 = tmp_path / "out1.json"
    target2 = tmp_path / "out2.json"

    with req_mock.Mocker() as m:
        m.get(_AZURE_RETAIL_BASE, json=_PAGE1_RESP)
        m.get(_PAGE2_URL, json=_PAGE2_RESP)
        digest1 = fetch_prices("serviceName eq 'Virtual Machines'", target1)

    with req_mock.Mocker() as m:
        m.get(_AZURE_RETAIL_BASE, json=_PAGE1_RESP)
        m.get(_PAGE2_URL, json=_PAGE2_RESP)
        digest2 = fetch_prices("serviceName eq 'Virtual Machines'", target2)

    assert digest1 == digest2


# ---------------------------------------------------------------------------
# Test 2: Empty result
# ---------------------------------------------------------------------------


def test_empty_result(tmp_path):
    target = tmp_path / "empty.json"
    with req_mock.Mocker() as m:
        m.get(_AZURE_RETAIL_BASE, json={"Items": [], "NextPageLink": None})

        digest = fetch_prices("serviceName eq 'Nope'", target)

    body = json.loads(target.read_text())
    assert body == {"Items": []}

    expected_digest = hashlib.sha256(
        json.dumps([], separators=(",", ":"), sort_keys=True).encode()
    ).hexdigest()
    assert digest == expected_digest


# ---------------------------------------------------------------------------
# Test 3: Filter URL-encoding
# ---------------------------------------------------------------------------


def test_filter_url_encoding(tmp_path):
    filter_str = "serviceName eq 'Virtual Machines' and armRegionName eq 'eastus'"
    target = tmp_path / "filter.json"

    with req_mock.Mocker() as m:
        m.get(_AZURE_RETAIL_BASE, json={"Items": [], "NextPageLink": None})

        fetch_prices(filter_str, target)

        first_req = m.request_history[0]
        # .qs normalizes casing; check the raw URL for the encoded filter value.
        raw_url = first_req.url
        assert "$filter" in raw_url or "%24filter" in raw_url
        # URL must contain URL-encoded apostrophes (%27) confirming encoding.
        assert "%27" in raw_url
        # And the key names must appear (case-sensitive) in the encoded URL.
        assert "serviceName" in raw_url
        assert "armRegionName" in raw_url


# ---------------------------------------------------------------------------
# Test 4: Page cap exceeded
# ---------------------------------------------------------------------------


def test_page_cap_exceeded(tmp_path, monkeypatch):
    """After _MAX_PAGES pages, raises RuntimeError('page_cap_exceeded')."""
    monkeypatch.setattr(azure_common, "_MAX_PAGES", 3)

    target = tmp_path / "cap.json"

    with req_mock.Mocker() as m:
        # Each page links to the next with a unique URL.
        for i in range(10):
            url = _AZURE_RETAIL_BASE if i == 0 else f"https://mocked/page{i}"
            next_url = f"https://mocked/page{i + 1}"
            m.get(url, json={"Items": [{"skuId": f"sku-{i}"}], "NextPageLink": next_url})

        with pytest.raises(RuntimeError, match="page_cap_exceeded"):
            fetch_prices("serviceName eq 'Anything'", target)


# ---------------------------------------------------------------------------
# Test 5: Retry on 5xx mid-pagination
# ---------------------------------------------------------------------------


def test_retry_on_5xx(tmp_path, monkeypatch):
    """Second page: 500 twice, then 200. Final data is correct; retries are counted."""
    monkeypatch.setattr("ingest.azure_common.time.sleep", lambda *_: None)

    target = tmp_path / "retry.json"
    call_counts: dict[str, int] = {"page2": 0}

    def page2_callback(request, context):
        call_counts["page2"] += 1
        if call_counts["page2"] <= 2:
            context.status_code = 500
            return {"error": "internal"}
        context.status_code = 200
        return _PAGE2_RESP

    with req_mock.Mocker() as m:
        m.get(_AZURE_RETAIL_BASE, json=_PAGE1_RESP)
        m.get(_PAGE2_URL, json=page2_callback)

        digest = fetch_prices("serviceName eq 'Virtual Machines'", target, retries=3)

    assert call_counts["page2"] == 3  # 2 failures + 1 success

    body = json.loads(target.read_text())
    assert len(body["Items"]) == 5

    expected_digest = _hash_items([_ITEM1, _ITEM2, _ITEM3, _ITEM4, _ITEM5])
    assert digest == expected_digest


# ---------------------------------------------------------------------------
# Test 6: User-Agent header
# ---------------------------------------------------------------------------


def test_user_agent_header(tmp_path):
    """Every request (first page + subsequent pages) carries the sku-pipeline User-Agent."""
    target = tmp_path / "ua.json"

    with req_mock.Mocker() as m:
        m.get(_AZURE_RETAIL_BASE, json=_PAGE1_RESP)
        m.get(_PAGE2_URL, json=_PAGE2_RESP)

        fetch_prices("serviceName eq 'VMs'", target)

        for req in m.request_history:
            ua = req.headers.get("User-Agent", "")
            assert "sku-pipeline" in ua, f"Missing sku-pipeline in User-Agent: {ua!r}"
            assert "github.com/sofq/sku" in ua, f"Missing repo URL in User-Agent: {ua!r}"
