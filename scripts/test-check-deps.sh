#!/bin/sh
# Script test for T-022: scripts/check-deps.sh (deps, size, go-lcov). Offline:
# throwaway git repos and an inline mock gh.
set -u

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SCRIPT="$ROOT/scripts/check-deps.sh"
FAILS=0
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM

pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1"; FAILS=$((FAILS + 1)); }
# expect NAME WANT_RC GREP -- command...
expect() {
  name=$1; want=$2; pat=$3; shift 4
  OUT=$("$@" 2>&1); RC=$?
  if [ "$RC" -eq "$want" ] && printf '%s' "$OUT" | grep -q -- "$pat"; then pass "$name"; else fail "$name (rc=$RC) $OUT"; fi
}
deps() { sh "$SCRIPT" deps "$TMP/$1.diff" "$TMP/$2.md"; }

REPO="$TMP/repo"
git init -q "$REPO"
git -C "$REPO" config user.email t@example.invalid
git -C "$REPO" config user.name t
mkdir -p "$REPO/svc"
printf 'module example.com/svc\n\ngo 1.22\n\nrequire github.com/old/dep v1.0.0\n' >"$REPO/svc/go.mod"
printf 'github.com/old/dep v1.0.0 h1:aaa=\n' >"$REPO/svc/go.sum"
POM_HEAD='<project>\n  <artifactId>svc-root</artifactId>\n  <dependencies>\n    <dependency>\n      <groupId>org.old</groupId>\n      <artifactId>old-lib</artifactId>\n      <version>1.0</version>\n    </dependency>\n'
printf "$POM_HEAD  </dependencies>\n</project>\n" >"$REPO/pom.xml"
git -C "$REPO" add -A
git -C "$REPO" commit -qm base
BASE=$(git -C "$REPO" rev-parse HEAD)
printf '## Constraints\nOnly `github.com/new/dep` and the `new-lib` artifact may be added.\n\n## Out of scope\nNot `github.com/other/dep`.\n' >"$TMP/allow.md"
printf '## Constraints\nNo new dependencies.\n' >"$TMP/none.md"

# mkdiff NAME: save the working-tree change as a full-context diff, then reset the repo.
mkdiff() {
  git -C "$REPO" add -A
  git -C "$REPO" diff --cached --unified=100000 "$BASE" >"$TMP/$1.diff"
  git -C "$REPO" reset -q --hard "$BASE"
  git -C "$REPO" clean -fdq
}
gomod() { printf 'module example.com/svc\n\ngo 1.22\n\nrequire (\n\tgithub.com/old/dep v1.0.0\n\t%s\n)\n' "$1" >"$REPO/svc/go.mod"; }

gomod 'github.com/new/dep v0.3.0 // indirect'; mkdiff added
expect "dependency added but not in ticket -> fail" 1 'github.com/new/dep' -- deps added none
expect "dependency named in ticket -> pass" 0 'deps-check ok' -- deps added allow
gomod 'github.com/other/dep v0.3.0'; mkdiff other
expect "dependency named only outside Constraints -> fail" 1 'github.com/other/dep' -- deps other allow
gomod 'github.com/new v1.0.0'; mkdiff prefix
expect "prefix of an allowed module is not a match" 1 'github.com/new ' -- deps prefix allow
printf 'module example.com/svc\n\ngo 1.22\n\nrequire github.com/old/dep v1.2.0\n' >"$REPO/svc/go.mod"
printf 'github.com/old/dep v1.2.0 h1:ccc=\n' >"$REPO/svc/go.sum"; mkdiff bump
expect "version bump of an existing module -> pass" 0 'deps-check ok' -- deps bump none
printf 'github.com/old/dep v1.0.0 h1:aaa=\ngithub.com/sneaky/mod v1.0.0 h1:zzz=\n' >"$REPO/svc/go.sum"; mkdiff gosum
expect "go.sum gaining a module not in ticket -> fail" 1 'github.com/sneaky/mod' -- deps gosum allow
printf 'module example.com/svc\n\ngo 1.22\n\nrequire github.com/old/dep v1.0.0\n\nreplace github.com/old/dep => ../local\n' >"$REPO/svc/go.mod"; mkdiff local
expect "replace to a local path -> pass" 0 'deps-check ok' -- deps local none

