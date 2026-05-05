"""Validation driver — orchestrates sampling, revalidation, and report writing.

CLI usage::

    python -m validate \\
      --shard aws-ec2 \\
      --shard-db dist/pipeline/aws-ec2.db \\
      --budget 20 \\
      --report out/aws-ec2.report.json \\
      [--vantage-json path/to/instances.json]

Exit 0 on pass, 1 on fail (any drift record or vantage drift record).
"""

from __future__ import annotations

import dataclasses
import json
import logging
from datetime import UTC, datetime
from pathlib import Path
from typing import Any, Protocol

from validate.sampler import Sample, sample

logger = logging.getLogger(__name__)


# Shards where the upstream API can't be compared one-to-one against catalog
# rows: catalog stores synthesized values (e.g. gcp-gce machine totals built
# from per-vCPU + per-GiB component prices) while the upstream API exposes the
# components. Listing here makes the validator skip revalidation and emit a
# pass with the reason recorded in the report.
SKIP_REVALIDATION: dict[str, str] = {
    "gcp-gce": (
        "ingest synthesizes machine totals from per-vCPU and per-GiB "
        "component SKUs (see pipeline/ingest/gcp_gce.py); validator "
        "compares against the raw component unitPrice, producing "
        "false-positive drift. Re-enable once a sidecar or component-aware "
        "comparison lands."
    ),
    # Azure DB shards: validator filters upstream by `meterName eq '{resource_name}'`
    # (pipeline/validate/azure.py:_filter_for_sample), but the Retail API exposes
    # `meterName="vCore"` and disambiguates via `productName` + `skuName`. Our
    # resource_name (e.g. "Gen5 2 vCore", "GP_Gen5_2") never matches the upstream
    # meterName, so 14-19 of 20 sampled SKUs are reported `missing_upstream` and the
    # 1-5 that incidentally match return unrelated line items. Re-enable once the
    # azure validator gains per-shard product/sku-name matching.
    "azure-mariadb": (
        "validator's meterName-based filter doesn't match this shard's "
        "resource_name format (Retail API uses meterName='vCore' with "
        "productName disambiguation). See pipeline/validate/azure.py."
    ),
    "azure-sql": (
        "validator's meterName-based filter doesn't match this shard's "
        "resource_name format. See pipeline/validate/azure.py."
    ),
    "azure-postgres": (
        "validator's meterName-based filter doesn't match this shard's "
        "resource_name format. See pipeline/validate/azure.py."
    ),
}


# Shards whose ingest fans-in additional price dimensions onto a primary SKU's
# row (e.g. gcp-gcs storage SKU also carries fanned-in global ops prices). The
# upstream API only knows about the primary dimension, so the validator can
# only meaningfully compare that one. Other dimensions are sampled out before
# revalidation. Fan-in dimensions remain unvalidated until the validator can
# look them up against their actual source SKUs (tracked separately).
PRIMARY_DIMENSIONS: dict[str, frozenset[str]] = {
    "gcp-gcs": frozenset({"storage"}),       # ops fanned-in from global SKUs
    "gcp-run": frozenset({"cpu-second"}),    # memory + requests fanned-in
    "gcp-functions": frozenset({"cpu-second"}),  # memory + requests fanned-in
}


# ---------------------------------------------------------------------------
# Types
# ---------------------------------------------------------------------------


class RevalidateFunc(Protocol):
    """Callable signature for per-provider revalidators."""

    def __call__(
        self,
        samples: list[Sample],
        **kwargs: Any,
    ) -> tuple[list[Any], list[str]]:
        ...


@dataclasses.dataclass
class ValidationReport:
    """Structured validation report."""

    shard: str
    generated_at: str
    sample_size: int
    drift_records: list[dict]
    missing_upstream: list[str]
    vantage_drift: list[dict]
    exit: str  # "pass" | "fail" | "skipped"
    skip_reason: str | None = None

    def as_dict(self) -> dict:
        return dataclasses.asdict(self)


# ---------------------------------------------------------------------------
# Per-shard revalidator dispatch
# ---------------------------------------------------------------------------


