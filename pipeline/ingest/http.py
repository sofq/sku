"""HTTP client for OpenRouter: live mode (requests) + fixture mode (local files).

FixtureClient is used by tests and by `make ingest SHARD=openrouter FIXTURE=...`
for deterministic shard builds. LiveClient is used by the maintainer bootstrap
run and, in M3a+, by the daily CI pipeline.
"""

from __future__ import annotations

import json
import time
from pathlib import Path
from typing import Any

import requests


class LiveClient:
    """HTTPS client for openrouter.ai with simple retry."""

    BASE = "https://openrouter.ai"

    def __init__(self, timeout_s: float = 15.0, retries: int = 3) -> None:
        self.timeout_s = timeout_s
        self.retries = retries
        self.session = requests.Session()
        self.session.headers.update({
            "User-Agent": "sku-pipeline/0.0 (+https://github.com/sofq/sku)",
            "Accept": "application/json",
        })

    def get(self, path: str) -> dict[str, Any]:
        url = self.BASE + path
        last: Exception | None = None
        for attempt in range(self.retries):
            try:
                resp = self.session.get(url, timeout=self.timeout_s)
                resp.raise_for_status()
                return resp.json()
            except requests.RequestException as e:
                last = e
                time.sleep(0.5 * (2**attempt))
        raise RuntimeError(f"GET {url} failed after {self.retries} attempts: {last}")


class FixtureClient:
    """File-backed client used by tests and offline shard builds.

    Maps API paths to local files under `root`:
      /api/v1/models                              -> models.json
      /api/v1/models/{author}/{slug}/endpoints    -> endpoints/{author}__{slug}.json
    """

    def __init__(self, root: Path) -> None:
        self.root = Path(root)

    def get(self, path: str) -> dict[str, Any]:
        if path == "/api/v1/models":
            return self._load("models.json")
        prefix = "/api/v1/models/"
        suffix = "/endpoints"
        if path.startswith(prefix) and path.endswith(suffix):
            slug = path[len(prefix) : -len(suffix)]
            flat = slug.replace("/", "__")
            return self._load(f"endpoints/{flat}.json")
        raise KeyError(f"FixtureClient: no fixture for path {path!r}")

    def _load(self, rel: str) -> dict[str, Any]:
        with (self.root / rel).open() as fh:
            return json.load(fh)
