"""Shared AWS ingest helpers: region normalization, SKU ID passthrough."""

from __future__ import annotations

import contextlib
import json
import os
import time
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path

import requests
import yaml

_REGIONS_YAML = Path(__file__).resolve().parent.parent / "normalize" / "regions.yaml"


@dataclass(frozen=True)
class RegionNormalizer:
    """Maps (provider, provider-region) -> canonical group."""

    table: Mapping[tuple[str, str], str]

    def normalize(self, provider: str, region: str) -> str:
        key = (provider, region)
        try:
            return self.table[key]
        except KeyError as exc:
            raise KeyError(f"{provider}/{region}") from exc


def load_region_normalizer() -> RegionNormalizer:
    """Load the repo's regions.yaml and build a (provider, region) -> group map."""
    with _REGIONS_YAML.open() as fh:
        doc = yaml.safe_load(fh)
    table: dict[tuple[str, str], str] = {}
    for group, entries in (doc.get("groups") or {}).items():
        for entry in entries:
            key = (entry["provider"], entry["region"])
            table[key] = group
    return RegionNormalizer(table)


_AWS_OFFER_BASE = "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws"
_AWS_SERVICE_CODES: dict[str, str] = {
    "aws_ec2": "AmazonEC2",
    "aws_rds": "AmazonRDS",
    "aws_s3": "AmazonS3",
    "aws_lambda": "AWSLambda",
    "aws_ebs": "AmazonEC2",
    "aws_dynamodb": "AmazonDynamoDB",
    "aws_cloudfront": "AmazonCloudFront",
}

_RETRY_STATUSES = {500, 502, 503, 504}


def fetch_offer(
    shard: str, target: Path, *, session: requests.Session | None = None, retries: int = 3
) -> str:
    """Download an AWS offer index.json for `shard` into `target`."""
    service_code = _AWS_SERVICE_CODES[shard]
    url = f"{_AWS_OFFER_BASE}/{service_code}/current/index.json"
    if session is None:
        session = requests.Session()
    ua = "sku-pipeline/0.0 (+https://github.com/sofq/sku)"
    part = target.with_suffix(target.suffix + ".part")
    last: Exception | None = None
    for attempt in range(retries):
        try:
            resp = session.get(
                url,
                stream=True,
                timeout=60,
                headers={"User-Agent": ua},
            )
        except requests.RequestException as exc:
            last = exc
            time.sleep(0.5 * 2**attempt)
            continue
        status = resp.status_code
        if status in _RETRY_STATUSES:
            last = RuntimeError(f"GET {url} returned {status}")
            time.sleep(0.5 * 2**attempt)
            continue
        if status != 200:
            raise RuntimeError(f"GET {url} returned {status}")
        declared_length: int | None = None
        raw_cl = resp.headers.get("Content-Length")
        if raw_cl is not None:
            with contextlib.suppress(ValueError):
                declared_length = int(raw_cl)
        try:
            with part.open("wb") as fh:
                for chunk in resp.iter_content(chunk_size=65536):
                    fh.write(chunk)
        except Exception as exc:
            part.unlink(missing_ok=True)
            raise RuntimeError(f"GET {url} stream error: {exc}") from exc
        actual_length = part.stat().st_size
        if declared_length is not None and actual_length != declared_length:
            part.unlink(missing_ok=True)
            raise RuntimeError(
                f"GET {url} truncated: got {actual_length} bytes, expected {declared_length}"
            )
        with part.open() as fh:
            doc = json.load(fh)
        os.replace(part, target)
        return doc["publicationDate"]
    raise RuntimeError(f"GET {url} failed after {retries} attempts: {last}")
