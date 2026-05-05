"""Tests for pipeline.validate.driver — orchestration and CLI."""

from __future__ import annotations

import json
import sqlite3
from pathlib import Path

from validate.driver import run_validation
from validate.sampler import Sample

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_minimal_shard(tmp_path: Path) -> Path:
    """Create a minimal shard with 2 SKUs, one per region."""
    db_path = tmp_path / "test.db"
    con = sqlite3.connect(db_path)
    try:
        con.executescript(
            """
            PRAGMA foreign_keys = ON;
            CREATE TABLE skus (
                sku_id TEXT NOT NULL PRIMARY KEY, provider TEXT NOT NULL,
                service TEXT NOT NULL, kind TEXT NOT NULL,
                resource_name TEXT NOT NULL, region TEXT NOT NULL,
                region_normalized TEXT NOT NULL, terms_hash TEXT NOT NULL
            ) WITHOUT ROWID;
            CREATE TABLE prices (
                sku_id TEXT NOT NULL REFERENCES skus(sku_id),
                dimension TEXT NOT NULL, tier TEXT NOT NULL DEFAULT '',
                tier_upper TEXT NOT NULL DEFAULT '',
                amount REAL NOT NULL, unit TEXT NOT NULL,
                PRIMARY KEY (sku_id, dimension, tier, tier_upper)
            ) WITHOUT ROWID;
            CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT);
            INSERT INTO metadata VALUES ('currency', 'USD');
            INSERT INTO skus VALUES ('sku-1', 'aws', 'ec2', 'compute.vm',
                'm5.large', 'us-east-1', 'us-east-1', 'h1');
            INSERT INTO skus VALUES ('sku-2', 'aws', 'ec2', 'compute.vm',
                'c5.large', 'eu-west-1', 'eu-west-1', 'h2');
            INSERT INTO prices VALUES ('sku-1', 'on-demand', '', '', 0.096, 'USD');
            INSERT INTO prices VALUES ('sku-2', 'on-demand', '', '', 0.085, 'USD');
            """
        )
        con.commit()
    finally:
        con.close()
    return db_path


def _no_drift_revalidator(
    samples: list[Sample],
    **kwargs,
) -> tuple[list, list]:
    return [], []


def _drift_revalidator(
    samples: list[Sample],
    **kwargs,
) -> tuple[list, list]:
    from validate.aws import DriftRecord

    drift = [
        DriftRecord(
            sku_id=samples[0].sku_id,
            catalog_amount=samples[0].price_amount,
            upstream_amount=samples[0].price_amount * 1.05,
            delta_pct=5.0,
            source="aws",
        )
    ]
    return drift, []


def _missing_revalidator(
    samples: list[Sample],
    **kwargs,
) -> tuple[list, list]:
    return [], [samples[0].sku_id]


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_driver_pass_no_drift(tmp_path: Path) -> None:
    """All samples within tolerance -> report.exit == 'pass', exit code 0."""
    db = _make_minimal_shard(tmp_path)
    report_path = tmp_path / "report.json"

    result = run_validation(
        shard="aws-ec2",
        shard_db=db,
        budget=5,
        report=report_path,
        revalidator=_no_drift_revalidator,
        seed=42,
    )
    assert result == 0
    assert report_path.exists()
    data = json.loads(report_path.read_text())
    assert data["shard"] == "aws-ec2"
    assert data["exit"] == "pass"
    assert data["drift_records"] == []
    assert "generated_at" in data
    assert "sample_size" in data


def test_driver_fail_on_drift(tmp_path: Path) -> None:
    """Drift detected -> report.exit == 'fail', exit code 1."""
    db = _make_minimal_shard(tmp_path)
    report_path = tmp_path / "report.json"

    result = run_validation(
        shard="aws-ec2",
        shard_db=db,
        budget=5,
        report=report_path,
        revalidator=_drift_revalidator,
        seed=42,
    )
    assert result == 1
    data = json.loads(report_path.read_text())
    assert data["exit"] == "fail"
    assert len(data["drift_records"]) == 1
    assert data["drift_records"][0]["source"] == "aws"


def test_driver_missing_not_fail(tmp_path: Path) -> None:
    """Missing upstream entries do not cause a failure."""
    db = _make_minimal_shard(tmp_path)
    report_path = tmp_path / "report.json"

    result = run_validation(
        shard="aws-ec2",
        shard_db=db,
        budget=5,
        report=report_path,
        revalidator=_missing_revalidator,
        seed=42,
    )
    assert result == 0
    data = json.loads(report_path.read_text())
    assert data["exit"] == "pass"
    assert len(data["missing_upstream"]) >= 1


