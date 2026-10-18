#!/bin/sh
# Script test for T-022: the changed-lines coverage gate (Go cover profile ->
# check-deps.sh go-lcov -> diff-cover --fail-under=80) fails on a diff with an
# untested changed line and passes once it is covered.
# Needs go, git and diff-cover (pip install diff-cover==10.0.0; or DIFF_COVER=<path>).
set -u

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SCRIPT="$ROOT/scripts/check-deps.sh"
DIFF_COVER=${DIFF_COVER:-diff-cover}
FAILS=0
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM

pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1"; FAILS=$((FAILS + 1)); }
command -v "$DIFF_COVER" >/dev/null 2>&1 || { echo "diff-cover not found (pip install diff-cover==10.0.0)" >&2; exit 1; }
command -v go >/dev/null 2>&1 || { echo "go not found" >&2; exit 1; }

REPO="$TMP/repo"
git init -q -b main "$REPO"
git -C "$REPO" config user.email t@example.invalid
git -C "$REPO" config user.name t
mkdir -p "$REPO/libs/m"
printf 'module example.com/m\n\ngo 1.22\n' >"$REPO/libs/m/go.mod"
printf 'package m\n\n// Add returns a+b.\nfunc Add(a, b int) int {\n\treturn a + b\n}\n' >"$REPO/libs/m/m.go"
printf 'package m\n\nimport "testing"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal("bad")\n\t}\n}\n' >"$REPO/libs/m/m_test.go"
git -C "$REPO" add -A
git -C "$REPO" commit -qm base

# The CI gate: test with a profile, convert, diff-cover against the base.
gate() {
  (cd "$REPO/libs/m" && go test -coverprofile=cover.out ./... >/dev/null) || return 3
  (cd "$REPO" && sh "$SCRIPT" go-lcov libs/m libs/m/cover.out >"$TMP/lcov.info") || return 3
  (cd "$REPO" && "$DIFF_COVER" "$TMP/lcov.info" --compare-branch=main --fail-under=80)
}

printf '\n// Sign returns -1, 0 or 1.\nfunc Sign(n int) int {\n\tif n < 0 {\n\t\treturn -1\n\t}\n\tif n > 0 {\n\t\treturn 1\n\t}\n\treturn 0\n}\n' >>"$REPO/libs/m/m.go"
printf '\nfunc TestSignNegative(t *testing.T) {\n\tif Sign(-5) != -1 {\n\t\tt.Fatal("bad")\n\t}\n}\n' >>"$REPO/libs/m/m_test.go"
OUT=$(gate 2>&1); RC=$?
{ [ "$RC" -eq 1 ] && printf '%s' "$OUT" | grep -q 'm.go'; } && pass "gate fails when changed lines are untested" || fail "expected gate failure (rc=$RC): $OUT"

printf '\nfunc TestSignRest(t *testing.T) {\n\tif Sign(5) != 1 || Sign(0) != 0 {\n\t\tt.Fatal("bad")\n\t}\n}\n' >>"$REPO/libs/m/m_test.go"
OUT=$(gate 2>&1); RC=$?
[ "$RC" -eq 0 ] && pass "gate passes when the changed lines are covered" || fail "expected gate pass (rc=$RC): $OUT"

[ "$FAILS" -eq 0 ] || { printf '%s test(s) failed\n' "$FAILS"; exit 1; }
echo "all tests passed"
