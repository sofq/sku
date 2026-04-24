#!/usr/bin/env bash
# Downloads the latest `shards-<provider>-<version>` artifact produced by
# the data-<provider>.yml workflow whose most recent run completed
# successfully. Called with $1 = provider name (aws|azure|gcp).
#
# IMPORTANT: this script flattens shard artifacts (*.db.zst, *.sql.gz,
# manifest fragments) into dist/pipeline/release/ but leaves each
# provider's state.json untouched inside dist/pipeline/${provider}-in/.
# The publish workflow merges the three state.json files separately
# via scripts/ci/merge_discover_state.py.
set -euo pipefail

provider="${1:?provider required}"
workflow="data-${provider}.yml"

# Find the most recent successful run of the provider workflow.
run_id=$(gh run list --workflow "$workflow" --status success --limit 1 \
  --json databaseId --jq '.[0].databaseId')

if [ -z "$run_id" ] || [ "$run_id" = "null" ]; then
  echo "no successful $workflow run found — skipping $provider" >&2
  exit 0
fi

mkdir -p "dist/pipeline/${provider}-in"

# Artifact name varies by date; glob it.
gh run download "$run_id" \
  --pattern "shards-${provider}-*" \
  --dir "dist/pipeline/${provider}-in" \
  || { echo "no shards artifact on run $run_id for $provider" >&2; exit 0; }

# Flatten shard artifacts into dist/pipeline/release/ — but NOT state.json
# (kept in place for merge_discover_state.py).
find "dist/pipeline/${provider}-in" -type f \
  ! -name 'state.json' \
  -exec cp -v {} dist/pipeline/release/ \;
