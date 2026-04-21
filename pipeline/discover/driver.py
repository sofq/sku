"""Discover driver — emits changed-shards.json by diffing upstream version
indicators against a saved state file.

Output schema (`changed.json`):

    {
      "schema_version": 1,
      "baseline_rebuild": false,
      "generated_at": "2026-04-19T03:00:00Z",
      "shards": ["aws_ec2", "gcp_gce"],
      "unchanged": ["aws_rds", ...],
      "errors": [{"shard": "azure_vm", "reason": "http_500", "detail": "..."}]
    }

Exit codes:
  0  — success (at least one shard resolved, or dry-run mode).
  2  — pipeline_failure: every requested shard errored during live discovery.
  4  — validation: unknown shard id passed via --shards.
"""

from __future__ import annotations

import json
import os
import time
from collections.abc import Iterable, Mapping
from pathlib import Path

import requests

from discover import aws as aws_disc
from discover import azure as azure_disc
from discover import gcp as gcp_disc
from discover import openrouter as or_disc
from discover.state import State, load, save
from ingest.http import LiveClient

_OUTPUT_SCHEMA_VERSION = 1

ALL_SHARDS: tuple[str, ...] = (
    "aws_ec2",
    "aws_rds",
    "aws_s3",
    "aws_lambda",
    "aws_ebs",
    "aws_dynamodb",
    "aws_cloudfront",
    "azure_vm",
    "azure_sql",
    "azure_blob",
    "azure_functions",
    "azure_disks",
    "gcp_gce",
    "gcp_cloud_sql",
    "gcp_gcs",
    "gcp_run",
    "gcp_functions",
    "openrouter",
)


def _provider_of(shard: str) -> str:
    if shard.startswith("aws_"):
        return "aws"
    if shard.startswith("azure_"):
        return "azure"
    if shard.startswith("gcp_"):
        return "gcp"
    if shard == "openrouter":
        return "openrouter"
    raise KeyError(shard)


def _validate_shards(shards: Iterable[str]) -> list[str]:
    out: list[str] = []
    for s in shards:
        # Accept dashed aliases from CLI users; normalise to underscored.
        canon = s.replace("-", "_")
        if canon not in ALL_SHARDS:
            raise ValueError(canon)
        out.append(canon)
    return out


def _now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def _write_output(out_path: Path, doc: Mapping[str, object]) -> None:
    out_path.parent.mkdir(parents=True, exist_ok=True)
    tmp = out_path.with_suffix(out_path.suffix + ".part")
    with tmp.open("w") as fh:
        json.dump(doc, fh, indent=2, sort_keys=True)
        fh.write("\n")
    os.replace(tmp, out_path)


def _run_live(
    shards: list[str],
    *,
    session: requests.Session | None,
    gcp_session: requests.Session | None,
) -> tuple[dict[str, str], list[dict[str, str]]]:
    """Fetch indicators for every shard. Returns (indicators, errors).

    Shard-level errors are recorded and do not abort the run; the driver
    decides whether the overall run is a failure based on how many shards
    succeeded.
    """
    by_provider: dict[str, list[str]] = {"aws": [], "azure": [], "gcp": [], "openrouter": []}
    for s in shards:
        by_provider[_provider_of(s)].append(s)

    indicators: dict[str, str] = {}
    errors: list[dict[str, str]] = []

    def _run_group(name: str, fn) -> None:
        group = by_provider[name]
        if not group:
            return
        try:
            indicators.update(fn(group))
        except Exception as exc:
            # Parent call failed: attribute the failure to each shard in the
            # group so downstream can decide per-shard retries.
            for s in group:
                errors.append({"shard": s, "reason": name + "_error", "detail": str(exc)})

    _run_group("aws", lambda g: aws_disc.discover(g, session=session))
    _run_group("azure", lambda g: azure_disc.discover(g, session=session))
    if by_provider["gcp"]:
        if gcp_session is None:
            # Production path: build an ADC-authenticated session. Tests pass
            # gcp_session explicitly so google.auth isn't invoked.
            try:
                from ingest.gcp_common import build_authenticated_session

                gcp_session = build_authenticated_session()
            except Exception as exc:
                for s in by_provider["gcp"]:
                    errors.append({"shard": s, "reason": "gcp_auth_error", "detail": str(exc)})
                gcp_session = None
        if gcp_session is not None:
            _run_group("gcp", lambda g: gcp_disc.discover(g, session=gcp_session))
    _run_group("openrouter", lambda g: or_disc.discover(g, client=LiveClient()))
    return indicators, errors


def run(
    *,
    state_path: Path,
    out_path: Path,
    live: bool,
    baseline_rebuild: bool = False,
    shards: Iterable[str] | None = None,
    session: requests.Session | None = None,
    gcp_session: requests.Session | None = None,
) -> int:
    """Execute one discovery pass; write `out_path`; return exit code."""
    try:
        requested = _validate_shards(shards) if shards is not None else list(ALL_SHARDS)
    except ValueError as exc:
        _write_output(
            out_path,
            {
                "schema_version": _OUTPUT_SCHEMA_VERSION,
                "baseline_rebuild": False,
                "generated_at": _now_iso(),
                "shards": [],
                "unchanged": [],
                "errors": [{"shard": str(exc), "reason": "unknown_shard", "detail": ""}],
            },
        )
        return 4

    previous = load(state_path)
    prev_map = dict(previous.indicators)

    if not live:
        # Dry-run mode: never hit the network. If state is empty, signal
        # baseline_rebuild so the caller knows indicators are unpopulated.
        doc = {
            "schema_version": _OUTPUT_SCHEMA_VERSION,
            "baseline_rebuild": not prev_map,
            "generated_at": _now_iso(),
            "shards": [],
            "unchanged": sorted(s for s in requested if s in prev_map),
            "errors": [],
        }
        _write_output(out_path, doc)
        return 0

    indicators, errors = _run_live(requested, session=session, gcp_session=gcp_session)

    errored_shards = {e["shard"] for e in errors}
    resolved = [s for s in requested if s in indicators]

    if baseline_rebuild or not prev_map:
        changed = [s for s in resolved]
    else:
        changed = [s for s in resolved if indicators[s] != prev_map.get(s)]

    unchanged = [s for s in resolved if s not in changed]

    # Persist the updated indicators (merge — don't wipe out shards we didn't
    # check this run).
    merged = dict(prev_map)
    merged.update(indicators)
    save(state_path, State(schema_version=previous.schema_version, indicators=merged))

    doc = {
        "schema_version": _OUTPUT_SCHEMA_VERSION,
        "baseline_rebuild": bool(baseline_rebuild or not prev_map),
        "generated_at": _now_iso(),
        "shards": sorted(changed),
        "unchanged": sorted(unchanged),
        "errors": errors,
    }
    _write_output(out_path, doc)

    if requested and not resolved and errored_shards == set(requested):
        return 2
    return 0
