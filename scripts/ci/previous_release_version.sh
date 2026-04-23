#!/usr/bin/env bash
# Print the catalog_version of the most recent `data-*` GitHub release.
#
# Output: bare version string (e.g. `2026.04.17`) with the `data-` prefix
# stripped. Empty string when no prior release exists (first-ever run).
#
# Exit 0 on success (including "no release yet"); non-zero only when the
# GitHub API is unreachable or auth is broken.
set -euo pipefail
tag=$(gh release list --limit 50 --json tagName | python3 -c '
import json
import re
import sys

try:
    releases = json.load(sys.stdin)
except json.JSONDecodeError:
    releases = []

versioned = re.compile(r"^data-[0-9]{4}\.[0-9]{2}\.[0-9]{2}$")
for release in releases:
    tag = release.get("tagName", "")
    if versioned.match(tag):
        print(tag)
        break
')
echo "${tag#data-}"
