"""Tiny wrapper around DuckDB: open an in-memory conn with terms_hash bound as a UDF."""

from __future__ import annotations

import json
import os
from typing import Any

import duckdb

from normalize.terms import terms_hash

# ubuntu-latest has ~16 GiB RAM. DuckDB's default cap (~80%) OOMs the runner
# on large AWS offers. A 4 GiB cap was too tight — non-spillable ops hit it
# mid-query. 8 GiB + 2 threads + preserve_insertion_order=false keeps total
# footprint well under the runner ceiling while letting hash joins and
# aggregations spill to _TEMP_DIR. See M3a.4.2 runbook.
_TEMP_DIR = "/tmp/sku_duckdb"
_MEMORY_LIMIT = "8GB"
_THREADS = 2


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
    con.execute(f"SET threads={_THREADS}")
    con.execute("SET preserve_insertion_order=false")
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
