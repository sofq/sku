"""Assemble the manifest.json asset for a data release.

Spec §3 (manifest structure): clients hit this file to discover where to
download baselines + deltas for each shard, how to verify them (sha256), and
what minimum binary version they require. This module walks the set of
artifacts built by the `diff-package` matrix job and emits the manifest per
that shape.

Artifact layout in `artifacts_dir` (produced by `scripts/ci/diff_package_shard.sh`):

    <shard>.db.zst             # present only on baseline-rebuild
    <shard>.db.zst.sha256      # matching sha256 file (sidecar, same basename)
    <shard>-<from>-to-<to>.sql.gz
    <shard>-<from>-to-<to>-part<N>.sql.gz   # when split
    <shard>.meta.json          # {"shard": "...", "row_count": N, "has_baseline": bool,
                                #  "delta_from": "...", "delta_to": "..."}

Every shard touched by today's run contributes a `<shard>.meta.json` sidecar;
that is the authoritative source of row_count + what file-types to expect.
Shards absent from today's run are carried forward from `previous_manifest`
unmodified so clients keep working against yesterday's baseline.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import sys
import time
from pathlib import Path
from typing import Any

_SCHEMA_VERSION = 1


def _sha256_of(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def _file_entry(path: Path, release_base_url: str) -> dict[str, Any]:
    return {
        "url": f"{release_base_url.rstrip('/')}/{path.name}",
        "sha256": _sha256_of(path),
        "size": path.stat().st_size,
    }


def _delta_parts_for(shard: str, delta_from: str, delta_to: str, artifacts_dir: Path) -> list[Path]:
    """Find every part of a single delta in apply-order."""
    prefix = f"{shard}-{delta_from}-to-{delta_to}"
    single = artifacts_dir / f"{prefix}.sql.gz"
    if single.is_file():
        return [single]
    parts: list[Path] = []
    for candidate in sorted(artifacts_dir.glob(f"{prefix}-part*.sql.gz")):
        parts.append(candidate)
    return parts


def _delta_entry(
    *,
    shard: str,
    delta_from: str,
    delta_to: str,
    artifacts_dir: Path,
    release_base_url: str,
) -> dict[str, Any]:
    parts = _delta_parts_for(shard, delta_from, delta_to, artifacts_dir)
    if not parts:
        raise FileNotFoundError(
            f"expected delta file(s) for {shard} {delta_from} → {delta_to} under {artifacts_dir}"
        )
    entry: dict[str, Any] = {"from": delta_from, "to": delta_to}
    if len(parts) == 1:
        entry.update(_file_entry(parts[0], release_base_url))
    else:
        entry["parts"] = [_file_entry(p, release_base_url) for p in parts]
        # Convenience: total size of all parts.
        entry["size"] = sum(p["size"] for p in entry["parts"])
    return entry


def _read_sidecar(path: Path) -> dict[str, Any]:
    doc = json.loads(path.read_text())
    for key in ("shard", "row_count", "has_baseline", "delta_to"):
        if key not in doc:
            raise KeyError(f"{path}: missing key {key!r}")
    return doc


def _load_previous(previous_manifest: Path | None) -> dict[str, Any]:
    if previous_manifest is None or not Path(previous_manifest).is_file():
        return {"shards": {}}
    return json.loads(Path(previous_manifest).read_text())


def _now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def build_manifest(
    *,
    artifacts_dir: Path,
    out: Path,
    catalog_version: str,
    release_base_url: str,
    previous_manifest: Path | None,
    min_binary_version: str,
    shard_schema_version: int = 1,
    now: str | None = None,
) -> dict[str, Any]:
    """Write manifest.json to `out` and return the document.

    Only shards with a `<shard>.meta.json` sidecar in `artifacts_dir` are
    considered "touched" today. Every other shard listed in
    `previous_manifest` is carried forward unchanged.
    """
    artifacts_dir = Path(artifacts_dir)
    out = Path(out)

    previous = _load_previous(previous_manifest)
    prev_shards: dict[str, Any] = previous.get("shards", {})

    # Discover sidecars → shard entries for today.
    sidecars = {
        s.stem[: -len(".meta")] if s.name.endswith(".meta.json") else s.stem: _read_sidecar(s)
        for s in artifacts_dir.glob("*.meta.json")
    }

    shards_out: dict[str, Any] = {}

    # Every shard that showed up today.
    for shard, meta in sidecars.items():
        prev_entry: dict[str, Any] = prev_shards.get(shard, {})
        has_baseline = bool(meta["has_baseline"])
        delta_to = meta["delta_to"]
        delta_from = meta.get("delta_from")

        if has_baseline:
            baseline_path = artifacts_dir / f"{shard}.db.zst"
            if not baseline_path.is_file():
                raise FileNotFoundError(
                    f"{shard} sidecar says has_baseline=true but {baseline_path} missing"
                )
            baseline_entry = _file_entry(baseline_path, release_base_url)
            entry: dict[str, Any] = {
                "baseline_version": delta_to,
                "baseline_url": baseline_entry["url"],
                "baseline_sha256": baseline_entry["sha256"],
                "baseline_size": baseline_entry["size"],
                "head_version": delta_to,
                "min_binary_version": min_binary_version,
                "shard_schema_version": shard_schema_version,
                "deltas": [],
                "row_count": int(meta["row_count"]),
                "last_updated": delta_to,
            }
        else:
            if not prev_entry:
                raise ValueError(
                    f"{shard}: no baseline this run and no previous manifest entry — "
                    "cannot assemble a delta-only entry without a baseline to chain from"
                )
            if not delta_from:
                raise ValueError(f"{shard}: sidecar missing delta_from for delta-only run")

            new_delta = _delta_entry(
                shard=shard,
                delta_from=delta_from,
                delta_to=delta_to,
                artifacts_dir=artifacts_dir,
                release_base_url=release_base_url,
            )

            entry = dict(prev_entry)
            entry["head_version"] = delta_to
            entry["last_updated"] = delta_to
            entry["min_binary_version"] = min_binary_version
            entry["shard_schema_version"] = shard_schema_version
            entry["row_count"] = int(meta["row_count"])
            entry["deltas"] = list(prev_entry.get("deltas", [])) + [new_delta]

        shards_out[shard] = entry

    # Carry-forward untouched shards from the previous manifest.
    for shard, prev_entry in prev_shards.items():
        if shard not in shards_out:
            shards_out[shard] = dict(prev_entry)

    # Alphabetise for diff-friendly output.
    sorted_shards = {k: shards_out[k] for k in sorted(shards_out.keys())}

    doc: dict[str, Any] = {
        "schema_version": _SCHEMA_VERSION,
        "generated_at": now or _now_iso(),
        "catalog_version": catalog_version,
        "min_binary_version": min_binary_version,
        "shards": sorted_shards,
    }

    out.parent.mkdir(parents=True, exist_ok=True)
    tmp = out.with_suffix(out.suffix + ".part")
    with tmp.open("w") as fh:
        json.dump(doc, fh, indent=2, sort_keys=False)
        fh.write("\n")
    tmp.replace(out)
    return doc


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(prog="package.build_manifest")
    ap.add_argument("--artifacts-dir", type=Path, required=True)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--catalog-version", required=True)
    ap.add_argument("--release-base-url", required=True)
    ap.add_argument("--previous-manifest", type=Path, default=None)
    ap.add_argument("--min-binary-version", default="1.0.0")
    ap.add_argument("--shard-schema-version", type=int, default=1)
    args = ap.parse_args(argv)

    build_manifest(
        artifacts_dir=args.artifacts_dir,
        out=args.out,
        catalog_version=args.catalog_version,
        release_base_url=args.release_base_url,
        previous_manifest=args.previous_manifest,
        min_binary_version=args.min_binary_version,
        shard_schema_version=args.shard_schema_version,
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
