"""Merge per-provider discover state.json into a single release-ready file.

Each data-<provider>.yml workflow writes dist/pipeline/discover/state.json
containing (yesterday's 21 indicators) ∪ (today's fresh indicators for that
provider). Naively concatenating loses two of three providers' fresh
indicators to whichever file is copied last. This script takes the three
per-provider files, unions their `indicators` maps (later-mtime wins on
conflict — which in practice means today's fresh value wins over yesterday's
frozen copy), and emits the merged state.json to the release directory.
"""
from __future__ import annotations

import json
import sys
from pathlib import Path


def main(argv: list[str]) -> int:
    if len(argv) < 2:
        print("usage: merge_discover_state.py <state.json> [<state.json> ...]", file=sys.stderr)
        return 2

    # Sort inputs by mtime ascending so later writes overlay earlier ones.
    inputs = sorted((Path(p) for p in argv[1:]), key=lambda p: p.stat().st_mtime)

    merged: dict[str, dict] = {}
    for path in inputs:
        doc = json.loads(path.read_text())
        indicators = doc.get("indicators") or {}
        if not isinstance(indicators, dict):
            print(f"{path}: indicators field not a mapping — skipping", file=sys.stderr)
            continue
        merged.update(indicators)

    out = {"schema_version": 1, "indicators": dict(sorted(merged.items()))}
    out_path = Path("dist/pipeline/release/state.json")
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(out, indent=2, sort_keys=True) + "\n")
    print(f"wrote {out_path} with {len(merged)} indicators")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
