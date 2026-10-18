#!/bin/sh
# Fails if a change touches files outside the linked ticket's "Files in scope".
#
# Usage:
#   scripts/check-scope.sh <issue-body-file> <changed-files-file>   # pure mode (tests)
#   scripts/check-scope.sh --pr <number>                            # CI mode (needs gh, GH_REPO/GH_TOKEN)
#
# "Files in scope" entries are backticked paths in the section of the issue body:
#   exact path          -> matches that file
#   path ending in "/"  -> matches everything below that directory
#   shell glob          -> matches with * and ? (a "*" also crosses "/")
set -eu
set -f # no filesystem globbing when splitting patterns

die() { printf '%s\n' "$*" >&2; exit 2; }

if [ "${1:-}" = "--pr" ]; then
  PR="${2:?pr number}"
  TMPD=$(mktemp -d)
  trap 'rm -rf "$TMPD"' EXIT INT TERM
  ISSUE=$(gh pr view "$PR" --json body -q .body |
    sed -n 's/.*[Cc]loses[[:space:]]*#\([0-9][0-9]*\).*/\1/p' | head -1)
  [ -n "$ISSUE" ] || die "PR #$PR does not link a ticket with 'Closes #N'"
  gh issue view "$ISSUE" --json body -q .body >"$TMPD/body.md"
  gh pr diff "$PR" --name-only >"$TMPD/files.txt"
  set -- "$TMPD/body.md" "$TMPD/files.txt"
fi

BODY="${1:?issue body file}"
FILES="${2:?changed files file}"
[ -r "$BODY" ] && [ -r "$FILES" ] || die "cannot read inputs"

# Extract backticked tokens from the "Files in scope" section only.
PATTERNS=$(awk '
  /^#+[[:space:]]*Files in scope/ { insec = 1; next }
  /^#+[[:space:]]/ { insec = 0 }
  insec { print }
' "$BODY" | grep -o '`[^`]*`' | tr -d '`' || true)
[ -n "$PATTERNS" ] || die "no 'Files in scope' entries found in ticket"

BAD=0
while IFS= read -r f; do
  [ -n "$f" ] || continue
  ok=0
  for p in $PATTERNS; do
    case "$p" in
      */) case "$f" in "$p"*) ok=1 ;; esac ;;
      *)  case "$f" in $p) ok=1 ;; esac ;;
    esac
    [ "$ok" -eq 1 ] && break
  done
  if [ "$ok" -eq 0 ]; then
    printf 'OUT OF SCOPE: %s\n' "$f" >&2
    BAD=1
  fi
done <"$FILES"

[ "$BAD" -eq 0 ] || { echo "scope-check failed: files outside the ticket's Files in scope" >&2; exit 1; }
echo "scope-check ok"
