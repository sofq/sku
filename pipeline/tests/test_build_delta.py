"""Unit tests for pipeline.package.build_delta."""

from __future__ import annotations

import gzip
import json
import sqlite3
from pathlib import Path

from package.build_delta import build_delta
from package.build_shard import build_shard


def _base_row(sku_id: str, amount: float) -> dict:
    return {
        "sku_id": sku_id,
        "provider": "anthropic",
        "service": "llm",
        "kind": "llm.text",
        "resource_name": sku_id.split("::")[0],
        "region": "",
        "region_normalized": "",
        "terms": {"commitment": "on_demand", "tenancy": "", "os": ""},
        "terms_hash": f"h-{sku_id}",
        "resource_attrs": {"context_length": 200_000},
        "prices": [
            {"dimension": "prompt", "tier": "", "amount": amount, "unit": "token"},
            {"dimension": "completion", "tier": "", "amount": amount * 5, "unit": "token"},
        ],
    }


def _write_shard(tmp_path: Path, name: str, rows: list[dict], version: str) -> Path:
    rows_path = tmp_path / f"{name}.rows.jsonl"
    rows_path.write_text("\n".join(json.dumps(r) for r in rows) + "\n")
    out = tmp_path / f"{name}.db"
    build_shard(
        rows_path=rows_path,
        shard=name,
        out_path=out,
        catalog_version=version,
        generated_at="2026-04-18T00:00:00Z",
        source_url="https://example.com",
    )
    return out


def _decompress(paths: list[Path]) -> str:
    parts = [gzip.decompress(p.read_bytes()).decode() for p in paths]
    return "\n".join(parts)


def _apply_and_snapshot(prev_db: Path, sql: str, tmp_path: Path) -> Path:
    target = tmp_path / "applied.db"
    target.write_bytes(prev_db.read_bytes())
    con = sqlite3.connect(target)
    try:
        con.execute("PRAGMA foreign_keys = OFF")
        con.executescript(sql)
        con.commit()
    finally:
        con.close()
    return target


def _table_snapshot(db: Path, table: str) -> list[tuple]:
    con = sqlite3.connect(db)
    try:
        cols = [r[1] for r in con.execute(f"PRAGMA table_info({table})").fetchall()]
        rows = list(con.execute(f"SELECT {', '.join(cols)} FROM {table}"))
        return sorted(rows)
    finally:
        con.close()


def test_round_trip_inserts_and_updates(tmp_path: Path):
    """prev has 10 rows, new has 12 (2 inserted, 1 updated). Applying the
    delta to prev should reproduce new (excluding metadata)."""
    prev_rows = [_base_row(f"sku-{i}::anthropic::default", 0.001 * (i + 1)) for i in range(10)]
    new_rows = [_base_row(f"sku-{i}::anthropic::default", 0.001 * (i + 1)) for i in range(12)]
    # Update row #3's price
    new_rows[3]["prices"][0]["amount"] = 0.999

    prev_db = _write_shard(tmp_path, "prev", prev_rows, "2026.04.17")
    new_db = _write_shard(tmp_path, "new", new_rows, "2026.04.18")

    parts = build_delta(prev_db, new_db, tmp_path / "delta.sql.gz")
    assert len(parts) == 1
    assert parts[0] == tmp_path / "delta.sql.gz"

    sql = _decompress(parts)
    applied = _apply_and_snapshot(prev_db, sql, tmp_path)

    for table in ("skus", "resource_attrs", "terms", "prices", "health"):
        assert _table_snapshot(applied, table) == _table_snapshot(new_db, table), (
            f"table {table} mismatch after applying delta"
        )


