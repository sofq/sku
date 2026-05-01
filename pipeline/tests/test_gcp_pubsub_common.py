"""Smoke tests for _gcp_pubsub_common shared helpers."""

from __future__ import annotations

from ingest._gcp_pubsub_common import _PUBSUB_SERVICE_ID, fetch_pubsub_skus


def test_pubsub_service_id_constant():
    assert _PUBSUB_SERVICE_ID == "A1E8-BE35-7EBC"


def test_fetch_pubsub_skus_is_callable():
    assert callable(fetch_pubsub_skus)


def test_module_exports_expected_symbols():
    import ingest._gcp_pubsub_common as m
    assert hasattr(m, "_PUBSUB_SERVICE_ID")
    assert hasattr(m, "fetch_pubsub_skus")
