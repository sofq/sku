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
    exit: str  # "pass" | "fail"

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
        from validate.gcp import revalidate, _SHARD_SERVICE_IDS, _DEFAULT_GCE_SERVICE_ID
        sid = _SHARD_SERVICE_IDS.get(shard, _DEFAULT_GCE_SERVICE_ID)
        return lambda samples, **kw: revalidate(samples, service_id=sid, **kw)
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
    if revalidator is None:
        revalidator = _default_revalidator(shard)

    # --- Sample ---
    samples = sample(shard_db, budget=budget, seed=seed)
    logger.info("Sampled %d rows from %s", len(samples), shard)

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
