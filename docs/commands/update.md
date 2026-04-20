# `sku update`

Fetch or refresh one or more shards from the CDN. The binary itself is offline; all data freshness flows through this command.

## Synopsis

```
sku update [<shard>...] [--status] [--channel <name>] [--force] [flags]
```

`<shard>` is one of the names emitted by `sku schema --list-shards`. With no arguments, all installed shards are refreshed.

## Flags

| Flag | Meaning |
|---|---|
| `--status` | Print per-shard freshness JSON; no fetch. |
| `--channel` | `stable` (default) or `unstable`. |
| `--force` | Re-apply deltas even if local state matches the manifest head. |

## Examples

```bash
sku update openrouter aws-ec2
sku update                   # refresh all installed
sku update --status --pretty
```

## Exit codes

`0`, `4` (binary too old for an advertised shard), `5` (rate-limited by CDN; retry-after in envelope), `6` (state conflict), `7` (CDN upstream error), `8` (catalog older than `SKU_STALE_ERROR_DAYS`).

See [`../guides/offline-use.md`](../guides/offline-use.md) for airgapped workflows.
