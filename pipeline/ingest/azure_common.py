"""Shared Azure ingest helpers: region normalization, unit-of-measure parsing."""

from __future__ import annotations

from .aws_common import load_region_normalizer  # re-export — same shared YAML loader

__all__ = ["load_region_normalizer", "parse_unit_of_measure"]

# m3b.1 only ingests per-hour meters. SQL Database price meters publish
# `unitOfMeasure = "1 Hour"` for vCore SKUs and `"100 Hours"` for some
# Hyperscale / DTU edge cases; we divide retailPrice by the divisor before
# emitting so prices.amount is consistently per-hour. Anything else fails
# the build so we never silently mis-price a non-time meter.
_HOUR_DIVISORS: dict[str, tuple[float, str]] = {
    "1 Hour": (1.0, "hrs"),
    "1 Hours": (1.0, "hrs"),  # Azure has both spellings in the wild
    "100 Hours": (100.0, "hrs"),
}


def parse_unit_of_measure(uom: str) -> tuple[float, str]:
    """Return (divisor, canonical_unit) for a per-hour meter; raise ValueError otherwise."""
    try:
        return _HOUR_DIVISORS[uom]
    except KeyError as exc:
        raise ValueError(f"unsupported unitOfMeasure: {uom}") from exc
