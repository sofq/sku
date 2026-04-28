"""Tests for aws_eks ingest module."""
from pathlib import Path

import pytest

from ingest.aws_eks import ingest


FIX = Path(__file__).parent / "fixtures" / "aws_eks"


def test_ingest_returns_control_plane_rows():
    rows = list(ingest(offer_path=FIX / "offer_us_east_1.json"))
    cp_rows = [r for r in rows if r["resource_attrs"]["extra"]["mode"] == "control-plane"]
    names = {r["resource_name"] for r in cp_rows}
    assert "eks-standard" in names
    assert "eks-extended-support" in names
    for r in cp_rows:
        assert r["resource_attrs"]["vcpu"] is None
        assert r["resource_attrs"]["memory_gb"] is None


def test_ingest_returns_fargate_rows_with_two_dimensions():
    rows = list(ingest(offer_path=FIX / "offer_us_east_1.json"))
    fargate = [r for r in rows if r["resource_attrs"]["extra"]["mode"] == "fargate"]
    assert fargate, "expected at least one fargate row"
    for r in fargate:
        dims = {p["dimension"] for p in r["prices"]}
        assert "vcpu" in dims
        assert "memory" in dims


def test_ingest_skips_unknown_region():
    rows = list(ingest(offer_path=FIX / "offer_synthetic_unknown_region.json"))
    assert rows == []


def test_terms_os_carries_tier():
    rows = list(ingest(offer_path=FIX / "offer_us_east_1.json"))
    by_name = {r["resource_name"]: r for r in rows}
    if "eks-standard" in by_name:
        assert by_name["eks-standard"]["terms"]["os"] == "standard"
    if "eks-extended-support" in by_name:
        assert by_name["eks-extended-support"]["terms"]["os"] == "extended-support"
    if "eks-fargate" in by_name:
        assert by_name["eks-fargate"]["terms"]["os"] == "fargate"


def test_terms_tenancy_is_kubernetes():
    rows = list(ingest(offer_path=FIX / "offer_us_east_1.json"))
    for r in rows:
        assert r["terms"]["tenancy"] == "kubernetes"
