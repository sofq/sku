"""Azure retail pricing revalidator.

Uses the anonymous ``https://prices.azure.com/api/retail/prices`` endpoint
filtered by ``meterName`` and ``armRegionName``. No authentication required.
"""

from __future__ import annotations

import logging
from dataclasses import dataclass

import requests

from validate.sampler import Sample

logger = logging.getLogger(__name__)

_AZURE_PRICES_URL = "https://prices.azure.com/api/retail/prices"
_DRIFT_THRESHOLD = 0.01  # 1%


@dataclass
class DriftRecord:
    """A single price-drift observation."""

    sku_id: str
    catalog_amount: float
    upstream_amount: float
    delta_pct: float
    source: str = "azure"


def revalidate(
    samples: list[Sample],
    *,
    session: requests.Session | None = None,
) -> tuple[list[DriftRecord], list[str]]:
    """Re-fetch each sample from the Azure retail pricing API.

    Parameters
    ----------
    samples:
        Samples to validate.
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
        filter_str = (
            f"meterName eq '{s.resource_name}' and armRegionName eq '{s.region}'"
        )
        try:
            resp = session.get(
                _AZURE_PRICES_URL,
                params={"$filter": filter_str},
                timeout=15,
            )
            resp.raise_for_status()
            data = resp.json()
        except Exception:
            logger.exception("Azure Pricing API call failed for %s", s.sku_id)
            missing.append(s.sku_id)
            continue

        items = data.get("Items", [])
        if not items:
            logger.debug("No Azure upstream price for %s", s.sku_id)
            missing.append(s.sku_id)
            continue

        # Use the first matching item's unitPrice.
        upstream = float(items[0].get("unitPrice", 0) or 0)
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
