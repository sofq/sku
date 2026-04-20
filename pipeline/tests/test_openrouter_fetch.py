"""Tests for pipeline/ingest/openrouter.py::fetch().

Uses requests_mock to intercept calls made by LiveClient underneath fetch().
"""

from __future__ import annotations

import json

import requests_mock as req_mock_module

from ingest.http import FixtureClient, LiveClient
from ingest.openrouter import fetch

BASE = LiveClient.BASE

MODELS_PAYLOAD = {
    "data": [
        {"id": "a/x"},
        {"id": "b/y"},
        {"id": "c/z"},
    ]
}

ENDPOINT_PAYLOADS = {
    "a/x": {"data": {"id": "a/x", "endpoints": []}},
    "b/y": {"data": {"id": "b/y", "endpoints": []}},
    "c/z": {"data": {"id": "c/z", "endpoints": []}},
}


def _register_mocks(m: req_mock_module.Mocker, models: dict, endpoints: dict) -> None:
    m.get(f"{BASE}/api/v1/models", json=models)
    for model_id, ep_payload in endpoints.items():
        m.get(f"{BASE}/api/v1/models/{model_id}/endpoints", json=ep_payload)


# ---------------------------------------------------------------------------
# Test 1: Happy path round-trip
# ---------------------------------------------------------------------------


def test_happy_path_round_trip(tmp_path):
    with req_mock_module.Mocker() as m:
        _register_mocks(m, MODELS_PAYLOAD, ENDPOINT_PAYLOADS)
        fetch(tmp_path)

    models_file = tmp_path / "models.json"
    assert models_file.exists(), "models.json must be written"

    loaded = json.loads(models_file.read_text())
    assert loaded == MODELS_PAYLOAD, "models.json content must match mocked payload"

    endpoints_dir = tmp_path / "endpoints"
    assert (endpoints_dir / "a__x.json").exists()
    assert (endpoints_dir / "b__y.json").exists()
    assert (endpoints_dir / "c__z.json").exists()

    actual_files = {p.name for p in endpoints_dir.iterdir()}
    assert actual_files == {"a__x.json", "b__y.json", "c__z.json"}, (
        "No extra files should exist in endpoints/"
    )


# ---------------------------------------------------------------------------
# Test 2: Endpoint file naming uses __ not /
# ---------------------------------------------------------------------------


def test_endpoint_file_naming(tmp_path):
    with req_mock_module.Mocker() as m:
        _register_mocks(m, MODELS_PAYLOAD, ENDPOINT_PAYLOADS)
        fetch(tmp_path)

    endpoints_dir = tmp_path / "endpoints"
    for name in endpoints_dir.iterdir():
        assert "/" not in name.stem, f"File name must not contain '/': {name.name}"
        assert "__" in name.stem, f"File name must use '__' separator: {name.name}"


# ---------------------------------------------------------------------------
# Test 3: Hash stability — two calls with identical mocks yield same digest
# ---------------------------------------------------------------------------


def test_hash_stability(tmp_path):
    dir_a = tmp_path / "run_a"
    dir_b = tmp_path / "run_b"

    with req_mock_module.Mocker() as m:
        _register_mocks(m, MODELS_PAYLOAD, ENDPOINT_PAYLOADS)
        digest_a = fetch(dir_a)

    with req_mock_module.Mocker() as m:
        _register_mocks(m, MODELS_PAYLOAD, ENDPOINT_PAYLOADS)
        digest_b = fetch(dir_b)

    assert digest_a == digest_b, "Identical model sets must produce identical digests"


# ---------------------------------------------------------------------------
# Test 4: Hash changes when model set changes
# ---------------------------------------------------------------------------


def test_hash_changes_on_different_model_set(tmp_path):
    models_v2 = {"data": [{"id": "a/x"}, {"id": "b/y"}]}
    endpoints_v2 = {
        "a/x": {"data": {"id": "a/x", "endpoints": []}},
        "b/y": {"data": {"id": "b/y", "endpoints": []}},
    }

    dir_a = tmp_path / "v1"
    dir_b = tmp_path / "v2"

    with req_mock_module.Mocker() as m:
        _register_mocks(m, MODELS_PAYLOAD, ENDPOINT_PAYLOADS)
        digest_v1 = fetch(dir_a)

    with req_mock_module.Mocker() as m:
        _register_mocks(m, models_v2, endpoints_v2)
        digest_v2 = fetch(dir_b)

    assert digest_v1 != digest_v2, "Different model sets must produce different digests"


# ---------------------------------------------------------------------------
# Test 5: FixtureClient compatibility
# ---------------------------------------------------------------------------


def test_fixture_client_compatibility(tmp_path):
    with req_mock_module.Mocker() as m:
        _register_mocks(m, MODELS_PAYLOAD, ENDPOINT_PAYLOADS)
        fetch(tmp_path)

    client = FixtureClient(tmp_path)

    models_result = client.get("/api/v1/models")
    assert models_result == MODELS_PAYLOAD, (
        "FixtureClient.get('/api/v1/models') must return the mocked payload"
    )

    ep_result = client.get("/api/v1/models/a/x/endpoints")
    assert ep_result == ENDPOINT_PAYLOADS["a/x"], (
        "FixtureClient.get('/api/v1/models/a/x/endpoints') must return the mocked endpoint"
    )


# ---------------------------------------------------------------------------
# Test 6: Empty models list
# ---------------------------------------------------------------------------


def test_empty_models(tmp_path):
    empty_models = {"data": []}

    with req_mock_module.Mocker() as m:
        m.get(f"{BASE}/api/v1/models", json=empty_models)
        digest = fetch(tmp_path)

    assert (tmp_path / "models.json").exists(), "models.json must be written even for empty list"

    endpoints_dir = tmp_path / "endpoints"
    if endpoints_dir.exists():
        items = list(endpoints_dir.iterdir())
        assert items == [], "endpoints/ must be empty for empty model list"

    # Hash of empty sorted list
    import hashlib

    expected_digest = hashlib.sha256(json.dumps([]).encode()).hexdigest()
    assert digest == expected_digest, "Digest must be hash of empty JSON array"
