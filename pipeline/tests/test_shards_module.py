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


def test_search_engine_is_a_valid_kind():
    assert "search.engine" in _VALID_KINDS


def test_search_engine_in_enums_yaml():
    assert "search.engine" in load_enums()["kind"]


def test_apply_kind_defaults_search_engine_returns_defaults():
    out = apply_kind_defaults("search.engine", {})
    assert out["commitment"] == "on_demand"
    assert out["os"] == "managed-cluster"
    assert out["tenancy"] == "shared"


def test_paas_app_is_a_valid_kind():
    assert "paas.app" in _VALID_KINDS


def test_paas_app_in_enums_yaml():
    assert "paas.app" in load_enums()["kind"]


def test_apply_kind_defaults_paas_app_returns_defaults():
    out = apply_kind_defaults("paas.app", {})
    assert out["commitment"] == "on_demand"
    assert out["os"] == "linux"
    assert out["tenancy"] == "dedicated"


def test_warehouse_query_is_a_valid_kind():
    assert "warehouse.query" in _VALID_KINDS


def test_warehouse_query_in_enums_yaml():
    assert "warehouse.query" in load_enums()["kind"]


def test_apply_kind_defaults_warehouse_query_returns_defaults():
    out = apply_kind_defaults("warehouse.query", {})
    assert out["commitment"] == "on_demand"
    assert out["os"] == "on-demand"
    assert out["tenancy"] == "shared"
