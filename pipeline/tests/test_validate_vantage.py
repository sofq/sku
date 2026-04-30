"""Tests for pipeline.validate.vantage — offline EC2 cross-check."""

from __future__ import annotations

import json
import sqlite3
from pathlib import Path

import pytest

from validate.vantage import cross_check

# ---------------------------------------------------------------------------
# Fixture helpers
# ---------------------------------------------------------------------------


def _make_ec2_shard(tmp_path: Path, rows: list[dict]) -> Path:
    """Create a minimal EC2 SQLite shard."""
    db_path = tmp_path / "aws-ec2.db"
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
                tier_upper TEXT NOT NULL DEFAULT '',
                amount     REAL NOT NULL,
                unit       TEXT NOT NULL,
                PRIMARY KEY (sku_id, dimension, tier, tier_upper)
            ) WITHOUT ROWID;
            CREATE TABLE terms (
                sku_id         TEXT NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
                commitment     TEXT NOT NULL,
                tenancy        TEXT NOT NULL DEFAULT '',
                os             TEXT NOT NULL DEFAULT '',
                support_tier   TEXT,
                upfront        TEXT,
                payment_option TEXT
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
                    "aws",
                    "ec2",
                    "compute.vm",
                    r["instance_type"],
                    r["region"],
                    r["region"],
                    "hash1",
                ),
            )
            con.execute(
                "INSERT INTO prices VALUES (?,?,?,?,?,?)",
                (r["sku_id"], "on-demand", "", "", r["amount"], "USD"),
            )
            con.execute(
                "INSERT INTO terms VALUES (?,?,?,?,?,?,?)",
                (r["sku_id"], "on_demand", r.get("tenancy", "shared"), r.get("os", "linux"),
                 None, None, None),
            )
        con.commit()
    finally:
        con.close()
    return db_path


def _make_instances_json(tmp_path: Path, instances: list[dict]) -> Path:
    """Write a vantage instances.json with the given entries."""
    path = tmp_path / "instances.json"
    path.write_text(json.dumps(instances))
    return path


def _vantage_entry(instance_type: str, region: str, price: float) -> dict:
    """Build a minimal vantage instances.json entry."""
    return {
        "instance_type": instance_type,
        "pricing": {
            region: {
                "linux": {
                    "ondemand": str(price),
                }
            }
        },
    }


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_cross_check_no_drift(tmp_path: Path) -> None:
    """All catalog prices match vantage -> no drift records."""
    rows = [
        {"sku_id": "m5.large::us-east-1", "instance_type": "m5.large", "region": "us-east-1", "amount": 0.096},
        {"sku_id": "c5.large::eu-west-1", "instance_type": "c5.large", "region": "eu-west-1", "amount": 0.085},
    ]
    instances = [
        _vantage_entry("m5.large", "us-east-1", 0.096),
        _vantage_entry("c5.large", "eu-west-1", 0.085),
    ]
    db = _make_ec2_shard(tmp_path, rows)
    inst_json = _make_instances_json(tmp_path, instances)
    drift = cross_check(db, instances_json=inst_json)
    assert drift == []


def test_cross_check_drift_detected(tmp_path: Path) -> None:
    """One catalog price >1% off vantage -> exactly 1 drift record."""
    rows = [
        {"sku_id": "m5.large::us-east-1", "instance_type": "m5.large", "region": "us-east-1", "amount": 0.096},
        {"sku_id": "c5.large::eu-west-1", "instance_type": "c5.large", "region": "eu-west-1", "amount": 0.085},
        {"sku_id": "r5.large::ap-1", "instance_type": "r5.large", "region": "ap-southeast-1", "amount": 0.126},
    ]
    # r5.large has upstream price 5% higher
    instances = [
        _vantage_entry("m5.large", "us-east-1", 0.096),
        _vantage_entry("c5.large", "eu-west-1", 0.085),
        _vantage_entry("r5.large", "ap-southeast-1", 0.126 * 1.05),
    ]
    db = _make_ec2_shard(tmp_path, rows)
    inst_json = _make_instances_json(tmp_path, instances)
    drift = cross_check(db, instances_json=inst_json)
    assert len(drift) == 1
    rec = drift[0]
    assert "r5.large" in rec.sku_id or rec.sku_id == "r5.large::ap-1"
    assert rec.catalog_amount == pytest.approx(0.126)
    assert rec.source == "vantage"


def test_cross_check_exact_one_mismatch(tmp_path: Path) -> None:
    """Exactly 1 drifted instance type is flagged, others pass."""
    rows = [
        {"sku_id": "m5.large::us-east-1", "instance_type": "m5.large", "region": "us-east-1", "amount": 0.200},
        {"sku_id": "m5.large::eu-west-1", "instance_type": "m5.large", "region": "eu-west-1", "amount": 0.220},
    ]
    # us-east-1 drifts, eu-west-1 matches
    instances = [
        _vantage_entry("m5.large", "us-east-1", 0.200 * 1.10),  # 10% diff
        _vantage_entry("m5.large", "eu-west-1", 0.220),
    ]
    db = _make_ec2_shard(tmp_path, rows)
    inst_json = _make_instances_json(tmp_path, instances)
    drift = cross_check(db, instances_json=inst_json)
    assert len(drift) == 1
    assert "us-east-1" in drift[0].sku_id


def test_cross_check_no_match_in_vantage(tmp_path: Path) -> None:
    """Instance type in shard not found in vantage -> not flagged as drift."""
    rows = [
        {"sku_id": "x99.large::us-east-1", "instance_type": "x99.large", "region": "us-east-1", "amount": 0.500},
    ]
    instances = [
        _vantage_entry("m5.large", "us-east-1", 0.096),
    ]
    db = _make_ec2_shard(tmp_path, rows)
    inst_json = _make_instances_json(tmp_path, instances)
    drift = cross_check(db, instances_json=inst_json)
    # No vantage data for x99.large -> not treated as drift (freshness issue)
    assert drift == []


def test_cross_check_filters_os_and_tenancy(tmp_path: Path) -> None:
    """Only linux+shared rows are checked; windows rows are ignored."""
    rows = [
        {
            "sku_id": "m5.large::us-east-1::linux",
            "instance_type": "m5.large",
            "region": "us-east-1",
            "amount": 0.096,
            "os": "linux",
            "tenancy": "shared",
        },
        {
            "sku_id": "m5.large::us-east-1::windows",
            "instance_type": "m5.large",
            "region": "us-east-1",
            "amount": 9999.0,  # would flag if checked
            "os": "windows",
            "tenancy": "shared",
        },
    ]
    instances = [_vantage_entry("m5.large", "us-east-1", 0.096)]
    db = _make_ec2_shard(tmp_path, rows)
    inst_json = _make_instances_json(tmp_path, instances)
    drift = cross_check(db, instances_json=inst_json)
    assert drift == []
