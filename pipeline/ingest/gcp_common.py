"""Shared GCP ingest helpers: region normalization, Cloud-Billing price decoding.

GCP's Cloud Billing Catalog API expresses prices as `{units, nanos}` pairs —
`units` is integer dollars, `nanos` is the sub-dollar remainder in billionths.
Usage units are ratios like `h`, `GiBy.h`, `GiBy.mo`; we canonicalize to the
`hrs` / `gb-hr` / `gb-mo` strings already used by the other ingesters.
"""

from __future__ import annotations

import hashlib
import json
import os
import time
from pathlib import Path

import requests

from .aws_common import load_region_normalizer  # re-export — same shared YAML loader

__all__ = [
    "load_region_normalizer",
    "parse_unit_price",
    "parse_usage_unit",
    "fetch_skus",
    "build_authenticated_session",
    "service_ids_for_shard",
]

_GCP_BILLING_BASE = "https://cloudbilling.googleapis.com/v1"

_GCP_SERVICE_IDS: dict[str, str | tuple[str, ...]] = {
    "gcp_gce": "6F81-5844-456A",  # Compute Engine
    "gcp_cloud_sql": "9662-B51E-5089",  # Cloud SQL
    "gcp_gcs": "95FF-2EF5-5EA1",  # Cloud Storage
    "gcp_run": "152E-C115-5142",  # Cloud Run
    "gcp_functions": "29E7-DA93-CA13",  # Cloud Functions
    "gcp_spanner": "CC63-0873-48FD",  # Cloud Spanner
    "gcp_memorystore": (
        "5AF5-2C11-D467",  # Cloud Memorystore for Redis
        "9C2E-5AAC-D058",  # Cloud Memorystore for Memcached
    ),
}

_USAGE_UNITS: dict[str, tuple[float, str]] = {
    "h": (1.0, "hrs"),
    "GiBy.h": (1.0, "gb-hr"),
    "GiBy.mo": (1.0, "gb-mo"),
    "GiBy.d": (1.0 / 30.4375, "gb-mo"),  # gb-day → gb-month (avg Gregorian month)
    "By.mo": (1.0 / (1024**3), "gb-mo"),  # byte-month → gb-month
    "count": (1.0, "requests"),
    "s": (1.0, "s"),  # serverless CPU/request duration
    "GiBy.s": (1.0, "gb-s"),  # serverless memory allocation time
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


def build_authenticated_session() -> requests.Session:
    """Return a `requests.Session` pre-authenticated via Google ADC.

    Uses `google.auth.default()` with the `cloud-billing.readonly` scope and
    sets a static `Authorization: Bearer <token>` header. Under GitHub
    Actions the token comes from Workload Identity Federation (see
    `docs/ops/validation.md`); locally any ADC path works (user login,
    service-account JSON, etc.).
    """
    import google.auth
    import google.auth.transport.requests

    creds, _ = google.auth.default(
        scopes=["https://www.googleapis.com/auth/cloud-billing.readonly"]
    )
    creds.refresh(google.auth.transport.requests.Request())
    sess = requests.Session()
    sess.headers["Authorization"] = f"Bearer {creds.token}"
    return sess


def service_ids_for_shard(shard: str) -> tuple[str, ...]:
    service_ids = _GCP_SERVICE_IDS[shard]
    if isinstance(service_ids, str):
        return (service_ids,)
    return service_ids


def fetch_skus(
    shard: str,
    target: Path,
    *,
    session: requests.Session | None = None,
    retries: int = 3,
) -> str:
    """Page through `/services/{service_id}/skus?pageSize=5000` and write
    `{"skus": [...]}` (sorted by skuId) to target. Returns SHA256 hex digest
    over the sorted skus (same serialization used for the file body's items).

    Auth: the caller-supplied `session` must carry an `Authorization: Bearer
    <token>` header (see `build_authenticated_session`). Requests without
    such a header will 401/403.

    Pagination via response `nextPageToken`; append `&pageToken=<token>` to
    each subsequent GET. Terminates when the response omits `nextPageToken`
    or it's empty.
    403 → RuntimeError("gcp_forbidden: <shard>/<service_id>") — no retry.
    Other 4xx → RuntimeError(url-contains) — no retry.
    500/502/503/504 retried with `time.sleep(0.5 * 2**attempt)` per page.
    Unknown `shard` → KeyError from the dict lookup — let it propagate.
    User-Agent header: `sku-pipeline/...` matching pipeline/ingest/http.py LiveClient.
    Atomic write via .part + os.replace.
    """
    # KeyError propagates naturally for unknown shards — this must happen before
    # any network setup so no requests are made.
    service_ids = service_ids_for_shard(shard)

    if session is None:
        session = requests.Session()

    session.headers.update(
        {
            "User-Agent": "sku-pipeline/0.0 (+https://github.com/sofq/sku)",
            "Accept": "application/json",
        }
    )

    all_skus: list[dict] = []

    for service_id in service_ids:
        base_url = f"{_GCP_BILLING_BASE}/services/{service_id}/skus"
        page_token: str | None = None

        while True:
            params: dict[str, str] = {"pageSize": "5000"}
            if page_token:
                params["pageToken"] = page_token

            # Per-page retry loop — resets between pages.
            last_exc: Exception | None = None
            resp = None
            for attempt in range(retries):
                try:
                    resp = session.get(base_url, params=params, timeout=30.0)
                    if resp.status_code == 403:
                        raise RuntimeError(f"gcp_forbidden: {shard}/{service_id}")
                    if 400 <= resp.status_code < 500:
                        raise RuntimeError(f"gcp fetch failed {resp.status_code}: {resp.url}")
                    if resp.status_code in (500, 502, 503, 504):
                        last_exc = RuntimeError(
                            f"gcp fetch server error {resp.status_code}: {resp.url}"
                        )
                        time.sleep(0.5 * (2**attempt))
                        continue
                    resp.raise_for_status()
                    break
                except RuntimeError:
                    raise
                except requests.RequestException as exc:
                    last_exc = exc
                    time.sleep(0.5 * (2**attempt))
            else:
                raise RuntimeError(f"gcp fetch failed after {retries} attempts: {last_exc}")

            data = resp.json()
            all_skus.extend(data.get("skus", []))

            page_token = data.get("nextPageToken") or None
            if not page_token:
                break

    sorted_skus = sorted(all_skus, key=lambda x: x.get("skuId", ""))

    # Compute SHA256 over the compact sorted serialization.
    serialized_bytes = json.dumps(sorted_skus, separators=(",", ":"), sort_keys=True).encode()
    digest = hashlib.sha256(serialized_bytes).hexdigest()

    # Atomic write.
    target = Path(target)
    target.parent.mkdir(parents=True, exist_ok=True)
    part_path = Path(str(target) + ".part")
    part_path.write_text(json.dumps({"skus": sorted_skus}, sort_keys=True))
    os.replace(part_path, target)

    return digest
