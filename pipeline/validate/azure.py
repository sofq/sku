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

_AKS_CONTROL_PLANE_METERS = {
    "aks-standard": "Standard Uptime SLA",
    "aks-premium": "Standard Long Term Support",
}

_AKS_VIRTUAL_NODE_METERS = {
    "vcpu": "Standard vCPU Duration",
    "memory": "Standard Memory Duration",
}


@dataclass
class DriftRecord:
    """A single price-drift observation."""

    sku_id: str
    catalog_amount: float
    upstream_amount: float
    delta_pct: float
    source: str = "azure"


# App Service SKU names as used in resource_name (e.g. "P1v3", "B2", "S1").
_APP_SERVICE_SKUS = frozenset({
    "F1", "D1", "B1", "B2", "B3", "S1", "S2", "S3",
    "P0v3", "P1v3", "P2v3", "P3v3", "P1v2", "P2v2", "P3v2",
    "I1v2", "I2v2", "I3v2", "I4v2", "I5v2", "I6v2",
    "I1", "I2", "I3",
})


def _filter_for_sample(s: Sample) -> str | None:
    if s.resource_name == "aks-free":
        return None
    if s.resource_name in _AKS_CONTROL_PLANE_METERS:
        meter = _AKS_CONTROL_PLANE_METERS[s.resource_name]
        return (
            "serviceName eq 'Azure Kubernetes Service' "
            f"and meterName eq '{meter}' "
            f"and armRegionName eq '{s.region}'"
        )
    if s.resource_name == "aks-virtual-nodes-linux":
        meter = _AKS_VIRTUAL_NODE_METERS.get(s.dimension)
        if meter is None:
            return ""
        return (
            "serviceName eq 'Container Instances' "
            "and skuName eq 'Standard' "
            f"and meterName eq '{meter}' "
            f"and armRegionName eq '{s.region}'"
        )
    if s.resource_name in _APP_SERVICE_SKUS:
        return (
            "serviceName eq 'Azure App Service' "
            f"and skuName eq '{s.resource_name}' "
            f"and armRegionName eq '{s.region}'"
        )
    return f"meterName eq '{s.resource_name}' and armRegionName eq '{s.region}'"


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
        filter_str = _filter_for_sample(s)
        if filter_str is None:
            upstream = 0.0
            delta_pct = 0.0 if s.price_amount == 0 else 100.0
            if delta_pct >= _DRIFT_THRESHOLD * 100:
                drift.append(
                    DriftRecord(
                        sku_id=s.sku_id,
                        catalog_amount=s.price_amount,
                        upstream_amount=upstream,
                        delta_pct=delta_pct,
                    )
                )
            continue
        if filter_str == "":
            missing.append(s.sku_id)
            continue
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

        # When the meterName+region filter returns multiple line items with
        # disagreeing positive prices (e.g. SQL vCore meters that span single,
        # elastic-pool, and storage line-items), we cannot disambiguate from
        # the catalog Sample alone — mark as missing rather than guessing
        # ``items[0]`` and filing a false-positive drift record.
        positive_prices = {
            float(it.get("unitPrice", 0) or 0)
            for it in items
            if float(it.get("unitPrice", 0) or 0) > 0
        }
        if not positive_prices:
            missing.append(s.sku_id)
            continue
        if len(positive_prices) > 1:
            logger.debug(
                "Azure response is ambiguous for %s (%d distinct prices)",
                s.sku_id,
                len(positive_prices),
            )
            missing.append(s.sku_id)
            continue
        upstream = positive_prices.pop()

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
