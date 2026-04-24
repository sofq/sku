"""Shared AWS ingest helpers: region normalization, SKU ID passthrough."""

from __future__ import annotations

import concurrent.futures
import contextlib
import json
import os
import time
from collections.abc import Iterable, Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import ijson
import requests
import yaml


class NotModified(Exception):
    """Raised when an upstream HEAD/GET returns 304."""


def fetch_with_etag(
    url: str,
    *,
    etag_cache,
    session: requests.Session | None = None,
    timeout: float = 30.0,
) -> bytes:
    """HEAD with If-None-Match; on 304 raise NotModified. Else GET body and
    update the cache with the response's ETag (if any)."""
    sess = session or requests
    known = etag_cache.get(url)
    headers: dict[str, str] = {}
    if known:
        headers["If-None-Match"] = known
    head = sess.head(url, headers=headers, timeout=timeout, allow_redirects=True)
    if head.status_code == 304:
        raise NotModified(url)
    if head.status_code != 200:
        raise RuntimeError(f"unexpected HEAD status {head.status_code} for {url}")
    resp = sess.get(url, timeout=timeout)
    resp.raise_for_status()
    new_etag = resp.headers.get("ETag")
    if new_etag:
        etag_cache.set(url, new_etag)
    return resp.content

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

    def try_normalize(self, provider: str, region: str) -> str | None:
        """Return canonical group, or None when the region is not tracked.

        Live AWS/Azure/GCP price feeds include regions outside our coverage
        (new launches, opt-in regions, etc.). Ingest callers use this helper
        to skip those rows instead of failing the whole shard.
        """
        return self.table.get((provider, region))


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

# Multiple shards share an upstream offer — aws_ec2 + aws_ebs both consume
# AmazonEC2. The stripped offer (filtered to Compute Instance + Storage
# families via _EC2_PRODUCT_FAMILIES) is identical for both shards, so we
# key the on-disk filename on this shared basename instead of the shard
# name. That way a `--offer-dir` reused across `ingest.aws_ec2` and
# `ingest.aws_ebs` reads the same `aws_ec2-<region>.json` file once
# rather than downloading + stripping the 400 MB per-region offer twice.
_OFFER_BASENAME: dict[str, str] = {
    "aws_ec2": "aws_ec2",
    "aws_ebs": "aws_ec2",
}

_PARALLEL_AWS_FETCH_WORKERS = int(os.environ.get("SKU_AWS_FETCH_WORKERS", "8"))


def shared_offer_basename(shard: str) -> str:
    """Return the filename prefix used for a shard's stripped per-region offer
    files. Shards that consume the same upstream offer (aws_ec2, aws_ebs both
    read AmazonEC2) share a basename so only one stripped copy sits on disk.
    Falls back to the shard name for shards with no overlap.
    """
    return _OFFER_BASENAME.get(shard, shard)

_RETRY_STATUSES = {500, 502, 503, 504}


def fetch_offer(
    shard: str,
    target: Path,
    *,
    session: requests.Session | None = None,
    retries: int = 3,
    etag_cache=None,
) -> None:
    """Download an AWS offer index.json for `shard` into `target`.

    Streams to disk in 64 KiB chunks — we never hold the file in memory because
    some offers (AmazonEC2 = 8+ GB) would OOM a 16 GiB runner.

    When `etag_cache` is provided, sends `If-None-Match` on the HEAD before
    streaming. Raises `NotModified` if the server returns 304.
    """
    service_code = _AWS_SERVICE_CODES[shard]
    url = f"{_AWS_OFFER_BASE}/{service_code}/current/index.json"
    if etag_cache is not None:
        known = etag_cache.get(url)
        if known:
            sess = session or requests.Session()
            head = sess.head(url, headers={"If-None-Match": known},
                             timeout=30.0, allow_redirects=True)
            if head.status_code == 304:
                raise NotModified(url)
    _stream_download(url, target, session=session, retries=retries)


def _stream_download(
    url: str,
    target: Path,
    *,
    session: requests.Session | None = None,
    retries: int = 3,
) -> None:
    """Stream `url` to `target` atomically, with retry on 5xx and truncation check."""
    if session is None:
        session = requests.Session()
    ua = "sku-pipeline/0.0 (+https://github.com/sofq/sku)"
    part = target.with_suffix(target.suffix + ".part")
    last: Exception | None = None
    for attempt in range(retries):
        try:
            resp = session.get(url, stream=True, timeout=60, headers={"User-Agent": ua})
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
        os.replace(part, target)
        return
    raise RuntimeError(f"GET {url} failed after {retries} attempts: {last}")


