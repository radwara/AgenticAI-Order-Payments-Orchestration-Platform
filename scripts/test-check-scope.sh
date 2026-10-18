#!/bin/sh
# Script test for T-002: proves scripts/check-scope.sh passes in-scope file lists
# and fails out-of-scope ones. Pure: uses only local temp files, no network.
set -u

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SCRIPT="$ROOT/scripts/check-scope.sh"
FAILS=0
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM

pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1"; FAILS=$((FAILS + 1)); }

cat >"$TMP/body.md" <<'BODY'
## Why
Something.

## Files in scope
`.github/workflows/ci.yml`, `scripts/check-scope.sh`, `services/orders/`, `libs/go-*/**`

## Depends on
T-001
BODY

# In-scope: exact paths, directory prefix, glob.
cat >"$TMP/in.txt" <<'LIST'
.github/workflows/ci.yml
scripts/check-scope.sh
services/orders/main.go
services/orders/internal/x/y.go
libs/go-money/money.go
LIST
if sh "$SCRIPT" "$TMP/body.md" "$TMP/in.txt" >/dev/null 2>&1; then
  pass "in-scope file list passes"
else
  fail "in-scope file list passes"
fi

# Out-of-scope: one file outside the list must fail and be named.
cat >"$TMP/out.txt" <<'LIST'
scripts/check-scope.sh
services/payments/main.go
LIST
OUT=$(sh "$SCRIPT" "$TMP/body.md" "$TMP/out.txt" 2>&1)
RC=$?
if [ "$RC" -ne 0 ] && printf '%s' "$OUT" | grep -q 'services/payments/main.go'; then
  pass "out-of-scope file list fails and names the offender"
else
  fail "out-of-scope file list fails and names the offender"
fi

# Prefix lookalike must not match a directory entry.
printf 'services/orders-evil/x.go\n' >"$TMP/look.txt"
if sh "$SCRIPT" "$TMP/body.md" "$TMP/look.txt" >/dev/null 2>&1; then
  fail "directory entry does not match sibling with same prefix"
else
  pass "directory entry does not match sibling with same prefix"
fi

# Missing 'Files in scope' section fails closed.
printf '## Why\nnothing\n' >"$TMP/nosec.md"
if sh "$SCRIPT" "$TMP/nosec.md" "$TMP/in.txt" >/dev/null 2>&1; then
  fail "missing Files in scope section fails"
else
  pass "missing Files in scope section fails"
fi

# Empty change list passes.
: >"$TMP/empty.txt"
if sh "$SCRIPT" "$TMP/body.md" "$TMP/empty.txt" >/dev/null 2>&1; then
  pass "empty change list passes"
else
  fail "empty change list passes"
fi

if [ "$FAILS" -ne 0 ]; then
  printf '%s test(s) failed\n' "$FAILS"
  exit 1
fi
echo "all tests passed"
