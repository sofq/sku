---
name: GCP arm verification recheck
about: Quarterly check whether GCP has added arm SKUs to Cloud Run / Functions
title: "chore(gcp): re-verify arm SKU status"
labels: ["chore", "data"]
---

Last verification: see `docs/coverage/gcp-arm-verification.md`.

Re-run Phase A of `docs/superpowers/plans/2026-04-22-gcp-arm64-serverless.md`:

1. Fetch fresh Cloud Run + Cloud Run Functions catalog snapshots
2. Grep for `(Arm)` / `arm64` in SKU descriptions
3. Update `docs/coverage/gcp-arm-verification.md` with findings + date
4. If arm SKUs now present, switch to Phase B branch 1

Schedule: quarterly.
