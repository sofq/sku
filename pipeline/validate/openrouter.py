"""OpenRouter endpoint revalidator.

Per-sample serving-provider-specific revalidation:

* Non-aggregated rows (``sku_id = "{model}::{serving_provider}::{quant}"``):
  fetch ``GET /api/v1/models/{model}/endpoints`` and match the endpoint whose
  ``tag`` (or ``provider_name``) equals ``serving_provider`` — and, when the
  catalog row pinned a real quantization, the endpoint's ``quantization`` too.

* Aggregated rows (``::openrouter::default``): the ingest synthesises these
  from the model's top-level ``pricing``. Re-fetch the model list once and
  compare against ``model.pricing``.

Drift threshold is 1% per dimension; samples with ambiguous or missing
upstream rows are returned in the ``missing`` list (shard-freshness, not a
mispricing).
"""

from __future__ import annotations

import logging
from dataclasses import dataclass
from typing import Any

import requests

from validate.sampler import Sample

logger = logging.getLogger(__name__)

_OPENROUTER_BASE = "https://openrouter.ai/api/v1"
_DRIFT_THRESHOLD = 0.01  # 1%
_AGGREGATED_PROVIDER = "openrouter"


@dataclass
class DriftRecord:
    """A single price-drift observation."""

    sku_id: str
    catalog_amount: float
    upstream_amount: float
    delta_pct: float
    source: str = "openrouter"


def _parse_sku_id(sku_id: str) -> tuple[str, str, str] | None:
    """Split ``"{model}::{serving_provider}::{quant}"``.

    Returns ``None`` if the sku_id doesn't match the expected three-segment
    form (defensive — the catalog has been writing this shape since M1, but a
    malformed row should be reported as missing rather than crashing).
    """
    parts = sku_id.split("::")
    if len(parts) != 3:
        return None
    model_id, serving_provider, quant = parts
    if not model_id or not serving_provider:
        return None
    return model_id, serving_provider, quant


def _endpoint_matches(endpoint: dict[str, Any], serving_provider: str, quant: str) -> bool:
    """Match an endpoint dict against (serving_provider, quant) from the sku_id.

    The ingest writes ``serving_provider = endpoint.tag or endpoint.provider_name``
    and ``quant_slug = endpoint.quantization or "default"``. Mirror that here
    rather than assuming endpoint ordering — OpenRouter returns endpoints in an
    unspecified order, and a model often has Bedrock + Anthropic + Vertex
    endpoints all priced differently.
    """
    tag = endpoint.get("tag") or endpoint.get("provider_name")
    if tag != serving_provider:
        return False
    ep_quant = endpoint.get("quantization") or "default"
    return ep_quant == quant


def _coerce_price(raw: Any) -> float | None:
    if raw is None or raw == "":
        return None
    try:
        return float(raw)
    except (ValueError, TypeError):
        return None


def revalidate(
    samples: list[Sample],
    *,
    session: requests.Session | None = None,
) -> tuple[list[DriftRecord], list[str]]:
    """Re-fetch each sample from OpenRouter and emit drift records.

    Caches per-model endpoint responses (and the full model list, fetched
    lazily for aggregated samples) so a 20-sample batch hitting the same
    model only makes one HTTP call.
    """
    if session is None:
        session = requests.Session()

    drift: list[DriftRecord] = []
    missing: list[str] = []

    endpoints_cache: dict[str, list[dict[str, Any]] | None] = {}
    models_cache: dict[str, dict[str, Any]] | None = None

    def _load_endpoints(model_id: str) -> list[dict[str, Any]] | None:
        if model_id in endpoints_cache:
            return endpoints_cache[model_id]
        url = f"{_OPENROUTER_BASE}/models/{model_id}/endpoints"
        try:
            resp = session.get(url, timeout=15)
            resp.raise_for_status()
            payload = resp.json()
        except Exception:
            logger.exception("OpenRouter endpoints API failed for %s", model_id)
            endpoints_cache[model_id] = None
            return None
        body = payload.get("data") or {}
        if isinstance(body, list):
            # Defensive: if OpenRouter ever flattens the response, accept it.
            endpoints = body
        else:
            endpoints = body.get("endpoints") or []
        endpoints_cache[model_id] = endpoints
        return endpoints

    def _load_models() -> dict[str, dict[str, Any]] | None:
        nonlocal models_cache
        if models_cache is not None:
            return models_cache
        url = f"{_OPENROUTER_BASE}/models"
        try:
            resp = session.get(url, timeout=15)
            resp.raise_for_status()
            payload = resp.json()
        except Exception:
            logger.exception("OpenRouter models API failed")
            models_cache = {}
            return None
        models_cache = {m["id"]: m for m in payload.get("data", []) if "id" in m}
        return models_cache

    for s in samples:
        parsed = _parse_sku_id(s.sku_id)
        if parsed is None:
            logger.debug("Unparseable sku_id %s", s.sku_id)
            missing.append(s.sku_id)
            continue
        model_id, serving_provider, quant = parsed

        if serving_provider == _AGGREGATED_PROVIDER:
            models = _load_models()
            if not models:
                missing.append(s.sku_id)
                continue
            model = models.get(model_id)
            if not model:
                missing.append(s.sku_id)
                continue
            pricing = model.get("pricing") or {}
        else:
            endpoints = _load_endpoints(model_id)
            if not endpoints:
                missing.append(s.sku_id)
                continue
            match = next(
                (ep for ep in endpoints if _endpoint_matches(ep, serving_provider, quant)),
                None,
            )
            if match is None:
                logger.debug(
                    "No upstream endpoint matched %s (provider=%s, quant=%s)",
                    s.sku_id,
                    serving_provider,
                    quant,
                )
                missing.append(s.sku_id)
                continue
            pricing = match.get("pricing") or {}

        upstream = _coerce_price(pricing.get(s.dimension))
        if upstream is None or upstream == 0:
            # Catalog publishes only non-zero dimensions, so a zero upstream
            # is either a freshness gap or a mid-rollout pricing change. Log
            # as missing rather than synthesising a drift against zero (which
            # would yield a divide-by-zero with no actionable signal).
            missing.append(s.sku_id)
            continue

        delta_pct = abs(s.price_amount - upstream) / upstream * 100
        if delta_pct >= _DRIFT_THRESHOLD * 100:
            drift.append(
                DriftRecord(
                    sku_id=s.sku_id,
                    catalog_amount=s.price_amount,
                    upstream_amount=upstream,
                    delta_pct=delta_pct,
                )
            )

    return drift, missing
