#!/bin/sh
# Fails if a change touches files outside the linked ticket's "Files in scope".
#
# Usage:
#   scripts/check-scope.sh <issue-body-file> <changed-files-file>   # pure mode (tests)
#   scripts/check-scope.sh --pr <number>                            # CI mode (needs gh + jq, GH_REPO/GH_TOKEN)
#
# The changed-files list must contain EVERY path the change touches, so for a
# rename both the old and the new path (CI mode builds it from the pulls/files
# API: filename + previous_filename).
#
# "Files in scope" entries are backticked paths on ENTRY LINES of the section.
# An entry line consists only of backticked tokens separated by commas, with an
# optional leading "- " / "* " bullet. Backticks in any other line (prose) are
# ignored. Each token is:
#   exact path          -> matches that file
#   path ending in "/"  -> matches everything below that directory
#   shell glob          -> matches with * and ? (a "*" also crosses "/")
# Tokens are rejected (exit 2) if they contain whitespace, start with "/", contain
# "..", or have a glob character in their first path segment (so "*", "**",
# "**/*", "*.go" can never widen the scope to the whole repo).
#
# CI mode also validates the linked ticket (the PR body is author-controlled):
#   - the PR body links exactly one distinct issue (Closes/Fixes/Resolves/Refs #N)
#   - the [T-XXX] id in the PR title equals the one in the issue title
#   - the issue carries an approved-or-later status:* label (not needs-approval)
# The "gh" binary can be replaced with GH=<path> (used by the tests).
set -eu
set -f # no filesystem globbing when splitting patterns

die() { printf '%s\n' "$*" >&2; exit 2; }

GH=${GH:-gh}
APPROVED_LABELS="status:approved status:coding status:in-review status:fixing status:human-review"
TMPD=""
TMPP=""
cleanup() { [ -z "$TMPP" ] || rm -f "$TMPP"; [ -z "$TMPD" ] || rm -rf "$TMPD"; }
trap cleanup EXIT INT TERM

