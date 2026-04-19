"""Tiny wrapper around DuckDB: open an in-memory conn with terms_hash bound as a UDF."""

from __future__ import annotations

import json
import os
from typing import Any

import duckdb

from normalize.terms import terms_hash

# Spill directory for intermediate hash joins / aggregations when the large
# AWS offer files blow past memory_limit. Without this, DuckDB falls back to
# the system RAM ceiling (~80% of runner RAM = ~12 GiB on ubuntu-latest) and
# OOMs before spilling. See d574924 post-mortem in M3a.4.2 runbook.
_TEMP_DIR = "/tmp/sku_duckdb"
_MEMORY_LIMIT = "4GB"


def _terms_hash_udf(
    commitment: str, tenancy: str, os: str,
    support_tier: str, upfront: str, payment_option: str,
) -> str:
    return terms_hash({
        "commitment": commitment or "",
        "tenancy": tenancy or "",
        "os": os or "",
        "support_tier": support_tier or "",
        "upfront": upfront or "",
        "payment_option": payment_option or "",
    })


def open_conn() -> duckdb.DuckDBPyConnection:
    """Return an in-memory conn with `sku_terms_hash(...)` bound as a scalar UDF."""
    os.makedirs(_TEMP_DIR, exist_ok=True)
    con = duckdb.connect(":memory:")
    con.execute(f"SET memory_limit='{_MEMORY_LIMIT}'")
    con.execute(f"SET temp_directory='{_TEMP_DIR}'")
    con.create_function(
        "sku_terms_hash",
        _terms_hash_udf,
        parameters=["VARCHAR"] * 6,
        return_type="VARCHAR",
    )
    return con


def dumps(obj: Any) -> str:
    """Compact JSON — identical to openrouter.py's encoding."""
    return json.dumps(obj, separators=(",", ":"), ensure_ascii=False)
