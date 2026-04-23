"""Normalize Azure retail-prices for 'Azure Database for PostgreSQL' into sku rows.

Separate shard (not azure_sql) because upstream uses a distinct serviceName.
One row per (sku_id, region) with a single compute dimension. Deployment
option from productName via the shared classifier: Flexible Server ->
'flexible-server', legacy Single Server -> 'single-az'.
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
        service_name="Azure Database for PostgreSQL",
        tenancy_slug="azure-postgres",
        service="postgres",
    )


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.azure_postgres")
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
    n = 0
    with args.out.open("w") as fh:
        for row in ingest(prices_path=prices_path):
            fh.write(dumps(row) + "\n")
            n += 1
    print(f"ingest.azure_postgres: wrote {n} rows", file=sys.stderr)
    return 0 if n > 0 else 2


if __name__ == "__main__":
    sys.exit(main())
