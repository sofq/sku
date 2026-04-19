"""AWS version-indicator discoverer.

For each AWS shard, issues a GET against the offer's per-service `index.json`
and returns the upstream `publicationDate` as the opaque indicator. The driver
uses this to decide whether the shard needs a full ingest pass.
"""

from __future__ import annotations

import json
from collections.abc import Iterable

import requests

from ingest.aws_common import _AWS_OFFER_BASE, _AWS_SERVICE_CODES

_UA = "sku-pipeline/0.0 (+https://github.com/sofq/sku)"


def discover(shards: Iterable[str], *, session: requests.Session | None = None) -> dict[str, str]:
    """Return `{shard_id: publicationDate}` for the given AWS shard ids.

    Unknown shards raise KeyError. HTTP failures raise RuntimeError (the
    driver catches per-shard and records into `errors`).
    """
    owned = False
    if session is None:
        session = requests.Session()
        owned = True
    try:
        out: dict[str, str] = {}
        for shard in shards:
            service = _AWS_SERVICE_CODES[shard]
            url = f"{_AWS_OFFER_BASE}/{service}/index.json"
            resp = session.get(url, headers={"User-Agent": _UA}, timeout=60)
            if resp.status_code != 200:
                raise RuntimeError(f"aws_discover_http_{resp.status_code}: {url}")
            doc = json.loads(resp.content)
            pub = doc.get("publicationDate")
            if not pub:
                raise RuntimeError(f"aws_discover_missing_publicationDate: {url}")
            out[shard] = str(pub)
        return out
    finally:
        if owned:
            session.close()
