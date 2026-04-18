"""Tiny wrapper around DuckDB: open an in-memory conn with terms_hash bound as a UDF."""

from __future__ import annotations

import json
from typing import Any

import duckdb

from normalize.terms import terms_hash


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
    con = duckdb.connect(":memory:")
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
