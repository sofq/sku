#!/usr/bin/env bash
# Run sanity check + build delta + optional baseline zst for one shard.
#
# Inputs (env):
#   SHARD              — underscored shard id (matrix value)
#   CATALOG_VERSION    — today's catalog tag
#   PREVIOUS_VERSION   — prior release's catalog tag (may be empty on first run)
#   BASELINE_REBUILD   — "true" on day-of-month ∈ {1,15} or force_baseline
#
# Inputs (on disk):
#   dist/pipeline/today/<dashed>.db        — today's shard (from ingest job artifact)
#   dist/pipeline/prev/<dashed>.db         — previous shard, decompressed by caller
#
# Output:
#   dist/pipeline/release/<dashed>-<prev>-to-<curr>.sql.gz  (delta; when previous exists)
#   dist/pipeline/release/<dashed>.db.zst                   (baseline; when rebuild)
#   dist/pipeline/release/<dashed>.db.zst.sha256            (baseline sha)
#   dist/pipeline/release/<dashed>.meta.json                (manifest sidecar)
set -euo pipefail

: "${SHARD:?SHARD env var required}"
: "${CATALOG_VERSION:?CATALOG_VERSION env var required}"
BASELINE_REBUILD="${BASELINE_REBUILD:-false}"
PREVIOUS_VERSION="${PREVIOUS_VERSION:-}"

public_shard=$(echo "$SHARD" | tr _ -)

today_db="dist/pipeline/today/${public_shard}.db"
prev_db="dist/pipeline/prev/${public_shard}.db"
release_dir="dist/pipeline/release"
mkdir -p "$release_dir"

test -f "$today_db" || { echo "missing today's shard at $today_db" >&2; exit 2; }

has_previous="false"
if [ -f "$prev_db" ]; then
  has_previous="true"
fi

# 1) Sanity check — row-count drift + schema + FK integrity.
python -m package.sanity_check \
  --shard "$public_shard" \
  --shard-db "$today_db" \
  $(if [ "$has_previous" = "true" ]; then echo "--previous-db $prev_db"; fi)

# 2) Delta — only when we have a previous baseline to chain from AND we're
#    not forcing a full baseline rebuild (rebuild invalidates old chain).
has_baseline="false"
delta_from=""
delta_to="$CATALOG_VERSION"

if [ "$BASELINE_REBUILD" = "true" ] || [ "$has_previous" != "true" ]; then
  has_baseline="true"
  zstd -19 --long=27 -T0 -q -f -o "$release_dir/${public_shard}.db.zst" "$today_db"
  (cd "$release_dir" && sha256sum "${public_shard}.db.zst" > "${public_shard}.db.zst.sha256")
fi

if [ "$has_previous" = "true" ] && [ "$BASELINE_REBUILD" != "true" ]; then
  delta_from="$PREVIOUS_VERSION"
  python -m package.build_delta \
    --prev "$prev_db" \
    --new  "$today_db" \
    --out  "$release_dir/${public_shard}-${delta_from}-to-${delta_to}.sql.gz"
fi

# 3) Row count sidecar for manifest assembly.
row_count=$(python -c "
import sqlite3, sys
con = sqlite3.connect('$today_db')
print(con.execute('SELECT COUNT(*) FROM skus').fetchone()[0])
con.close()
")

python - <<PYEOF > "$release_dir/${public_shard}.meta.json"
import json
print(json.dumps({
    "shard": "$public_shard",
    "row_count": int("$row_count"),
    "has_baseline": "$has_baseline" == "true",
    "delta_from": "$delta_from" or None,
    "delta_to": "$delta_to",
}, indent=2))
PYEOF

echo "diff_package_shard.sh: wrote $release_dir/ for $public_shard (baseline=$has_baseline delta_from='$delta_from')"
