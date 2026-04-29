"""Tests for pipeline.validate.openrouter — OpenRouter endpoint revalidator."""

from __future__ import annotations

import pytest
import requests_mock as requests_mock_module

from validate.openrouter import revalidate
from validate.sampler import Sample

_BASE_URL = "https://openrouter.ai/api/v1"
_MODEL_ID = "anthropic/claude-opus-4.6"

_SAMPLE_PROMPT_ANTHROPIC = Sample(
    sku_id=f"{_MODEL_ID}::anthropic::default",
    region="",
    resource_name=_MODEL_ID,
    price_amount=15e-6,
    price_currency="USD",
    dimension="prompt",
)

_SAMPLE_COMPLETION_ANTHROPIC = Sample(
    sku_id=f"{_MODEL_ID}::anthropic::default",
    region="",
    resource_name=_MODEL_ID,
    price_amount=75e-6,
    price_currency="USD",
    dimension="completion",
)

_SAMPLE_PROMPT_BEDROCK = Sample(
    sku_id=f"{_MODEL_ID}::aws-bedrock::default",
    region="",
    resource_name=_MODEL_ID,
    price_amount=15e-6,
    price_currency="USD",
    dimension="prompt",
)

_SAMPLE_AGGREGATED = Sample(
    sku_id=f"{_MODEL_ID}::openrouter::default",
    region="",
    resource_name=_MODEL_ID,
    price_amount=15e-6,
    price_currency="USD",
    dimension="prompt",
)


def _endpoints_response(*endpoints: dict) -> dict:
    return {
        "data": {
            "id": _MODEL_ID,
            "name": "Claude Opus 4.6",
            "endpoints": list(endpoints),
        }
    }


def _endpoint(provider: str, prompt: float, completion: float, *, quantization: str | None = None) -> dict:
    return {
        "tag": provider,
        "provider_name": provider,
        "quantization": quantization,
        "pricing": {
            "prompt": str(prompt),
            "completion": str(completion),
            "image": "0",
            "request": "0",
            "currency": "USD",
        },
    }


def test_no_drift(requests_mock: requests_mock_module.Mocker) -> None:
    requests_mock.get(
        f"{_BASE_URL}/models/{_MODEL_ID}/endpoints",
        json=_endpoints_response(_endpoint("anthropic", 15e-6, 75e-6)),
    )
    drift, missing = revalidate([_SAMPLE_PROMPT_ANTHROPIC])
    assert drift == []
    assert missing == []


def test_drift_detected(requests_mock: requests_mock_module.Mocker) -> None:
    upstream_prompt = 15e-6 * 1.10
    requests_mock.get(
        f"{_BASE_URL}/models/{_MODEL_ID}/endpoints",
        json=_endpoints_response(_endpoint("anthropic", upstream_prompt, 75e-6)),
    )
    drift, missing = revalidate([_SAMPLE_PROMPT_ANTHROPIC])
    assert len(drift) == 1
    rec = drift[0]
    assert rec.sku_id == _SAMPLE_PROMPT_ANTHROPIC.sku_id
    assert rec.source == "openrouter"
    assert rec.upstream_amount == pytest.approx(upstream_prompt)


def test_picks_endpoint_by_serving_provider(
    requests_mock: requests_mock_module.Mocker,
) -> None:
    """Bedrock endpoint at $20 vs Anthropic at $15 — sample is Bedrock, so
    drift must be measured against Bedrock's price, not endpoints[0]."""
    requests_mock.get(
        f"{_BASE_URL}/models/{_MODEL_ID}/endpoints",
        json=_endpoints_response(
            _endpoint("anthropic", 15e-6, 75e-6),
            _endpoint("aws-bedrock", 20e-6, 100e-6),
        ),
    )
    drift, missing = revalidate([_SAMPLE_PROMPT_BEDROCK])
    assert missing == []
    assert len(drift) == 1
    assert drift[0].upstream_amount == pytest.approx(20e-6)


