"""Shared GCP ingest helpers: region normalization, Cloud-Billing price decoding.

GCP's Cloud Billing Catalog API expresses prices as `{units, nanos}` pairs —
`units` is integer dollars, `nanos` is the sub-dollar remainder in billionths.
Usage units are ratios like `h`, `GiBy.h`, `GiBy.mo`; we canonicalize to the
`hrs` / `gb-hr` / `gb-mo` strings already used by the other ingesters.
"""

from __future__ import annotations

from .aws_common import load_region_normalizer  # re-export — same shared YAML loader

__all__ = [
    "load_region_normalizer",
    "parse_unit_price",
    "parse_usage_unit",
]

_USAGE_UNITS: dict[str, tuple[float, str]] = {
    "h":        (1.0, "hrs"),
    "GiBy.h":   (1.0, "gb-hr"),
    "GiBy.mo":  (1.0, "gb-mo"),
    "By.mo":    (1.0 / (1024 ** 3), "gb-mo"),   # byte-month → gb-month
    "count":    (1.0, "requests"),
    "s":        (1.0, "s"),                     # serverless CPU/request duration
    "GiBy.s":   (1.0, "gb-s"),                  # serverless memory allocation time
}


def parse_unit_price(*, units: str, nanos: int) -> float:
    """Decode a Cloud Billing `{units, nanos}` pair into a float dollar amount."""
    return int(units) + (nanos / 1_000_000_000)


def parse_usage_unit(unit: str) -> tuple[float, str]:
    """Return (divisor, canonical_unit) for a Cloud Billing `usageUnit` string."""
    try:
        return _USAGE_UNITS[unit]
    except KeyError as exc:
        raise ValueError(f"unsupported usageUnit: {unit}") from exc
