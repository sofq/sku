"""Build gcp-gce _MACHINE_SPECS from the Compute Engine metadata API.

Replaces the hand-maintained 3-entry allowlist. Fixture path (fixture_path
argument) is the authoritative test / offline-CI entry; live path
(project_id argument) calls compute.googleapis.com with Application
Default Credentials and is exercised only by the weekly refresh target.

Each family's Cloud-Billing-Catalog CPU/RAM description prefix is encoded
in _FAMILY_PREFIX_MAP — adding a new family requires one map entry.
Unknown families raise KeyError on purpose so silent coverage regressions
can't happen.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

# Family token -> (cpu_prefix, ram_prefix) as they appear in the SKU description.
# Strings are matched with str.startswith() by gcp_gce.ingest().
#
# n1 and e2 verified against skus.json fixture; n2 and c2 verified by construction
# (Task 3 fixture inserts use these exact strings). Remaining families (n2d/n4/c2d/
# c3/c3d/c4/t2a/t2d/m1/m2/m3) are unverified draft strings — the Task 5 per-family
# coverage gate catches silent mismatches when run against live data.
_FAMILY_PREFIX_MAP: dict[str, tuple[str, str]] = {
    "n1":  ("N1 Predefined Instance Core",       "N1 Predefined Instance Ram"),
    "n2":  ("N2 Instance Core",                  "N2 Instance Ram"),
    "n2d": ("N2D AMD Instance Core",             "N2D AMD Instance Ram"),
    "n4":  ("N4 Instance Core",                  "N4 Instance Ram"),
    "e2":  ("E2 Instance Core",                  "E2 Instance Ram"),
    "c2":  ("Compute optimized Core",            "Compute optimized Ram"),
    "c2d": ("C2D AMD Instance Core",             "C2D AMD Instance Ram"),
    "c3":  ("C3 Instance Core",                  "C3 Instance Ram"),
    "c3d": ("C3D AMD Instance Core",             "C3D AMD Instance Ram"),
    "c4":  ("C4 Instance Core",                  "C4 Instance Ram"),
    "t2a": ("T2A Arm Instance Core",             "T2A Arm Instance Ram"),
    "t2d": ("T2D AMD Instance Core",             "T2D AMD Instance Ram"),
    "m1":  ("Memory-optimized Instance Core",    "Memory-optimized Instance Ram"),
    "m2":  ("M2 Memory-optimized Instance Core", "M2 Memory-optimized Instance Ram"),
    "m3":  ("M3 Memory-optimized Instance Core", "M3 Memory-optimized Instance Ram"),
}


def _family_of(machine_type: str) -> str:
    """Return the family token, e.g. 'n1-standard-2' -> 'n1', 'c2d-highcpu-4' -> 'c2d'."""
    return machine_type.split("-", 1)[0]


def _specs_for(entry: dict[str, Any]) -> tuple[int, float, str, str]:
    """Turn one aggregatedList machineType entry into (vcpu, ram_gib, cpu_pfx, ram_pfx).

    Raises KeyError on unknown family — intentional, see module docstring.
    """
    name = entry["name"]
    family = _family_of(name)
    if family not in _FAMILY_PREFIX_MAP:
        raise KeyError(f"unknown machine family {family!r} (machineType={name})")
    cpu_pfx, ram_pfx = _FAMILY_PREFIX_MAP[family]
    vcpu = int(entry["guestCpus"])
    ram_gib = float(entry["memoryMb"]) / 1024.0
    return (vcpu, ram_gib, cpu_pfx, ram_pfx)


def _should_skip(entry: dict[str, Any]) -> bool:
    """Exclude GPU-attached, sole-tenant, and custom-shape machines."""
    if entry.get("accelerators"):
        return True
    name = entry["name"]
    if name.startswith("custom-") or "-sole-tenant-" in name:
        return True
    return False


def load_specs(
    *,
    fixture_path: Path | None = None,
    project_id: str | None = None,
) -> dict[str, tuple[int, float, str, str]]:
    """Return {machine_type: (vcpu, ram_gib, cpu_pfx, ram_pfx)} for all supported families.

    Exactly one of fixture_path / project_id must be provided.
    """
    if fixture_path is None and project_id is None:
        raise ValueError("must provide fixture_path or project_id")
    if fixture_path is not None and project_id is not None:
        raise ValueError("provide exactly one of fixture_path / project_id")

    if fixture_path is not None:
        with fixture_path.open() as fh:
            doc = json.load(fh)
    else:
        doc = _fetch_live(project_id=project_id)  # type: ignore[arg-type]

    out: dict[str, tuple[int, float, str, str]] = {}
    for _zone_key, zone_bucket in (doc.get("items") or {}).items():
        for entry in zone_bucket.get("machineTypes") or []:
            if _should_skip(entry):
                continue
            name = entry["name"]
            if name in out:
                continue  # same type repeats per zone; keep first
            out[name] = _specs_for(entry)
    return out


def _fetch_live(*, project_id: str) -> dict[str, Any]:
    """Call compute.googleapis.com with ADC — exercised only by the weekly refresh."""
    import google.auth
    import google.auth.transport.requests

    creds, _proj = google.auth.default(
        scopes=["https://www.googleapis.com/auth/compute.readonly"]
    )
    session = google.auth.transport.requests.AuthorizedSession(creds)
    base = f"https://compute.googleapis.com/compute/v1/projects/{project_id}/aggregated/machineTypes"
    merged: dict[str, Any] = {"items": {}}
    page_token: str | None = None
    while True:
        params: dict[str, Any] = {"maxResults": 500}
        if page_token:
            params["pageToken"] = page_token
        resp = session.get(base, params=params, timeout=60)
        resp.raise_for_status()
        doc = resp.json()
        for zone_key, bucket in (doc.get("items") or {}).items():
            merged["items"].setdefault(zone_key, {"machineTypes": []})
            merged["items"][zone_key]["machineTypes"].extend(
                bucket.get("machineTypes") or []
            )
        page_token = doc.get("nextPageToken")
        if not page_token:
            break
    return merged
