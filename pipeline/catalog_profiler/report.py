"""Markdown renderer for CoverageRow lists.

Emits a deterministic document so diffs reflect upstream feed changes,
not ordering noise. Rows sorted by sku_count desc, then bucket_label asc.
UNCOVERED buckets are surfaced via the Status column.
"""

from __future__ import annotations

from collections.abc import Sequence

from .types import CoverageRow


def render_markdown(*, cloud: str, rows: Sequence[CoverageRow], as_of: str) -> str:
    sorted_rows = sorted(rows, key=lambda r: (-r.sku_count, r.bucket_label))
    title = {"aws": "AWS", "azure": "Azure", "gcp": "GCP"}.get(cloud, cloud.upper())
    lines: list[str] = [
        f"# {title} pricing-feed coverage",
        "",
        f"_Generated {as_of}_",
        "",
        "Raw SKU counts per bucket, and which `sku` shard (if any) ingests them.",
        "Generated weekly by `make profile` against live feeds.",
        "",
        "| Bucket | SKUs | Covered by | Coverage | Attrs fingerprint | Sample SKU ids |",
        "| --- | ---: | --- | ---: | --- | --- |",
    ]
    for r in sorted_rows:
        shard = r.covered_by_shard or "**UNCOVERED**"
        coverage = "—" if r.covered_by_shard is None else f"{int(round(r.coverage_ratio*100))}%"
        attrs = ", ".join(r.attribute_keys[:6])
        if len(r.attribute_keys) > 6:
            attrs += f", +{len(r.attribute_keys)-6} more"
        samples = ", ".join(f"`{s}`" for s in r.sample_sku_ids[:3])
        lines.append(
            f"| {r.bucket_label} | {r.sku_count:,} | {shard} | {coverage} | {attrs} | {samples} |"
        )
    lines.append("")
    return "\n".join(lines)
