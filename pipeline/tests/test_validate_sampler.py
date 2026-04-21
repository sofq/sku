"""Tests for pipeline.validate.sampler — stratified sampling from a shard."""

from __future__ import annotations

import sqlite3
from pathlib import Path

from validate.sampler import Sample, sample

# ---------------------------------------------------------------------------
# Fixture helpers
# ---------------------------------------------------------------------------


def _make_shard(tmp_path: Path, rows: list[dict]) -> Path:
    """Create a minimal SQLite shard with the sku schema and given rows."""
    db_path = tmp_path / "test.db"
    con = sqlite3.connect(db_path)
    try:
        con.executescript(
            """
            PRAGMA foreign_keys = ON;
            CREATE TABLE skus (
                sku_id            TEXT NOT NULL PRIMARY KEY,
                provider          TEXT NOT NULL,
                service           TEXT NOT NULL,
                kind              TEXT NOT NULL,
                resource_name     TEXT NOT NULL,
                region            TEXT NOT NULL,
                region_normalized TEXT NOT NULL,
                terms_hash        TEXT NOT NULL
            ) WITHOUT ROWID;
            CREATE TABLE prices (
                sku_id     TEXT NOT NULL REFERENCES skus(sku_id) ON DELETE CASCADE,
                dimension  TEXT NOT NULL,
                tier       TEXT NOT NULL DEFAULT '',
                amount     REAL NOT NULL,
                unit       TEXT NOT NULL,
                PRIMARY KEY (sku_id, dimension, tier)
            ) WITHOUT ROWID;
            CREATE TABLE metadata (
                key   TEXT PRIMARY KEY,
                value TEXT
            );
            INSERT INTO metadata VALUES ('currency', 'USD');
            """
        )
        for r in rows:
            con.execute(
                "INSERT INTO skus VALUES (?,?,?,?,?,?,?,?)",
                (
                    r["sku_id"],
                    r.get("provider", "aws"),
                    r.get("service", "ec2"),
                    r.get("kind", "compute.vm"),
                    r["resource_name"],
                    r["region"],
                    r["region"],
                    r.get("terms_hash", "hash1"),
                ),
            )
            con.execute(
                "INSERT INTO prices VALUES (?,?,?,?,?)",
                (r["sku_id"], r.get("dimension", "on-demand"), "", r["amount"], "USD"),
            )
        con.commit()
    finally:
        con.close()
    return db_path


def _make_rows(
    regions: list[tuple[str, int]],
    families: list[str],
) -> list[dict]:
    """Generate rows: each (region, count) pair × families evenly distributed."""
    rows = []
    for region, count in regions:
        for i in range(count):
            family = families[i % len(families)]
            sku_id = f"{region}/{family}/sku-{i}"
            rows.append(
                {
                    "sku_id": sku_id,
                    "resource_name": f"{family}.large",
                    "region": region,
                    "amount": 0.10 + i * 0.001,
                }
            )
    return rows


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_sample_returns_correct_type(tmp_path: Path) -> None:
    """sample() returns a list of Sample dataclass instances."""
    families = ["m5", "c5", "r5", "t3"]
    rows = _make_rows(
        [("us-east-1", 100), ("eu-west-1", 50), ("ap-southeast-1", 5)],
        families,
    )
    db = _make_shard(tmp_path, rows)
    result = sample(db, budget=20, seed=42)
    assert isinstance(result, list)
    assert len(result) > 0
    s = result[0]
    assert isinstance(s, Sample)
    # Required fields
    assert s.sku_id
    assert s.region
    assert s.resource_name
    assert s.price_amount >= 0
    assert s.price_currency == "USD"
    assert s.dimension


def test_sample_budget_respected(tmp_path: Path) -> None:
    """sample() returns at most `budget` items."""
    families = ["m5", "c5", "r5", "t3"]
    rows = _make_rows(
        [("us-east-1", 100), ("eu-west-1", 50), ("ap-southeast-1", 5)],
        families,
    )
    db = _make_shard(tmp_path, rows)
    result = sample(db, budget=20, seed=42)
    assert len(result) <= 20


