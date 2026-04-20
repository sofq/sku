"""Emit a gzipped SQL delta that transforms one shard SQLite into another.

Spec §3 (daily update flow): each data release publishes a set of delta SQL
files alongside baselines. Clients walk the delta chain between their current
baseline and `head_version`, applying each file in order.

This module is the *server-side* half of that flow. Run in the workflow's
`diff-package` job against (previous_release's shard, today's shard) — writes
one or more gzipped `.sql.gz` files that, when applied to the previous shard,
reproduce today's shard bytes (modulo the `metadata` table, which is replayed
in full every time).
"""

from __future__ import annotations

import argparse
import gzip
import sqlite3
import sys
from collections.abc import Iterable, Sequence
from pathlib import Path

# All non-metadata tables owned by the shard schema (pipeline/package/schema.sql).
# `skus` is parent; the rest FK to skus.sku_id with ON DELETE CASCADE.
_SKU_CHILD_TABLES: tuple[str, ...] = ("resource_attrs", "terms", "prices", "health")
_SKU_TABLES: tuple[str, ...] = ("skus",) + _SKU_CHILD_TABLES


def _sql_literal(value: object) -> str:
    """Render a Python scalar as a SQLite literal."""
    if value is None:
        return "NULL"
    if isinstance(value, bool):
        return "1" if value else "0"
    if isinstance(value, (int, float)):
        return repr(value)
    if isinstance(value, (bytes, bytearray)):
        return "X'" + bytes(value).hex() + "'"
    text = str(value).replace("'", "''")
    return f"'{text}'"


def _columns(con: sqlite3.Connection, table: str) -> list[str]:
    return [r[1] for r in con.execute(f"PRAGMA table_info({table})").fetchall()]


def _row_payloads(
    con: sqlite3.Connection, table: str, columns: Sequence[str]
) -> dict[tuple, tuple]:
    """Map PK tuple → full row tuple, ordered by PK columns."""
    pk_cols = [r[1] for r in con.execute(f"PRAGMA table_info({table})").fetchall() if r[5]]
    if not pk_cols:
        pk_cols = list(columns)
    col_list = ", ".join(columns)
    rows: dict[tuple, tuple] = {}
    for row in con.execute(f"SELECT {col_list} FROM {table}"):
        pk = tuple(row[columns.index(c)] for c in pk_cols)
        rows[pk] = tuple(row)
    return rows


def _insert_stmt(table: str, columns: Sequence[str], row: Sequence[object]) -> str:
    cols = ", ".join(columns)
    vals = ", ".join(_sql_literal(v) for v in row)
    return f"INSERT OR REPLACE INTO {table}({cols}) VALUES({vals});"


def _delete_by_sku_id_stmt(table: str, sku_id: str) -> str:
    return f"DELETE FROM {table} WHERE sku_id = {_sql_literal(sku_id)};"


def _delete_prices_by_pk_stmt(sku_id: str, dimension: str, tier: str) -> str:
    return (
        f"DELETE FROM prices WHERE sku_id = {_sql_literal(sku_id)}"
        f" AND dimension = {_sql_literal(dimension)}"
        f" AND tier = {_sql_literal(tier)};"
    )


