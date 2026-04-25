"""GCP version-indicator discoverer.

Cloud Billing Catalog API has no `services.get`, only `services.list` and
`services/{id}/skus.list`. For each shard we fetch a single SKU
(`pageSize=1`) and hash the row — the hash changes whenever the catalog
publishes a new or changed SKU at the head of the page, triggering a
rebuild. Weekly `--baseline-rebuild` covers any head-of-page
reordering we miss.

Auth: the caller's `session` must carry an `Authorization: Bearer <token>`
header (see `ingest.gcp_common.build_authenticated_session`).
"""

from __future__ import annotations

import hashlib
import json
from collections.abc import Iterable

import requests

from ingest.gcp_common import _GCP_BILLING_BASE, service_ids_for_shard

_UA = "sku-pipeline/0.0 (+https://github.com/sofq/sku)"


def discover(shards: Iterable[str], *, session: requests.Session | None = None) -> dict[str, str]:
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
            shard_skus = []
            for service_id in service_ids_for_shard(shard):
                url = f"{_GCP_BILLING_BASE}/services/{service_id}/skus"
                resp = session.get(
                    url,
                    params={"pageSize": "1"},
                    headers={"User-Agent": _UA},
                    timeout=30,
                )
                if resp.status_code != 200:
                    raise RuntimeError(f"gcp_discover_http_{resp.status_code}: {shard}")
                shard_skus.extend(resp.json().get("skus", []))
            payload = json.dumps(shard_skus, separators=(",", ":"), sort_keys=True).encode()
            out[shard] = "sha256:" + hashlib.sha256(payload).hexdigest()
        return out
    finally:
        if owned:
            session.close()
