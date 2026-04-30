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


# M-δ new kinds ---------------------------------------------------------------

def test_messaging_queue_is_a_valid_kind():
    assert "messaging.queue" in _VALID_KINDS


def test_messaging_queue_in_enums_yaml():
    assert "messaging.queue" in load_enums()["kind"]


def test_apply_kind_defaults_messaging_queue_returns_defaults():
    out = apply_kind_defaults("messaging.queue", {})
    assert out["commitment"] == "on_demand"
    assert out["tenancy"] == ""
    assert out["os"] == ""


def test_messaging_topic_is_a_valid_kind():
    assert "messaging.topic" in _VALID_KINDS


def test_messaging_topic_in_enums_yaml():
    assert "messaging.topic" in load_enums()["kind"]


def test_apply_kind_defaults_messaging_topic_returns_defaults():
    out = apply_kind_defaults("messaging.topic", {})
    assert out["commitment"] == "on_demand"
    assert out["tenancy"] == ""
    assert out["os"] == ""


def test_dns_zone_is_a_valid_kind():
    assert "dns.zone" in _VALID_KINDS


def test_dns_zone_in_enums_yaml():
    assert "dns.zone" in load_enums()["kind"]


def test_apply_kind_defaults_dns_zone_returns_defaults():
    out = apply_kind_defaults("dns.zone", {})
    assert out["commitment"] == "on_demand"
    assert out["tenancy"] == ""
    assert out["os"] == ""


def test_api_gateway_is_a_valid_kind():
    assert "api.gateway" in _VALID_KINDS


def test_api_gateway_in_enums_yaml():
    assert "api.gateway" in load_enums()["kind"]


def test_apply_kind_defaults_api_gateway_returns_defaults():
    out = apply_kind_defaults("api.gateway", {})
    assert out["commitment"] == "on_demand"
    assert out["tenancy"] == ""
    assert out["os"] == ""


def test_new_m_delta_tenancy_tokens_in_enums_yaml():
    enums = load_enums()
    for token in ("messaging", "dns", "api-gateway", "cdn", "firestore-native"):
        assert token in enums["tenancy"], f"tenancy token {token!r} missing from enums.yaml"


def test_new_m_delta_os_tokens_in_enums_yaml():
    enums = load_enums()
    expected_tokens = (
        "fifo", "topic",
        "apim-consumption", "apim-developer", "apim-basic", "apim-standard",
        "apim-premium", "apim-premium-v2", "apim-isolated",
        "rest-api", "http-api",
        "dns-public", "dns-private",
        "cdn-egress", "cdn-request", "cdn-origin-shield", "cdn-cache-fill",
        "pubsub-throughput", "event-hubs-standard", "event-hubs-premium",
    )
    for token in expected_tokens:
        assert token in enums["os"], f"os token {token!r} missing from enums.yaml"