def test_missing_when_no_matching_provider(
    requests_mock: requests_mock_module.Mocker,
) -> None:
    """Sample asks for `aws-bedrock` but upstream only lists `anthropic` —
    treat as missing rather than picking the wrong endpoint."""
    requests_mock.get(
        f"{_BASE_URL}/models/{_MODEL_ID}/endpoints",
        json=_endpoints_response(_endpoint("anthropic", 15e-6, 75e-6)),
    )
    drift, missing = revalidate([_SAMPLE_PROMPT_BEDROCK])
    assert drift == []
    assert missing == [_SAMPLE_PROMPT_BEDROCK.sku_id]


def test_quantization_disambiguates_endpoints(
    requests_mock: requests_mock_module.Mocker,
) -> None:
    """Two endpoints from the same provider, different quantizations: must
    pick the one matching the sku_id's quant slug."""
    requests_mock.get(
        f"{_BASE_URL}/models/{_MODEL_ID}/endpoints",
        json=_endpoints_response(
            _endpoint("anthropic", 10e-6, 50e-6, quantization="fp8"),
            _endpoint("anthropic", 15e-6, 75e-6, quantization=None),
        ),
    )
    sample = Sample(
        sku_id=f"{_MODEL_ID}::anthropic::fp8",
        region="",
        resource_name=_MODEL_ID,
        price_amount=10e-6,
        price_currency="USD",
        dimension="prompt",
    )
    drift, missing = revalidate([sample])
    assert drift == []
    assert missing == []


def test_aggregated_row_uses_model_pricing(
    requests_mock: requests_mock_module.Mocker,
) -> None:
    """`::openrouter::default` is the synthetic aggregated row — validate it
    against the model list's top-level pricing, not against /endpoints."""
    requests_mock.get(
        f"{_BASE_URL}/models",
        json={
            "data": [
                {
                    "id": _MODEL_ID,
                    "pricing": {"prompt": "0.000015", "completion": "0.000075"},
                }
            ]
        },
    )
    drift, missing = revalidate([_SAMPLE_AGGREGATED])
    assert drift == []
    assert missing == []


def test_aggregated_row_drift(requests_mock: requests_mock_module.Mocker) -> None:
    requests_mock.get(
        f"{_BASE_URL}/models",
        json={
            "data": [
                {
                    "id": _MODEL_ID,
                    "pricing": {"prompt": "0.000018", "completion": "0.000075"},
                }
            ]
        },
    )
    drift, missing = revalidate([_SAMPLE_AGGREGATED])
    assert missing == []
    assert len(drift) == 1
    assert drift[0].upstream_amount == pytest.approx(18e-6)


def test_endpoints_response_with_no_endpoints(
    requests_mock: requests_mock_module.Mocker,
) -> None:
    requests_mock.get(
        f"{_BASE_URL}/models/{_MODEL_ID}/endpoints",
        json={"data": {"id": _MODEL_ID, "endpoints": []}},
    )
    drift, missing = revalidate([_SAMPLE_PROMPT_ANTHROPIC])
    assert drift == []
    assert missing == [_SAMPLE_PROMPT_ANTHROPIC.sku_id]


def test_both_dimensions_share_endpoint_call(
    requests_mock: requests_mock_module.Mocker,
) -> None:
    """Prompt in-tolerance, completion drifted, both samples on the same
    (model, provider) — only one /endpoints call should be made (cached)."""
    upstream_completion = 75e-6 * 1.20
    matcher = requests_mock.get(
        f"{_BASE_URL}/models/{_MODEL_ID}/endpoints",
        json=_endpoints_response(_endpoint("anthropic", 15e-6, upstream_completion)),
    )
    drift, missing = revalidate([_SAMPLE_PROMPT_ANTHROPIC, _SAMPLE_COMPLETION_ANTHROPIC])
    assert len(drift) == 1
    assert drift[0].sku_id == _SAMPLE_COMPLETION_ANTHROPIC.sku_id
    assert missing == []
    assert matcher.call_count == 1


def test_malformed_sku_id_is_missing() -> None:
    """A row with the wrong number of `::` segments shouldn't crash the
    validator — record it as missing."""
    bad = Sample(
        sku_id="malformed-sku-id",
        region="",
        resource_name="malformed-sku-id",
        price_amount=1e-6,
        price_currency="USD",
        dimension="prompt",
    )
    drift, missing = revalidate([bad])
    assert drift == []
    assert missing == ["malformed-sku-id"]
