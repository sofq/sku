---
name: Catalog drift detected (automated)
about: Weekly validate workflow detected >1% price drift in a catalog shard
title: "Catalog drift in <shard> (catalog <YYYY-MM-DD>)"
labels: ["pipeline", "catalog-drift"]
---

The weekly `data-validate.yml` workflow sampled prices from the shard and found one or more SKUs where the catalog price differs from the upstream provider API by more than 1%.

**Impact:** The catalog may contain stale prices for the flagged SKUs. Agents reading from this shard may receive incorrect pricing data.

**Action:**

1. Open the workflow run linked below and download the `validate-<shard>` artifact for the full drift report.
2. Inspect `drift_records` in the report JSON to identify the specific SKUs and delta percentages.
3. Determine whether the drift is a real upstream price change or a transient API glitch.
   - **Real price change:** Temporarily disable `data-daily.yml`, investigate the ingest module for the affected shard, fix the parsing or mapping, re-ingest, and re-publish.
   - **API glitch:** Re-run `data-validate.yml` for the affected shard via `workflow_dispatch`. Close this issue if the re-run passes.
4. If the `vantage_drift` field in the report is non-empty, the EC2 shard also diverges from the vantage-sh/ec2instances.info reference data — follow the same triage steps.

Close this issue once the validate workflow passes cleanly for the affected shard.
