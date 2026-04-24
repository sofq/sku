"""The YAML set must match today's ALL_SHARDS tuple exactly.

This test is the safety net for Task 2; later tasks swap the ALL_SHARDS
source to the YAMLs, at which point this test becomes a simple presence
check.
"""
from __future__ import annotations

from pathlib import Path

from discover.driver import ALL_SHARDS
from shards import load_all


def test_yaml_set_matches_all_shards() -> None:
    root = Path(__file__).resolve().parents[1] / "shards"
    yaml_shards = set(load_all(root))
    assert yaml_shards == set(ALL_SHARDS), (
        f"missing: {set(ALL_SHARDS) - yaml_shards}, "
        f"extra: {yaml_shards - set(ALL_SHARDS)}"
    )