printf "$POM_HEAD    <dependency><groupId>org.new</groupId><artifactId>new-lib</artifactId></dependency>\n    <dependency>\n      <artifactId>evil-lib</artifactId>\n    </dependency>\n  </dependencies>\n</project>\n" >"$REPO/pom.xml"; mkdiff pom
expect "pom: unapproved artifact -> fail" 1 'evil-lib' -- deps pom allow
deps pom allow 2>&1 | grep -q 'new-lib' && fail "pom: named artifact must not be reported" || pass "pom: named artifact is not reported"
printf "$POM_HEAD  </dependencies>\n</project>\n" | sed 's/svc-root/renamed/; s/1.0</2.0</' >"$REPO/pom.xml"; mkdiff pombump
expect "pom: project rename and version bump -> pass" 0 'deps-check ok' -- deps pombump none
mkdir -p "$REPO/new"
printf '<project><artifactId>fresh</artifactId><dependencies><dependency><artifactId>x-lib</artifactId></dependency></dependencies></project>\n' >"$REPO/new/pom.xml"
printf 'module example.com/new\n\ngo 1.22\n' >"$REPO/new/go.mod"; mkdiff fresh
expect "new files: only dependency coordinates count" 1 'x-lib' -- deps fresh allow
: >"$TMP/empty.diff"
expect "empty diff -> pass" 0 'deps-check ok' -- deps empty none

# --pr: ticket link logic is check-scope.sh's; the files API is stubbed out
cat >"$TMP/gh1" <<'M'
#!/bin/sh
case "$1 $2" in
  "pr view") printf '%s\n' '{"title":"[T-900] x","body":"Closes #900"}' ;;
  "issue view") printf '%s\n' '{"title":"[T-900] x","labels":[{"name":"status:coding"}],"body":"## Constraints\nAdd `github.com/new/dep`.\n\n## Files in scope\n`svc/go.mod`\n"}' ;;
  *) echo "mock-gh: unexpected $*" >&2; exit 1 ;;
esac
M
sed 's/Add `github.com\/new\/dep`/Nothing/' "$TMP/gh1" >"$TMP/gh2"
sed 's/Closes #900/no link/' "$TMP/gh1" >"$TMP/gh3"
chmod +x "$TMP"/gh1 "$TMP"/gh2 "$TMP"/gh3
gomod 'github.com/new/dep v0.3.0'; git -C "$REPO" add -A; git -C "$REPO" commit -qm head
prmode() { env GH="$TMP/$1" GH_REPO=o/r sh -c "cd '$REPO' && sh '$SCRIPT' deps --pr 1 --base $BASE"; }
expect "--pr: dependency named in the linked ticket's Constraints -> pass" 0 'deps-check ok' -- prmode gh1
expect "--pr: dependency missing from Constraints -> fail" 1 'github.com/new/dep' -- prmode gh2
expect "--pr: no ticket link -> fail via check-scope.sh" 2 'ticket link validation failed' -- prmode gh3

# size
printf '500\t0\tsvc/main.go\n9000\t0\tsvc/go.sum\n-\t-\tdocs/logo.png\n50\t50\tsvc/other.go\n' >"$TMP/n600"
expect "600 changed lines (go.sum, binary excluded) -> pass" 0 'changed lines: 600 ' -- sh "$SCRIPT" size "$TMP/n600"
printf '500\t0\tsvc/main.go\n1\t0\tsvc/go.sum\n50\t51\tsvc/other.go\n' >"$TMP/n601"
expect "601 changed lines -> fail" 1 'diff too large' -- sh "$SCRIPT" size "$TMP/n601"
printf '700\t0\tapi/gen/x.go\n700\t0\tsvc/x.pb.go\n10\t0\tyarn.lock\n3\t3\tsvc/a.go\n' >"$TMP/ngen"
expect "generated files and lockfiles are excluded" 0 'changed lines: 6 ' -- sh "$SCRIPT" size "$TMP/ngen"
expect "size --base reads the real git diff" 0 'changed lines: 5 ' -- sh -c "cd '$REPO' && sh '$SCRIPT' size --base $BASE"

# go-lcov
mkdir -p "$TMP/lc/libs/m"
printf 'module example.com/m\n\ngo 1.22\n' >"$TMP/lc/libs/m/go.mod"
printf 'mode: atomic\nexample.com/m/a.go:3.10,5.2 2 1\nexample.com/m/a.go:6.10,8.2 1 0\nother.example/x/b.go:1.1,2.2 1 1\n' >"$TMP/cover.out"
LCOV=$(cd "$TMP/lc" && sh "$SCRIPT" go-lcov libs/m "$TMP/cover.out" 2>&1)
if printf '%s' "$LCOV" | grep -q '^SF:libs/m/a.go$' && printf '%s' "$LCOV" | grep -q '^DA:4,1$' && printf '%s' "$LCOV" | grep -q '^DA:7,0$' && ! printf '%s' "$LCOV" | grep -q 'other.example'; then
  pass "go-lcov maps a cover profile to repo-relative lcov"; else fail "go-lcov output: $LCOV"; fi

[ "$FAILS" -eq 0 ] || { printf '%s test(s) failed\n' "$FAILS"; exit 1; }
echo "all tests passed"
