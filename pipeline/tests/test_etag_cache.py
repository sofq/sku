# pipeline/tests/test_etag_cache.py
from __future__ import annotations

from pathlib import Path

from ingest._etag_cache import EtagCache


def test_roundtrip(tmp_path: Path) -> None:
    c = EtagCache(tmp_path / "etags.json")
    assert c.get("https://example/foo") is None
    c.set("https://example/foo", '"abc123"')
    c.save()

    c2 = EtagCache(tmp_path / "etags.json")
    assert c2.get("https://example/foo") == '"abc123"'


def test_missing_file_is_empty(tmp_path: Path) -> None:
    c = EtagCache(tmp_path / "does-not-exist.json")
    assert c.get("anything") is None
