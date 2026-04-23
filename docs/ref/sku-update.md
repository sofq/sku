# sku update

Downloads and installs (or incrementally updates) a pricing shard.

## Synopsis

```
sku update <shard> [--channel stable|daily] [global flags]
```

## Positional arguments

| Argument | Description |
|----------|-------------|
| `<shard>` | Shard name, e.g. `aws-ec2`, `openrouter`, `azure-postgres` |

Supported shards: `openrouter`, `aws-ec2`, `aws-rds`, `aws-s3`, `aws-lambda`, `aws-ebs`, `aws-dynamodb`, `aws-cloudfront`, `azure-vm`, `azure-sql`, `azure-blob`, `azure-functions`, `azure-disks`, `azure-postgres`, `azure-mysql`, `azure-mariadb`, `gcp-gce`, `gcp-cloud-sql`, `gcp-gcs`, `gcp-run`, `gcp-functions`.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--channel` | `""` (inherits config) | Update channel: `stable` or `daily`. |

## Channel behaviour

### `stable` (default)

Always downloads the latest full baseline shard (`.db.zst`). Most established shards use their bootstrap release URL for fresh installs; newer shards may resolve the same baseline through the manifest-backed data release.

### `daily`

1. Fetches `manifest.json` from the primary URL (or the jsDelivr CDN fallback).
2. Reads the local shard's `catalog_version` from its `metadata` table.
3. If the manifest returns HTTP 304 (Not Modified / ETag match) — no-op; exits 0.
4. Filters the manifest's delta list to entries with `from >= local_version`.
5. Applies the delta chain in a single `BEGIN IMMEDIATE` transaction; verifies each delta's SHA-256 before applying.
6. **Fallback to baseline**: if the chain is too long (`max_delta_chain` exceeded), starts at the wrong version, or the shard file is missing — falls back to a full baseline download automatically.

### `max_delta_chain`

Maximum number of deltas applied in one run. Default: **20**. There is no config key for this yet; it is hard-coded in the binary.

## Update channel precedence

```
--channel flag  >  SKU_UPDATE_CHANNEL env  >  profile.channel config  >  "stable"
```

## Configuration

### Environment variables

| Variable | Description |
|----------|-------------|
| `SKU_UPDATE_CHANNEL` | Set the default channel without a flag (`stable` or `daily`). |
| `SKU_UPDATE_BASE_URL` | Override the asset base URL for all shards (used in tests). |
| `SKU_UPDATE_BASE_URL_<SHARD>` | Per-shard base URL override (hyphens become underscores, e.g. `SKU_UPDATE_BASE_URL_AWS_EC2`). |

### Config file

In `~/.config/sku/config.yaml` (or platform equivalent):

```yaml
profiles:
  default:
    channel: daily   # "stable" | "daily"
```

The key is `update.channel` conceptually; in the YAML structure it lives under the profile as `channel:`.

## Manifest URL resolution

Primary: `$SKU_UPDATE_BASE_URL/manifest.json` (if `SKU_UPDATE_BASE_URL` is set), otherwise `https://github.com/sofq/sku/releases/download/data-latest/manifest.json`.

Fallback: `https://cdn.jsdelivr.net/gh/sofq/sku@data/manifest.json` (jsDelivr CDN mirror of the `data` branch).

A 5xx error on the primary causes one automatic retry against the fallback. Both failing → non-zero exit (code 7, server error).

## Exit codes

| Code | Value | Meaning |
|------|-------|---------|
| 0 | success | Shard installed or already up to date. |
| 4 | validation | Unknown `--channel` value; unsupported shard name; shard schema version mismatch. |
| 6 | conflict | SHA-256 mismatch on downloaded asset; another `sku update` holds the advisory lock. |
| 7 | server | Upstream HTTP error (both primary and fallback failed). |

## Verbose log events

With `--verbose`, structured log lines are emitted to stderr:

| Event | When |
|-------|------|
| `update.fetch` | Before fetching the manifest or baseline. |
| `update.304` | Manifest server returned 304 — nothing to do. |
| `update.delta-applied` | After each individual delta is applied (includes `from`/`to` versions). |
| `update.fallback-to-baseline` | Chain fallback triggered (chain-too-long or starts-elsewhere). |
| `update.baseline-installed` | Full baseline downloaded and installed. |

## Examples

```bash
# Fresh install (stable channel — downloads full baseline)
sku update aws-ec2

# Daily channel — tries delta chain first
sku update aws-ec2 --channel daily

# Override via environment
SKU_UPDATE_CHANNEL=daily sku update openrouter

# Use a local test server for CI
SKU_UPDATE_BASE_URL=http://localhost:9000 sku update aws-ec2
```