def _compute_statements(prev_db: Path, new_db: Path) -> list[str]:
    """Assemble the ordered statement list."""
    prev = sqlite3.connect(f"file:{prev_db}?mode=ro", uri=True)
    curr = sqlite3.connect(f"file:{new_db}?mode=ro", uri=True)
    try:
        prev_sku_ids = {r[0] for r in prev.execute("SELECT sku_id FROM skus")}
        curr_sku_ids = {r[0] for r in curr.execute("SELECT sku_id FROM skus")}

        deleted = sorted(prev_sku_ids - curr_sku_ids)
        added = sorted(curr_sku_ids - prev_sku_ids)

        # Gather per-table column lists once.
        cols_by_table = {t: _columns(curr, t) for t in _SKU_TABLES}

        # For every changed/new sku_id, determine which rows differ per child table.
        changed_sku_ids: set[str] = set()

        # Table-level diffs we'll emit.
        # Each element is (apply_order_sort_key, sql_statement).
        stmts: list[tuple[int, str]] = []

        # 1) DELETE rows for removed skus (cascades handle child tables, but
        #    we emit explicit DELETEs per table so the SQL is explicit and
        #    independent of PRAGMA foreign_keys state on the client side).
        for sku_id in deleted:
            for table in reversed(_SKU_TABLES):  # children first, skus last
                stmts.append((0, _delete_by_sku_id_stmt(table, sku_id)))

        # 2) For skus still present, diff each child table + skus itself.
        #    When anything changed for a sku_id, mark it changed so we can
        #    rewrite the skus row (terms_hash may have bumped).
        for table in _SKU_TABLES:
            cols = cols_by_table[table]
            prev_rows = _row_payloads(prev, table, cols)
            curr_rows = _row_payloads(curr, table, cols)

            # Rows whose PK exists in both but contents differ.
            common_pks = prev_rows.keys() & curr_rows.keys()
            for pk in common_pks:
                if prev_rows[pk] != curr_rows[pk]:
                    # sku_id is always the first element of the PK in our schema.
                    changed_sku_ids.add(str(pk[0]))

            # Rows in curr but not prev: these are additions.
            new_pks = curr_rows.keys() - prev_rows.keys()
            for pk in new_pks:
                changed_sku_ids.add(str(pk[0]))

            # Rows in prev but not curr (same sku_id, but PK like
            # (sku_id, dimension, tier) removed on `prices`).
            # Only `prices` has a compound PK; for other tables this set is
            # empty when the sku_id is still present because those tables are
            # 1:1 with skus.
            orphan_pks = prev_rows.keys() - curr_rows.keys()
            for pk in orphan_pks:
                sku_id = str(pk[0])
                if sku_id in curr_sku_ids:
                    if table == "prices":
                        stmts.append(
                            (
                                1,
                                _delete_prices_by_pk_stmt(
                                    sku_id=str(pk[0]),
                                    dimension=str(pk[1]),
                                    tier=str(pk[2]),
                                ),
                            )
                        )
                    else:
                        # Should not happen for 1:1 tables unless the new shard
                        # dropped the child row entirely. Emit a by-sku delete
                        # and let the INSERT re-add it.
                        changed_sku_ids.add(sku_id)

        # 3) Add newly-introduced sku_ids unconditionally.
        for sku_id in added:
            changed_sku_ids.add(sku_id)

        # 4) For each changed sku_id, emit INSERT OR REPLACE for its rows in
        #    skus + every child table (order: skus first so FK on children
        #    resolves).
        for sku_id in sorted(changed_sku_ids):
            for table in _SKU_TABLES:
                cols = cols_by_table[table]
                rows = curr.execute(
                    f"SELECT {', '.join(cols)} FROM {table} WHERE sku_id = ?",
                    (sku_id,),
                ).fetchall()
                for row in rows:
                    stmts.append((2, _insert_stmt(table, cols, row)))

        # 5) Metadata: always rewritten in full (small table, changes every
        #    release — catalog_version, generated_at, row_count).
        meta_cols = _columns(curr, "metadata")
        stmts.append((3, "DELETE FROM metadata;"))
        meta_rows = curr.execute(
            f"SELECT {', '.join(meta_cols)} FROM metadata ORDER BY key"
        ).fetchall()
        for row in meta_rows:
            stmts.append((3, _insert_stmt("metadata", meta_cols, row)))
    finally:
        prev.close()
        curr.close()

    # Stable apply order: group 0 (deletes) → 1 (prices PK deletes) → 2 (upserts) → 3 (metadata).
    stmts.sort(key=lambda pair: pair[0])
    return [s for _, s in stmts]


def _is_noop(statements: Sequence[str]) -> bool:
    """Return True when the only statements are the metadata replay."""
    for stmt in statements:
        if not stmt.startswith("DELETE FROM metadata") and not stmt.startswith(
            "INSERT OR REPLACE INTO metadata"
        ):
            return False
    return True