# ---------------------------------------------------------------------------
# Per-region stripped offer fetch (m3a.4.2 OOM fix)
# ---------------------------------------------------------------------------
#
# AmazonEC2 combined offer is 8.4 GB. Loading it in any form — DuckDB json_each
# or Python json.load — overflows the 16 GB runner. AWS also publishes per-region
# index.json files (~400 MB each) via `region_index.json`. We fetch each per-region
# file, stream-parse it with ijson, and write a trimmed JSON containing only the
# ~12 leaf fields our ingesters consume. Peak memory: one product dict at a time
# (~KBs). Peak disk: ~400 MB raw + ~30 MB stripped; raw is deleted after strip.

# Which AWS service codes support per-region offers (i.e. have a region_index.json).
_PER_REGION_AWS_OFFERS: frozenset[str] = frozenset({"AmazonEC2"})

# Product-family allowlist per shard — drives the ingesters' consumed rows.
# Kept here (not per-ingester) so one stripped file can serve multiple ingesters
# (aws_ec2 + aws_ebs both read the AmazonEC2 offer, one is Compute Instance,
# the other is Storage). Extra families filtered at ingest time.
_EC2_PRODUCT_FAMILIES: frozenset[str] = frozenset({"Compute Instance", "Storage"})

# Leaf attributes any AWS ingester consumes under products.<sku>.attributes.
# Superset across aws_ec2 (Compute Instance) + aws_ebs (Storage). Keeping the
# superset means one stripped file per region works for both shards without
# re-fetching upstream.
_EC2_KEEP_ATTRS: frozenset[str] = frozenset(
    {
        "instanceType",
        "regionCode",
        "operatingSystem",
        "tenancy",
        "preInstalledSw",
        "capacitystatus",
        "vcpu",
        "memory",
        "physicalProcessor",
        "networkPerformance",
        "volumeApiName",
    }
)


def _fetch_one_region_stripped(
    shard: str,
    region: str,
    target_dir: Path,
    *,
    rel_url: str,
    session: requests.Session | None,
    retries: int,
) -> Path:
    """Fetch and strip the AWS offer JSON for a single region.

    `rel_url` is the `currentVersionUrl` path from `region_index.json` (e.g.
    ``/offers/v1.0/aws/AmazonEC2/20260418/us-east-1/index.json``).  Using the
    version-pinned URL ensures all regions in one pipeline run are drawn from
    the same published snapshot.  Downloads the per-region offer, stream-strips
    it via `_strip_ec2_offer`, deletes the raw file, and returns the stripped
    output path.
    """
    basename = shared_offer_basename(shard)
    region_url = f"https://pricing.us-east-1.amazonaws.com{rel_url}"
    raw_path = target_dir / f"{basename}-{region}.raw.json"
    stripped_path = target_dir / f"{basename}-{region}.json"
    try:
        _stream_download(region_url, raw_path, session=session, retries=retries)
        _strip_ec2_offer(raw_path, stripped_path)
    finally:
        raw_path.unlink(missing_ok=True)
    return stripped_path


