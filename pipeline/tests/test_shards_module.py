from shards import _VALID_KINDS
from normalize.enums import apply_kind_defaults, load_enums


def test_cache_kv_is_a_valid_kind():
    assert "cache.kv" in _VALID_KINDS


def test_cache_kv_in_enums_yaml():
    assert "cache.kv" in load_enums()["kind"]


def test_apply_kind_defaults_cache_kv_returns_defaults():
    out = apply_kind_defaults("cache.kv", {"tenancy": "redis"})
    assert out["commitment"] == "on_demand"
    assert out["tenancy"] == "redis"


def test_container_orchestration_is_a_valid_kind():
    assert "container.orchestration" in _VALID_KINDS


def test_container_orchestration_in_enums_yaml():
    assert "container.orchestration" in load_enums()["kind"]


def test_apply_kind_defaults_container_orchestration_returns_defaults():
    out = apply_kind_defaults("container.orchestration", {"tenancy": "kubernetes"})
    assert out["commitment"] == "on_demand"
    assert out["tenancy"] == "kubernetes"
    assert out["os"] == "standard"
