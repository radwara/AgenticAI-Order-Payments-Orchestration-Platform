#!/bin/sh
# Applies branch protection to `main`. RUN BY A HUMAN REPO ADMIN, never from CI
# (refuses when CI/GITHUB_ACTIONS is set). Idempotent: PUT replaces the whole protection.
#   scripts/apply-branch-protection.sh --dry-run   print the request and exact payload, change nothing
#   scripts/apply-branch-protection.sh             apply (admin-scoped gh login); BRANCH/REPO env override
#
# Codifies what docs/agent-identities.md records as applied: PR required, 1 code-owner
# approval, stale approvals dismissed, linear history, conversation resolution, no
# force-push or deletion; enforce_admins false (agents must never bypass).
#
# Required checks (all pinned to GitHub Actions, app_id 15368, as live: a bare "contexts"
# list would drop the pin and let any status-posting app satisfy them): secrets-scan and
# scope-check (ci.yml), dep-diff (guardrails.yml). Not required: the path-filtered `go`/`java`
# jobs (skipped job = never-reporting required check), and diff-size/coverage, which become
# required after T-028 (header-verified generated files). Apply only after this merges, so
# dep-diff exists on main, and always with --dry-run first.
set -eu

BRANCH=${BRANCH:-main}
CHECKS=${CHECKS:-"secrets-scan scope-check dep-diff"}
GH=${GH:-gh}
REPO=${REPO:-'{owner}/{repo}'} # gh fills the placeholders from the current repository
ENDPOINT="repos/$REPO/branches/$BRANCH/protection"
DRY=0
case "${1:-}" in
  --dry-run) DRY=1 ;;
  "") ;;
  *) echo "usage: $0 [--dry-run]" >&2; exit 2 ;;
esac

contexts=""
for c in $CHECKS; do contexts="${contexts:+$contexts, }{\"context\": \"$c\", \"app_id\": 15368}"; done
payload() {
  cat <<EOF
{
  "required_status_checks": {"strict": false, "checks": [$contexts]},
  "enforce_admins": false,
  "required_pull_request_reviews": {
    "dismiss_stale_reviews": true,
    "require_code_owner_reviews": true,
    "required_approving_review_count": 1,
    "require_last_push_approval": false
  },
  "restrictions": null,
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "block_creations": false,
  "required_conversation_resolution": true,
  "lock_branch": false,
  "allow_fork_syncing": false
}
EOF
}

if [ "$DRY" -eq 1 ]; then echo "PUT $ENDPOINT"; payload; exit 0; fi
if [ -n "${CI:-}" ] || [ -n "${GITHUB_ACTIONS:-}" ]; then
  echo "refusing to change branch protection from CI; run this by hand as a repo admin" >&2
  exit 2
fi
payload | "$GH" api --method PUT -H "Accept: application/vnd.github+json" "$ENDPOINT" --input - >/dev/null
echo "branch protection applied to $BRANCH"
