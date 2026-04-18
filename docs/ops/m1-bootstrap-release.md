# M1 bootstrap OpenRouter release

Purpose: one-shot upload of a manually-built `openrouter.db.zst` to the
`data-bootstrap-openrouter` GitHub release so `sku update openrouter` has
something to download until the daily data pipeline goes live in M3a.

Run this once per M1, and again if the shard schema changes before M3a.

## Prereqs

- `gh` CLI authenticated against `github.com/sofq/sku` with `repo` scope.
- Python venv set up (`make -C pipeline setup`).
- OpenRouter reachable from the build host (no auth required for `/api/v1/*`).

## Steps

1. Build a fresh live shard (not from fixtures):

```bash
cd pipeline
SKU_FIXED_OBSERVED_AT=$(date +%s) .venv/bin/python -m ingest.openrouter \
  --out ../dist/pipeline/openrouter.rows.jsonl \
  --generated-at $(date -u +%Y-%m-%dT%H:%M:%SZ)
.venv/bin/python -m package.build_shard \
  --rows ../dist/pipeline/openrouter.rows.jsonl \
  --shard openrouter \
  --out ../dist/pipeline/openrouter.db \
  --catalog-version $(date -u +%Y.%m.%d)
cd ..
```

2. Compress + checksum:

```bash
zstd -19 dist/pipeline/openrouter.db -o dist/pipeline/openrouter.db.zst
cd dist/pipeline
sha256sum openrouter.db.zst > openrouter.db.zst.sha256
cd -
```

3. Upload:

```bash
gh release create data-bootstrap-openrouter \
  --title "M1 bootstrap OpenRouter shard" \
  --notes "Bootstrap shard for sku llm price during M1. Replaced by daily pipeline in M3a." \
  dist/pipeline/openrouter.db.zst \
  dist/pipeline/openrouter.db.zst.sha256
```

If the release already exists:

```bash
gh release upload data-bootstrap-openrouter \
  dist/pipeline/openrouter.db.zst dist/pipeline/openrouter.db.zst.sha256 \
  --clobber
```

4. Verify from a clean env:

```bash
tmp=$(mktemp -d)
SKU_DATA_DIR="$tmp" ./bin/sku update openrouter
SKU_DATA_DIR="$tmp" ./bin/sku llm price --model anthropic/claude-opus-4.6
```

Expected: JSON lines on stdout.

## Rollback

If a bootstrap upload turns out to be bad, delete the assets on the release and re-upload the previous good build:

```bash
gh release delete-asset data-bootstrap-openrouter openrouter.db.zst openrouter.db.zst.sha256
# re-run step 3 with the known-good local artifacts
```

Clients fail closed with exit code 6 (`conflict` — sha256 mismatch) until a new upload lands.
