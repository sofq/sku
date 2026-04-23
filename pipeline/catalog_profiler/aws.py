"""Scan AWS offer JSON files and emit CoverageRow per productFamily.

Uses ijson so the 400 MB EC2 offer stays streamable — we never hold the
full products dict in memory. Attribute-key fingerprinting uses a sample
of the first 50 SKUs per family (enough to capture schema; full scan
would be wasteful).
"""

from __future__ import annotations

from collections.abc import Iterable
from pathlib import Path

import ijson

_SHARD_BY_SERVICE_FAMILY: dict[tuple[str, str], str] = {
    ("AmazonEC2", "Compute Instance"):   "aws_ec2",
    ("AmazonEC2", "Storage"):            "aws_ebs",
    ("AmazonRDS", "Database Instance"):  "aws_rds",
    ("AmazonS3",  "Storage"):            "aws_s3",
    ("AmazonS3",  "API Request"):        "aws_s3",
    ("AWSLambda", "Serverless"):         "aws_lambda",
    ("AmazonDynamoDB", "Amazon DynamoDB PayPerRequest Throughput"): "aws_dynamodb",
    ("AmazonCloudFront", "Data Transfer"): "aws_cloudfront",
}


def _emit_per_family(*, service_code: str, offer_path: Path) -> dict[str, dict]:
    """Stream-parse one offer and return `{family: {count, attrs, samples}}`."""
    acc: dict[str, dict] = {}
    with offer_path.open("rb") as fh:
        for sku_id, prod in ijson.kvitems(fh, "products"):
            family = prod.get("productFamily") or "(no productFamily)"
            bucket = acc.setdefault(family, {"count": 0, "attrs": set(), "samples": []})
            bucket["count"] += 1
            if len(bucket["samples"]) < 3:
                bucket["samples"].append(sku_id)
            if bucket["count"] <= 50:
                for k in (prod.get("attributes") or {}):
                    bucket["attrs"].add(k)
    return acc


def scan_offer(*, service_code: str, offer_paths: Iterable[Path]) -> list:
    from .types import CoverageRow

    merged: dict[str, dict] = {}
    for p in offer_paths:
        per_family = _emit_per_family(service_code=service_code, offer_path=p)
        for fam, bucket in per_family.items():
            m = merged.setdefault(fam, {"count": 0, "attrs": set(), "samples": []})
            m["count"] += bucket["count"]
            m["attrs"].update(bucket["attrs"])
            for s in bucket["samples"]:
                if len(m["samples"]) < 3 and s not in m["samples"]:
                    m["samples"].append(s)

    rows = []
    for family, bucket in merged.items():
        shard = _SHARD_BY_SERVICE_FAMILY.get((service_code, family))
        rows.append(CoverageRow(
            bucket_key=f"{service_code}/{family}",
            bucket_label=family,
            sku_count=bucket["count"],
            attribute_keys=tuple(sorted(bucket["attrs"])),
            sample_sku_ids=tuple(bucket["samples"]),
            covered_by_shard=shard,
            coverage_ratio=1.0 if shard else 0.0,
        ))
    return rows
