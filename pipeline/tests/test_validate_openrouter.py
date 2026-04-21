"""Tests for pipeline.validate.openrouter — OpenRouter endpoint revalidator."""

from __future__ import annotations

import pytest
import requests_mock as requests_mock_module

from validate.openrouter import revalidate
from validate.sampler import Sample

_BASE_URL = "https://openrouter.ai/api/v1/models"

_SAMPLE_PROMPT = Sample(
    sku_id="openrouter/anthropic/claude-opus-4.6/anthropic",
    region="",
    resource_name="anthropic/claude-opus-4.6",
    price_amount=15e-6,   # $15 per 1M tokens -> $0.000015 per token
    price_currency="USD",
    dimension="prompt",
)

_SAMPLE_COMPLETION = Sample(
    sku_id="openrouter/anthropic/claude-opus-4.6/anthropic",
    region="",
    resource_name="anthropic/claude-opus-4.6",
    price_amount=75e-6,
    price_currency="USD",
    dimension="completion",
)


def _endpoint_response(prompt_price: float, completion_price: float) -> dict:
    return {
        "data": [
            {
                "id": "anthropic/claude-opus-4.6",
                "name": "Claude Opus 4.6",
                "pricing": {
                    "prompt": str(prompt_price),
                    "completion": str(completion_price),
                },
                "provider": "anthropic",
            }
        ]
    }


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_openrouter_no_drift(requests_mock: requests_mock_module.Mocker) -> None:
    """Exact match -> no drift."""
    requests_mock.get(
        f"{_BASE_URL}/anthropic/claude-opus-4.6/endpoints",
        json=_endpoint_response(15e-6, 75e-6),
    )
    drift, missing = revalidate([_SAMPLE_PROMPT])
    assert drift == []
    assert missing == []


def test_openrouter_drift_detected(requests_mock: requests_mock_module.Mocker) -> None:
    """Upstream prompt price 10% higher -> drift record."""
    upstream_prompt = 15e-6 * 1.10
    requests_mock.get(
        f"{_BASE_URL}/anthropic/claude-opus-4.6/endpoints",
        json=_endpoint_response(upstream_prompt, 75e-6),
    )
    drift, missing = revalidate([_SAMPLE_PROMPT])
    assert len(drift) == 1
    rec = drift[0]
    assert rec.sku_id == _SAMPLE_PROMPT.sku_id
    assert rec.source == "openrouter"
    assert rec.upstream_amount == pytest.approx(upstream_prompt)


def test_openrouter_missing_upstream(requests_mock: requests_mock_module.Mocker) -> None:
    """404 or empty response -> missing."""
    requests_mock.get(
        f"{_BASE_URL}/anthropic/claude-opus-4.6/endpoints",
        json={"data": []},
    )
    drift, missing = revalidate([_SAMPLE_PROMPT])
    assert drift == []
    assert len(missing) == 1


def test_openrouter_both_dimensions(requests_mock: requests_mock_module.Mocker) -> None:
    """Prompt in-tolerance, completion drifted -> 1 drift record."""
    upstream_completion = 75e-6 * 1.20
    requests_mock.get(
        f"{_BASE_URL}/anthropic/claude-opus-4.6/endpoints",
        [
            {"json": _endpoint_response(15e-6, 75e-6)},         # for prompt sample
            {"json": _endpoint_response(15e-6, upstream_completion)},  # for completion sample
        ],
    )
    drift, missing = revalidate([_SAMPLE_PROMPT, _SAMPLE_COMPLETION])
    assert len(drift) == 1
    assert drift[0].sku_id == _SAMPLE_COMPLETION.sku_id
    assert missing == []
