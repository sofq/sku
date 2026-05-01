"""Canonical CDN edge-location → AWS region mapping.

Shared by aws_cloudfront (ingest) and future Front Door / Cloud CDN shards
so that all three can fan out to the same R1-region set without diverging.

Every value in LOCATION_MAP must also exist in pipeline/normalize/regions.yaml;
test_cdn_locations.test_location_map_exhaustiveness enforces this.
"""

from __future__ import annotations

# Upstream edge-location strings → canonical AWS region code.
LOCATION_MAP: dict[str, str] = {
    # Legacy fixture strings (kept for fixture compatibility).
    "United States, Mexico, & Canada":         "us-east-1",
    "Europe, Israel":                          "eu-west-1",
    "Asia Pacific (including Japan & Taiwan)": "ap-northeast-1",
    # Current upstream fromLocation strings (as of 2025-Q4).
    "United States":  "us-east-1",
    "Canada":         "ca-central-1",
    "Europe":         "eu-west-1",
    "Asia Pacific":   "ap-northeast-1",
    "Japan":          "ap-northeast-1",
    "Australia":      "ap-southeast-2",
    "India":          "ap-south-1",
    "South America":  "sa-east-1",
    "Middle East":    "me-central-1",
    "South Africa":   "af-south-1",
    # Note: upstream also ships a fromLocation="Any" SKU carrying the
    # CloudFront free-tier (1 TB/mo @ $0). It is intentionally NOT mapped
    # — the ingestor drops "Any" rows because they would overwrite real
    # regional pricing.
}


def lookup(location_raw: str) -> str | None:
    """Return the canonical region for *location_raw*, or None if unknown."""
    return LOCATION_MAP.get(location_raw)