def test_driver_report_schema(tmp_path: Path) -> None:
    """Verify all required keys appear in the JSON report."""
    db = _make_minimal_shard(tmp_path)
    report_path = tmp_path / "report.json"
    run_validation(
        shard="openrouter",
        shard_db=db,
        budget=5,
        report=report_path,
        revalidator=_no_drift_revalidator,
        seed=42,
    )
    data = json.loads(report_path.read_text())
    for key in ("shard", "generated_at", "sample_size", "drift_records",
                "missing_upstream", "vantage_drift", "exit"):
        assert key in data, f"Missing key: {key}"


def test_driver_vantage_drift_in_report(tmp_path: Path) -> None:
    """Vantage drift records appear in report and cause fail."""
    db = _make_minimal_shard(tmp_path)
    report_path = tmp_path / "report.json"
    from validate.vantage import DriftRecord as VDriftRecord
    vantage_drift = [
        VDriftRecord(
            sku_id="sku-1",
            catalog_amount=0.096,
            upstream_amount=0.110,
            delta_pct=14.5,
        )
    ]

    result = run_validation(
        shard="aws-ec2",
        shard_db=db,
        budget=5,
        report=report_path,
        revalidator=_no_drift_revalidator,
        vantage_drift=vantage_drift,
        seed=42,
    )
    assert result == 1
    data = json.loads(report_path.read_text())
    assert data["exit"] == "fail"
    assert len(data["vantage_drift"]) == 1


def test_driver_aggregates_multiple_drift_records(tmp_path: Path) -> None:
    """Multiple drift records from revalidator are all captured."""
    from validate.aws import DriftRecord

    db = _make_minimal_shard(tmp_path)
    report_path = tmp_path / "report.json"

    def multi_drift(samples, **kwargs):
        return [
            DriftRecord(sku_id=s.sku_id, catalog_amount=s.price_amount,
                        upstream_amount=s.price_amount * 1.10, delta_pct=10.0,
                        source="aws")
            for s in samples
        ], []

    result = run_validation(
        shard="aws-ec2",
        shard_db=db,
        budget=5,
        report=report_path,
        revalidator=multi_drift,
        seed=42,
    )
    data = json.loads(report_path.read_text())
    assert result == 1
    assert len(data["drift_records"]) == data["sample_size"]



def test_driver_skips_listed_shard(tmp_path: Path) -> None:
    """Shards in SKIP_REVALIDATION are skipped — revalidator is not called."""
    from validate.driver import SKIP_REVALIDATION

    db = _make_minimal_shard(tmp_path)
    report_path = tmp_path / "report.json"

    called = False

    def must_not_be_called(samples, **kwargs):
        nonlocal called
        called = True
        return [], []

    skipped_shard = next(iter(SKIP_REVALIDATION))
    result = run_validation(
        shard=skipped_shard,
        shard_db=db,
        budget=5,
        report=report_path,
        revalidator=must_not_be_called,
        seed=42,
    )
    assert result == 0
    assert not called
    data = json.loads(report_path.read_text())
    assert data["exit"] == "skipped"
    assert data["skip_reason"]
    assert data["sample_size"] == 0
    assert data["drift_records"] == []


def test_driver_filters_to_primary_dimensions(tmp_path: Path) -> None:
    """Samples whose dimension isn't in PRIMARY_DIMENSIONS[shard] are filtered out."""
    from validate.driver import PRIMARY_DIMENSIONS

    db = _make_minimal_shard(tmp_path)
    report_path = tmp_path / "report.json"

    seen_dims: list[str] = []

    def capture(samples, **kwargs):
        seen_dims.extend(s.dimension for s in samples)
        return [], []

    # Inject a temporary primary-dim filter for the test shard so the existing
    # minimal fixture (whose dimension is "on-demand") survives.
    PRIMARY_DIMENSIONS["aws-ec2"] = frozenset({"on-demand"})
    try:
        run_validation(
            shard="aws-ec2",
            shard_db=db,
            budget=10,
            report=report_path,
            revalidator=capture,
            seed=42,
        )
    finally:
        del PRIMARY_DIMENSIONS["aws-ec2"]

    assert seen_dims, "revalidator received no samples"
    assert all(d == "on-demand" for d in seen_dims)