if [ "${1:-}" = "--pr" ]; then
  PR="${2:?pr number}"
  TMPD=$(mktemp -d)

  "$GH" pr view "$PR" --json title,body >"$TMPD/pr.json" || die "cannot read PR #$PR"
  PR_TITLE=$(jq -r '.title // ""' "$TMPD/pr.json")
  # Drop <!-- ... --> comments (template placeholders), also multi-line.
  jq -r '.body // ""' "$TMPD/pr.json" | tr -d '\r' |
    awk '{
      line = $0; out = ""
      while (length(line) > 0) {
        if (inc) { i = index(line, "-->"); if (i == 0) { line = "" } else { line = substr(line, i + 3); inc = 0 } }
        else { i = index(line, "<!--"); if (i == 0) { out = out line; line = "" } else { out = out substr(line, 1, i - 1); line = substr(line, i + 4); inc = 1 } }
      }
      print out }' >"$TMPD/prbody.txt"

  # Exactly one distinct linked issue.
  grep -oiE '(close[sd]?|fix(e[sd])?|resolve[sd]?|refs?)[: ]+#[0-9]+' "$TMPD/prbody.txt" |
    sed 's/.*#//' | sort -un >"$TMPD/links.txt" || true
  NLINKS=$(grep -c . "$TMPD/links.txt" || true)
  [ "$NLINKS" -ge 1 ] || die "PR #$PR does not link a ticket (use 'Closes #N' or 'Refs #N' in the PR body)"
  [ "$NLINKS" -eq 1 ] || die "PR #$PR links $NLINKS different issues ($(tr '\n' ' ' <"$TMPD/links.txt")); exactly one ticket per PR"
  ISSUE=$(cat "$TMPD/links.txt")

  "$GH" issue view "$ISSUE" --json title,body,labels >"$TMPD/issue.json" || die "cannot read issue #$ISSUE"
  ISSUE_TITLE=$(jq -r '.title // ""' "$TMPD/issue.json")

  # Ticket id must match between PR title and issue title.
  PR_ID=$(printf '%s\n' "$PR_TITLE" | grep -oE '\[T-[0-9]+\]' | head -1 || true)
  ISSUE_ID=$(printf '%s\n' "$ISSUE_TITLE" | grep -oE '\[T-[0-9]+\]' | head -1 || true)
  [ -n "$PR_ID" ] || die "PR title has no [T-XXX] ticket id"
  [ -n "$ISSUE_ID" ] || die "issue #$ISSUE title has no [T-XXX] ticket id"
  [ "$PR_ID" = "$ISSUE_ID" ] || die "PR title ticket $PR_ID does not match linked issue #$ISSUE ($ISSUE_ID)"

  # Issue must have been approved (status:approved or later; never needs-approval/escalated).
  if jq -e '[.labels[]?.name] | (index("status:needs-approval") or index("status:escalated"))' "$TMPD/issue.json" >/dev/null 2>&1; then
    die "issue #$ISSUE carries status:needs-approval or status:escalated"
  fi
  OKLABEL=0
  for l in $APPROVED_LABELS; do
    if jq -e --arg l "$l" '[.labels[]?.name] | index($l)' "$TMPD/issue.json" >/dev/null 2>&1; then OKLABEL=1; fi
  done
  [ "$OKLABEL" -eq 1 ] || die "issue #$ISSUE has no approved-or-later status label ($APPROVED_LABELS)"

  jq -r '.body // ""' "$TMPD/issue.json" >"$TMPD/body.md"
  # Both sides of renames: filename and previous_filename.
  "$GH" api --paginate "repos/${GH_REPO:?GH_REPO}/pulls/$PR/files?per_page=100" >"$TMPD/files.json" || die "cannot list files of PR #$PR"
  jq -r '.[] | .filename, (.previous_filename // empty)' "$TMPD/files.json" | sort -u >"$TMPD/files.txt"
  set -- "$TMPD/body.md" "$TMPD/files.txt"
fi

BODY="${1:?issue body file}"
FILES="${2:?changed files file}"
[ -r "$BODY" ] && [ -r "$FILES" ] || die "cannot read inputs"

TMPP=$(mktemp)

# Entry lines of the "Files in scope" section only; backticked tokens only.
tr -d '\r' <"$BODY" |
  awk '
    /^#+[[:space:]]*Files in scope/ { insec = 1; next }
    /^#+[[:space:]]/ { insec = 0 }
    insec { print }
  ' |
  grep -E '^[[:space:]]*([-*][[:space:]]+)?`[^`]+`([[:space:]]*,[[:space:]]*`[^`]+`)*[[:space:]]*,?[[:space:]]*$' |
  grep -o '`[^`]*`' | sed 's/^`//; s/`$//' >"$TMPP" || true
[ -s "$TMPP" ] || die "no 'Files in scope' entry lines found in ticket"

while IFS= read -r p; do
  case "$p" in
    *[[:space:]]*) die "invalid scope pattern (contains whitespace): '$p'" ;;
    /*) die "invalid scope pattern (absolute path): '$p'" ;;
    *..*) die "invalid scope pattern (contains '..'): '$p'" ;;
  esac
  first=${p%%/*}
  case "$first" in
    *[*?[]*) die "invalid scope pattern (wildcard in first path segment would be too broad): '$p'" ;;
  esac
  [ -n "$first" ] || die "invalid scope pattern: '$p'"
done <"$TMPP"
PATTERNS=$(cat "$TMPP")

BAD=0
while IFS= read -r f; do
  f=$(printf '%s' "$f" | tr -d '\r')
  [ -n "$f" ] || continue
  ok=0
  for p in $PATTERNS; do
    case "$p" in
      */) case "$f" in "$p"*) ok=1 ;; esac ;;
      # shellcheck disable=SC2254 # $p is intentionally a glob pattern
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
