"""Golden-row tests: fixture Pub/Sub skus.json -> normalized NDJSON matches golden."""

from __future__ import annotations

import json
from pathlib import Path

from ingest.gcp_pubsub_topics import ingest

FIXTURE_DIR = Path(__file__).resolve().parent.parent / "testdata" / "gcp_pubsub_topics"
FIXTURE = FIXTURE_DIR / "skus.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "gcp_pubsub_topics_rows.jsonl"


def _rows() -> list[dict]:
    return list(ingest(skus_path=FIXTURE))


def test_fixture_matches_golden():
    rows = _rows()
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert rows == expected


def test_single_global_row():
    rows = _rows()
    assert len(rows) == 1
    assert rows[0]["region"] == "global"
    assert rows[0]["region_normalized"] == "global"


def test_kind_is_messaging_topic():
    rows = _rows()
    assert rows
    assert all(r["kind"] == "messaging.topic" for r in rows)


def test_resource_name_is_throughput():
    rows = _rows()
    assert rows
    assert all(r["resource_name"] == "throughput" for r in rows)


def test_row_has_storage_dimension_or_throughput_dimension():
    rows = _rows()
    assert rows
    for row in rows:
        dims = {p["dimension"] for p in row["prices"]}
        assert "throughput" in dims, f"expected throughput dimension, got: {dims}"
    # Price per GiB should be ~0.039 (40 USD/TiB / 1024)
    amount = rows[0]["prices"][0]["amount"]
    assert abs(amount - 0.0390625) < 1e-9
    assert rows[0]["prices"][0]["unit"] == "gb-mo"
