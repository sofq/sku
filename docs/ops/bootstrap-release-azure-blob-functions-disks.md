# Bootstrap release: azure-blob + azure-functions + azure-disks (m3b.2)

## Prerequisites

- `gh` authenticated against `sofq/sku` with release-write scope.
- `make azure-blob-shard azure-functions-shard azure-disks-shard` completed — three
  `.db` files present under `dist/pipeline/`.
- `zstd(1)` on PATH.

## Steps

For each shard in `(azure-blob, azure-functions, azure-disks)`:

1. Compress the shard:

   ```bash
   zstd -19 --rm dist/pipeline/<shard>.db -o dist/pipeline/<shard>.db.zst
   sha256sum dist/pipeline/<shard>.db.zst | awk '{print $1}' > dist/pipeline/<shard>.db.zst.sha256
   ```

2. Cut the bootstrap release tag:

   ```bash
   gh release create data-bootstrap-<shard> \
     --title "data-bootstrap-<shard>" \
     --notes "Initial bootstrap of the <shard> shard for m3b.2." \
     dist/pipeline/<shard>.db.zst \
     dist/pipeline/<shard>.db.zst.sha256
   ```

3. Verify the URL resolves:

   ```bash
   curl -sI https://github.com/sofq/sku/releases/download/data-bootstrap-<shard>/<shard>.db.zst \
     | head -n 1
   ```

   Expected: `HTTP/2 302` → `HTTP/2 200` on the redirect target.

4. Smoke the end-to-end client flow against a scratch data dir:

   ```bash
   export SKU_DATA_DIR=$(mktemp -d)
   ./bin/sku update <shard>
   case <shard> in
     azure-blob)      ./bin/sku azure blob      price --tier hot          --region eastus ;;
     azure-functions) ./bin/sku azure functions price --architecture x86_64 --region eastus ;;
     azure-disks)     ./bin/sku azure disks     price --disk-type standard-ssd --region eastus ;;
   esac
   ```

   Expected: one JSON envelope with `provider:"azure"`.

## Rollback

If the bootstrap release is discovered to be wrong (mis-built shard, missing
metadata row, bad `currency`), do NOT edit the release in place — `sku update`
clients cache by sha256 and will skip the fixed asset.

Instead:

1. Delete the tag: `gh release delete data-bootstrap-<shard> --yes`.
2. Rebuild: `make <shard-target>`.
3. Re-run Steps 1–4.

Releases after the bootstrap (normal daily releases) follow the `data-YYYY.MM.DD`
convention and are cut by `data-daily.yml`; this runbook covers the one-time
bootstrap only.
