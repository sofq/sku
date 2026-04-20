"""Unit tests for pipeline.package.build_manifest."""

from __future__ import annotations

import hashlib
import json
from pathlib import Path

import pytest

from package.build_manifest import build_manifest

NOW = "2026-04-18T00:00:00Z"
BASE_URL = "https://github.com/sofq/sku/releases/download/data-2026.04.18"


def _write(path: Path, contents: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(contents)


def _sha(b: bytes) -> str:
    return hashlib.sha256(b).hexdigest()


def _sidecar(path: Path, **fields) -> None:
    path.write_text(json.dumps(fields))


def test_fresh_first_release_every_shard_has_baseline_empty_deltas(tmp_path: Path):
    artifacts = tmp_path / "artifacts"
    artifacts.mkdir()

    shard_bytes = b"fake sqlite zstd bytes"
    for shard in ("aws-ec2", "openrouter"):
        _write(artifacts / f"{shard}.db.zst", shard_bytes)
        _sidecar(
            artifacts / f"{shard}.meta.json",
            shard=shard,
            row_count=42,
            has_baseline=True,
            delta_from=None,
            delta_to="2026.04.18",
        )

    out = tmp_path / "manifest.json"
    doc = build_manifest(
        artifacts_dir=artifacts,
        out=out,
        catalog_version="2026.04.18",
        release_base_url=BASE_URL,
        previous_manifest=None,
        min_binary_version="1.0.0",
        now=NOW,
    )

    assert doc["schema_version"] == 1
    assert doc["catalog_version"] == "2026.04.18"
    assert doc["generated_at"] == NOW
    assert doc["min_binary_version"] == "1.0.0"
    assert list(doc["shards"].keys()) == ["aws-ec2", "openrouter"]  # sorted

    for shard in ("aws-ec2", "openrouter"):
        entry = doc["shards"][shard]
        assert entry["baseline_version"] == "2026.04.18"
        assert entry["head_version"] == "2026.04.18"
        assert entry["baseline_url"] == f"{BASE_URL}/{shard}.db.zst"
        assert entry["baseline_sha256"] == _sha(shard_bytes)
        assert entry["baseline_size"] == len(shard_bytes)
        assert entry["deltas"] == []
        assert entry["row_count"] == 42
        assert entry["shard_schema_version"] == 1
        assert entry["min_binary_version"] == "1.0.0"
        assert entry["last_updated"] == "2026.04.18"

    # File on disk matches returned doc (pretty-printed + trailing newline).
    assert json.loads(out.read_text()) == doc


def test_normal_day_extends_deltas_and_preserves_baseline(tmp_path: Path):
    # Previous manifest says baseline was 2026.04.17 for aws-ec2.
    prev_manifest = tmp_path / "prev-manifest.json"
    prev_manifest.write_text(
        json.dumps(
            {
                "schema_version": 1,
                "generated_at": "2026-04-17T03:00:00Z",
                "catalog_version": "2026.04.17",
                "min_binary_version": "1.0.0",
                "shards": {
                    "aws-ec2": {
                        "baseline_version": "2026.04.15",
                        "baseline_url": f"{BASE_URL}/../data-2026.04.15/aws-ec2.db.zst",
                        "baseline_sha256": "a" * 64,
                        "baseline_size": 1024,
                        "head_version": "2026.04.17",
                        "min_binary_version": "1.0.0",
                        "shard_schema_version": 1,
                        "deltas": [
                            {
                                "from": "2026.04.15",
                                "to": "2026.04.16",
                                "url": "https://example/d1.sql.gz",
                                "sha256": "b" * 64,
                                "size": 10,
                            },
                            {
                                "from": "2026.04.16",
                                "to": "2026.04.17",
                                "url": "https://example/d2.sql.gz",
                                "sha256": "c" * 64,
                                "size": 12,
                            },
                        ],
                        "row_count": 50,
                        "last_updated": "2026.04.17",
                    }
                },
            }
        )
    )

    artifacts = tmp_path / "artifacts"
    artifacts.mkdir()
    delta_bytes = b"-- today delta\n"
    _write(artifacts / "aws-ec2-2026.04.17-to-2026.04.18.sql.gz", delta_bytes)
    _sidecar(
        artifacts / "aws-ec2.meta.json",
        shard="aws-ec2",
        row_count=55,
        has_baseline=False,
        delta_from="2026.04.17",
        delta_to="2026.04.18",
    )

    doc = build_manifest(
        artifacts_dir=artifacts,
        out=tmp_path / "manifest.json",
        catalog_version="2026.04.18",
        release_base_url=BASE_URL,
        previous_manifest=prev_manifest,
        min_binary_version="1.0.0",
        now=NOW,
    )

    entry = doc["shards"]["aws-ec2"]
    assert entry["baseline_version"] == "2026.04.15"  # preserved
    assert entry["head_version"] == "2026.04.18"
    assert entry["last_updated"] == "2026.04.18"
    assert entry["row_count"] == 55

    # Deltas: old two + today's one.
    assert len(entry["deltas"]) == 3
    today_delta = entry["deltas"][-1]
    assert today_delta["from"] == "2026.04.17"
    assert today_delta["to"] == "2026.04.18"
    assert today_delta["url"] == f"{BASE_URL}/aws-ec2-2026.04.17-to-2026.04.18.sql.gz"
    assert today_delta["sha256"] == _sha(delta_bytes)
    assert today_delta["size"] == len(delta_bytes)


def test_baseline_rebuild_resets_deltas(tmp_path: Path):
    prev_manifest = tmp_path / "prev-manifest.json"
    prev_manifest.write_text(
        json.dumps(
            {
                "schema_version": 1,
                "shards": {
                    "aws-ec2": {
                        "baseline_version": "2026.04.01",
                        "baseline_url": "https://stale/baseline.db.zst",
                        "baseline_sha256": "x" * 64,
                        "baseline_size": 1,
                        "head_version": "2026.04.17",
                        "deltas": [{"from": "x", "to": "y", "url": "z", "sha256": "q", "size": 1}],
                        "row_count": 50,
                        "last_updated": "2026.04.17",
                    }
                },
            }
        )
    )
    artifacts = tmp_path / "artifacts"
    artifacts.mkdir()
    new_baseline = b"new baseline bytes"
    _write(artifacts / "aws-ec2.db.zst", new_baseline)
    _sidecar(
        artifacts / "aws-ec2.meta.json",
        shard="aws-ec2",
        row_count=60,
        has_baseline=True,
        delta_from=None,
        delta_to="2026.04.18",
    )

    doc = build_manifest(
        artifacts_dir=artifacts,
        out=tmp_path / "manifest.json",
        catalog_version="2026.04.18",
        release_base_url=BASE_URL,
        previous_manifest=prev_manifest,
        min_binary_version="1.0.0",
        now=NOW,
    )

    entry = doc["shards"]["aws-ec2"]
    assert entry["baseline_version"] == "2026.04.18"
    assert entry["head_version"] == "2026.04.18"
    assert entry["baseline_url"] == f"{BASE_URL}/aws-ec2.db.zst"
    assert entry["baseline_sha256"] == _sha(new_baseline)
    assert entry["deltas"] == []
    assert entry["row_count"] == 60


def test_shard_not_in_today_artifacts_is_carried_forward(tmp_path: Path):
    prev_manifest = tmp_path / "prev-manifest.json"
    prev_entry = {
        "baseline_version": "2026.04.10",
        "baseline_url": "https://prev/baseline.db.zst",
        "baseline_sha256": "z" * 64,
        "baseline_size": 100,
        "head_version": "2026.04.17",
        "deltas": [],
        "row_count": 99,
        "last_updated": "2026.04.17",
    }
    prev_manifest.write_text(
        json.dumps(
            {
                "schema_version": 1,
                "shards": {
                    "aws-ec2": prev_entry,
                    "azure-vm": {
                        "baseline_version": "x",
                        "baseline_url": "u",
                        "baseline_sha256": "s",
                        "baseline_size": 1,
                        "head_version": "x",
                        "deltas": [],
                        "row_count": 1,
                    },
                },
            }
        )
    )
    artifacts = tmp_path / "artifacts"
    artifacts.mkdir()
    # Only aws-ec2 was touched today.
    _write(artifacts / "aws-ec2.db.zst", b"new")
    _sidecar(
        artifacts / "aws-ec2.meta.json",
        shard="aws-ec2",
        row_count=1,
        has_baseline=True,
        delta_from=None,
        delta_to="2026.04.18",
    )

    doc = build_manifest(
        artifacts_dir=artifacts,
        out=tmp_path / "manifest.json",
        catalog_version="2026.04.18",
        release_base_url=BASE_URL,
        previous_manifest=prev_manifest,
        min_binary_version="1.0.0",
        now=NOW,
    )

    # aws-ec2 rebuilt, azure-vm carried forward unchanged.
    assert list(doc["shards"].keys()) == ["aws-ec2", "azure-vm"]
    assert doc["shards"]["azure-vm"]["baseline_version"] == "x"
    assert doc["shards"]["azure-vm"]["baseline_url"] == "u"


def test_multi_part_delta_recorded_with_parts(tmp_path: Path):
    prev_manifest = tmp_path / "prev-manifest.json"
    prev_manifest.write_text(
        json.dumps(
            {
                "schema_version": 1,
                "shards": {
                    "aws-ec2": {
                        "baseline_version": "2026.04.10",
                        "baseline_url": "u",
                        "baseline_sha256": "a" * 64,
                        "baseline_size": 1,
                        "head_version": "2026.04.17",
                        "deltas": [],
                        "row_count": 5,
                        "last_updated": "2026.04.17",
                    }
                },
            }
        )
    )
    artifacts = tmp_path / "artifacts"
    artifacts.mkdir()
    p1 = b"aaa"
    p2 = b"bbbb"
    _write(artifacts / "aws-ec2-2026.04.17-to-2026.04.18-part1.sql.gz", p1)
    _write(artifacts / "aws-ec2-2026.04.17-to-2026.04.18-part2.sql.gz", p2)
    _sidecar(
        artifacts / "aws-ec2.meta.json",
        shard="aws-ec2",
        row_count=7,
        has_baseline=False,
        delta_from="2026.04.17",
        delta_to="2026.04.18",
    )

    doc = build_manifest(
        artifacts_dir=artifacts,
        out=tmp_path / "manifest.json",
        catalog_version="2026.04.18",
        release_base_url=BASE_URL,
        previous_manifest=prev_manifest,
        min_binary_version="1.0.0",
        now=NOW,
    )

    delta = doc["shards"]["aws-ec2"]["deltas"][0]
    assert delta["from"] == "2026.04.17"
    assert delta["to"] == "2026.04.18"
    assert "parts" in delta
    assert [p["url"] for p in delta["parts"]] == [
        f"{BASE_URL}/aws-ec2-2026.04.17-to-2026.04.18-part1.sql.gz",
        f"{BASE_URL}/aws-ec2-2026.04.17-to-2026.04.18-part2.sql.gz",
    ]
    assert delta["size"] == len(p1) + len(p2)


def test_delta_only_without_previous_manifest_raises(tmp_path: Path):
    artifacts = tmp_path / "artifacts"
    artifacts.mkdir()
    _write(artifacts / "aws-ec2-2026.04.17-to-2026.04.18.sql.gz", b"d")
    _sidecar(
        artifacts / "aws-ec2.meta.json",
        shard="aws-ec2",
        row_count=1,
        has_baseline=False,
        delta_from="2026.04.17",
        delta_to="2026.04.18",
    )

    with pytest.raises(ValueError, match="no baseline"):
        build_manifest(
            artifacts_dir=artifacts,
            out=tmp_path / "manifest.json",
            catalog_version="2026.04.18",
            release_base_url=BASE_URL,
            previous_manifest=None,
            min_binary_version="1.0.0",
            now=NOW,
        )
