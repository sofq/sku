"""Tests for discover.driver.run (and __main__ entrypoint smoke)."""

from __future__ import annotations

import json
from pathlib import Path

import pytest
import requests_mock

from discover.driver import ALL_SHARDS, run
from ingest.aws_common import _AWS_OFFER_BASE
from ingest.azure_common import _AZURE_RETAIL_BASE
from ingest.gcp_common import _GCP_BILLING_BASE, _GCP_SERVICE_IDS


def _mock_all_providers(m: requests_mock.Mocker, *, aws_pub: str = "2026-04-18T00:00:00Z") -> None:
    """Register mocks for every shard so a full ALL_SHARDS discover passes."""
    from ingest.aws_common import _AWS_SERVICE_CODES

    for service_code in set(_AWS_SERVICE_CODES.values()):
        m.get(
            f"{_AWS_OFFER_BASE}/{service_code}/index.json",
            json={"publicationDate": aws_pub},
        )
    m.get(_AZURE_RETAIL_BASE, json={"Items": []})
    for service_id in _GCP_SERVICE_IDS.values():
        m.get(
            f"{_GCP_BILLING_BASE}/services/{service_id}/skus",
            json={"skus": [{"skuId": f"sku-{service_id}"}]},
        )
    m.get("https://openrouter.ai/api/v1/models", json={"data": []})


def test_dry_run_empty_state_baseline_rebuild_true(tmp_path: Path) -> None:
    state = tmp_path / "state.json"
    out = tmp_path / "changed.json"
    rc = run(state_path=state, out_path=out, live=False)
    assert rc == 0
    doc = json.loads(out.read_text())
    assert doc["baseline_rebuild"] is True
    assert doc["shards"] == []
    assert doc["errors"] == []


def test_dry_run_populated_state_baseline_rebuild_false(tmp_path: Path) -> None:
    state = tmp_path / "state.json"
    # Pre-populate state.
    state.write_text(json.dumps({"schema_version": 1, "indicators": {"aws_ec2": "v1"}}))
    out = tmp_path / "changed.json"
    rc = run(state_path=state, out_path=out, live=False)
    assert rc == 0
    doc = json.loads(out.read_text())
    assert doc["baseline_rebuild"] is False
    assert doc["shards"] == []


def test_first_live_run_all_shards_appear(tmp_path: Path) -> None:
    state = tmp_path / "state.json"
    out = tmp_path / "changed.json"
    with requests_mock.Mocker() as m:
        _mock_all_providers(m)
        rc = run(
            state_path=state,
            out_path=out,
            live=True,
            gcp_api_key="test-key",
        )
    assert rc == 0
    doc = json.loads(out.read_text())
    assert doc["baseline_rebuild"] is True
    assert set(doc["shards"]) == set(ALL_SHARDS)
    assert doc["errors"] == []
    # State was persisted.
    saved = json.loads(state.read_text())
    assert set(saved["indicators"].keys()) == set(ALL_SHARDS)


def test_second_live_run_unchanged_upstream_empty_shards(tmp_path: Path) -> None:
    state = tmp_path / "state.json"
    out = tmp_path / "changed.json"
    with requests_mock.Mocker() as m:
        _mock_all_providers(m)
        run(state_path=state, out_path=out, live=True, gcp_api_key="k")
    # Second run with identical upstream → no changes.
    with requests_mock.Mocker() as m:
        _mock_all_providers(m)
        rc = run(state_path=state, out_path=out, live=True, gcp_api_key="k")
    assert rc == 0
    doc = json.loads(out.read_text())
    assert doc["baseline_rebuild"] is False
    assert doc["shards"] == []
    assert set(doc["unchanged"]) == set(ALL_SHARDS)


def test_one_shard_changed(tmp_path: Path) -> None:
    state = tmp_path / "state.json"
    out = tmp_path / "changed.json"
    with requests_mock.Mocker() as m:
        _mock_all_providers(m, aws_pub="2026-04-18T00:00:00Z")
        run(state_path=state, out_path=out, live=True, gcp_api_key="k")
    # Second run: AWS publicationDate changes.
    with requests_mock.Mocker() as m:
        _mock_all_providers(m, aws_pub="2026-04-19T00:00:00Z")
        rc = run(state_path=state, out_path=out, live=True, gcp_api_key="k")
    assert rc == 0
    doc = json.loads(out.read_text())
    aws_shards = [s for s in ALL_SHARDS if s.startswith("aws_")]
    assert set(doc["shards"]) == set(aws_shards)


def test_baseline_rebuild_flag_forces_all(tmp_path: Path) -> None:
    state = tmp_path / "state.json"
    out = tmp_path / "changed.json"
    # Seed a live state so the "!prev_map" branch wouldn't already force all shards.
    with requests_mock.Mocker() as m:
        _mock_all_providers(m)
        run(state_path=state, out_path=out, live=True, gcp_api_key="k")
    # Now rerun with --baseline-rebuild even though upstream is identical.
    with requests_mock.Mocker() as m:
        _mock_all_providers(m)
        rc = run(state_path=state, out_path=out, live=True, baseline_rebuild=True, gcp_api_key="k")
    assert rc == 0
    doc = json.loads(out.read_text())
    assert doc["baseline_rebuild"] is True
    assert set(doc["shards"]) == set(ALL_SHARDS)


