#!/usr/bin/env bash
# Print the catalog_version of the most recent `data-*` GitHub release.
#
# Output: bare version string (e.g. `2026.04.17`) with the `data-` prefix
# stripped. Empty string when no prior release exists (first-ever run).
#
# Exit 0 on success (including "no release yet"); non-zero only when the
# GitHub API is unreachable or auth is broken.
set -euo pipefail
tag=$(gh release list --limit 50 --json tagName --jq \
  '[.[] | select(.tagName | startswith("data-"))] | .[0].tagName // ""')
echo "${tag#data-}"
