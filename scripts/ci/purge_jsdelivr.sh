#!/usr/bin/env bash
# Purge the jsDelivr edge cache for the `data` branch manifest.
#
# jsDelivr mirrors `https://cdn.jsdelivr.net/gh/sofq/sku@latest/data/manifest.json`
# from the `data` branch pushed by the publish job. When the branch updates
# the CDN copy may stay cached for up to 12h; calling the purge endpoint
# forces an invalidation.
#
# Retries 3 times with exponential backoff (2s, 8s, 30s). On final failure
# files an issue via `gh issue create` so the maintainer notices before
# users start seeing stale data.
set -euo pipefail

URL="https://purge.jsdelivr.net/gh/sofq/sku@data/manifest.json"

for attempt in 1 2 3; do
  if curl --fail --silent --show-error "$URL" >/dev/null; then
    echo "jsdelivr purge OK on attempt $attempt"
    exit 0
  fi
  case $attempt in
    1) sleep 2 ;;
    2) sleep 8 ;;
    3) sleep 30 ;;
  esac
done

echo "jsdelivr purge FAILED after 3 attempts" >&2

body="Automated issue — the daily manifest push succeeded but the jsDelivr purge API did not acknowledge after 3 retries (2s/8s/30s backoff). Users on the CDN fallback may see a stale manifest for up to 12h.

Workflow run: ${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY:-sofq/sku}/actions/runs/${GITHUB_RUN_ID:-unknown}

To recover manually: curl --fail --silent --show-error \"$URL\""

gh issue create \
  --title "jsDelivr cache purge failed for catalog $(date -u +%Y-%m-%d)" \
  --label "pipeline" \
  --label "cdn-purge-failed" \
  --body "$body"
exit 1