def test_shards_allowlist_restricts_and_validates(tmp_path: Path) -> None:
    state = tmp_path / "state.json"
    out = tmp_path / "changed.json"
    with requests_mock.Mocker() as m:
        _mock_all_providers(m)
        rc = run(
            state_path=state,
            out_path=out,
            live=True,
            shards=["aws_ec2", "aws_rds"],
            gcp_api_key="k",
        )
    assert rc == 0
    doc = json.loads(out.read_text())
    # only those two possible
    assert set(doc["shards"]) <= {"aws_ec2", "aws_rds"}


def test_dashed_shard_alias_accepted(tmp_path: Path) -> None:
    state = tmp_path / "state.json"
    out = tmp_path / "changed.json"
    with requests_mock.Mocker() as m:
        _mock_all_providers(m)
        rc = run(
            state_path=state,
            out_path=out,
            live=True,
            shards=["aws-ec2"],
            gcp_api_key="k",
        )
    assert rc == 0


def test_unknown_shard_exits_4(tmp_path: Path) -> None:
    state = tmp_path / "state.json"
    out = tmp_path / "changed.json"
    rc = run(
        state_path=state,
        out_path=out,
        live=True,
        shards=["made_up_shard"],
        gcp_api_key="k",
    )
    assert rc == 4
    doc = json.loads(out.read_text())
    assert any(e["reason"] == "unknown_shard" for e in doc["errors"])


def test_one_shard_errors_others_still_succeed(tmp_path: Path) -> None:
    state = tmp_path / "state.json"
    out = tmp_path / "changed.json"
    from ingest.aws_common import _AWS_SERVICE_CODES

    with requests_mock.Mocker() as m:
        # Set up AWS discover to FAIL (the AWS group all errors as a unit),
        # but openrouter succeeds.
        for service_code in set(_AWS_SERVICE_CODES.values()):
            m.get(f"{_AWS_OFFER_BASE}/{service_code}/index.json", status_code=500)
        m.get("https://openrouter.ai/api/v1/models", json={"data": []})
        rc = run(
            state_path=state,
            out_path=out,
            live=True,
            shards=["aws_ec2", "openrouter"],
            gcp_api_key="k",
        )
    assert rc == 0
    doc = json.loads(out.read_text())
    assert any(e["shard"] == "aws_ec2" for e in doc["errors"])
    assert "openrouter" in doc["shards"] or "openrouter" in doc["unchanged"]


def test_all_shards_error_exits_2(tmp_path: Path) -> None:
    state = tmp_path / "state.json"
    out = tmp_path / "changed.json"
    from ingest.aws_common import _AWS_SERVICE_CODES

    with requests_mock.Mocker() as m:
        for service_code in set(_AWS_SERVICE_CODES.values()):
            m.get(f"{_AWS_OFFER_BASE}/{service_code}/index.json", status_code=500)
        rc = run(
            state_path=state,
            out_path=out,
            live=True,
            shards=["aws_ec2", "aws_rds"],
            gcp_api_key="k",
        )
    assert rc == 2
    doc = json.loads(out.read_text())
    assert len(doc["errors"]) >= 2
    assert doc["shards"] == []


def test_main_entrypoint_smoke(tmp_path: Path) -> None:
    """python -m discover parses args and returns exit code."""
    from discover.__main__ import main

    state = tmp_path / "state.json"
    out = tmp_path / "changed.json"
    rc = main(["--state", str(state), "--out", str(out)])
    assert rc == 0
    assert out.exists()
    doc = json.loads(out.read_text())
    assert doc["baseline_rebuild"] is True


def test_main_entrypoint_invalid_shard(tmp_path: Path) -> None:
    from discover.__main__ import main

    state = tmp_path / "state.json"
    out = tmp_path / "changed.json"
    rc = main(["--state", str(state), "--out", str(out), "--shards", "bogus"])
    assert rc == 4


def test_dry_run_output_is_indented_and_sorted(tmp_path: Path) -> None:
    state = tmp_path / "state.json"
    out = tmp_path / "changed.json"
    run(state_path=state, out_path=out, live=False)
    raw = out.read_text()
    assert "\n" in raw and "  " in raw  # indented
    doc = json.loads(raw)
    assert list(doc.keys()) == sorted(doc.keys())


def test_gcp_live_without_api_key_records_error(tmp_path: Path) -> None:
    state = tmp_path / "state.json"
    out = tmp_path / "changed.json"
    with requests_mock.Mocker() as m:
        _mock_all_providers(m)
        rc = run(
            state_path=state,
            out_path=out,
            live=True,
            shards=["gcp_gce"],
            gcp_api_key=None,
        )
    # Single-shard all-errored → exit 2.
    assert rc == 2
    doc = json.loads(out.read_text())
    assert any(e["reason"] == "gcp_missing_api_key" for e in doc["errors"])


if __name__ == "__main__":
    pytest.main([__file__])
