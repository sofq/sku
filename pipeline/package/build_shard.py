"""Build a SQLite shard from normalized NDJSON rows. Spec §5."""

from __future__ import annotations

import argparse
import json
import sqlite3
import sys
from pathlib import Path
from typing import Any, Iterable

_HERE = Path(__file__).resolve().parent
_SCHEMA_SQL = (_HERE / "schema.sql").read_text()


def _iter_rows(path: Path) -> Iterable[dict[str, Any]]:
    with path.open() as fh:
        for line in fh:
            line = line.strip()
            if line:
                yield json.loads(line)


def _insert_row(con: sqlite3.Connection, row: dict[str, Any]) -> None:
    sku_id = row["sku_id"]
    con.execute(
        "INSERT INTO skus(sku_id, provider, service, kind, resource_name, "
        "region, region_normalized, terms_hash) VALUES(?,?,?,?,?,?,?,?)",
        (sku_id, row["provider"], row["service"], row["kind"],
         row["resource_name"], row["region"], row["region_normalized"],
         row["terms_hash"]),
    )

    attrs = row.get("resource_attrs") or {}
    con.execute(
        "INSERT INTO resource_attrs(sku_id, vcpu, memory_gb, storage_gb, "
        "gpu_count, gpu_model, architecture, context_length, max_output_tokens, "
        "modality, capabilities, quantization, durability_nines, "
        "availability_tier, extra) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
        (
            sku_id,
            attrs.get("vcpu"), attrs.get("memory_gb"), attrs.get("storage_gb"),
            attrs.get("gpu_count"), attrs.get("gpu_model"),
            attrs.get("architecture"),
            attrs.get("context_length"), attrs.get("max_output_tokens"),
            json.dumps(attrs["modality"]) if attrs.get("modality") is not None else None,
            json.dumps(attrs["capabilities"]) if attrs.get("capabilities") is not None else None,
            attrs.get("quantization"),
            attrs.get("durability_nines"), attrs.get("availability_tier"),
            json.dumps(attrs["extra"]) if attrs.get("extra") is not None else None,
        ),
    )

    t = row["terms"]
    con.execute(
        "INSERT INTO terms(sku_id, commitment, tenancy, os, support_tier, "
        "upfront, payment_option) VALUES(?,?,?,?,?,?,?)",
        (sku_id, t["commitment"], t.get("tenancy", ""), t.get("os", ""),
         t.get("support_tier") or None, t.get("upfront") or None,
         t.get("payment_option") or None),
    )

    for p in row.get("prices") or []:
        con.execute(
            "INSERT INTO prices(sku_id, dimension, tier, amount, unit) "
            "VALUES(?,?,?,?,?)",
            (sku_id, p["dimension"], p.get("tier", ""), p["amount"], p["unit"]),
        )

    h = row.get("health")
    if h:
        con.execute(
            "INSERT INTO health(sku_id, uptime_30d, latency_p50_ms, "
            "latency_p95_ms, throughput_tokens_per_sec, observed_at) "
            "VALUES(?,?,?,?,?,?)",
            (sku_id, h.get("uptime_30d"), h.get("latency_p50_ms"),
             h.get("latency_p95_ms"), h.get("throughput_tokens_per_sec"),
             h.get("observed_at")),
        )


def build_shard(
    *,
    rows_path: Path,
    shard: str,
    out_path: Path,
    catalog_version: str,
    generated_at: str,
    source_url: str,
) -> None:
    """Rebuild out_path from scratch based on rows_path."""
    if out_path.exists():
        out_path.unlink()
    out_path.parent.mkdir(parents=True, exist_ok=True)

    con = sqlite3.connect(out_path)
    try:
        con.executescript(_SCHEMA_SQL)
        con.execute("PRAGMA foreign_keys = ON")

        rows = list(_iter_rows(rows_path))
        kinds: set[str] = set()
        commitments: set[str] = set()
        tenancies: set[str] = set()
        oses: set[str] = set()
        providers: set[str] = set()

        con.execute("BEGIN")
        for row in rows:
            _insert_row(con, row)
            kinds.add(row["kind"])
            commitments.add(row["terms"]["commitment"])
            tenancies.add(row["terms"].get("tenancy", ""))
            oses.add(row["terms"].get("os", ""))
            providers.add(row["provider"])
        con.execute("COMMIT")

        meta = {
            "schema_version": "1",
            "catalog_version": catalog_version,
            "currency": "USD",
            "generated_at": generated_at,
            "source_url": source_url,
            "row_count": str(len(rows)),
            "allowed_kinds": json.dumps(sorted(kinds)),
            "allowed_commitments": json.dumps(sorted(commitments)),
            "allowed_tenancies": json.dumps(sorted(tenancies)),
            "allowed_oses": json.dumps(sorted(oses)),
            "serving_providers": json.dumps(sorted(providers)),
            "shard": shard,
            "head_version": catalog_version,
        }
        con.executemany("INSERT INTO metadata(key, value) VALUES(?,?)", meta.items())
        con.commit()
        # Clean up WAL artefacts from the single-transaction write.
        con.execute("PRAGMA wal_checkpoint(TRUNCATE)")
        con.execute("VACUUM")
    finally:
        con.close()


def main() -> int:
    ap = argparse.ArgumentParser(prog="package.build_shard")
    ap.add_argument("--rows", type=Path, required=True)
    ap.add_argument("--shard", required=True)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--catalog-version", default="dev")
    ap.add_argument("--generated-at", default="")
    ap.add_argument("--source-url", default="https://openrouter.ai/api/v1/models")
    args = ap.parse_args()

    import time
    generated_at = args.generated_at or time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())

    build_shard(
        rows_path=args.rows,
        shard=args.shard,
        out_path=args.out,
        catalog_version=args.catalog_version,
        generated_at=generated_at,
        source_url=args.source_url,
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
