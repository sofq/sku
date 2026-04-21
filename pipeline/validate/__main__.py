"""Entry point for ``python -m validate``.

Usage::

    python -m validate \\
      --shard aws-ec2 \\
      --shard-db dist/pipeline/aws-ec2.db \\
      --budget 20 \\
      --report out/aws-ec2.report.json \\
      [--vantage-json path/to/instances.json]
"""

from __future__ import annotations

import argparse
import logging
import sys
from pathlib import Path

from validate.driver import run_validation


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        prog="python -m validate",
        description="Validate a sku shard against upstream providers.",
    )
    parser.add_argument("--shard", required=True, help="Shard identifier, e.g. aws-ec2")
    parser.add_argument("--shard-db", required=True, type=Path, help="Path to the SQLite shard")
    parser.add_argument("--budget", type=int, default=20, help="Sampling budget (default: 20)")
    parser.add_argument("--report", required=True, type=Path, help="Path for the JSON report")
    parser.add_argument(
        "--vantage-json",
        type=Path,
        default=None,
        help="Path to vantage instances.json (required for aws-ec2 shard)",
    )
    parser.add_argument("--seed", type=int, default=None, help="Random seed for sampler")
    parser.add_argument("--verbose", action="store_true", help="Enable DEBUG logging")
    args = parser.parse_args(argv)

    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(levelname)s %(name)s: %(message)s",
    )

    vantage_drift = None
    if args.vantage_json is not None:
        from validate.vantage import cross_check
        vantage_drift = cross_check(args.shard_db, instances_json=args.vantage_json)

    return run_validation(
        shard=args.shard,
        shard_db=args.shard_db,
        budget=args.budget,
        report=args.report,
        vantage_drift=vantage_drift,
        seed=args.seed,
    )


if __name__ == "__main__":
    sys.exit(main())
