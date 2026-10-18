#!/bin/sh
# Script test for T-022: apply-branch-protection.sh prints the exact payload on
# --dry-run, is idempotent, sends that payload when applying, and refuses in CI.
set -u

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SCRIPT="$ROOT/scripts/apply-branch-protection.sh"
FAILS=0
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM

pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1"; FAILS=$((FAILS + 1)); }

printf '#!/bin/sh\nprintf "%%s\\n" "$*" >>"$MOCK_LOG.args"\ncat >>"$MOCK_LOG.stdin"\n' >"$TMP/gh"
chmod +x "$TMP/gh"
run() { env -u CI -u GITHUB_ACTIONS GH="$TMP/gh" MOCK_LOG="$TMP/log" "$@"; }

OUT1=$(run sh "$SCRIPT" --dry-run)
[ "$OUT1" = "$(run sh "$SCRIPT" --dry-run)" ] && pass "dry-run output is identical on every run" || fail "dry-run output differs"
[ ! -e "$TMP/log.args" ] && pass "dry-run makes no API call" || fail "dry-run called gh"
printf '%s\n' "$OUT1" | sed 1d >"$TMP/payload.json"
printf '%s\n' "$OUT1" | head -1 | grep -q '^PUT repos/.*/branches/main/protection$' && pass "dry-run names the PUT endpoint" || fail "endpoint line missing"

# Expected = the live protection recorded in docs/agent-identities.md + the new checks (go/java not required).
python3 - "$TMP/payload.json" <<'PY' && pass "dry-run payload is exactly the expected protection" || fail "payload differs from expected"
import json, sys
want = {
    "required_status_checks": {"strict": False, "contexts": ["secrets-scan", "scope-check", "dep-diff", "diff-size", "coverage"]},
    "enforce_admins": False,
    "required_pull_request_reviews": {"dismiss_stale_reviews": True, "require_code_owner_reviews": True,
                                      "required_approving_review_count": 1, "require_last_push_approval": False},
    "restrictions": None, "required_linear_history": True, "allow_force_pushes": False,
    "allow_deletions": False, "block_creations": False, "required_conversation_resolution": True,
    "lock_branch": False, "allow_fork_syncing": False,
}
sys.exit(0 if json.load(open(sys.argv[1])) == want else 1)
PY

run sh "$SCRIPT" >/dev/null; run sh "$SCRIPT" >/dev/null
[ "$(grep -c '^api --method PUT ' "$TMP/log.args")" = 2 ] && pass "each apply is one PUT" || fail "unexpected gh calls: $(cat "$TMP/log.args")"
python3 - "$TMP/log.stdin" "$TMP/payload.json" <<'PY' && pass "applied payload (twice) equals the dry-run payload: idempotent" || fail "applied payload differs"
import json, sys
dec, raw, i, docs = json.JSONDecoder(), open(sys.argv[1]).read(), 0, []
while i < len(raw):
    if raw[i].isspace(): i += 1; continue
    d, i = dec.raw_decode(raw, i); docs.append(d)
sys.exit(0 if len(docs) == 2 and docs[0] == docs[1] == json.load(open(sys.argv[2])) else 1)
PY

rm -f "$TMP/log.args"
for v in CI GITHUB_ACTIONS; do
  OUT=$(env "$v=true" GH="$TMP/gh" MOCK_LOG="$TMP/log" sh "$SCRIPT" 2>&1); RC=$?
  { [ "$RC" -eq 2 ] && [ ! -e "$TMP/log.args" ] && printf '%s' "$OUT" | grep -q 'refusing'; } && pass "refuses to apply when $v is set" || fail "$v guard failed (rc=$RC)"
done
env CI=true sh "$SCRIPT" --dry-run >/dev/null && pass "dry-run (read-only) is allowed in CI" || fail "dry-run blocked in CI"

[ "$FAILS" -eq 0 ] || { printf '%s test(s) failed\n' "$FAILS"; exit 1; }
echo "all tests passed"