def _part_path(out_sql_gz: Path, index: int, total: int) -> Path:
    """Insert `-partN` before the `.sql.gz` suffix when splitting."""
    if total <= 1:
        return out_sql_gz
    name = out_sql_gz.name
    if name.endswith(".sql.gz"):
        base = name[: -len(".sql.gz")]
        return out_sql_gz.with_name(f"{base}-part{index}.sql.gz")
    # Fallback: append before the last suffix.
    return out_sql_gz.with_name(f"{out_sql_gz.stem}-part{index}{out_sql_gz.suffix}")


def _write_gzip(path: Path, payload: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with (
        open(path, "wb") as raw,
        gzip.GzipFile(fileobj=raw, mode="wb", compresslevel=9, mtime=0) as fh,
    ):
        fh.write(payload)


def _format_payload(statements: Iterable[str], *, header: str | None = None) -> bytes:
    lines: list[str] = []
    if header:
        lines.append(f"-- {header}")
    lines.extend(statements)
    return ("\n".join(lines) + "\n").encode("utf-8")


def build_delta(
    prev_db: Path,
    new_db: Path,
    out_sql_gz: Path,
    *,
    max_bytes: int = 8 * 1024 * 1024,
) -> list[Path]:
    """Emit a gzipped SQL delta from `prev_db` → `new_db`.

    When the single gzipped artifact would exceed `max_bytes`, the delta is
    split row-by-row into apply-order `-part{N}.sql.gz` files, each ≤ roughly
    `max_bytes` gzipped. Returns the list of part files written (in apply
    order). Always returns at least one path; empty diffs produce a single
    `-- noop` file containing only the metadata replay.
    """
    prev_db = Path(prev_db)
    new_db = Path(new_db)
    out_sql_gz = Path(out_sql_gz)

    statements = _compute_statements(prev_db, new_db)

    if _is_noop(statements):
        payload = _format_payload(statements, header="noop")
        path = _part_path(out_sql_gz, 1, 1)
        _write_gzip(path, payload)
        return [path]

    # First attempt: dump all statements in a single file.
    single_payload = _format_payload(statements)
    if len(gzip.compress(single_payload)) <= max_bytes:
        path = _part_path(out_sql_gz, 1, 1)
        _write_gzip(path, single_payload)
        return [path]

    # Split: walk statements and close a part whenever gzip output would exceed
    # max_bytes. Each part is a self-contained SQL snippet (apply-order
    # preserved; the client concatenates + applies them sequentially).
    parts_payloads: list[bytes] = []
    current: list[str] = []
    for stmt in statements:
        candidate = current + [stmt]
        candidate_bytes = _format_payload(candidate)
        if current and len(gzip.compress(candidate_bytes)) > max_bytes:
            parts_payloads.append(_format_payload(current))
            current = [stmt]
        else:
            current = candidate
    if current:
        parts_payloads.append(_format_payload(current))

    total = len(parts_payloads)
    written: list[Path] = []
    for idx, payload in enumerate(parts_payloads, start=1):
        path = _part_path(out_sql_gz, idx, total)
        _write_gzip(path, payload)
        written.append(path)
    return written


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(prog="package.build_delta")
    ap.add_argument("--prev", type=Path, required=True, help="previous release shard .db")
    ap.add_argument("--new", type=Path, required=True, help="today's shard .db")
    ap.add_argument("--out", type=Path, required=True, help="output .sql.gz path")
    ap.add_argument(
        "--max-bytes",
        type=int,
        default=8 * 1024 * 1024,
        help="split into -partN.sql.gz when a single gzipped file would exceed this",
    )
    args = ap.parse_args(argv)

    paths = build_delta(args.prev, args.new, args.out, max_bytes=args.max_bytes)
    for p in paths:
        print(p)
    return 0


if __name__ == "__main__":
    sys.exit(main())
