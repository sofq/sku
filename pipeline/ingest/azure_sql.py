"""Normalize Azure retail-prices JSON for SQL Database into sku row dicts.

Spec §5 db.relational kind. Engine slot is set to 'azure-sql' (single value
distinguishes from AWS RDS rows in cross-provider compare). Deployment
option rides the terms.os slot — Single Database -> 'single-az',
Managed Instance -> 'managed-instance', Elastic Pool -> 'elastic-pool'.
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Iterable
from pathlib import Path
from typing import Any

from ._duckdb import dumps
from .azure_db_common import ingest_hosted_db


def ingest(*, prices_path: Path) -> Iterable[dict[str, Any]]:
    yield from ingest_hosted_db(
        prices_path=prices_path,
        service_name="SQL Database",
        tenancy_slug="azure-sql",
        service="sql",
    )


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.azure_sql")
    ap.add_argument("--fixture", type=Path)
    ap.add_argument("--prices", type=Path)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--catalog-version", default=None)
    args = ap.parse_args()
    if args.fixture:
        prices_path = args.fixture / "prices.json" if args.fixture.is_dir() else args.fixture
    elif args.prices:
        prices_path = args.prices
    else:
        print("either --fixture or --prices required", file=sys.stderr)
        return 2
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with args.out.open("w") as fh:
        for row in ingest(prices_path=prices_path):
            fh.write(dumps(row) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
