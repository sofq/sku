from __future__ import annotations

from pathlib import Path

from catalog_profiler.aws import scan_offer


def test_scan_ec2_fixture_reports_known_families():
    offer = Path(__file__).resolve().parent.parent / "testdata" / "aws_ec2" / "offer.json"
    rows = scan_offer(service_code="AmazonEC2", offer_paths=[offer])
    labels = {r.bucket_label for r in rows}
    assert "Compute Instance" in labels
    compute = next(r for r in rows if r.bucket_label == "Compute Instance")
    assert compute.sku_count > 0
    assert compute.covered_by_shard == "aws_ec2"
    assert "instanceType" in compute.attribute_keys
