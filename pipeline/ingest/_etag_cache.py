"""Tiny JSON-backed ETag sidecar for the discover state directory.

Keyed by full URL; value is the ETag header value exactly as received.
Safe across concurrent runs only because the pipeline serializes: discover
job writes the cache, ingest jobs read it, diff-package doesn't touch it.
"""

from __future__ import annotations

import json
import tempfile
from pathlib import Path


class EtagCache:
    def __init__(self, path: Path | str) -> None:
        self._path = Path(path)
        self._data: dict[str, str] = {}
        if self._path.exists():
            try:
                self._data = json.loads(self._path.read_text()) or {}
            except (json.JSONDecodeError, OSError):
                self._data = {}

    def get(self, url: str) -> str | None:
        return self._data.get(url)

    def set(self, url: str, etag: str) -> None:
        self._data[url] = etag

    def save(self) -> None:
        self._path.parent.mkdir(parents=True, exist_ok=True)
        with tempfile.NamedTemporaryFile(
            "w", dir=self._path.parent, delete=False, suffix=".part"
        ) as fh:
            json.dump(self._data, fh, indent=2, sort_keys=True)
            fh.write("\n")
            tmp = Path(fh.name)
        tmp.replace(self._path)
