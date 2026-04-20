#!/usr/bin/env bash
# Assemble manifest.json from today's release artifacts.
#
# Inputs (env):
#   CATALOG_VERSION    — today's catalog tag
#   PREVIOUS_VERSION   — prior release's catalog tag (may be empty)
#   MIN_BINARY         — minimum client binary version
#   GH_TOKEN           — for gh CLI
#
# Inputs (on disk): `dist/pipeline/release/*` populated by the diff_package matrix.
# Output: `dist/pipeline/release/manifest.json`.
set -euo pipefail

: "${CATALOG_VERSION:?CATALOG_VERSION env var required}"
: "${MIN_BINARY:?MIN_BINARY env var required}"
PREVIOUS_VERSION="${PREVIOUS_VERSION:-}"

release_dir="dist/pipeline/release"
prev_manifest=""

if [ -n "$PREVIOUS_VERSION" ]; then
  prev_dir="dist/pipeline/prev-manifest"
  mkdir -p "$prev_dir"
  if gh release download "data-${PREVIOUS_VERSION}" \
        --pattern 'manifest.json' \
        --dir "$prev_dir" 2>/dev/null; then
    prev_manifest="$prev_dir/manifest.json"
  fi
fi

repo="${GITHUB_REPOSITORY:-sofq/sku}"
server="${GITHUB_SERVER_URL:-https://github.com}"
base_url="${server}/${repo}/releases/download/data-${CATALOG_VERSION}"

cmd=(
  python -m package.build_manifest
  --artifacts-dir "$release_dir"
  --out "$release_dir/manifest.json"
  --catalog-version "$CATALOG_VERSION"
  --release-base-url "$base_url"
  --min-binary-version "$MIN_BINARY"
)
if [ -n "$prev_manifest" ]; then
  cmd+=(--previous-manifest "$prev_manifest")
fi

"${cmd[@]}"

echo "build_manifest.sh: wrote $release_dir/manifest.json"
