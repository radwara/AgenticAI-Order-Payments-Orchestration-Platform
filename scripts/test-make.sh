#!/bin/sh
# Script test for T-001: proves the root Makefile acceptance criteria.
# Runs in throwaway temp copies so the real tree is never modified.
set -u

ROOT=$(cd "$(dirname "$0")/.." && pwd)
FAILS=0
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM

pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1"; FAILS=$((FAILS + 1)); }

# fresh_tree DIR: a tree holding only the Makefile and the layout directories.
fresh_tree() {
  mkdir -p "$1/services" "$1/libs" "$1/contracts" "$1/deploy"
  cp "$ROOT/Makefile" "$1/Makefile"
}

# AC1: make build test lint exits 0 on an empty tree.
T1="$TMP/empty"
fresh_tree "$T1"
if (cd "$T1" && make build test lint >/dev/null 2>&1); then
  pass "AC1 make build test lint exits 0 on empty tree"
else
  fail "AC1 make build test lint exits 0 on empty tree"
fi

# AC1b: also works when services/ and libs/ do not exist at all.
T1B="$TMP/bare"
mkdir -p "$T1B"
cp "$ROOT/Makefile" "$T1B/Makefile"
if (cd "$T1B" && make build test lint >/dev/null 2>&1); then
  pass "AC1b make succeeds when services/ and libs/ are absent"
else
  fail "AC1b make succeeds when services/ and libs/ are absent"
fi

# Modules without a Makefile are a no-op.
T1C="$TMP/nomk"
fresh_tree "$T1C"
mkdir -p "$T1C/services/nomk" "$T1C/libs/go-nomk"
if (cd "$T1C" && make build test lint >/dev/null 2>&1); then
  pass "module without Makefile is a no-op"
else
  fail "module without Makefile is a no-op"
fi

# Module Makefile defining only `build:` is skipped for test/lint (R1-02).
T1D="$TMP/onlybuild"
fresh_tree "$T1D"
mkdir -p "$T1D/services/m"
printf 'build:\n\t@true\n' > "$T1D/services/m/Makefile"
if (cd "$T1D" && make build test lint >/dev/null 2>&1); then
  pass "module lacking a target is skipped for that target"
else
  fail "module lacking a target is skipped for that target"
fi

# Broken module Makefiles must fail, never be skipped (R1-01).
T1E="$TMP/broken-syntax"
fresh_tree "$T1E"
mkdir -p "$T1E/services/a"
printf 'test:\n\t@true\nthis is not valid make\n' > "$T1E/services/a/Makefile"
if (cd "$T1E" && make test >/dev/null 2>&1); then
  fail "module Makefile with syntax error fails make test"
else
  pass "module Makefile with syntax error fails make test"
fi
T1F="$TMP/broken-include"
fresh_tree "$T1F"
mkdir -p "$T1F/services/a"
printf 'include nonexist.mk\ntest:\n\t@true\n' > "$T1F/services/a/Makefile"
if (cd "$T1F" && make test >/dev/null 2>&1); then
  fail "module Makefile with missing include fails make test"
else
  pass "module Makefile with missing include fails make test"
fi

# AC2: a failing test target in services/x/Makefile makes make test fail.
T2="$TMP/failing"
fresh_tree "$T2"
mkdir -p "$T2/services/x"
printf 'build:\n\t@true\ntest:\n\t@exit 1\nlint:\n\t@true\n' > "$T2/services/x/Makefile"
if (cd "$T2" && make test >/dev/null 2>&1); then
  fail "AC2 failing services/x test makes make test fail"
else
  pass "AC2 failing services/x test makes make test fail"
fi
# Its passing targets still pass.
if (cd "$T2" && make build lint >/dev/null 2>&1); then
  pass "AC2b other targets of that module still succeed"
else
  fail "AC2b other targets of that module still succeed"
fi

# libs/* are iterated too.
T2B="$TMP/libfail"
fresh_tree "$T2B"
mkdir -p "$T2B/libs/go-y"
printf 'lint:\n\t@exit 1\n' > "$T2B/libs/go-y/Makefile"
if (cd "$T2B" && make lint >/dev/null 2>&1); then
  fail "libs/* modules are iterated (failing lint must fail)"
else
  pass "libs/* modules are iterated (failing lint must fail)"
fi

# Every module runs even after an earlier one fails, then make still fails.
T2C="$TMP/continue"
fresh_tree "$T2C"
mkdir -p "$T2C/services/a" "$T2C/services/b"
printf 'test:\n\t@exit 1\n' > "$T2C/services/a/Makefile"
printf 'test:\n\t@touch "$(CURDIR)/ran"\n' > "$T2C/services/b/Makefile"
(cd "$T2C" && make test >/dev/null 2>&1)
if [ -f "$T2C/services/b/ran" ]; then
  pass "later modules still run after an earlier failure"
else
  fail "later modules still run after an earlier failure"
fi

# AC3: READMEs document the layout.
for d in services libs contracts deploy; do
  if [ -s "$ROOT/$d/README.md" ]; then
    pass "AC3 $d/README.md exists"
  else
    fail "AC3 $d/README.md exists"
  fi
done
if grep -q 'services/' "$ROOT/services/README.md" && grep -q 'make' "$ROOT/services/README.md"; then
  pass "AC3 services/README.md documents layout and make targets"
else
  fail "AC3 services/README.md documents layout and make targets"
fi

# Config files exist.
for f in .editorconfig .gitignore; do
  if [ -s "$ROOT/$f" ]; then pass "$f present"; else fail "$f present"; fi
done

if [ "$FAILS" -ne 0 ]; then
  printf '%s check(s) failed\n' "$FAILS"
  exit 1
fi
echo "all checks passed"
