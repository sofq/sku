# Data workflow topology (M-α)

As of M-α, the daily data pipeline is split into four workflows plus a
dispatcher. This file describes the topology, scheduling, failure semantics,
and manual re-dispatch procedures.

## Topology

```
                  ┌─────────────────────────────────────────┐
                  │   gh workflow run data-dispatch.yml      │
                  │   (manual dispatch — waits end-to-end)   │
                  └────────────┬────────────────────────────┘
                               │ dispatches
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
        data-aws.yml    data-azure.yml   data-gcp.yml
        (AWS shards)   (Azure shards)  (GCP + OpenRouter)
              │                │                │
              └────────────────┼────────────────┘
                               │ data-dispatch.yml polls all three,
                               │ then dispatches (if: always())
                               ▼
                       data-publish.yml
                 (download artifacts → merge state →
                  build manifest → create release →
                  push data branch → purge CDN)
```

`data-publish.yml` also runs on its own **04:30 UTC cron** as an unattended
nightly fallback, downloading whatever provider artifacts the three earlier
crons already produced.

## Schedule

| Workflow | Trigger |
|---|---|
| `data-aws.yml` | cron 03:00 UTC daily |
| `data-azure.yml` | cron 03:15 UTC daily |
| `data-gcp.yml` | cron 03:30 UTC daily |
| `data-publish.yml` | cron 04:30 UTC daily (nightly fallback) |
| `data-dispatch.yml` | manual dispatch only (no cron) |

The maintainer-initiated `gh workflow run data-dispatch.yml` path waits for the
three providers and then fires publish end-to-end. The 04:30 publish cron is
the unattended nightly fallback — it runs regardless of how the provider
workflows completed.

## Failure isolation

If one provider workflow fails end-to-end, publish still runs. The
`data-publish.yml` `download-artifacts` job uses
`scripts/ci/download_provider_artifacts.sh` which tolerates a missing provider
artifact (`|| echo "no artifact"`) and falls back to yesterday's release for
that provider's shards. The `state.json` merge step (driven by
`scripts/ci/merge_discover_state.py`) unions whichever per-provider
`state.json` files were produced; a missing file is silently skipped.

Result: a failed provider leaves that provider's shards at yesterday's
versions. The release still publishes on schedule with the other providers'
fresh data.

## Manual re-dispatch

```bash
# Retry a single provider (e.g. after an upstream outage recovers):
gh workflow run data-aws.yml
gh workflow run data-azure.yml
gh workflow run data-gcp.yml

# Bypass AWS ETag fast path (force full re-ingest regardless of 304s):
gh workflow run data-aws.yml -f force_full_ingest=true

# Dry-run publish against last-good provider artifacts (no release writes):
gh workflow run data-publish.yml

# Re-publish against last-good provider artifacts (replace today's release):
gh workflow run data-publish.yml -f dry_run=false -f replace_existing_release=true

# Full end-to-end dry run with wait:
gh workflow run data-dispatch.yml

# Full end-to-end pipeline with wait and publish:
gh workflow run data-dispatch.yml -f dry_run=false
```

## Verifying codegen outputs

The `test-codegen-clean` CI job asserts that generated files (`package/budgets.py`,
`discover/_shards_gen.py`) are committed and up-to-date. To reproduce locally:

```bash
make generate      # re-runs pipeline/tools/gen_python.py from pipeline/shards/*.yaml
git diff           # must be empty; any diff means uncommitted generated output
```

If `git diff` is non-empty, stage and commit the generated files before opening a PR.

## Related files

- `.github/workflows/data-aws.yml` — AWS discover / ingest / diff_package / bundle
- `.github/workflows/data-azure.yml` — Azure discover / ingest / diff_package / bundle
- `.github/workflows/data-gcp.yml` — GCP + OpenRouter discover / ingest / diff_package / bundle
- `.github/workflows/data-publish.yml` — artifact download / manifest / release / CDN
- `.github/workflows/data-dispatch.yml` — thin dispatcher (manual dispatch entry point)
- `scripts/ci/download_provider_artifacts.sh` — per-provider artifact download
- `scripts/ci/merge_discover_state.py` — union of per-provider discover state
- `docs/ops/data-dispatch-runbook.md` — legacy operational concerns (secrets, bootstrap, incident recovery)
