"""`python -m pipeline.discover` entrypoint."""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

from discover.driver import run


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(prog="discover")
    ap.add_argument("--state", type=Path, required=True, help="path to state.json")
    ap.add_argument("--out", type=Path, required=True, help="path for changed-shards.json")
    ap.add_argument("--live", action="store_true", help="hit real upstreams (default: dry-run)")
    ap.add_argument("--baseline-rebuild", action="store_true", help="force every shard into output")
    ap.add_argument("--shards", default=None, help="comma-separated shard ids; default = all known")
    ap.add_argument(
        "--gcp-api-key-env",
        default="GCP_BILLING_API_KEY",
        help="env var holding the GCP billing API key (default: GCP_BILLING_API_KEY)",
    )
    args = ap.parse_args(argv)

    shards = None
    if args.shards:
        shards = [s.strip() for s in args.shards.split(",") if s.strip()]
    api_key = os.environ.get(args.gcp_api_key_env) if args.live else None

    return run(
        state_path=args.state,
        out_path=args.out,
        live=args.live,
        baseline_rebuild=args.baseline_rebuild,
        shards=shards,
        gcp_api_key=api_key,
    )


if __name__ == "__main__":
    sys.exit(main())
