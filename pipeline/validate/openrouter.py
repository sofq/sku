"""OpenRouter endpoint revalidator.

Calls ``GET https://openrouter.ai/api/v1/models/{id}/endpoints`` (anonymous)
and compares per-provider prompt/completion token prices.
"""

from __future__ import annotations

import logging
from dataclasses import dataclass

import requests

from validate.sampler import Sample

logger = logging.getLogger(__name__)

_OPENROUTER_BASE = "https://openrouter.ai/api/v1/models"
_DRIFT_THRESHOLD = 0.01  # 1%


@dataclass
class DriftRecord:
    """A single price-drift observation."""

    sku_id: str
    catalog_amount: float
    upstream_amount: float
    delta_pct: float
    source: str = "openrouter"


def revalidate(
    samples: list[Sample],
    *,
    session: requests.Session | None = None,
) -> tuple[list[DriftRecord], list[str]]:
    """Re-fetch each sample from the OpenRouter endpoints API.

    Parameters
    ----------
    samples:
        Samples to validate; ``dimension`` must be ``"prompt"`` or
        ``"completion"``.
    session:
        Optional ``requests.Session`` for dependency injection in tests.

    Returns
    -------
    tuple[list[DriftRecord], list[str]]
        ``(drift_records, missing_upstream_sku_ids)``.
    """
    if session is None:
        session = requests.Session()

    drift: list[DriftRecord] = []
    missing: list[str] = []

    for s in samples:
        model_id = s.resource_name
        url = f"{_OPENROUTER_BASE}/{model_id}/endpoints"
        try:
            resp = session.get(url, timeout=15)
            resp.raise_for_status()
            data = resp.json()
        except Exception:
            logger.exception("OpenRouter API call failed for %s", s.sku_id)
            missing.append(s.sku_id)
            continue

        endpoints = data.get("data", [])
        if not endpoints:
            logger.debug("No OpenRouter endpoint data for %s", s.sku_id)
            missing.append(s.sku_id)
            continue

        # Use the first endpoint's pricing for the relevant dimension.
        pricing = endpoints[0].get("pricing", {})
        dimension_key = s.dimension  # "prompt" or "completion"
        raw = pricing.get(dimension_key)
        if raw is None:
            missing.append(s.sku_id)
            continue

        try:
            upstream = float(raw)
        except (ValueError, TypeError):
            missing.append(s.sku_id)
            continue

        if upstream == 0:
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
