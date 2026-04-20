# m3b.3 Bootstrap Release Runbook

The `gcp-gce` and `gcp-cloud-sql` shards published by `internal/updater` resolve to
`https://github.com/sofq/sku/releases/download/data-bootstrap-gcp-{gce,cloud-sql}/<shard>.db.zst`.
Until the daily `data-daily.yml` pipeline lands in m3b.4, the maintainer
publishes one bootstrap release per shard manually. This document is the
checklist for that one-shot upload.

## Prerequisites

- Local main is even with origin and clean.
- `make gcp-shards` builds successfully (runs in CI on every PR via
  `make pipeline-test`; here we just need the `.db` artifacts).
- `gh` CLI is authenticated against `sofq/sku`.

## Steps

1. Build both shards from the committed fixtures:

   ```bash
   make gcp-shards
   ```

2. Compress with zstd and produce sha256 sidecars:

   ```bash
   for shard in gcp-gce gcp-cloud-sql; do
     zstd -19 -f dist/pipeline/$shard.db -o dist/pipeline/$shard.db.zst
     sha256sum dist/pipeline/$shard.db.zst | awk '{print $1}' \
       > dist/pipeline/$shard.db.zst.sha256
   done
   ```

3. Create one release per shard with the artifact + its sha256 sidecar:

   ```bash
   for shard in gcp-gce gcp-cloud-sql; do
     gh release create data-bootstrap-$shard \
       --title "Bootstrap shard: $shard" \
       --notes "One-shot bootstrap shard until daily pipeline (m3b.4) takes over." \
       dist/pipeline/$shard.db.zst dist/pipeline/$shard.db.zst.sha256
   done
   ```

4. Verify a fresh client can install both shards:

   ```bash
   rm -rf $HOME/.cache/sku
   ./bin/sku update gcp-gce
   ./bin/sku update gcp-cloud-sql
   ./bin/sku gcp gce       price --machine-type n1-standard-2 --region us-east1
   ./bin/sku gcp cloud-sql price --tier db-custom-2-7680 --region us-east1 \
                                 --engine postgres --deployment-option zonal
   ```

   Expected: each `update` reports a sha256-verified download; each `price`
   returns a JSON envelope with the seeded amount.

## Rollback

Either delete the release (`gh release delete data-bootstrap-gcp-gce --yes
--cleanup-tag`) or upload a corrected `.db.zst` + `.sha256` pair to the same
tag (`gh release upload data-bootstrap-gcp-gce dist/pipeline/gcp-gce.db.zst
--clobber`). Clients holding the previous version are unaffected until they
next run `sku update`.
