import json
import sqlite3
from pathlib import Path

from package.build_shard import build_shard

FIX_ROWS = [
    {
        "sku_id": "anthropic/claude-opus-4.6::anthropic::default",
        "provider": "anthropic",
        "service": "llm",
        "kind": "llm.text",
        "resource_name": "anthropic/claude-opus-4.6",
        "region": "",
        "region_normalized": "",
        "terms": {"commitment": "on_demand", "tenancy": "", "os": "",
                  "support_tier": "", "upfront": "", "payment_option": ""},
        "terms_hash": "ee2303ad38b3e0b0e4f01bfbb1bcba8f",
        "resource_attrs": {
            "context_length": 200000, "max_output_tokens": 64000,
            "modality": ["text"], "capabilities": ["tools"],
            "quantization": None,
        },
        "prices": [
            {"dimension": "prompt", "tier": "", "amount": 1.5e-5, "unit": "token"},
            {"dimension": "completion", "tier": "", "amount": 7.5e-5, "unit": "token"},
        ],
        "health": {"uptime_30d": 0.998, "latency_p50_ms": 420,
                   "latency_p95_ms": 1100, "throughput_tokens_per_sec": 62.5,
                   "observed_at": 1745020800},
        "is_aggregated": False,
    },
]


def write_rows(tmp_path: Path, rows: list[dict]) -> Path:
    p = tmp_path / "rows.jsonl"
    with p.open("w") as fh:
        for r in rows:
            fh.write(json.dumps(r) + "\n")
    return p


def test_build_shard_populates_all_tables(tmp_path: Path):
    rows_path = write_rows(tmp_path, FIX_ROWS)
    out = tmp_path / "openrouter.db"
    build_shard(
        rows_path=rows_path,
        shard="openrouter",
        out_path=out,
        catalog_version="2026.04.18",
        generated_at="2026-04-18T00:00:00Z",
        source_url="https://openrouter.ai/api/v1/models",
    )

    con = sqlite3.connect(out)
    try:
        con.execute("PRAGMA foreign_keys = ON")
        assert con.execute("SELECT count(*) FROM skus").fetchone()[0] == 1
        assert con.execute("SELECT count(*) FROM resource_attrs").fetchone()[0] == 1
        assert con.execute("SELECT count(*) FROM terms").fetchone()[0] == 1
        assert con.execute("SELECT count(*) FROM prices").fetchone()[0] == 2
        assert con.execute("SELECT count(*) FROM health").fetchone()[0] == 1

        meta = dict(con.execute("SELECT key, value FROM metadata").fetchall())
        assert meta["schema_version"] == "1"
        assert meta["catalog_version"] == "2026.04.18"
        assert meta["currency"] == "USD"
        assert meta["generated_at"] == "2026-04-18T00:00:00Z"
        assert meta["row_count"] == "1"
        assert json.loads(meta["allowed_kinds"]) == ["llm.text"]

        # Index presence
        idx = {r[0] for r in con.execute(
            "SELECT name FROM sqlite_master WHERE type='index' AND name LIKE 'idx_%'"
        ).fetchall()}
        for want in ("idx_skus_lookup", "idx_resource_llm", "idx_skus_region",
                     "idx_prices_by_dim", "idx_terms_commitment"):
            assert want in idx, f"missing index {want}: got {idx}"

        # FK cascade
        con.execute("DELETE FROM skus WHERE sku_id=?",
                    ("anthropic/claude-opus-4.6::anthropic::default",))
        con.commit()
        assert con.execute("SELECT count(*) FROM prices").fetchone()[0] == 0
        assert con.execute("SELECT count(*) FROM terms").fetchone()[0] == 0
    finally:
        con.close()


def test_build_shard_seeds_metadata_from_rows(tmp_path: Path):
    rows = list(FIX_ROWS)
    # Add a multimodal row so allowed_kinds has two entries
    rows.append({**FIX_ROWS[0],
                 "sku_id": "openai/gpt-5::openai::default",
                 "resource_name": "openai/gpt-5",
                 "provider": "openai",
                 "kind": "llm.multimodal"})
    rows_path = write_rows(tmp_path, rows)
    out = tmp_path / "openrouter.db"
    build_shard(rows_path=rows_path, shard="openrouter", out_path=out,
                catalog_version="2026.04.18", generated_at="x",
                source_url="y")

    con = sqlite3.connect(out)
    try:
        meta = dict(con.execute("SELECT key, value FROM metadata").fetchall())
        assert sorted(json.loads(meta["allowed_kinds"])) == ["llm.multimodal", "llm.text"]
        assert meta["row_count"] == "2"
        assert "serving_providers" in meta
        assert sorted(json.loads(meta["serving_providers"])) == ["anthropic", "openai"]
    finally:
        con.close()
