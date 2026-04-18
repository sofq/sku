from pathlib import Path

from ingest.http import FixtureClient


def test_fixture_client_loads_models_and_endpoint():
    root = Path(__file__).resolve().parents[1] / "testdata" / "openrouter"
    client = FixtureClient(root)

    models = client.get("/api/v1/models")
    assert "data" in models
    ids = [m["id"] for m in models["data"]]
    assert "anthropic/claude-opus-4.6" in ids

    ep = client.get("/api/v1/models/anthropic/claude-opus-4.6/endpoints")
    assert ep["data"]["id"] == "anthropic/claude-opus-4.6"
    assert len(ep["data"]["endpoints"]) == 2
