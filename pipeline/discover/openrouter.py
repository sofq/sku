"""OpenRouter version-indicator discoverer.

Single GET against `/api/v1/models`; indicator is a SHA256 digest of the
sorted list of model ids. Whenever OpenRouter adds / removes / renames a
model, the indicator changes and the shard re-ingests.
"""

from __future__ import annotations

import hashlib
import json
from collections.abc import Iterable

from ingest.http import LiveClient

_OPENROUTER_SHARDS = frozenset({"openrouter"})


def discover(shards: Iterable[str], *, client: LiveClient | None = None) -> dict[str, str]:
    """Return `{shard_id: sha256_hex}` for the given OpenRouter shard ids.

    Only `"openrouter"` is known; anything else raises KeyError.
    """
    shard_list = list(shards)
    for s in shard_list:
        if s not in _OPENROUTER_SHARDS:
            raise KeyError(s)
    if not shard_list:
        return {}
    c = client or LiveClient()
    payload = c.get("/api/v1/models")
    ids = sorted(m["id"] for m in payload.get("data", []))
    digest = hashlib.sha256(json.dumps(ids).encode()).hexdigest()
    return dict.fromkeys(shard_list, digest)
