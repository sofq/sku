"""Shared Azure ingest helpers: region normalization, unit-of-measure parsing.

m3b.2 extends the m3b.1 per-hour-only API with two new UoM families:
- storage meters use `"1 GB/Month"` or `"1/Month"` and divide out Month.
- request / execution meters use `"1000000"` / `"10K"` and divide to per-request.
The helpers live here (not in each shard's ingest module) so the canonical-unit
strings (`hrs` / `gb-mo` / `month` / `requests`) are defined exactly once.
"""

from __future__ import annotations

from .aws_common import load_region_normalizer  # re-export — same shared YAML loader

__all__ = [
    "load_region_normalizer",
    "parse_unit_of_measure",
    "parse_storage_uom",
    "parse_request_uom",
]

_HOUR_DIVISORS: dict[str, tuple[float, str]] = {
    "1 Hour": (1.0, "hrs"),
    "1 Hours": (1.0, "hrs"),
    "100 Hours": (100.0, "hrs"),
}

_STORAGE_DIVISORS: dict[str, tuple[float, str]] = {
    "1 GB/Month": (1.0, "gb-mo"),
    "1 GB/Hour": (1.0, "gb-hr"),  # rarely used by disks but keep for future
    "1/Month": (1.0, "month"),
}

_REQUEST_DIVISORS: dict[str, tuple[float, str]] = {
    "1000000": (1_000_000.0, "requests"),
    "10K": (10_000.0, "requests"),
    "1K": (1_000.0, "requests"),
    "1": (1.0, "requests"),
}


def parse_unit_of_measure(uom: str) -> tuple[float, str]:
    """Per-hour meter parser (m3b.1 surface; kept for backwards-compat)."""
    try:
        return _HOUR_DIVISORS[uom]
    except KeyError as exc:
        raise ValueError(f"unsupported unitOfMeasure: {uom}") from exc


def parse_storage_uom(uom: str) -> tuple[float, str]:
    """Storage / per-month meter parser."""
    try:
        return _STORAGE_DIVISORS[uom]
    except KeyError as exc:
        raise ValueError(f"unsupported storage unitOfMeasure: {uom}") from exc


def parse_request_uom(uom: str) -> tuple[float, str]:
    """Request / per-execution meter parser."""
    try:
        return _REQUEST_DIVISORS[uom]
    except KeyError as exc:
        raise ValueError(f"unsupported request unitOfMeasure: {uom}") from exc
