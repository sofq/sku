# Data-dispatch workflow runbook (legacy)

> **M-α note:** As of M-α, the daily pipeline is four workflows (three
> provider + one publish), fronted by the `data-dispatch.yml` dispatcher
> (formerly `data-daily.yml`) which preserves the existing end-to-end entry
> point. See [`data-workflows.md`](data-workflows.md) for the current
> topology. This runbook still covers legacy operational concerns shared
> across them — the **cron-related sections below are historical** (the
> dispatcher itself has no cron; provider-level crons live in
> `data-aws.yml`/`data-azure.yml`/`data-gcp.yml` and `data-publish.yml`).

Maintainer guide for `.github/workflows/data-dispatch.yml` — the manual
end-to-end entry point that publishes shard baselines, SQL deltas, and
`manifest.json` under the `data-YYYY.MM.DD` tag.

## Secrets + variables required

| Name | Kind | Purpose | How to create |
|---|---|---|---|
| `GCP_WIF_PROVIDER` | Repo **variable** | Workload Identity Federation provider resource name (e.g. `projects/NNN/locations/global/workloadIdentityPools/github/providers/sofq-sku`). Consumed by `google-github-actions/auth@v2` to impersonate the SA via short-lived OIDC. | See `docs/ops/validation.md` — same variable as `data-validate.yml`; one-time provisioning. |
| `GCP_VALIDATE_SA` | Repo **variable** | Service-account email impersonated by the workflow (`sku-validator@<project>.iam`; `roles/billing.viewer` on the billing account). | Same provisioning as above. |
| `GITHUB_TOKEN` | Secret | Auto-injected by Actions; permits release creation, `data` branch push, incident issues. | No action — GitHub provides. |

AWS pricing, Azure retail-prices, and OpenRouter are anonymous. GCP Cloud
Billing used to require a long-lived `GCP_BILLING_API_KEY` secret; it has
been retired in favor of the short-lived OIDC → WIF → SA token path shared
with `data-validate.yml`.

## First green run (bootstrap)

The cron schedule is **commented out** in `data-dispatch.yml` when it first lands.
Before enabling it, prove the end-to-end flow with two manual dispatches:

```bash
# 1. Dry run — discover + ingest + diff-package, but skip publish.
gh workflow run data-dispatch.yml \
  -F force_baseline=true

gh run watch   # wait for completion; confirm all jobs green
```

Inspect the `release-*` artifacts on the run page: each should contain a
`<shard>.db.zst` + `<shard>.db.zst.sha256` + `<shard>.meta.json`. Decompress
one and run `sqlite3 aws-ec2.db '.tables'` — expect `skus`, `prices`,
`resource_attrs`, `terms`, `health`, `metadata`.

```bash
# 2. Publish run — creates the first `data-YYYY.MM.DD` release.
gh workflow run data-dispatch.yml \
  -F dry_run=false \
  -F force_baseline=true

gh run watch
```

After this run:

- `gh release view data-$(date -u +%Y.%m.%d)` should list every shard's
  `.db.zst` + `.sha256`, a `manifest.json`, and `state.json`.
- The `data` branch must exist and contain `manifest.json` at its root.
- `curl https://cdn.jsdelivr.net/gh/sofq/sku@data/manifest.json`
  should return the new manifest (may take up to 30s after the purge step).

Only after both runs are green do you enable the cron schedule (next section).

## Enabling the cron schedule

Edit `.github/workflows/data-dispatch.yml` and uncomment the `- cron:` line under
`on.schedule`:

```yaml
on:
  schedule:
    - cron: "0 3 * * *"
```

Open a PR titled `ci: enable daily cron for data-dispatch.yml`, merge after
review. The first cron fire is at the next 03:00 UTC after merge.

## Disabling on incident

`gh workflow disable data-dispatch.yml` pauses the cron and blocks further
manual dispatch. Use when upstream pricing pages are degraded, when the
sanity check is failing consistently, or when the pipeline needs a pause for
remediation work.

Re-enable with `gh workflow enable data-dispatch.yml`. The next run picks up
whatever has changed since the last green release via the discover state.

## Manually republishing today's catalog

If a scheduled run has already completed but you need to re-publish (e.g. to
pick up a fixed ingest module):

1. `gh workflow run data-dispatch.yml -F dry_run=false -F force_baseline=true -F replace_existing_release=true`
2. Watch the run; the publish step replaces `data-$(date -u +%Y.%m.%d)` before
   recreating `data-latest`.

`force_baseline=true` rebuilds every shard baseline, bypassing the discover
"nothing changed" short-circuit and resetting every client's delta chain
onto today's baseline.

## Recovering a purge-failed state

When the CDN purge call exhausts its retries, the workflow auto-files an
issue tagged `cdn-purge-failed` and exits non-zero. The release itself is
already live — clients on the GitHub URL are unaffected; only jsDelivr
callers may see a stale manifest for up to 12h.

To clear:

```bash
curl --fail --silent --show-error \
  https://purge.jsdelivr.net/gh/sofq/sku@data/manifest.json

gh issue close <issue-number> --comment "Purge verified; manifest fresh on jsDelivr."
```

## Signals that something is wrong

- A `diff-package` matrix leg failing `sanity_check` → typically a sudden
  row-count cliff. Inspect the ingest job's `<shard>.rows.jsonl` artifact;
  compare to yesterday's release. Upstream may have renamed/retired a SKU.
- `discover` job reports every shard errored → upstream site (pricing feed)
  is probably down. Workflow exits 2; no release published. Re-run once
  upstream recovers.
- `publish` job fails with `release already exists` → today's release already
  exists. Dry-run for testing, or re-run with `replace_existing_release=true`
  when you intend to replace the public release.

## Related files

- `.github/workflows/data-dispatch.yml` — the workflow itself
- `.github/actions/setup-pipeline/action.yml` — composite Python setup
- `scripts/ci/ingest_shard.sh` — per-shard live fetch + ingest
- `scripts/ci/diff_package_shard.sh` — sanity + delta + optional baseline
- `scripts/ci/build_manifest.sh` — `manifest.json` assembly
- `scripts/ci/push_data_branch.sh` — jsDelivr mirror
- `scripts/ci/purge_jsdelivr.sh` — CDN purge with retry + auto-issue
- `pipeline/package/build_delta.py` — SQL.gz delta builder
- `pipeline/package/build_manifest.py` — manifest shape
- `pipeline/package/sanity_check.py` — drift guard
