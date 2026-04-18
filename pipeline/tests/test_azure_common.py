"""Unit tests for shared Azure ingest helpers."""

import pytest

from ingest.azure_common import (
    load_region_normalizer,
    parse_unit_of_measure,
)


def test_normalizer_maps_azure_regions():
    n = load_region_normalizer()
    assert n.normalize("azure", "eastus") == "us-east"
    assert n.normalize("azure", "eastus2") == "us-east"
    assert n.normalize("azure", "westus2") == "us-west"
    assert n.normalize("azure", "westeurope") == "eu-west"
    assert n.normalize("azure", "japaneast") == "asia-ne"


def test_normalizer_still_maps_aws_regions():
    """The shared loader returns one table covering every provider already seeded."""
    n = load_region_normalizer()
    assert n.normalize("aws", "us-east-1") == "us-east"


def test_normalizer_rejects_unknown_azure_region():
    n = load_region_normalizer()
    with pytest.raises(KeyError, match="azure/centralus"):
        n.normalize("azure", "centralus")


def test_parse_unit_of_measure_hour():
    divisor, unit = parse_unit_of_measure("1 Hour")
    assert divisor == 1.0 and unit == "hrs"


def test_parse_unit_of_measure_hundred_hours():
    """Azure SQL meters frequently price per `100 Hours`; renderer expects per-hour."""
    divisor, unit = parse_unit_of_measure("100 Hours")
    assert divisor == 100.0 and unit == "hrs"


def test_parse_unit_of_measure_unknown_raises():
    with pytest.raises(ValueError, match="GB/Month"):
        parse_unit_of_measure("1 GB/Month")


def test_parse_storage_uom_per_month():
    from ingest.azure_common import parse_storage_uom
    assert parse_storage_uom("1 GB/Month") == (1.0, "gb-mo")
    assert parse_storage_uom("1/Month") == (1.0, "month")


def test_parse_storage_uom_unknown_raises():
    from ingest.azure_common import parse_storage_uom
    with pytest.raises(ValueError, match="unsupported"):
        parse_storage_uom("1 Hour")


def test_parse_request_uom_per_million():
    from ingest.azure_common import parse_request_uom
    assert parse_request_uom("1000000") == (1_000_000.0, "requests")
    assert parse_request_uom("10K") == (10_000.0, "requests")


def test_parse_request_uom_unknown_raises():
    from ingest.azure_common import parse_request_uom
    with pytest.raises(ValueError, match="unsupported"):
        parse_request_uom("1 Hour")