def fetch_offer_regions_stripped(
    shard: str,
    out_dir: Path,
    *,
    regions: Iterable[str],
    session: requests.Session | None = None,
    retries: int = 3,
    max_workers: int = _PARALLEL_AWS_FETCH_WORKERS,
) -> list[Path]:
    """Fetch per-region AWS offer files for `shard`, stream-strip to JSON.

    For each region in `regions`, download that region's `index.json`, parse it
    with ijson, write a trimmed JSON of the same shape (products/terms.OnDemand
    with only the leaf fields we consume) to `out_dir/{shard}-{region}.json`.
    Raw per-region downloads are deleted after stripping.

    Concurrency is capped at `max_workers` (default 8, tunable via
    ``SKU_AWS_FETCH_WORKERS`` env).  This is pure I/O-bound work — 8
    concurrent streams saturate a GitHub runner's bandwidth without
    tripping AWS rate-limits on the public pricing endpoint.

    Skips regions not listed in the upstream `region_index.json` (e.g. new
    AWS regions not yet in a shard's offer). Returns the list of stripped
    output paths that were successfully produced (sorted).
    """
    service_code = _AWS_SERVICE_CODES[shard]
    if service_code not in _PER_REGION_AWS_OFFERS:
        raise ValueError(f"per-region offer not supported for shard {shard!r}")
    if session is None:
        session = requests.Session()
    out_dir.mkdir(parents=True, exist_ok=True)

    basename = shared_offer_basename(shard)

    # Pre-emptively collect any stripped files already present (e.g. a sibling
    # shard — aws_ebs when aws_ec2 ran first — produced them). We still fetch
    # the region_index to discover which regions upstream actually has, but
    # we skip redownload + restrip for regions already on disk.
    existing: dict[str, Path] = {}
    for p in out_dir.glob(f"{basename}-*.json"):
        # Strip the basename prefix and .json suffix to recover the region.
        name = p.name
        region = name[len(basename) + 1 : -len(".json")]
        if region and region != "region_index":
            existing[region] = p

    region_index_url = f"{_AWS_OFFER_BASE}/{service_code}/current/region_index.json"
    raw_index = out_dir / f"{basename}-region_index.json"
    _stream_download(region_index_url, raw_index, session=session, retries=retries)
    index_doc = json.loads(raw_index.read_text())
    raw_index.unlink(missing_ok=True)

    upstream_regions: dict[str, str] = {
        code: entry["currentVersionUrl"]
        for code, entry in (index_doc.get("regions") or {}).items()
    }

    # Regions already on disk go straight to produced; only fetch the rest.
    regions_list = list(regions)
    produced: list[Path] = []
    todo_regions: list[str] = []
    for region in regions_list:
        if upstream_regions.get(region) is None:
            continue
        stripped_path = out_dir / f"{basename}-{region}.json"
        if region in existing and stripped_path.exists():
            produced.append(stripped_path)
        else:
            todo_regions.append(region)

    with concurrent.futures.ThreadPoolExecutor(max_workers=max_workers) as pool:
        futures = {
            pool.submit(
                _fetch_one_region_stripped, shard, region, out_dir,
                rel_url=upstream_regions[region],
                session=None, retries=retries,
            ): region
            for region in todo_regions
        }
        for fut in concurrent.futures.as_completed(futures):
            region = futures[fut]
            try:
                produced.append(fut.result())
            except Exception as exc:
                pool.shutdown(wait=False, cancel_futures=True)
                raise RuntimeError(f"fetch failed for region={region}") from exc

    return sorted(produced)


def _strip_ec2_offer(raw_path: Path, out_path: Path) -> None:
    """Stream-parse an AWS EC2-shape offer JSON, emit a trimmed version.

    Keeps:
      - products.<sku>.productFamily + whitelisted attributes leaves
        (filtered to families actually consumed — Compute Instance, Storage).
      - terms.OnDemand.<sku>.<termKey>.priceDimensions.<pdKey>.{unit, pricePerUnit}
      - top-level offerCode, version, publicationDate for traceability.

    Drops: terms.Reserved (bulk of terms), product attrs we don't consume,
    offerTermCode, rateCode, effectiveDate, termAttributes, appliesTo,
    priceDimensions.description/beginRange/endRange, etc.
    """
    products_kept: dict[str, dict[str, Any]] = {}
    with raw_path.open("rb") as fh:
        for sku_id, product in ijson.kvitems(fh, "products", use_float=True):
            family = product.get("productFamily")
            if family not in _EC2_PRODUCT_FAMILIES:
                continue
            attrs_raw = product.get("attributes") or {}
            products_kept[sku_id] = {
                "productFamily": family,
                "attributes": {k: attrs_raw[k] for k in _EC2_KEEP_ATTRS if k in attrs_raw},
            }

    terms_kept: dict[str, dict[str, Any]] = {}
    with raw_path.open("rb") as fh:
        for sku_id, term_data in ijson.kvitems(fh, "terms.OnDemand", use_float=True):
            if sku_id not in products_kept:
                continue
            stripped_terms: dict[str, dict[str, Any]] = {}
            for term_key, term_obj in (term_data or {}).items():
                pds_in = (term_obj or {}).get("priceDimensions") or {}
                stripped_pds = {
                    pd_key: {
                        "unit": pd_obj.get("unit"),
                        "pricePerUnit": pd_obj.get("pricePerUnit") or {},
                    }
                    for pd_key, pd_obj in pds_in.items()
                }
                stripped_terms[term_key] = {"priceDimensions": stripped_pds}
            terms_kept[sku_id] = stripped_terms

    out_doc = {
        "products": products_kept,
        "terms": {"OnDemand": terms_kept},
    }
    part = out_path.with_suffix(out_path.suffix + ".part")
    part.write_text(json.dumps(out_doc, separators=(",", ":")))
    os.replace(part, out_path)


def aws_regions_from_yaml() -> list[str]:
    """Distinct AWS regions referenced by regions.yaml (sorted)."""
    normalizer = load_region_normalizer()
    return sorted({region for (provider, region) in normalizer.table if provider == "aws"})
