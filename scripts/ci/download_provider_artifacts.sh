#!/usr/bin/env bash
# Downloads the latest `shards-<provider>-<version>` artifact produced by
# the data-<provider>.yml workflow whose most recent run completed
# successfully. Called with $1 = provider name (aws|azure|gcp|openrouter) and
# $2 = expected catalog version (YYYY.MM.DD).
#
# IMPORTANT: this script flattens shard artifacts (*.db.zst, *.sql.gz,
# manifest fragments) into dist/pipeline/release/ but leaves each
# provider's state.json untouched inside dist/pipeline/${provider}-in/.
# The publish workflow merges the three state.json files separately
# via scripts/ci/merge_discover_state.py.
set -euo pipefail

provider="${1:?provider required}"
expected_version="${2:?catalog version required}"
workflow="data-${provider}.yml"

# Find recent successful runs of the provider workflow. Dry runs are successful
# but intentionally do not upload shard artifacts, so try a small window until
# we find the artifact for the exact catalog version publish is building.
mapfile -t run_ids < <(gh run list --workflow "$workflow" --status success --limit 20 \
  --json databaseId --jq '.[].databaseId')

if [ "${#run_ids[@]}" -eq 0 ]; then
  echo "no successful $workflow run found — skipping $provider" >&2
  exit 0
fi

mkdir -p "dist/pipeline/${provider}-in"

for run_id in "${run_ids[@]}"; do
  if gh run download "$run_id" \
      --pattern "shards-${provider}-${expected_version}" \
      --dir "dist/pipeline/${provider}-in"; then
    break
  fi
  echo "no shards-${provider}-${expected_version} artifact on run $run_id" >&2
done

if ! find "dist/pipeline/${provider}-in" -type f | grep -q .; then
  echo "no shards-${provider}-${expected_version} artifact found in recent $workflow runs" >&2
  exit 0
fi

# Flatten shard artifacts into dist/pipeline/release/ — but NOT state.json
# (kept in place for merge_discover_state.py).
find "dist/pipeline/${provider}-in" -type f \
  ! -name 'state.json' \
  -exec cp -v {} dist/pipeline/release/ \;
