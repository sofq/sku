"""Azure version-indicator discoverer.

For each Azure shard, issues a single-page retail-prices query with `$top=1`
and returns a SHA256 digest of that page's items as the cheap fingerprint.
The per-shard filter strings live here; they mirror the shapes used by the
full ingest layer in `ingest.azure_*`.
"""

from __future__ import annotations

import hashlib
import json
from collections.abc import Iterable

import requests

from ingest.azure_common import _AZURE_RETAIL_BASE

_UA = "sku-pipeline/0.0 (+https://github.com/sofq/sku)"

_SHARD_FILTERS: dict[str, str] = {
    "azure_vm": "serviceName eq 'Virtual Machines'",
    "azure_sql": "serviceName eq 'SQL Database'",
    "azure_blob": "serviceName eq 'Storage' and productName eq 'Blob Storage'",
    "azure_functions": "serviceName eq 'Functions'",
    "azure_disks": "serviceName eq 'Storage' and (productName eq 'Standard HDD Managed Disks' or productName eq 'Standard SSD Managed Disks' or productName eq 'Premium SSD Managed Disks')",
    "azure_postgres": "serviceName eq 'Azure Database for PostgreSQL'",
    "azure_mysql": "serviceName eq 'Azure Database for MySQL'",
    "azure_mariadb": "serviceName eq 'Azure Database for MariaDB'",
    "azure_cosmosdb": "serviceName eq 'Azure Cosmos DB'",
    "azure_redis": "serviceName eq 'Redis Cache'",
}


def discover(shards: Iterable[str], *, session: requests.Session | None = None) -> dict[str, str]:
    """Return `{shard_id: sha256_hex}` for the given Azure shard ids.

    Unknown shards raise KeyError. HTTP failures raise RuntimeError.
    """
    owned = False
    if session is None:
        session = requests.Session()
        owned = True
    try:
        out: dict[str, str] = {}
        for shard in shards:
            filter_str = _SHARD_FILTERS[shard]
            params = {
                "$filter": filter_str,
                "$top": "1",
                "api-version": "2023-01-01-preview",
            }
            resp = session.get(
                _AZURE_RETAIL_BASE, params=params, headers={"User-Agent": _UA}, timeout=60
            )
            if resp.status_code != 200:
                raise RuntimeError(f"azure_discover_http_{resp.status_code}: {shard}")
            items = resp.json().get("Items", [])
            payload = json.dumps(items, separators=(",", ":"), sort_keys=True).encode()
            out[shard] = hashlib.sha256(payload).hexdigest()
        return out
    finally:
        if owned:
            session.close()
