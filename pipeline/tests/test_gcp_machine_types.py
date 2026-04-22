from __future__ import annotations

from pathlib import Path

import pytest

from ingest.gcp_machine_types import load_specs, _family_of

_FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "gcp_gce" / "machine_types.json"


def test_load_specs_from_fixture_covers_n1_standard():
    specs = load_specs(fixture_path=_FIXTURE)
    assert "n1-standard-2" in specs
    vcpu, ram_gib, cpu_pfx, ram_pfx = specs["n1-standard-2"]
    assert vcpu == 2
    assert ram_gib == pytest.approx(7.5)
    assert cpu_pfx == "N1 Predefined Instance Core"
    assert ram_pfx == "N1 Predefined Instance Ram"


def test_load_specs_covers_new_families():
    specs = load_specs(fixture_path=_FIXTURE)
    assert "n2-standard-2" in specs
    assert "e2-standard-4" in specs
    assert "c2-standard-4" in specs
    assert "c2d-standard-4" in specs


def test_load_specs_drops_gpu_attached_machines():
    specs = load_specs(fixture_path=_FIXTURE)
    # a2-highgpu-1g carries accelerators[] — must be excluded (non-goal)
    assert "a2-highgpu-1g" not in specs


def test_load_specs_is_deduplicated_across_zones():
    # If the fixture ever lists the same type in two zones, load_specs must
    # not emit duplicate keys (dict semantics enforce it; this test locks it in).
    specs = load_specs(fixture_path=_FIXTURE)
    names = [k for k in specs]
    assert len(names) == len(set(names))


def test_family_of_parses_leading_token():
    assert _family_of("n1-standard-2") == "n1"
    assert _family_of("c2d-standard-4") == "c2d"
    assert _family_of("t2a-standard-1") == "t2a"
    assert _family_of("e2-micro") == "e2"


def test_load_specs_raises_for_unknown_family():
    # If Google adds a new family that isn't in _FAMILY_PREFIX_MAP yet,
    # we want a loud failure, not silent drops — otherwise future coverage
    # regressions are invisible.
    with pytest.raises(KeyError, match="unknown machine family"):
        from ingest.gcp_machine_types import _specs_for
        _specs_for({"name": "z9-standard-2", "guestCpus": 2, "memoryMb": 8192})
