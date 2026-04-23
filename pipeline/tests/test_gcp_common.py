"""Unit tests for GCP ingest helpers."""

import pytest

from ingest.gcp_common import (
    load_region_normalizer,
    parse_unit_price,
    parse_usage_unit,
)


def test_parse_unit_price_whole_dollars():
    assert parse_unit_price(units="2", nanos=0) == 2.0


def test_parse_unit_price_sub_dollar_nanos():
    # $0.031611000 -> 31_611_000 nanos
    assert parse_unit_price(units="0", nanos=31_611_000) == pytest.approx(0.031611)


def test_parse_unit_price_combined():
    # $1.50 = units=1 nanos=500_000_000
    assert parse_unit_price(units="1", nanos=500_000_000) == pytest.approx(1.50)


def test_parse_usage_unit_hour():
    assert parse_usage_unit("h") == (1.0, "hrs")


def test_parse_usage_unit_gib_hour():
    assert parse_usage_unit("GiBy.h") == (1.0, "gb-hr")


def test_parse_usage_unit_gib_month():
    assert parse_usage_unit("GiBy.mo") == (1.0, "gb-mo")


def test_parse_usage_unit_unknown_raises():
    with pytest.raises(ValueError, match="unsupported usageUnit"):
        parse_usage_unit("parsecs/fortnight")


def test_region_normalizer_covers_gcp():
    n = load_region_normalizer()
    assert n.normalize("gcp", "us-east1") == "us-east"
    assert n.normalize("gcp", "europe-west1") == "eu-west"


def test_region_normalizer_unknown_gcp_region_raises():
    n = load_region_normalizer()
    with pytest.raises(KeyError, match="gcp/me-central1"):
        n.normalize("gcp", "me-central1")
