"""Tests for gcp_gke ingest module."""
from pathlib import Path

import pytest

from ingest.gcp_gke import ingest


FIX = Path(__file__).parent / "fixtures" / "gcp_gke"


def _rows():
    return list(ingest(skus_path=FIX / "skus.json"))


def test_standard_control_plane_row_emitted():
    rows = _rows()
    standard = [r for r in rows if r["resource_name"] == "gke-standard"]
    assert standard, "expected at least one gke-standard row"
    row = standard[0]
    assert row["resource_attrs"]["extra"]["mode"] == "control-plane"
    prices = row["prices"]
    cluster_prices = [p for p in prices if p["dimension"] == "cluster"]
    assert cluster_prices, "expected a 'cluster' price dimension"
    assert abs(cluster_prices[0]["amount"] - 0.10) < 1e-6


def test_autopilot_row_has_three_price_dimensions():
    rows = _rows()
    autopilot = [r for r in rows if r["resource_name"] == "gke-autopilot"]
    assert autopilot, "expected at least one gke-autopilot row"
    row = autopilot[0]
    dims = {p["dimension"] for p in row["prices"]}
    assert "vcpu" in dims
    assert "memory" in dims
    assert "storage" in dims


def test_zonal_sku_is_filtered():
    rows = _rows()
    names = {r["resource_name"] for r in rows}
    assert "gke-zonal" not in names, "Zonal Kubernetes Clusters SKU should not produce any row"


def test_spot_sku_is_filtered():
    rows = _rows()
    # Spot mCPU price from fixture: 13300 / 1e9 = 0.0000133
    spot_price = 13300 / 1e9
    for row in rows:
        for price in row["prices"]:
            assert abs(price["amount"] - spot_price) > 1e-10, (
                f"Spot mCPU price {spot_price} found in row {row['resource_name']} — should be filtered"
            )


def test_terms_tenancy_is_kubernetes():
    rows = _rows()
    assert rows, "expected rows from fixture"
    for row in rows:
        assert row["terms"]["tenancy"] == "kubernetes", (
            f"row {row['resource_name']} has tenancy={row['terms']['tenancy']!r}, expected 'kubernetes'"
        )
