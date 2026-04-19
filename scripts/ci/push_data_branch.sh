#!/usr/bin/env bash
# Force-push manifest.json to the `data` branch so jsDelivr can serve it at
#   https://cdn.jsdelivr.net/gh/<repo>@latest/data/manifest.json
#
# Inputs (env):
#   GH_TOKEN           — repo-scoped token (GITHUB_TOKEN from the workflow)
#   GITHUB_REPOSITORY  — `owner/name`
#   CATALOG_VERSION    — today's catalog tag (used in commit message)
#
# The `data` branch is an orphan branch carrying **only** the manifest file,
# which keeps the tracked tree tiny (~1 KB) so jsDelivr serves it quickly and
# the CDN purge cost stays low.
set -euo pipefail

: "${GH_TOKEN:?GH_TOKEN required}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY required}"
: "${CATALOG_VERSION:?CATALOG_VERSION required}"

MANIFEST_SRC="dist/pipeline/release/manifest.json"
test -f "$MANIFEST_SRC" || { echo "missing $MANIFEST_SRC" >&2; exit 2; }

workspace="${GITHUB_WORKSPACE:-$(pwd)}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

cd "$tmp"
git init -q
git remote add origin "https://x-access-token:${GH_TOKEN}@github.com/${GITHUB_REPOSITORY}.git"
git checkout -q --orphan data
git rm -rfq . 2>/dev/null || true

# Manifest lives at the branch root so jsDelivr serves it as
#   https://cdn.jsdelivr.net/gh/<repo>@data/manifest.json
cp "$workspace/$MANIFEST_SRC" manifest.json
git add manifest.json
git -c user.email="bot@sku" -c user.name="sku-data-bot" commit -q -m "data-${CATALOG_VERSION}"
git push -qf origin data

echo "push_data_branch.sh: force-pushed data branch for data-${CATALOG_VERSION}"
