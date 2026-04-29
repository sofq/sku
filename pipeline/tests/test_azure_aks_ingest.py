"""Tests for azure_aks ingest module."""
from pathlib import Path

from ingest.azure_aks import ingest


FIX = Path(__file__).parent / "fixtures" / "azure_aks"


def test_aks_standard_tier_emits_row():
    rows = list(ingest(prices_path=FIX / "aks_eastus.json"))
    std = [r for r in rows if r["resource_name"] == "aks-standard"]
    assert std, "expected aks-standard row"
    assert std[0]["prices"][0]["amount"] == 0.1


def test_aks_free_tier_emits_zero_priced_row():
    rows = list(ingest(prices_path=FIX / "aks_eastus.json"))
    free = [r for r in rows if r["resource_name"] == "aks-free"]
    assert free, "free tier row must be emitted"
    assert free[0]["prices"][0]["amount"] == 0.0


def test_aks_premium_tier_emits_row():
    rows = list(ingest(prices_path=FIX / "aks_eastus.json"))
    prem = [r for r in rows if r["resource_name"] == "aks-premium"]
    assert prem, "expected aks-premium row"
    assert prem[0]["prices"][0]["amount"] == 0.6


def test_virtual_nodes_emits_linux_row():
    rows = list(ingest(
        prices_path=FIX / "aks_eastus.json",
        aci_prices_path=FIX / "aci_eastus.json",
    ))
    vn = [r for r in rows if r["resource_attrs"]["extra"].get("mode") == "virtual-nodes"]
    names = {r["resource_name"] for r in vn}
    assert "aks-virtual-nodes-linux" in names
    # Verify both vcpu and memory dimensions are present
    for r in vn:
        if r["resource_name"] == "aks-virtual-nodes-linux":
            dims = {p["dimension"] for p in r["prices"]}
            assert "vcpu" in dims
            assert "memory" in dims


def test_virtual_nodes_dedupes_per_second_meter():
    # Azure publishes both "1 GB Hour" and "1 GB Second" meters for some
    # regions (e.g. francesouth). The ingest must keep only the per-hour
    # denomination — otherwise the shard build hits a UNIQUE constraint on
    # (sku_id, dimension, tier).
    rows = list(ingest(
        prices_path=FIX / "aks_eastus.json",
        aci_prices_path=FIX / "aci_francesouth_dup.json",
    ))
    vn = [r for r in rows if r["region"] == "francesouth"
          and r["resource_attrs"]["extra"].get("mode") == "virtual-nodes"]
    assert len(vn) == 1
    prices = vn[0]["prices"]
    keys = [(p["dimension"], p["tier"]) for p in prices]
    assert len(keys) == len(set(keys)), f"duplicate price keys: {keys}"
    by_dim = {p["dimension"]: p for p in prices}
    assert by_dim["memory"]["amount"] == 0.00722
    assert by_dim["memory"]["unit"] == "gb-hour"
    assert by_dim["vcpu"]["amount"] == 0.0658
    assert by_dim["vcpu"]["unit"] == "hour"


def test_terms_tenancy_is_kubernetes():
    rows = list(ingest(prices_path=FIX / "aks_eastus.json"))
    for r in rows:
        assert r["terms"]["tenancy"] == "kubernetes"


def test_terms_os_carries_tier():
    rows = list(ingest(prices_path=FIX / "aks_eastus.json"))
    by_name = {r["resource_name"]: r for r in rows}
    if "aks-standard" in by_name:
        assert by_name["aks-standard"]["terms"]["os"] == "standard"
    if "aks-free" in by_name:
        assert by_name["aks-free"]["terms"]["os"] == "free"
    if "aks-premium" in by_name:
        assert by_name["aks-premium"]["terms"]["os"] == "premium"
