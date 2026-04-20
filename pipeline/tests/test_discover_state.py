"""Tests for pipeline.discover.state."""

from __future__ import annotations

import json
import os
from pathlib import Path

import pytest

from discover import state as state_mod
from discover.state import State, load, save


def test_missing_file_returns_empty_state(tmp_path: Path) -> None:
    s = load(tmp_path / "state.json")
    assert s.schema_version == state_mod._STATE_SCHEMA_VERSION
    assert dict(s.indicators) == {}


def test_round_trip(tmp_path: Path) -> None:
    path = tmp_path / "state.json"
    original = State(schema_version=1, indicators={"aws-ec2": "abc", "gcp-gce": "xyz"})
    save(path, original)
    assert path.exists()
    loaded = load(path)
    assert loaded.schema_version == original.schema_version
    assert dict(loaded.indicators) == dict(original.indicators)


def test_save_produces_sorted_indented_json(tmp_path: Path) -> None:
    path = tmp_path / "state.json"
    save(path, State(schema_version=1, indicators={"z": "1", "a": "2", "m": "3"}))
    raw = path.read_text()
    doc = json.loads(raw)
    assert list(doc["indicators"].keys()) == ["a", "m", "z"]
    assert "\n" in raw and "  " in raw  # indented


def test_schema_version_mismatch_raises(tmp_path: Path) -> None:
    path = tmp_path / "state.json"
    path.write_text(json.dumps({"schema_version": 99, "indicators": {}}))
    with pytest.raises(RuntimeError, match="state_schema_version_mismatch"):
        load(path)


def test_missing_schema_version_raises(tmp_path: Path) -> None:
    path = tmp_path / "state.json"
    path.write_text(json.dumps({"indicators": {}}))
    with pytest.raises(RuntimeError, match="state_schema_version_mismatch"):
        load(path)


def test_atomic_write_no_partial_on_crash(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    path = tmp_path / "state.json"
    save(path, State(schema_version=1, indicators={"original": "ok"}))
    original_body = path.read_text()

    def boom(src, dst):
        raise RuntimeError("crash_mid_write")

    monkeypatch.setattr(os, "replace", boom)
    with pytest.raises(RuntimeError, match="crash_mid_write"):
        save(path, State(schema_version=1, indicators={"new": "changed"}))

    # Original file unchanged.
    assert path.read_text() == original_body
    # A .part may be left behind since os.replace never ran, but it must not
    # have clobbered the original file — that's the atomic-write invariant.
    part = path.with_suffix(path.suffix + ".part")
    assert not (part.exists() and part.stat().st_size == 0)
