import copy
import json
from pathlib import Path

import pytest

from ingest.http import FixtureClient
from ingest.openrouter import NonUSDError, ingest

ROOT = Path(__file__).resolve().parents[1]
FIX = ROOT / "testdata" / "openrouter"
GOLDEN = ROOT / "testdata" / "golden" / "openrouter_rows.jsonl"


def test_ingest_matches_golden_rows(monkeypatch):
    monkeypatch.setenv("SKU_FIXED_OBSERVED_AT", "1745020800")
    # Skip the non-USD fixture model entirely so this test exercises only the
    # USD models; the USD guard gets its own test below.
    client = FixtureClient(FIX)
    models = client.get("/api/v1/models")
    models["data"] = [m for m in models["data"] if m["id"] != "non-usd-model/some-provider"]

    # Shim the client so ingest sees the filtered model list but still fetches
    # endpoints from disk for the remaining models.
    real_get = client.get

    def fake_get(path):
        if path == "/api/v1/models":
            return models
        return real_get(path)

    client.get = fake_get  # type: ignore[assignment]

    rows = ingest(client, generated_at="2026-04-18T00:00:00Z")
    rows_sorted = sorted(rows, key=lambda r: r["sku_id"])

    expected = []
    with GOLDEN.open() as fh:
        for line in fh:
            line = line.strip()
            if line:
                expected.append(json.loads(line))
    expected_sorted = sorted(expected, key=lambda r: r["sku_id"])

    assert len(rows_sorted) == len(expected_sorted)
    for got, want in zip(rows_sorted, expected_sorted, strict=True):
        assert got == want, got["sku_id"]


def test_ingest_rejects_non_usd_endpoint():
    client = FixtureClient(FIX)
    # Limit to the non-USD fixture model so the guard fires first.
    models = client.get("/api/v1/models")
    models["data"] = [m for m in models["data"] if m["id"] == "non-usd-model/some-provider"]
    real_get = client.get

    def fake_get(path):
        if path == "/api/v1/models":
            return models
        return real_get(path)

    client.get = fake_get  # type: ignore[assignment]

    with pytest.raises(NonUSDError) as ei:
        ingest(client, generated_at="2026-04-18T00:00:00Z")
    assert "non-usd-model/some-provider" in str(ei.value)
    assert "some-provider" in str(ei.value)


def test_synthetic_aggregated_row_present():
    client = FixtureClient(FIX)
    models = client.get("/api/v1/models")
    models["data"] = [m for m in models["data"] if m["id"] == "openai/gpt-5"]
    real_get = client.get

    def fake_get(path):
        if path == "/api/v1/models":
            return models
        return real_get(path)

    client.get = fake_get  # type: ignore[assignment]

    rows = ingest(client, generated_at="2026-04-18T00:00:00Z")
    agg = [r for r in rows if r["is_aggregated"]]
    assert len(agg) == 1
    assert agg[0]["sku_id"] == "openai/gpt-5::openrouter::default"
    assert agg[0]["provider"] == "openrouter"
    assert agg[0]["health"] is None


def _dup_model(
    top_pricing: dict[str, str],
    ep_pricings: list[dict[str, str]],
    uptimes: list[float],
) -> dict:
    model = {
        "id": "dup/model",
        "name": "Dup Model",
        "architecture": {
            "modality": "text",
            "input_modalities": ["text"],
            "output_modalities": ["text"],
        },
        "pricing": top_pricing,
        "top_provider": {
            "context_length": 1000,
            "max_completion_tokens": 100,
            "is_moderated": False,
        },
        "supported_parameters": [],
    }
    endpoints = []
    for p, up in zip(ep_pricings, uptimes, strict=True):
        endpoints.append({
            "provider_name": "dup-provider",
            "tag": "dup-provider",
            "context_length": 1000,
            "max_completion_tokens": 100,
            "quantization": "unknown",
            "pricing": p,
            "uptime_last_30m": up,
            "latency": {"p50_ms": 10, "p95_ms": 20},
            "throughput_tokens_per_second": 1.0,
        })
    return {"model": model, "endpoints": endpoints}


class _FakeClient:
    def __init__(self, models: list[dict], ep_by_slug: dict[str, list[dict]]):
        self._models = {"data": models}
        self._ep = ep_by_slug

    def get(self, path: str):
        if path == "/api/v1/models":
            return copy.deepcopy(self._models)
        slug = path.removeprefix("/api/v1/models/").removesuffix("/endpoints")
        return {"data": {"id": slug, "endpoints": copy.deepcopy(self._ep[slug])}}


def test_dedupe_collapses_duplicate_sku_id_keeping_higher_uptime(monkeypatch):
    monkeypatch.setenv("SKU_FIXED_OBSERVED_AT", "1745020800")
    p = {"prompt": "0.000001", "completion": "0.000002", "currency": "USD"}
    data = _dup_model(top_pricing=p, ep_pricings=[p, p], uptimes=[0.90, 0.99])
    client = _FakeClient([data["model"]], {"dup/model": data["endpoints"]})

    rows = ingest(client, generated_at="2026-04-18T00:00:00Z")

    ep_rows = [r for r in rows if not r["is_aggregated"]]
    assert len(ep_rows) == 1, ep_rows
    assert ep_rows[0]["sku_id"] == "dup/model::dup-provider::unknown"
    assert ep_rows[0]["health"]["uptime_30d"] == pytest.approx(0.99)


def test_dedupe_drops_duplicates_with_divergent_prices(monkeypatch, capsys):
    monkeypatch.setenv("SKU_FIXED_OBSERVED_AT", "1745020800")
    p1 = {"prompt": "0.000001", "completion": "0.000002", "currency": "USD"}
    p2 = {"prompt": "0.000003", "completion": "0.000004", "currency": "USD"}
    data = _dup_model(top_pricing=p1, ep_pricings=[p1, p2], uptimes=[0.90, 0.99])
    client = _FakeClient([data["model"]], {"dup/model": data["endpoints"]})

    rows = ingest(client, generated_at="2026-04-18T00:00:00Z")
    err = capsys.readouterr().err
    assert "dropped duplicate sku_id with divergent prices" in err
    assert "dup/model::dup-provider::unknown" in err

    ep_rows = [r for r in rows if not r["is_aggregated"]]
    assert ep_rows == []
    agg = [r for r in rows if r["is_aggregated"]]
    assert len(agg) == 1
    assert agg[0]["sku_id"] == "dup/model::openrouter::default"
