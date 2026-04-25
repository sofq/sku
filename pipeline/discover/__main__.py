"""`python -m pipeline.discover` entrypoint.

GCP auth: when `--live` hits GCP shards, credentials come from Google
Application Default Credentials (ADC). Under GitHub Actions this is the
OIDC-federated service-account token injected by
`google-github-actions/auth`; locally, any gcloud login or
`GOOGLE_APPLICATION_CREDENTIALS` path works.
"""

from __future__ import annotations

import argparse
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
    args = ap.parse_args(argv)

    shards = None
    if args.shards:
        shards = [s.strip() for s in args.shards.split(",") if s.strip()]

    return run(
        state_path=args.state,
        out_path=args.out,
        live=args.live,
        baseline_rebuild=args.baseline_rebuild,
        shards=shards,
    )


if __name__ == "__main__":
    sys.exit(main())
