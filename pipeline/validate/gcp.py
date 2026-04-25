"""GCP Cloud Billing revalidator.

Uses Application Default Credentials (``google.auth.default()``) — under
GitHub Actions, this picks up the Workload Identity Federation token injected
by ``google-github-actions/auth``. Then iterates
``cloudbilling.googleapis.com/v1/services/{sid}/skus`` filtered by region.
"""

from __future__ import annotations

import logging
from dataclasses import dataclass

import google.auth
import google.auth.transport.requests
import requests

from validate.sampler import Sample

logger = logging.getLogger(__name__)

_BILLING_BASE = "https://cloudbilling.googleapis.com/v1/services"
_DRIFT_THRESHOLD = 0.01  # 1%

# GCP Cloud Billing service IDs (callers may override).
_DEFAULT_GCE_SERVICE_ID = "6F81-5844-456A"   # Compute Engine
_GKE_SERVICE_ID = "CCD8-9BF1-090E"           # Kubernetes Engine
_MEMORYSTORE_SERVICE_ID = "58CD-E7C3-72CA"   # Memorystore

_SHARD_SERVICE_IDS: dict[str, str] = {
    "gcp-gke": _GKE_SERVICE_ID,
    "gcp-memorystore": _MEMORYSTORE_SERVICE_ID,
}


@dataclass
class DriftRecord:
    """A single price-drift observation."""

    sku_id: str
    catalog_amount: float
    upstream_amount: float
    delta_pct: float
    source: str = "gcp"


def _nanos_to_float(units: int | str, nanos: int) -> float:
    """Convert Google Money (units + nanos) to a float."""
    return int(units or 0) + nanos / 1e9


def _get_bearer_token() -> str:
    """Return a fresh bearer token via ADC."""
    creds, _ = google.auth.default(scopes=["https://www.googleapis.com/auth/cloud-billing.readonly"])
    auth_req = google.auth.transport.requests.Request()
    creds.refresh(auth_req)
    return creds.token


def revalidate(
    samples: list[Sample],
    *,
    service_id: str = _DEFAULT_GCE_SERVICE_ID,
    session: requests.Session | None = None,
) -> tuple[list[DriftRecord], list[str]]:
    """Re-fetch each sample from the GCP Cloud Billing API.

    Parameters
    ----------
    samples:
        Samples to validate.
    service_id:
        Cloud Billing service ID (e.g. ``"6F81-5844-456A"`` for GCE).
    session:
        Optional ``requests.Session`` for dependency injection in tests.

    Returns
    -------
    tuple[list[DriftRecord], list[str]]
        ``(drift_records, missing_upstream_sku_ids)``.
    """
    if session is None:
        session = requests.Session()

    try:
        token = _get_bearer_token()
        session.headers["Authorization"] = f"Bearer {token}"
    except Exception:
        logger.exception("Failed to obtain GCP credentials")
        return [], [s.sku_id for s in samples]

    drift: list[DriftRecord] = []
    missing: list[str] = []

    for s in samples:
        url = f"{_BILLING_BASE}/{service_id}/skus"
        params: dict = {}
        upstream_price = _fetch_sku_price(session, url, params, s.region, s.resource_name)

        if upstream_price is None:
            logger.debug("No GCP upstream price for %s", s.sku_id)
            missing.append(s.sku_id)
            continue

        delta_pct = abs(s.price_amount - upstream_price) / upstream_price * 100
        if delta_pct >= _DRIFT_THRESHOLD * 100:
            drift.append(
                DriftRecord(
                    sku_id=s.sku_id,
                    catalog_amount=s.price_amount,
                    upstream_amount=upstream_price,
                    delta_pct=delta_pct,
                )
            )

    return drift, missing


def _fetch_sku_price(
    session: requests.Session,
    url: str,
    params: dict,
    region: str,
    resource_name: str,
) -> float | None:
    """Paginate the SKU list and return the first matching price."""
    page_token = ""
    while True:
        try:
            query: dict = dict(params)
            if page_token:
                query["pageToken"] = page_token
            resp = session.get(url, params=query, timeout=20)
            resp.raise_for_status()
            data = resp.json()
        except Exception:
            logger.exception("GCP Billing API call failed for %s / %s", resource_name, region)
            return None

        for sku in data.get("skus", []):
            regions = sku.get("serviceRegions", [])
            if region not in regions:
                continue
            pricing_info = sku.get("pricingInfo", [])
            if not pricing_info:
                continue
            expr = pricing_info[0].get("pricingExpression", {})
            rates = expr.get("tieredRates", [])
            if not rates:
                continue
            up = rates[0].get("unitPrice", {})
            nanos = int(up.get("nanos", 0))
            units = up.get("units", "0")
            price = _nanos_to_float(units, nanos)
            if price > 0:
                return price

        page_token = data.get("nextPageToken", "")
        if not page_token:
            break

    return None
