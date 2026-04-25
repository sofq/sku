import json
from pathlib import Path

from ingest.gcp_memorystore import ingest

FIXTURE = Path(__file__).parent.parent / "testdata" / "gcp_memorystore"


def test_ingest_emits_cache_kv():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    assert rows
    assert all(r["kind"] == "cache.kv" for r in rows)


def test_ingest_includes_both_engines():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    engines = {r["resource_attrs"]["extra"]["engine"] for r in rows}
    assert engines == {"redis", "memcached"}


def test_ingest_memory_gb_populated():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    for r in rows:
        assert r["resource_attrs"]["memory_gb"] is not None


def test_ingest_resource_name_includes_engine():
    rows = list(ingest(skus_path=FIXTURE / "skus.json"))
    for r in rows:
        engine = r["resource_attrs"]["extra"]["engine"]
        assert r["resource_name"].startswith(f"memorystore-{engine}")


def _sku(
    *,
    sku_id: str,
    description: str,
    service: str,
    service_regions: list[str] | None = None,
    nanos: int = 1000000,
    usage_unit: str = "h",
) -> dict:
    return {
        "skuId": sku_id,
        "description": description,
        "serviceRegions": service_regions or ["us-east1"],
        "category": {
            "serviceDisplayName": service,
            "usageType": "OnDemand",
        },
        "pricingInfo": [
            {
                "pricingExpression": {
                    "usageUnit": usage_unit,
                    "tieredRates": [
                        {
                            "unitPrice": {
                                "currencyCode": "USD",
                                "units": "0",
                                "nanos": nanos,
                            }
                        }
                    ],
                }
            }
        ],
    }


def test_ingest_handles_live_redis_capacity_tiers_without_gb(tmp_path):
    skus_path = tmp_path / "skus.json"
    skus_path.write_text(
        json.dumps(
            {
                "skus": [
                    _sku(
                        sku_id="redis-basic-m1-us-east1",
                        description="Redis Capacity Basic M1 Iowa",
                        service="Cloud Memorystore for Redis",
                    )
                ]
            }
        )
    )

    rows = list(ingest(skus_path=skus_path))

    assert len(rows) == 1
    assert rows[0]["resource_attrs"]["extra"] == {"engine": "redis", "tier": "basic"}
    assert rows[0]["resource_attrs"]["memory_gb"] == 5


def test_ingest_handles_live_memcached_ram_and_skips_core(tmp_path):
    skus_path = tmp_path / "skus.json"
    skus_path.write_text(
        json.dumps(
            {
                "skus": [
                    _sku(
                        sku_id="memcached-ram-m1-us-east1",
                        description="Memorystore for Memcached Custom RAM M1 Iowa",
                        service="Cloud Memorystore for Memcached",
                        usage_unit="GiBy.h",
                    ),
                    _sku(
                        sku_id="memcached-core-m1-us-east1",
                        description="Memorystore for Memcached Custom Core M1 Iowa",
                        service="Cloud Memorystore for Memcached",
                        usage_unit="h",
                    ),
                ]
            }
        )
    )

    rows = list(ingest(skus_path=skus_path))

    assert len(rows) == 1
    assert rows[0]["sku_id"] == "memcached-ram-m1-us-east1"
    assert rows[0]["resource_attrs"]["extra"] == {
        "engine": "memcached",
        "tier": "standard",
    }
    assert rows[0]["resource_attrs"]["memory_gb"] == 1


def test_ingest_makes_region_scoped_sku_ids_for_multi_region_skus(tmp_path):
    skus_path = tmp_path / "skus.json"
    skus_path.write_text(
        json.dumps(
            {
                "skus": [
                    _sku(
                        sku_id="redis-basic-m1",
                        description="Redis Capacity Basic M1 Iowa",
                        service="Cloud Memorystore for Redis",
                        service_regions=["us-east1", "us-central1"],
                    )
                ]
            }
        )
    )

    rows = list(ingest(skus_path=skus_path))

    assert len(rows) == 2
    assert {r["region"] for r in rows} == {"us-east1", "us-central1"}
    assert len({r["sku_id"] for r in rows}) == len(rows)
