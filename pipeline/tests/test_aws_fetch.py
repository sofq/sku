"""Tests for fetch_offer() in ingest.aws_common."""

from __future__ import annotations

import json
from pathlib import Path

import pytest
import requests_mock as req_mock

from ingest.aws_common import fetch_offer

_EC2_URL = "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/index.json"
_BODY = json.dumps(
    {
        "publicationDate": "2026-04-18T00:00:00Z",
        "offers": {"AmazonEC2": {}},
    }
).encode()


@pytest.fixture()
def _patch_sleep(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr("ingest.aws_common.time.sleep", lambda _x: None)


# ---------------------------------------------------------------------------
# 1. Happy path
# ---------------------------------------------------------------------------


def test_happy_path(tmp_path: Path) -> None:
    target = tmp_path / "offer.json"
    with req_mock.Mocker() as m:
        m.get(
            _EC2_URL,
            content=_BODY,
            headers={"Content-Length": str(len(_BODY))},
        )
        pub_date = fetch_offer("aws_ec2", target)

    assert pub_date == "2026-04-18T00:00:00Z"
    assert target.exists()
    assert target.read_bytes() == _BODY


# ---------------------------------------------------------------------------
# 2. EBS shares EC2 offer URL
# ---------------------------------------------------------------------------


def test_ebs_shares_ec2_url(tmp_path: Path) -> None:
    target = tmp_path / "offer.json"
    with req_mock.Mocker() as m:
        m.get(
            _EC2_URL,
            content=_BODY,
            headers={"Content-Length": str(len(_BODY))},
        )
        fetch_offer("aws_ebs", target)
        assert m.last_request.url == _EC2_URL


# ---------------------------------------------------------------------------
# 3. 404 — no retry on 4xx
# ---------------------------------------------------------------------------


def test_404_raises_no_retry(tmp_path: Path) -> None:
    target = tmp_path / "offer.json"
    with req_mock.Mocker() as m:
        m.get(_EC2_URL, status_code=404)
        with pytest.raises(RuntimeError, match=r"https://"):
            fetch_offer("aws_ec2", target)
        assert m.call_count == 1


# ---------------------------------------------------------------------------
# 4. 500 → 500 → 200 (two retries then success)
# ---------------------------------------------------------------------------


def test_500_then_success(tmp_path: Path, _patch_sleep: None) -> None:
    target = tmp_path / "offer.json"
    responses = [
        {"status_code": 500},
        {"status_code": 500},
        {
            "status_code": 200,
            "content": _BODY,
            "headers": {"Content-Length": str(len(_BODY))},
        },
    ]
    with req_mock.Mocker() as m:
        m.get(_EC2_URL, response_list=responses)
        pub_date = fetch_offer("aws_ec2", target)
        assert pub_date == "2026-04-18T00:00:00Z"
        assert m.call_count == 3


# ---------------------------------------------------------------------------
# 5. 500 all 3 → RuntimeError; call_count == retries (3)
# ---------------------------------------------------------------------------


def test_500_exhausted(tmp_path: Path, _patch_sleep: None) -> None:
    target = tmp_path / "offer.json"
    with req_mock.Mocker() as m:
        m.get(_EC2_URL, status_code=500)
        with pytest.raises(RuntimeError):
            fetch_offer("aws_ec2", target, retries=3)
        assert m.call_count == 3


# ---------------------------------------------------------------------------
# 6. Truncated body — .part and target must not exist after failure
# ---------------------------------------------------------------------------


def test_truncated_body(tmp_path: Path) -> None:
    target = tmp_path / "offer.json"
    short_body = _BODY[:50]
    with req_mock.Mocker() as m:
        m.get(
            _EC2_URL,
            content=short_body,
            headers={"Content-Length": "1000"},
        )
        with pytest.raises(RuntimeError, match="truncated"):
            fetch_offer("aws_ec2", target)

    assert not target.exists()
    assert not target.with_suffix(target.suffix + ".part").exists()


# ---------------------------------------------------------------------------
# 7. User-Agent header
# ---------------------------------------------------------------------------


def test_user_agent(tmp_path: Path) -> None:
    target = tmp_path / "offer.json"
    with req_mock.Mocker() as m:
        m.get(
            _EC2_URL,
            content=_BODY,
            headers={"Content-Length": str(len(_BODY))},
        )
        fetch_offer("aws_ec2", target)
        ua = m.last_request.headers.get("User-Agent", "")

    assert ua.startswith("sku-pipeline/")
    assert "https://github.com/sofq/sku" in ua
