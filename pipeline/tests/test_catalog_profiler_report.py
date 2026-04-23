from __future__ import annotations

from catalog_profiler.report import render_markdown
from catalog_profiler.types import CoverageRow


def test_render_markdown_matches_golden():
    rows = [
        CoverageRow(
            bucket_key="Compute Instance",
            bucket_label="Compute Instance",
            sku_count=35098,
            attribute_keys=["instanceType", "regionCode", "operatingSystem"],
            sample_sku_ids=["ABCD1234", "EFGH5678"],
            covered_by_shard="aws_ec2",
            coverage_ratio=1.0,
        ),
        CoverageRow(
            bucket_key="Database Instance",
            bucket_label="Database Instance",
            sku_count=8000,
            attribute_keys=["databaseEngine", "instanceType"],
            sample_sku_ids=["RDS1", "RDS2"],
            covered_by_shard="aws_rds",
            coverage_ratio=0.5,
        ),
        CoverageRow(
            bucket_key="Provisioned IOPS",
            bucket_label="Provisioned IOPS",
            sku_count=420,
            attribute_keys=["iopsQuantity"],
            sample_sku_ids=["IOPS1"],
            covered_by_shard=None,
            coverage_ratio=0.0,
        ),
    ]
    md = render_markdown(cloud="aws", rows=rows, as_of="2026-04-22")
    assert "# AWS pricing-feed coverage" in md
    assert "_Generated 2026-04-22_" in md
    assert md.index("Compute Instance") < md.index("Database Instance")
    assert md.index("Database Instance") < md.index("Provisioned IOPS")
    assert "UNCOVERED" in md
    assert "aws_ec2" in md
    # Thousand-separator formatting
    assert "35,098" in md
    # Coverage percentage
    assert "100%" in md
    assert "50%" in md
    # Em-dash for uncovered rows (Coverage column)
    assert "—" in md


def test_render_markdown_truncates_attrs():
    row = CoverageRow(
        bucket_key="k",
        bucket_label="Wide",
        sku_count=1,
        attribute_keys=["a", "b", "c", "d", "e", "f", "g", "h"],
        sample_sku_ids=[],
        covered_by_shard=None,
        coverage_ratio=0.0,
    )
    md = render_markdown(cloud="aws", rows=[row], as_of="2026-04-22")
    assert "+2 more" in md
