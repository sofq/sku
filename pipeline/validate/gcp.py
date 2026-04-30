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

# Default service ID used when callers don't specify (matches the legacy
# behaviour of single-service GCE shards). The driver should always pass an
# explicit service_id for known shards.
_DEFAULT_GCE_SERVICE_ID = "6F81-5844-456A"   # Compute Engine


def service_ids_for_shard(shard: str) -> tuple[str, ...] | None:
    """Return the ingest-side service IDs for ``shard`` (e.g. ``"gcp-spanner"``).

    Source of truth lives in :mod:`pipeline.ingest.gcp_common` — duplicating
    it here is what produced the false-positive drift records in #22-#37.
    Returns ``None`` if the shard is unknown to the ingest registry.
    """
    from ingest.gcp_common import _GCP_SERVICE_IDS

    key = shard.replace("-", "_")
    raw = _GCP_SERVICE_IDS.get(key)
    if raw is None:
        return None
    if isinstance(raw, str):
        return (raw,)
    return tuple(raw)


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
    service_id: str | tuple[str, ...] = _DEFAULT_GCE_SERVICE_ID,
    session: requests.Session | None = None,
) -> tuple[list[DriftRecord], list[str]]:
    """Re-fetch each sample from the GCP Cloud Billing API.

    Parameters
    ----------
    samples:
        Samples to validate.
    service_id:
        Cloud Billing service ID, or a tuple of IDs to query in turn (e.g.
        Memorystore is split into Redis + Memcached services). The first
        service that yields a match wins.
    session:
        Optional ``requests.Session`` for dependency injection in tests.

    Returns
    -------
    tuple[list[DriftRecord], list[str]]
        ``(drift_records, missing_upstream_sku_ids)``.
    """
    if session is None:
        session = requests.Session()
    service_ids: tuple[str, ...] = (
        (service_id,) if isinstance(service_id, str) else tuple(service_id)
    )

    try:
        token = _get_bearer_token()
        session.headers["Authorization"] = f"Bearer {token}"
    except Exception:
        logger.exception("Failed to obtain GCP credentials")
        return [], [s.sku_id for s in samples]

    drift: list[DriftRecord] = []
    missing: list[str] = []

    for s in samples:
        upstream_price: float | None = None
        for sid in service_ids:
            url = f"{_BILLING_BASE}/{sid}/skus"
            upstream_price = _fetch_sku_price(
                session, url, {}, s.region, s.resource_name, s.dimension, s.sku_id
            )
            if upstream_price is not None:
                break

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
    dimension: str = "",
    sku_id: str = "",
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
            if not _sku_matches_sample(sku, region, resource_name, dimension, sku_id):
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


def _sku_matches_sample(
    sku: dict,
    region: str,
    resource_name: str,
    dimension: str,
    sku_id: str,
) -> bool:
    desc = sku.get("description", "")
    regions = sku.get("serviceRegions", [])
    if resource_name == "gke-standard":
        return sku.get("skuId") == "B561-BFBD-1264" or desc == "Regional Kubernetes Clusters"
    if resource_name == "gke-autopilot":
        if region not in regions:
            return False
        if dimension == "vcpu":
            return (
                "Autopilot Pod mCPU Requests" in desc
                and "Spot" not in desc
                and "Arm" not in desc
            )
        if dimension == "memory":
            return (
                "Autopilot Pod Memory Requests" in desc
                and "Spot" not in desc
                and "Arm" not in desc
            )
        if dimension == "storage":
            return (
                "Autopilot Pod Ephemeral Storage Requests" in desc
                and "Spot" not in desc
                and "Arm" not in desc
            )
        return False
    if not sku_id:
        return False
    upstream_id = sku.get("skuId", "")
    if upstream_id == sku_id:
        return True
    # Some ingest paths append a suffix to disambiguate duplicate upstream
    # skuIds, using either ``-`` (e.g. Memorystore: ``{skuId}-{region}``) or
    # ``:`` (e.g. GCE: ``{skuId}:{machine_type}``). Accept those as matches —
    # but require the separator so we don't get spurious prefix hits.
    if upstream_id and (
        sku_id.startswith(upstream_id + "-") or sku_id.startswith(upstream_id + ":")
    ):
        return _region_matches(region, regions)
    return False


def _region_matches(region: str, upstream_regions: list[str]) -> bool:
    if region in upstream_regions:
        return True
    bigquery_multiregions = {
        "bq-us": "US",
        "bq-eu": "EU",
    }
    upstream_region = bigquery_multiregions.get(region)
    return upstream_region in upstream_regions if upstream_region else False
