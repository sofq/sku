"""Lightweight sanity guard for the daily release chain (spec §3 line 198).

Runs per-shard in the `diff-package` job against today's freshly-built SQLite
shard vs. yesterday's decompressed baseline. Catches the common failure modes
before we publish — row-count cliff from a broken fetcher, upstream schema
drift, orphaned foreign keys, or a forgotten `generated_at` in metadata. Every
miss here would poison the client caches until the next manual intervention.

Does *not* attempt a full upstream cross-check — that is `data-validate.yml`'s
job (m3a.4.3).
"""

from __future__ import annotations

import argparse
import sqlite3
import sys
from pathlib import Path


class SanityError(Exception):
    """Raised when a sanity check fails. Carries the shard name in `shard`."""

    def __init__(self, shard: str, reason: str, detail: str = "") -> None:
        super().__init__(f"[{shard}] {reason}: {detail}" if detail else f"[{shard}] {reason}")
        self.shard = shard
        self.reason = reason
        self.detail = detail


def _count_rows(con: sqlite3.Connection, table: str) -> int:
    return con.execute(f"SELECT COUNT(*) FROM {table}").fetchone()[0]


def _compute_vm_families(con: sqlite3.Connection) -> set[str]:
    """Return the set of compute.vm family tokens (prefix before first '-' or '.')."""
    rows = con.execute(
        "SELECT DISTINCT resource_name FROM skus WHERE kind = 'compute.vm'"
    ).fetchall()
    families: set[str] = set()
    for (name,) in rows:
        sep = len(name)
        if "-" in name:
            sep = min(sep, name.index("-"))
        if "." in name:
            sep = min(sep, name.index("."))
        families.add(name[:sep] if sep < len(name) else name)
    return families


def _schema_ddl(con: sqlite3.Connection) -> dict[str, str]:
    rows = con.execute(
        "SELECT name, sql FROM sqlite_master "
        "WHERE type='table' AND name NOT LIKE 'sqlite_%' "
        "ORDER BY name"
    ).fetchall()
    return {name: (sql or "").strip() for name, sql in rows}


def _metadata_value(con: sqlite3.Connection, key: str) -> str | None:
    row = con.execute("SELECT value FROM metadata WHERE key = ?", (key,)).fetchone()
    return row[0] if row else None


def sanity_check(
    *,
    shard: str,
    shard_db: Path,
    previous_db: Path | None,
    tolerance_pct: float = 5.0,
) -> None:
    """Raise :class:`SanityError` when any guard trips.

    When `previous_db` is `None` (first release) only the intra-shard checks
    run: schema validity, FK integrity, `metadata.generated_at` populated.
    """
    shard_db = Path(shard_db)
    con = sqlite3.connect(f"file:{shard_db}?mode=ro", uri=True)
    try:
        con.execute("PRAGMA foreign_keys = ON")

        generated_at = _metadata_value(con, "generated_at")
        if not generated_at:
            raise SanityError(shard, "missing_generated_at", "metadata.generated_at is NULL/empty")

        # FK integrity: every price row must reference a live sku row.
        orphan_prices = con.execute(
            "SELECT COUNT(*) FROM prices p "
            "LEFT JOIN skus s ON s.sku_id = p.sku_id "
            "WHERE s.sku_id IS NULL"
        ).fetchone()[0]
        if orphan_prices:
            raise SanityError(
                shard, "fk_orphan_prices", f"{orphan_prices} price rows without matching sku"
            )

        curr_skus = _count_rows(con, "skus")
        curr_schema = _schema_ddl(con)

        if previous_db is None:
            # Fresh release — nothing to compare against.
            if curr_skus == 0:
                raise SanityError(shard, "empty_shard", "skus table is empty on first release")
            return

        prev_path = Path(previous_db)
        prev = sqlite3.connect(f"file:{prev_path}?mode=ro", uri=True)
        try:
            prev_skus = _count_rows(prev, "skus")
            prev_schema = _schema_ddl(prev)

            if curr_schema != prev_schema:
                added = sorted(curr_schema.keys() - prev_schema.keys())
                removed = sorted(prev_schema.keys() - curr_schema.keys())
                changed = sorted(
                    k
                    for k in curr_schema.keys() & prev_schema.keys()
                    if curr_schema[k] != prev_schema[k]
                )
                raise SanityError(
                    shard,
                    "schema_drift",
                    f"added={added} removed={removed} changed={changed}",
                )

            if prev_skus > 0:
                drift_pct = 100.0 * abs(curr_skus - prev_skus) / prev_skus
                if drift_pct > tolerance_pct:
                    raise SanityError(
                        shard,
                        "row_count_drift",
                        f"prev={prev_skus} curr={curr_skus} drift={drift_pct:.2f}% "
                        f"exceeds tolerance {tolerance_pct:.2f}%",
                    )

            dropped_families = _compute_vm_families(prev) - _compute_vm_families(con)
            if dropped_families:
                raise SanityError(
                    shard,
                    "family_coverage_drop",
                    f"compute.vm families with zero rows: {sorted(dropped_families)}",
                )
        finally:
            prev.close()
    finally:
        con.close()


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(prog="package.sanity_check")
    ap.add_argument("--shard", required=True)
    ap.add_argument("--shard-db", type=Path, required=True)
    ap.add_argument("--previous-db", type=Path, default=None)
    ap.add_argument("--tolerance-pct", type=float, default=5.0)
    args = ap.parse_args(argv)

    try:
        sanity_check(
            shard=args.shard,
            shard_db=args.shard_db,
            previous_db=args.previous_db,
            tolerance_pct=args.tolerance_pct,
        )
    except SanityError as exc:
        print(f"sanity check failed: {exc}", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
