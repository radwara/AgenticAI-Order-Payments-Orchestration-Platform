#!/usr/bin/env bash
# Usage: scripts/create-issues.sh <phase-dir> [owner/repo]
# Creates one GitHub issue per tickets/<phase>/T-*.md (front matter: title, labels). Skips titles that already exist.
set -euo pipefail
dir="${1:?phase dir, e.g. tickets/phase-1}"
repo="${2:-$(gh repo view --json nameWithOwner -q .nameWithOwner)}"
for f in "$dir"/T-*.md; do
  title=$(sed -n 's/^title: "\(.*\)"$/\1/p' "$f" | head -1)
  labels=$(sed -n 's/^labels: //p' "$f" | head -1 | tr -d ' ')
  if gh issue list --repo "$repo" --state all --search "\"$title\" in:title" --json title -q '.[].title' | grep -qxF "$title"; then
    echo "skip (exists): $title"; continue
  fi
  body=$(mktemp); awk 'BEGIN{n=0} /^---$/{n++; next} n>=2' "$f" > "$body"
  gh issue create --repo "$repo" --title "$title" --body-file "$body" --label "$labels"
  rm -f "$body"
done