def test_round_trip_deletions(tmp_path: Path):
    """prev has 10 rows, new has 7 (3 deleted). Delta contains DELETEs."""
    prev_rows = [_base_row(f"sku-{i}::anthropic::default", 0.001) for i in range(10)]
    new_rows = prev_rows[:7]

    prev_db = _write_shard(tmp_path, "prev", prev_rows, "2026.04.17")
    new_db = _write_shard(tmp_path, "new", new_rows, "2026.04.18")

    parts = build_delta(prev_db, new_db, tmp_path / "delta.sql.gz")
    sql = _decompress(parts)
    applied = _apply_and_snapshot(prev_db, sql, tmp_path)

    assert _table_snapshot(applied, "skus") == _table_snapshot(new_db, "skus")
    # Confirm we actually emitted DELETE statements for the 3 removed skus.
    assert sql.count("DELETE FROM skus WHERE sku_id =") == 3
    # No bulk upsert of unchanged rows.
    assert (
        "sku-0::anthropic::default" not in sql
        or "INSERT OR REPLACE INTO skus"
        not in sql.split("DELETE FROM metadata")[0].split("sku-0::anthropic::default")[0]
    )


def test_split_when_gzipped_exceeds_max_bytes(tmp_path: Path):
    """A max_bytes cap forces the delta to split into multiple parts, and the
    concatenation still reproduces the target."""
    prev_rows = [_base_row(f"sku-{i}::anthropic::default", 0.001) for i in range(5)]
    new_rows = [_base_row(f"sku-{i}::anthropic::default", 0.001 * (i + 1)) for i in range(25)]

    prev_db = _write_shard(tmp_path, "prev", prev_rows, "2026.04.17")
    new_db = _write_shard(tmp_path, "new", new_rows, "2026.04.18")

    parts = build_delta(prev_db, new_db, tmp_path / "delta.sql.gz", max_bytes=256)
    assert len(parts) >= 2, f"expected split; got {parts}"
    for p in parts:
        assert p.stat().st_size <= 512  # generous headroom over the 256-byte target
        assert "-part" in p.name

    sql = _decompress(parts)
    applied = _apply_and_snapshot(prev_db, sql, tmp_path)
    assert _table_snapshot(applied, "skus") == _table_snapshot(new_db, "skus")
    assert _table_snapshot(applied, "prices") == _table_snapshot(new_db, "prices")


def test_noop_when_shards_identical(tmp_path: Path):
    """Identical shards produce a single gzipped file containing only the
    metadata replay + a `-- noop` header."""
    rows = [_base_row(f"sku-{i}::anthropic::default", 0.001) for i in range(3)]
    prev_db = _write_shard(tmp_path, "prev", rows, "2026.04.18")
    new_db = _write_shard(tmp_path, "new", rows, "2026.04.18")

    parts = build_delta(prev_db, new_db, tmp_path / "delta.sql.gz")
    assert len(parts) == 1
    sql = _decompress(parts)
    assert sql.startswith("-- noop")
    assert "DELETE FROM metadata" in sql
    # No data-table mutations when no skus changed.
    assert "DELETE FROM skus" not in sql
    assert "INSERT OR REPLACE INTO skus" not in sql
    assert "INSERT OR REPLACE INTO prices" not in sql


def test_prices_row_deletion_within_same_sku(tmp_path: Path):
    """When a sku_id remains but one of its price rows is dropped, the delta
    emits a DELETE for just that (sku_id, dimension, tier)."""
    prev_rows = [_base_row("sku-0::anthropic::default", 0.001)]
    new_rows = [_base_row("sku-0::anthropic::default", 0.001)]
    new_rows[0]["prices"] = [new_rows[0]["prices"][0]]  # drop the completion row

    prev_db = _write_shard(tmp_path, "prev", prev_rows, "2026.04.17")
    new_db = _write_shard(tmp_path, "new", new_rows, "2026.04.18")

    parts = build_delta(prev_db, new_db, tmp_path / "delta.sql.gz")
    sql = _decompress(parts)
    assert "DELETE FROM prices WHERE sku_id = 'sku-0::anthropic::default'" in sql
    assert "dimension = 'completion'" in sql

    applied = _apply_and_snapshot(prev_db, sql, tmp_path)
    assert _table_snapshot(applied, "prices") == _table_snapshot(new_db, "prices")
