# Offline & airgapped use

The binary is offline-first. The only network call is `sku update`, fetching pricing data from the CDN. Everything else — `price`, `search`, `compare`, `estimate`, `batch` — reads from local SQLite shards.

## Data directory

Default layout:

| OS | `$SKU_DATA_DIR` default |
|---|---|
| Linux | `$XDG_DATA_HOME/sku` (typically `~/.local/share/sku`) |
| macOS | `$HOME/Library/Application Support/sku` |
| Windows | `%APPDATA%\sku` |

Layout:

```
$SKU_DATA_DIR/
  manifest.json         # what's installed, last-updated, head versions
  openrouter.db
  aws-ec2.db
  aws-rds.db
  ...
```

## Seeding in an airgap

The shards are ordinary SQLite files. To install them on a host without CDN access:

1. On a connected host:

   ```bash
   sku update openrouter aws-ec2
   tar czf sku-bundle.tgz -C "$SKU_DATA_DIR" manifest.json openrouter.db aws-ec2.db
   ```

2. Transfer `sku-bundle.tgz` to the airgapped host.

3. On the airgapped host:

   ```bash
   mkdir -p "${SKU_DATA_DIR:-$HOME/.local/share/sku}"
   tar xzf sku-bundle.tgz -C "${SKU_DATA_DIR:-$HOME/.local/share/sku}"
   sku update --status --pretty
   ```

Expected: each shard listed with `installed: true` and the date it was fetched. Subsequent lookups work offline.

## Disabling network entirely

There is no opt-in "go online" flag. The *only* command that makes a network call is `sku update`. If your agent never runs `sku update`, the binary never talks to the network.

For defense-in-depth, block egress to the known CDN hosts:

```
jsdelivr.net
gh-release-assets (varies; use allow-list rather than deny)
```

## `--stale-ok` and `stale_error_days`

By default the binary warns when a shard hasn't been updated in 14 days. Suppress the warning:

```bash
sku --stale-ok aws ec2 price ...
SKU_STALE_OK=1 sku aws ec2 price ...
```

To **fail** (exit code 8) when shards get too stale, set a `stale_error_days` on a profile:

```bash
sku configure --profile ci \
  --set stale_warning_days=3 \
  --set stale_error_days=7
SKU_PROFILE=ci sku aws ec2 price ...
```

## Verifying a bundle

Bundle tarballs carry whatever signatures you add outside `sku` — it does not sign data shards itself. The CDN serves shards through jsDelivr; releases of the binary are cosign-signed separately (see [install.md](../install.md)).

## CI patterns

- **Deterministic**: turn off `auto_fetch`; fail fast if a shard is missing.

  ```bash
  sku configure --profile ci --set auto_fetch=false --set preset=price
  ```

- **Hermetic**: cache `$SKU_DATA_DIR` in your CI cache key; keyed on `sku version | jq -r '.version'` + a pinned data-release tag so the cache invalidates on either a binary upgrade or a deliberate data bump.

- **Audit**: `sku --verbose <cmd>` emits one JSON log line per operation on stderr (shard read, delta apply, preset applied). Pipe to your log sink.
