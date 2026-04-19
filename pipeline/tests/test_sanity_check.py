"""Unit tests for pipeline.package.sanity_check."""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from package.build_shard import build_shard
from package.sanity_check import SanityError, sanity_check


def _row(sku_id: str, amount: float = 0.001) -> dict:
    return {
        "sku_id": sku_id,
        "provider": "anthropic",
        "service": "llm",
        "kind": "llm.text",
        "resource_name": sku_id.split("::")[0],
        "region": "",
        "region_normalized": "",
        "terms": {"commitment": "on_demand", "tenancy": "", "os": ""},
        "terms_hash": "h",
        "resource_attrs": {"context_length": 100_000},
        "prices": [{"dimension": "prompt", "tier": "", "amount": amount, "unit": "token"}],
    }


def _make_shard(
    tmp_path: Path,
    name: str,
    rows: list[dict],
    *,
    version: str = "2026.04.18",
    generated_at: str = "2026-04-18T00:00:00Z",
) -> Path:
    rows_path = tmp_path / f"{name}.rows.jsonl"
    rows_path.write_text("\n".join(json.dumps(r) for r in rows) + "\n")
    db = tmp_path / f"{name}.db"
    build_shard(
        rows_path=rows_path,
        shard=name,
        out_path=db,
        catalog_version=version,
        generated_at=generated_at,
        source_url="https://example.com",
    )
    return db


def test_happy_path(tmp_path: Path):
    prev = _make_shard(tmp_path, "prev", [_row(f"sku-{i}") for i in range(100)], version="1")
    curr = _make_shard(tmp_path, "curr", [_row(f"sku-{i}") for i in range(102)], version="2")
    sanity_check(shard="test", shard_db=curr, previous_db=prev)


def test_first_release_allows_missing_previous(tmp_path: Path):
    curr = _make_shard(tmp_path, "curr", [_row("sku-0")])
    sanity_check(shard="test", shard_db=curr, previous_db=None)


def test_first_release_empty_shard_raises(tmp_path: Path):
    curr = _make_shard(tmp_path, "curr", [])
    with pytest.raises(SanityError) as exc:
        sanity_check(shard="test", shard_db=curr, previous_db=None)
    assert exc.value.reason == "empty_shard"
    assert exc.value.shard == "test"


def test_row_count_drift_raises(tmp_path: Path):
    prev = _make_shard(tmp_path, "prev", [_row(f"sku-{i}") for i in range(100)])
    curr = _make_shard(tmp_path, "curr", [_row(f"sku-{i}") for i in range(50)])  # -50%
    with pytest.raises(SanityError) as exc:
        sanity_check(shard="aws-ec2", shard_db=curr, previous_db=prev, tolerance_pct=5.0)
    assert exc.value.reason == "row_count_drift"
    assert exc.value.shard == "aws-ec2"
    assert "prev=100" in str(exc.value)
    assert "curr=50" in str(exc.value)


def test_row_count_within_tolerance_passes(tmp_path: Path):
    prev = _make_shard(tmp_path, "prev", [_row(f"sku-{i}") for i in range(100)])
    curr = _make_shard(tmp_path, "curr", [_row(f"sku-{i}") for i in range(104)])  # +4%
    sanity_check(shard="test", shard_db=curr, previous_db=prev, tolerance_pct=5.0)


def test_schema_drift_raises(tmp_path: Path):
    prev = _make_shard(tmp_path, "prev", [_row("sku-0")])
    curr = _make_shard(tmp_path, "curr", [_row("sku-0")])
    # Mutate curr: drop a column via schema surgery.
    import sqlite3

    con = sqlite3.connect(curr)
    try:
        con.execute("ALTER TABLE skus ADD COLUMN unexpected TEXT")
        con.commit()
    finally:
        con.close()

    with pytest.raises(SanityError) as exc:
        sanity_check(shard="aws-ec2", shard_db=curr, previous_db=prev)
    assert exc.value.reason == "schema_drift"


def test_fk_orphan_raises(tmp_path: Path):
    curr = _make_shard(tmp_path, "curr", [_row("sku-0")])
    # Inject an orphaned price row by temporarily disabling FK enforcement.
    import sqlite3

    con = sqlite3.connect(curr)
    try:
        con.execute("PRAGMA foreign_keys = OFF")
        con.execute(
            "INSERT INTO prices(sku_id, dimension, tier, amount, unit) "
            "VALUES('orphan', 'prompt', '', 0.001, 'token')"
        )
        con.commit()
    finally:
        con.close()

    with pytest.raises(SanityError) as exc:
        sanity_check(shard="openrouter", shard_db=curr, previous_db=None)
    assert exc.value.reason == "fk_orphan_prices"
    assert exc.value.shard == "openrouter"


def test_missing_generated_at_raises(tmp_path: Path):
    curr = _make_shard(tmp_path, "curr", [_row("sku-0")], generated_at="")
    # build_shard stores "" so the metadata value is empty — should trip.
    with pytest.raises(SanityError) as exc:
        sanity_check(shard="test", shard_db=curr, previous_db=None)
    assert exc.value.reason == "missing_generated_at"
