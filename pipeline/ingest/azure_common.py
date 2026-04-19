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


# ---------------------------------------------------------------------------
# Live Azure retail-prices fetcher (m3a.4.1)
# ---------------------------------------------------------------------------

import hashlib  # noqa: E402
import json  # noqa: E402
import os  # noqa: E402
import time  # noqa: E402
from pathlib import Path  # noqa: E402
from urllib.parse import urlencode  # noqa: E402

import requests  # noqa: E402

_AZURE_RETAIL_BASE = "https://prices.azure.com/api/retail/prices"
_MAX_PAGES = 10_000
_UA = "sku-pipeline/0.0 (+https://github.com/sofq/sku)"


def fetch_prices(filter_str: str, target: Path, *, session=None, retries: int = 3) -> str:
    """Page through Azure retail-prices with $filter=<filter_str> and write
    the concatenated items to `target` as ``{"Items": [...]}`` with items sorted
    by skuId.  Returns SHA256 hex digest of the sorted normalised item list.

    First request: ``{_AZURE_RETAIL_BASE}?$filter=<filter_str>&api-version=2023-01-01-preview``.
    Subsequent requests follow ``NextPageLink`` verbatim (Azure returns full URLs).
    Caps at ``_MAX_PAGES`` pages → ``RuntimeError("page_cap_exceeded")``.
    Retries 500/502/503/504 per-page with ``time.sleep(0.5 * 2**attempt)``; 4xx no retry.
    Atomic write via .part + os.replace.
    User-Agent: ``sku-pipeline/<anything> (+https://github.com/sofq/sku)``.
    """
    _RETRY_STATUSES = frozenset({500, 502, 503, 504})
    headers = {"User-Agent": _UA}

    own_session = session is None
    sess = requests.Session() if own_session else session

    try:
        items: list[dict] = []
        first_url = (
            _AZURE_RETAIL_BASE
            + "?"
            + urlencode({"$filter": filter_str, "api-version": "2023-01-01-preview"})
        )
        next_url: str | None = first_url

        for _ in range(_MAX_PAGES):
            if next_url is None:
                break

            url = next_url
            last_exc: Exception | None = None

            for attempt in range(retries):
                try:
                    resp = sess.get(url, headers=headers, timeout=15.0)
                    if resp.status_code in _RETRY_STATUSES:
                        if attempt < retries - 1:
                            time.sleep(0.5 * 2**attempt)
                            continue
                        raise RuntimeError(
                            f"GET {url} failed with status {resp.status_code}"
                            f" after {retries} attempts"
                        )
                    if resp.status_code >= 400:
                        raise RuntimeError(
                            f"GET {url} failed with status {resp.status_code} (no retry)"
                        )
                    resp.raise_for_status()
                    data = resp.json()
                    items.extend(data.get("Items") or [])
                    next_url = data.get("NextPageLink") or None
                    last_exc = None
                    break
                except RuntimeError:
                    raise
                except requests.RequestException as exc:
                    last_exc = exc
                    if attempt < retries - 1:
                        time.sleep(0.5 * 2**attempt)
            else:
                if last_exc is not None:
                    raise RuntimeError(f"GET {url} failed after {retries} attempts: {last_exc}")

            if next_url is None:
                break
        else:
            raise RuntimeError("page_cap_exceeded")

        # Sort items by skuId for determinism.
        sorted_items = sorted(items, key=lambda x: x.get("skuId", ""))
        serialised = json.dumps(sorted_items, separators=(",", ":"), sort_keys=True).encode()
        digest = hashlib.sha256(serialised).hexdigest()

        # Atomic write.
        target = Path(target)
        target.parent.mkdir(parents=True, exist_ok=True)
        part = target.with_suffix(target.suffix + ".part")
        body = json.dumps({"Items": sorted_items}, sort_keys=True)
        part.write_text(body, encoding="utf-8")
        os.replace(part, target)

        return digest
    finally:
        if own_session:
            sess.close()
