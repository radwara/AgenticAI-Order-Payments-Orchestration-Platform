#!/bin/sh
# Script test for T-002: proves scripts/check-scope.sh passes in-scope file lists
# and fails out-of-scope ones, handles renames, validates the linked ticket and
# parses the scope section strictly. Offline: temp files plus a mocked gh
# (scripts/fixtures/check-scope).
set -u

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SCRIPT="$ROOT/scripts/check-scope.sh"
FIX="$ROOT/scripts/fixtures/check-scope"
FAILS=0
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM

pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1"; FAILS=$((FAILS + 1)); }

# pure NAME WANT_RC BODY FILES [GREP]: run pure mode, check exit code and optional output text.
pure() {
  OUT=$(sh "$SCRIPT" "$3" "$4" 2>&1); RC=$?
  if [ "$RC" -eq "$2" ] && { [ -z "${5:-}" ] || printf '%s' "$OUT" | grep -q -- "$5"; }; then pass "$1"; else fail "$1 (rc=$RC) $OUT"; fi
}

# prmode NAME WANT_RC PR_FIXTURE FILES_FIXTURE [GREP]: run --pr mode against the mock gh.
prmode() {
  OUT=$(GH="$FIX/mock-gh.sh" MOCK_DIR="$FIX" MOCK_PR="$3" MOCK_FILES="$4" GH_REPO=o/r \
    sh "$SCRIPT" --pr 1 2>&1); RC=$?
  if [ "$RC" -eq "$2" ] && { [ -z "${5:-}" ] || printf '%s' "$OUT" | grep -q -- "$5"; }; then pass "$1"; else fail "$1 (rc=$RC) $OUT"; fi
}

cat >"$TMP/body.md" <<'BODY'
## Why
Something.

## Files in scope
`.github/workflows/ci.yml`, `scripts/check-scope.sh`, `services/orders/`, `libs/go-*/**`

## Depends on
T-001
BODY

# --- Basic matching -----------------------------------------------------------
cat >"$TMP/in.txt" <<'LIST'
.github/workflows/ci.yml
scripts/check-scope.sh
services/orders/main.go
services/orders/internal/x/y.go
libs/go-money/money.go
LIST
pure "in-scope file list passes" 0 "$TMP/body.md" "$TMP/in.txt"

printf 'scripts/check-scope.sh\nservices/payments/main.go\n' >"$TMP/out.txt"
pure "out-of-scope file list fails and names the offender" 1 "$TMP/body.md" "$TMP/out.txt" 'services/payments/main.go'

printf 'services/orders-evil/x.go\n' >"$TMP/look.txt"
pure "directory entry does not match sibling with same prefix" 1 "$TMP/body.md" "$TMP/look.txt"

printf '## Why\nnothing\n' >"$TMP/nosec.md"
pure "missing Files in scope section fails" 2 "$TMP/nosec.md" "$TMP/in.txt"

: >"$TMP/empty.txt"
pure "empty change list passes" 0 "$TMP/body.md" "$TMP/empty.txt"

# --- R1-01: renames (list carries old and new path) ---------------------------
printf '## Files in scope\n`deploy/local/**`\n' >"$TMP/deploy.md"
printf 'deploy/local/foo.go\nservices/x/foo.go\n' >"$TMP/rename.txt"
pure "rename into scope fails because the old out-of-scope path is listed" 1 "$TMP/deploy.md" "$TMP/rename.txt" 'services/x/foo.go'
printf 'deploy/local/foo.go\n' >"$TMP/newonly.txt"
pure "new path alone passes (hence the list must carry both names)" 0 "$TMP/deploy.md" "$TMP/newonly.txt"
printf 'deploy/local/new.yml\ndeploy/local/old.yml\n' >"$TMP/rename-in.txt"
pure "rename entirely inside scope passes" 0 "$TMP/deploy.md" "$TMP/rename-in.txt"
printf 'deploy/local/foo.go\r\nservices/x/foo.go\r\n' >"$TMP/crlf.txt"
pure "CRLF in the file list does not hide a path" 1 "$TMP/deploy.md" "$TMP/crlf.txt" 'services/x/foo.go'

prmode "--pr: rename into scope fails (previous_filename read from API)" 1 ok rename-into-scope 'services/x/foo.go'
prmode "--pr: rename inside scope passes" 0 ok rename-in-scope 'scope-check ok'
prmode "--pr: paginated pages of files pass" 0 ok paged 'scope-check ok'

