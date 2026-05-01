"""Shared GCP Cloud Pub/Sub ingest helpers."""
from __future__ import annotations

from pathlib import Path

from .gcp_common import fetch_skus, service_ids_for_shard  # noqa: F401

_PUBSUB_SERVICE_ID = "A1E8-BE35-7EBC"


def fetch_pubsub_skus(target: Path, *, session=None) -> list[dict]:
    """Fetch all Cloud Pub/Sub SKUs from the Cloud Billing Catalog API."""
    return fetch_skus("gcp_pubsub_queues", target, session=session)
