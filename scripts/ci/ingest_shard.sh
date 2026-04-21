#!/usr/bin/env bash
# Fetch upstream data → run ingest → build SQLite shard.
#
# Inputs (env):
#   SHARD              — underscored shard id (e.g. `aws_ec2`, `openrouter`)
#   CATALOG_VERSION    — catalog tag for today's release (e.g. `2026.04.18`)
#
# GCP auth: `gcp_*` shards build an ADC-authenticated session at run time
# (see `ingest.gcp_common.build_authenticated_session`). CI ADC is populated
# by `google-github-actions/auth@v2`; no static API key required.
#
# Output: `dist/pipeline/<dashed-shard>.db` + `.rows.jsonl` (dashed names
# match what the Go binary expects in SKU_DATA_DIR).
set -euo pipefail

: "${SHARD:?SHARD env var required}"
: "${CATALOG_VERSION:?CATALOG_VERSION env var required}"

RAW_DIR="dist/pipeline/raw"
OUT_DIR="dist/pipeline"
mkdir -p "$RAW_DIR" "$OUT_DIR"

public_shard=$(echo "$SHARD" | tr _ -)

case "$SHARD" in
  aws_ec2|aws_ebs)
    # AmazonEC2 offer is 8+ GB combined. Drive per-region fetch + streaming
    # strip inside the ingest module so we never materialize the full file.
    stripped_dir="$RAW_DIR/${SHARD}-stripped"
    mkdir -p "$stripped_dir"
    python -m "ingest.${SHARD}" \
      --offer-dir "$stripped_dir" \
      --out "$OUT_DIR/$SHARD.rows.jsonl" \
      --catalog-version "$CATALOG_VERSION"
    ;;

  aws_*)
    offer="$RAW_DIR/${SHARD}-offer.json"
    python - <<PYEOF
from pathlib import Path
from ingest.aws_common import fetch_offer
fetch_offer("$SHARD", Path("$offer"))
PYEOF
    python -m "ingest.${SHARD}" \
      --offer "$offer" \
      --out "$OUT_DIR/$SHARD.rows.jsonl" \
      --catalog-version "$CATALOG_VERSION"
    ;;

  azure_*)
    prices="$RAW_DIR/${SHARD}-prices.json"
    python - <<PYEOF
from pathlib import Path
from discover.azure import _SHARD_FILTERS
from ingest.azure_common import fetch_prices
fetch_prices(_SHARD_FILTERS["$SHARD"], Path("$prices"))
PYEOF
    python -m "ingest.${SHARD}" \
      --prices "$prices" \
      --out "$OUT_DIR/$SHARD.rows.jsonl" \
      --catalog-version "$CATALOG_VERSION"
    ;;

  gcp_*)
    skus="$RAW_DIR/${SHARD}-skus.json"
    python - <<PYEOF
from pathlib import Path
from ingest.gcp_common import build_authenticated_session, fetch_skus
with build_authenticated_session() as sess:
    fetch_skus("$SHARD", Path("$skus"), session=sess)
PYEOF
    python -m "ingest.${SHARD}" \
      --skus "$skus" \
      --out "$OUT_DIR/$SHARD.rows.jsonl" \
      --catalog-version "$CATALOG_VERSION"
    ;;

  openrouter)
    dir="$RAW_DIR/openrouter"
    mkdir -p "$dir"
    python - <<PYEOF
from pathlib import Path
from ingest.openrouter import fetch
fetch(Path("$dir"))
PYEOF
    python -m ingest.openrouter \
      --fixture "$dir" \
      --out "$OUT_DIR/$SHARD.rows.jsonl" \
      --skip-non-usd \
      --generated-at "${CATALOG_VERSION}T00:00:00Z"
    ;;

  *)
    echo "ingest_shard.sh: unknown SHARD='$SHARD'" >&2
    exit 2
    ;;
esac

python -m package.build_shard \
  --rows "$OUT_DIR/$SHARD.rows.jsonl" \
  --shard "$SHARD" \
  --out "$OUT_DIR/$SHARD.db" \
  --catalog-version "$CATALOG_VERSION"

# Rename to dashed form so the filename matches the Go binary's expectation
# (SKU_DATA_DIR lookups) and the public release-asset URL shape.
if [ "$SHARD" != "$public_shard" ]; then
  mv "$OUT_DIR/$SHARD.db"          "$OUT_DIR/${public_shard}.db"
  mv "$OUT_DIR/$SHARD.rows.jsonl"  "$OUT_DIR/${public_shard}.rows.jsonl"
fi

echo "ingest_shard.sh: built $OUT_DIR/${public_shard}.db"