# --- R1-02: linked ticket validation ------------------------------------------
prmode "--pr: Closes #N to an approved ticket passes" 0 ok paged
prmode "--pr: 'Refs #N' counts as the link (comment-wrapped Closes #99 ignored)" 0 refs paged
prmode "--pr: 'REFS #N' (case-insensitive) passes" 0 refs-upper paged
prmode "--pr: 'fixes #N' passes" 0 fixes paged
prmode "--pr: mixed keywords for the same issue pass" 0 mixed-same paged
prmode "--pr: mixed keywords for two different issues fail" 2 mixed-diff paged 'different issues'
prmode "--pr: same issue linked twice passes" 0 same-twice paged
prmode "--pr: two different issues fail" 2 multi paged 'different issues'
prmode "--pr: no link fails" 2 nolink paged 'does not link'
prmode "--pr: title id differs from linked issue (broader ticket #3) fails" 2 wrongid paged 'does not match'
prmode "--pr: PR title without [T-XXX] fails" 2 notitleid paged 'no \[T-XXX\]'
prmode "--pr: needs-approval ticket fails" 2 unapproved paged 'needs-approval'
prmode "--pr: ticket with no status label fails" 2 nostatus paged 'approved-or-later'
prmode "--pr: needs-approval alongside coding fails" 2 bothlabels paged 'needs-approval'
prmode "--pr: escalated ticket fails" 2 escalated paged 'escalated'
prmode "--pr: human-review ticket passes" 0 humanreview scripts-only
prmode "--pr: unreadable issue fails" 2 missing paged 'cannot read issue'
prmode "--pr: out-of-scope file in PR fails" 1 ok out-of-scope 'services/evil.go'

# --- R1-03: strict scope parsing ----------------------------------------------
printf 'services/evil.go\n' >"$TMP/evil.txt"
printf '## Files in scope\nAll of it, but not `*`, thanks.\n`scripts/`\n' >"$TMP/prose.md"
pure "backticks in prose do not widen scope" 1 "$TMP/prose.md" "$TMP/evil.txt" 'services/evil.go'
printf 'scripts/a.sh\n' >"$TMP/ok1.txt"
pure "entry line next to prose still works" 0 "$TMP/prose.md" "$TMP/ok1.txt"

for pat in '*' '**' '**/*' '*.go' '*/x' '?' '[a-z]*/x'; do
  printf '## Files in scope\n`%s`\n' "$pat" >"$TMP/wild.md"
  pure "bare/leading wildcard '$pat' is rejected" 2 "$TMP/wild.md" "$TMP/evil.txt" 'invalid scope pattern'
done
printf '## Files in scope\n`/etc/passwd`\n' >"$TMP/abs.md"
pure "absolute path pattern is rejected" 2 "$TMP/abs.md" "$TMP/evil.txt" 'absolute'
printf '## Files in scope\n`scripts/../services/x`\n' >"$TMP/dots.md"
pure "pattern with .. is rejected" 2 "$TMP/dots.md" "$TMP/evil.txt" "'\\.\\.'"
printf '## Files in scope\n`a b/c`\n' >"$TMP/space.md"
pure "pattern with a space is rejected (not split in two)" 2 "$TMP/space.md" "$TMP/evil.txt" 'whitespace'
printf '## Files in scope\n- `scripts/`\n- `libs/go-*/**`\n' >"$TMP/bullets.md"
printf 'scripts/x.sh\nlibs/go-money/a.go\n' >"$TMP/bl.txt"
pure "bullet list of entries is accepted" 0 "$TMP/bullets.md" "$TMP/bl.txt"
printf '## Files in scope\r\n`scripts/`\r\n\r\n## Next\r\n' >"$TMP/crlf.md"
printf 'scripts/x.sh\n' >"$TMP/c1.txt"
pure "CRLF issue body is parsed" 0 "$TMP/crlf.md" "$TMP/c1.txt"
printf '## Files in scope\nSee `scripts/` for details and `*`\n' >"$TMP/mixed.md"
pure "line mixing prose and tokens contributes no patterns" 2 "$TMP/mixed.md" "$TMP/c1.txt" "no 'Files in scope'"

if [ "$FAILS" -ne 0 ]; then
  printf '%s test(s) failed\n' "$FAILS"
  exit 1
fi
echo "all tests passed"
