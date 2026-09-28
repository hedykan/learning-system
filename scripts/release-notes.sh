#!/usr/bin/env bash
# Print the CHANGELOG section of a tag, e.g. scripts/release-notes.sh v0.1.7 [CHANGELOG.md]
set -euo pipefail
TAG=${1:?usage: scripts/release-notes.sh <tag> [changelog]}
awk -v head="## $TAG " 'index($0, head) == 1 {on = 1; next} on && /^## / {exit} on' "${2:-CHANGELOG.md}"
