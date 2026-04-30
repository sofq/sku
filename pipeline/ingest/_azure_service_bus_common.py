"""Shared fetch helper for Azure Service Bus shards (queues + topics).

Both azure_service_bus_queues and azure_service_bus_topics share the same
upstream serviceName filter. This module centralises the live-fetch call so
the two shards can share a single cache file when run in the same pipeline
pass.
"""

from __future__ import annotations

from pathlib import Path

from .azure_common import fetch_prices

_SERVICE_BUS_FILTER = "serviceName eq 'Service Bus'"


def fetch_service_bus_prices(target: Path, *, session=None) -> list[dict]:
    """Fetch Azure Retail Prices for Service Bus and cache to *target*.

    Returns the list of item dicts (same shape as the ``Items`` array in the
    cached JSON).  The target file is written atomically as
    ``{"Items": [...]}`` sorted by skuId for determinism.
    """
    fetch_prices(_SERVICE_BUS_FILTER, target, session=session)
    import json
    with open(target, encoding="utf-8") as fh:
        return json.load(fh).get("Items", [])
