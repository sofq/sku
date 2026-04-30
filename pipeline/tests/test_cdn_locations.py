"""Tests for pipeline/normalize/cdn_locations.py."""

from normalize.cdn_locations import LOCATION_MAP, lookup


def test_location_map_exhaustiveness():
    # Any edge-location string we know about must map to a region that
    # regions.yaml would accept. Guards against silent drift if someone
    # adds to LOCATION_MAP without extending regions.yaml.
    from ingest.aws_common import load_region_normalizer
    norm = load_region_normalizer()
    for region in LOCATION_MAP.values():
        assert norm.normalize("aws", region), f"{region} not in regions.yaml"


def test_lookup_known_location():
    assert lookup("United States") == "us-east-1"
    assert lookup("South America") == "sa-east-1"
    assert lookup("India") == "ap-south-1"


def test_lookup_unknown_returns_none():
    assert lookup("Antarctica") is None
    assert lookup("") is None


def test_location_map_fans_out_to_p1_regions():
    values = set(LOCATION_MAP.values())
    assert "sa-east-1" in values, "South America must fan out to sa-east-1"
    assert "ap-southeast-2" in values, "Australia must fan out to ap-southeast-2"
    assert "ap-south-1" in values, "India must fan out to ap-south-1"
    assert "ca-central-1" in values, "Canada must fan out to ca-central-1"
    assert "me-central-1" in values, "Middle East must fan out to me-central-1"
    assert "af-south-1" in values, "South Africa must fan out to af-south-1"