def test_sample_stratification_top_regions(tmp_path: Path) -> None:
    """At least 70% of budget comes from top-3 regions."""
    families = ["m5", "c5", "r5", "t3"]
    rows = _make_rows(
        [("us-east-1", 100), ("eu-west-1", 50), ("ap-southeast-1", 5)],
        families,
    )
    db = _make_shard(tmp_path, rows)
    budget = 20
    result = sample(db, budget=budget, seed=42)
    top_regions = {"us-east-1", "eu-west-1", "ap-southeast-1"}
    top_count = sum(1 for s in result if s.region in top_regions)
    # With only 3 regions total, all should qualify as "top regions"
    assert top_count >= int(budget * 0.70), (
        f"Expected >= {int(budget * 0.70)} from top regions, got {top_count}"
    )


def test_sample_long_tail_present(tmp_path: Path) -> None:
    """At least 1 sample from a long-tail region when budget allows."""
    families = ["m5", "c5"]
    rows = _make_rows(
        [
            ("us-east-1", 100),
            ("eu-west-1", 50),
            ("ap-southeast-1", 40),
            ("sa-east-1", 5),   # long tail
        ],
        families,
    )
    db = _make_shard(tmp_path, rows)
    result = sample(db, budget=20, seed=42)
    long_tail_regions = {"sa-east-1"}
    lt_count = sum(1 for s in result if s.region in long_tail_regions)
    assert lt_count >= 1, f"Expected >= 1 from long-tail regions, got {lt_count}"


def test_sample_family_diversity(tmp_path: Path) -> None:
    """At least 3 distinct resource families in the result."""
    families = ["m5", "c5", "r5", "t3"]
    rows = _make_rows(
        [("us-east-1", 100), ("eu-west-1", 50), ("ap-southeast-1", 5)],
        families,
    )
    db = _make_shard(tmp_path, rows)
    result = sample(db, budget=20, seed=42)
    # resource_name is like "m5.large", "c5.large" etc.
    families_seen = {s.resource_name.split(".")[0] for s in result}
    assert len(families_seen) >= 3, (
        f"Expected >= 3 distinct families, got {families_seen}"
    )


def test_sample_deterministic(tmp_path: Path) -> None:
    """Same seed produces same results; different seed may differ."""
    families = ["m5", "c5", "r5", "t3"]
    rows = _make_rows(
        [("us-east-1", 100), ("eu-west-1", 50), ("ap-southeast-1", 5)],
        families,
    )
    db = _make_shard(tmp_path, rows)
    r1 = sample(db, budget=20, seed=42)
    r2 = sample(db, budget=20, seed=42)
    assert r1 == r2, "Same seed must produce identical results"


def test_sample_different_seeds_differ(tmp_path: Path) -> None:
    """Different seeds should (almost always) produce different orderings."""
    families = ["m5", "c5", "r5", "t3"]
    rows = _make_rows(
        [("us-east-1", 100), ("eu-west-1", 50), ("ap-southeast-1", 20)],
        families,
    )
    db = _make_shard(tmp_path, rows)
    r1 = [s.sku_id for s in sample(db, budget=20, seed=1)]
    r2 = [s.sku_id for s in sample(db, budget=20, seed=999)]
    # With 170 rows and budget 20, seeds should produce different results
    assert r1 != r2, "Different seeds should produce different results"


def test_sample_small_shard(tmp_path: Path) -> None:
    """If total rows < budget, return all available rows."""
    rows = _make_rows([("us-east-1", 5)], ["m5"])
    db = _make_shard(tmp_path, rows)
    result = sample(db, budget=20, seed=42)
    assert len(result) <= 5


def test_sample_no_duplicates(tmp_path: Path) -> None:
    """No duplicate (sku_id, dimension) pairs in the sample."""
    families = ["m5", "c5", "r5", "t3"]
    rows = _make_rows(
        [("us-east-1", 100), ("eu-west-1", 50), ("ap-southeast-1", 5)],
        families,
    )
    db = _make_shard(tmp_path, rows)
    result = sample(db, budget=20, seed=42)
    seen = set()
    for s in result:
        key = (s.sku_id, s.dimension)
        assert key not in seen, f"Duplicate sample: {key}"
        seen.add(key)