def _default_revalidator(shard: str) -> RevalidateFunc:
    """Return the appropriate revalidator based on the shard prefix."""
    if shard.startswith("aws-"):
        from validate.aws import revalidate
        return revalidate
    if shard.startswith("azure-"):
        from validate.azure import revalidate
        return revalidate
    if shard.startswith("gcp-"):
        from validate.gcp import revalidate, service_ids_for_shard, _DEFAULT_GCE_SERVICE_ID
        sids = service_ids_for_shard(shard) or (_DEFAULT_GCE_SERVICE_ID,)
        return lambda samples, **kw: revalidate(samples, service_id=sids, **kw)
    if shard.startswith("openrouter"):
        from validate.openrouter import revalidate
        return revalidate
    # Fallback: no-op
    return lambda samples, **_: ([], [])


# ---------------------------------------------------------------------------
# Core orchestration
# ---------------------------------------------------------------------------


def run_validation(
    shard: str,
    shard_db: Path,
    budget: int,
    report: Path,
    *,
    revalidator: RevalidateFunc | None = None,
    vantage_drift: list | None = None,
    seed: int | None = None,
) -> int:
    """Run the full validation pipeline for one shard.

    Parameters
    ----------
    shard:
        Shard identifier (e.g. ``"aws-ec2"``).
    shard_db:
        Path to the SQLite shard file.
    budget:
        Number of samples to draw.
    report:
        Path to write the JSON report.
    revalidator:
        Injected revalidator callable for testing. ``None`` uses the
        default per-prefix dispatch.
    vantage_drift:
        Pre-computed vantage drift records (for aws-ec2 only). ``None``
        means no vantage check was done.
    seed:
        Random seed forwarded to the sampler.

    Returns
    -------
    int
        0 on pass, 1 on fail.
    """
    if shard in SKIP_REVALIDATION:
        reason = SKIP_REVALIDATION[shard]
        logger.info("Skipping revalidation for %s: %s", shard, reason)
        report_data = ValidationReport(
            shard=shard,
            generated_at=datetime.now(UTC).isoformat(),
            sample_size=0,
            drift_records=[],
            missing_upstream=[],
            vantage_drift=[],
            exit="skipped",
            skip_reason=reason,
        )
        report.parent.mkdir(parents=True, exist_ok=True)
        report.write_text(json.dumps(report_data.as_dict(), indent=2))
        return 0

    if revalidator is None:
        revalidator = _default_revalidator(shard)

    # --- Sample ---
    samples = sample(shard_db, budget=budget, seed=seed)
    logger.info("Sampled %d rows from %s", len(samples), shard)

    # --- Filter to primary dimensions for fan-in shards ---
    if shard in PRIMARY_DIMENSIONS:
        allowed = PRIMARY_DIMENSIONS[shard]
        before = len(samples)
        samples = [s for s in samples if s.dimension in allowed]
        logger.info(
            "Filtered %s samples to primary dimensions %s: %d → %d",
            shard, sorted(allowed), before, len(samples),
        )

    # --- Revalidate ---
    drift_objs, missing = revalidator(samples)

    # --- Serialise drift records ---
    drift_records: list[dict] = []
    for rec in drift_objs:
        if dataclasses.is_dataclass(rec):
            drift_records.append(dataclasses.asdict(rec))
        else:
            drift_records.append(dict(rec))

    # --- Vantage ---
    vantage_drift_dicts: list[dict] = []
    if vantage_drift:
        for rec in vantage_drift:
            if dataclasses.is_dataclass(rec):
                vantage_drift_dicts.append(dataclasses.asdict(rec))
            else:
                vantage_drift_dicts.append(dict(rec))

    # --- Determine pass/fail ---
    has_drift = bool(drift_records) or bool(vantage_drift_dicts)
    exit_status = "fail" if has_drift else "pass"

    # --- Write report ---
    report_data = ValidationReport(
        shard=shard,
        generated_at=datetime.now(UTC).isoformat(),
        sample_size=len(samples),
        drift_records=drift_records,
        missing_upstream=list(missing),
        vantage_drift=vantage_drift_dicts,
        exit=exit_status,
    )
    report.parent.mkdir(parents=True, exist_ok=True)
    report.write_text(json.dumps(report_data.as_dict(), indent=2))
    logger.info("Report written to %s (exit=%s)", report, exit_status)

    return 0 if exit_status == "pass" else 1
