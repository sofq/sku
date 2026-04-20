"""Persistent per-shard version-indicator store for the discover module.

`State` holds the schema version + an opaque `{shard_id: indicator}` map. The
indicator values are produced by `pipeline.discover.{provider}` and consumed by
the driver to decide which shards' upstream data has changed since the last run.
"""

from __future__ import annotations

import json
import os
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path

_STATE_SCHEMA_VERSION = 1


@dataclass(frozen=True)
class State:
    schema_version: int
    indicators: Mapping[str, str]


def load(path: Path) -> State:
    """Return the state at `path`; if missing, return an empty state."""
    if not path.exists():
        return State(schema_version=_STATE_SCHEMA_VERSION, indicators={})
    with path.open() as fh:
        doc = json.load(fh)
    version = doc.get("schema_version")
    if version != _STATE_SCHEMA_VERSION:
        raise RuntimeError(
            f"state_schema_version_mismatch: expected {_STATE_SCHEMA_VERSION}, got {version!r}"
        )
    indicators = doc.get("indicators") or {}
    return State(schema_version=version, indicators=dict(indicators))


def save(path: Path, state: State) -> None:
    """Atomic write via .part + os.replace; indented JSON with sorted keys."""
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".part")
    doc = {
        "schema_version": state.schema_version,
        "indicators": dict(sorted(state.indicators.items())),
    }
    with tmp.open("w") as fh:
        json.dump(doc, fh, indent=2, sort_keys=True)
        fh.write("\n")
    os.replace(tmp, path)
