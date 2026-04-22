"""Unit tests for shared AWS ingest helpers."""

from __future__ import annotations

import time
from unittest.mock import patch

import pytest

from ingest import aws_common
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


def test_fetch_offer_regions_stripped_is_parallel(tmp_path):
    """8 regions × 200 ms artificial latency should complete in <1s, not 1.6s+."""
    import json as _json

    regions = [f"r{i}" for i in range(8)]

    def fake_stream_download(url, target, *, session=None, retries=3):
        # Serve a synthetic region_index.json containing all test regions.
        target.write_text(_json.dumps({
            "regions": {
                r: {"currentVersionUrl": f"/offers/v1.0/aws/AmazonEC2/current/{r}/index.json"}
                for r in regions
            }
        }))

    def fake_fetch_one(shard, region, target_dir, *, rel_url, session, retries):
        time.sleep(0.2)
        out = target_dir / f"{aws_common.shared_offer_basename(shard)}-{region}.json"
        out.write_text('{"products":{}, "terms":{"OnDemand":{}}}')
        return out

    with patch.object(aws_common, "_stream_download", side_effect=fake_stream_download), \
         patch.object(aws_common, "_fetch_one_region_stripped", side_effect=fake_fetch_one):
        start = time.perf_counter()
        aws_common.fetch_offer_regions_stripped(
            "aws_ec2", tmp_path, regions=regions,
        )
        elapsed = time.perf_counter() - start

    # Lower bound: if this is near-zero, _fetch_one_region_stripped was never
    # called (vacuous pass), meaning the test logic is broken.
    assert elapsed >= 0.15, f"mock not called? elapsed={elapsed:.3f}s"
    assert elapsed < 1.0, f"expected parallel dispatch, took {elapsed:.2f}s"
