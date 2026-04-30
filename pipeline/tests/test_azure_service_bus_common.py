"""Smoke tests for _azure_service_bus_common: verify exported symbols."""

from __future__ import annotations

from ingest import _azure_service_bus_common as sb_common


def test_filter_constant_exported():
    assert hasattr(sb_common, "_SERVICE_BUS_FILTER")
    assert sb_common._SERVICE_BUS_FILTER == "serviceName eq 'Service Bus'"


def test_fetch_function_exported():
    assert hasattr(sb_common, "fetch_service_bus_prices")
    assert callable(sb_common.fetch_service_bus_prices)
