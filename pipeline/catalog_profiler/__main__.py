# pipeline/catalog_profiler/__main__.py
"""`python -m catalog_profiler aws|azure|gcp ...` — emit docs/coverage/<cloud>.md from raw feeds."""

from __future__ import annotations

import argparse
import datetime as _dt
import sys
from pathlib import Path


def _cmd_aws(args: argparse.Namespace) -> int:
    from .aws import scan_offer
    from .report import render_markdown

    all_rows = []
    for service_code, offer_glob in (
        ("AmazonEC2",        "aws_ec2-*.json"),
        ("AmazonRDS",        "aws_rds-*.json"),
        ("AmazonS3",         "aws_s3-*.json"),
        ("AWSLambda",        "aws_lambda-*.json"),
        ("AmazonDynamoDB",   "aws_dynamodb-*.json"),
        ("AmazonCloudFront", "aws_cloudfront-*.json"),
    ):
        paths = sorted(args.offer_dir.glob(offer_glob))
        if not paths:
            print(f"warn: no files for {service_code} in {args.offer_dir}", file=sys.stderr)
            continue
        all_rows.extend(scan_offer(service_code=service_code, offer_paths=paths))

    md = render_markdown(cloud="aws", rows=all_rows, as_of=args.as_of)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(md + "\n")
    return 0


def _cmd_azure(args: argparse.Namespace) -> int:
    from .azure import scan_prices
    from .report import render_markdown

    rows = scan_prices(prices_path=args.prices)
    md = render_markdown(cloud="azure", rows=rows, as_of=args.as_of)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(md + "\n")
    return 0


def _cmd_gcp(args: argparse.Namespace) -> int:
    from .gcp import scan_catalog
    from .report import render_markdown

    catalog_paths = [p for p in args.catalog_paths if p.is_file()]
    if not catalog_paths:
        print("error: no GCP catalog JSON files found", file=sys.stderr)
        return 2

    all_rows = []
    for skus_path in catalog_paths:
        all_rows.extend(scan_catalog(skus_path=skus_path))
    md = render_markdown(cloud="gcp", rows=all_rows, as_of=args.as_of)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(md + "\n")
    return 0


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(prog="catalog_profiler")
    ap.add_argument("--as-of", default=_dt.date.today().isoformat())
    sub = ap.add_subparsers(dest="cloud", required=True)

    aws = sub.add_parser("aws")
    aws.add_argument("--offer-dir", type=Path, required=True)
    aws.add_argument("--out", type=Path, required=True)
    aws.set_defaults(func=_cmd_aws)

    azure = sub.add_parser("azure")
    azure.add_argument("--prices", type=Path, required=True)
    azure.add_argument("--out", type=Path, required=True)
    azure.set_defaults(func=_cmd_azure)

    gcp = sub.add_parser("gcp")
    gcp.add_argument("--catalog-paths", nargs="+", type=Path, required=True)
    gcp.add_argument("--out", type=Path, required=True)
    gcp.set_defaults(func=_cmd_gcp)

    args = ap.parse_args(argv)
    return int(args.func(args) or 0)


if __name__ == "__main__":
    sys.exit(main())
