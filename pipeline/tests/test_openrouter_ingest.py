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
    monkeypatch = None  # not needed for this test
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
