"""GCP version-indicator discoverer.

For each GCP shard, issues a parent-service GET (`/services/{service_id}`) and
returns `displayName + '|' + updateTime` when both are present. If the parent
record omits `updateTime`, falls back to a `pageSize=1` SKU fetch and hashes
the single row. The fallback is rare — Cloud Billing populates `updateTime`
for all the shards we care about — but it keeps the discoverer self-consistent
across upstream quirks.
"""

from __future__ import annotations

import hashlib
import json
from collections.abc import Iterable

import requests

from ingest.gcp_common import _GCP_BILLING_BASE, _GCP_SERVICE_IDS

_UA = "sku-pipeline/0.0 (+https://github.com/sofq/sku)"


def discover(
    shards: Iterable[str], *, api_key: str, session: requests.Session | None = None
) -> dict[str, str]:
    """Return `{shard_id: indicator}` for the given GCP shard ids.

    Unknown shards raise KeyError. HTTP failures raise RuntimeError.
    """
    owned = False
    if session is None:
        session = requests.Session()
        owned = True
    try:
        out: dict[str, str] = {}
        for shard in shards:
            service_id = _GCP_SERVICE_IDS[shard]
            # Parent service record — cheap (< 1 KB).
            url = f"{_GCP_BILLING_BASE}/services/{service_id}"
            resp = session.get(
                url, params={"key": api_key}, headers={"User-Agent": _UA}, timeout=30
            )
            if resp.status_code != 200:
                raise RuntimeError(f"gcp_discover_http_{resp.status_code}: {shard}")
            doc = resp.json()
            update_time = doc.get("updateTime") or doc.get("serviceInfo", {}).get("updateTime")
            display_name = doc.get("displayName", "")
            if update_time:
                out[shard] = f"{display_name}|{update_time}"
                continue
            # Fallback: hash over a single SKU.
            sku_url = f"{_GCP_BILLING_BASE}/services/{service_id}/skus"
            sku_resp = session.get(
                sku_url,
                params={"key": api_key, "pageSize": "1"},
                headers={"User-Agent": _UA},
                timeout=30,
            )
            if sku_resp.status_code != 200:
                raise RuntimeError(f"gcp_discover_fallback_http_{sku_resp.status_code}: {shard}")
            skus = sku_resp.json().get("skus", [])
            payload = json.dumps(skus, separators=(",", ":"), sort_keys=True).encode()
            out[shard] = "sha256:" + hashlib.sha256(payload).hexdigest()
        return out
    finally:
        if owned:
            session.close()
