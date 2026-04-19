---
name: CDN purge failed (automated)
about: jsDelivr cache purge failed after 3 retries
title: "jsDelivr cache purge failed for catalog <YYYY-MM-DD>"
labels: ["pipeline", "cdn-purge-failed"]
---

The daily data release succeeded but the jsDelivr purge API did not acknowledge after 3 retries with exponential backoff (2s, 8s, 30s).

**Impact:** Users pinned to `cdn.jsdelivr.net/gh/sofq/sku@latest/data/manifest.json` may see a stale manifest for up to 12h. The release-asset URL (`github.com/sofq/sku/releases/latest/download/manifest.json`) is unaffected.

**Action:** Run `scripts/ci/purge_jsdelivr.sh` from a maintainer machine with `gh` authenticated, or `curl --fail https://purge.jsdelivr.net/gh/sofq/sku@latest/data/manifest.json` directly. Close this issue once the purge URL returns 200.
