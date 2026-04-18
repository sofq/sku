"""Unit tests for shared AWS ingest helpers."""

import pytest

from ingest.aws_common import RegionNormalizer, load_region_normalizer


def test_normalizer_maps_aws_regions():
    n = load_region_normalizer()
    assert n.normalize("aws", "us-east-1") == "us-east"
    assert n.normalize("aws", "us-east-2") == "us-east"
    assert n.normalize("aws", "us-west-2") == "us-west"
    assert n.normalize("aws", "eu-west-1") == "eu-west"
    assert n.normalize("aws", "ap-northeast-1") == "asia-ne"


def test_normalizer_rejects_unknown_region():
    n = load_region_normalizer()
    with pytest.raises(KeyError, match="aws/ap-south-9"):
        n.normalize("aws", "ap-south-9")


def test_normalizer_wraps_table_for_testing():
    n = RegionNormalizer({("aws", "us-east-1"): "us-east"})
    assert n.normalize("aws", "us-east-1") == "us-east"
