# Agent identities and merge control

## Why
`agents/actors.md` requires the ticket, code and review agents to be separate actors that cannot approve or merge. Until they have separate credentials, GitHub shows every action as the maintainer and nothing technical stops an agent merging. (This happened once: PRs #27, #29, #31, #32 were merged with open review findings; they are being fixed forward.)

## Interim controls (in place)
- `.claude/settings.json` forces an interactive approval for `gh pr merge`, PR approval, merge via API, pushes to `main`, force-pushes and applying `status:approved`. Background agents cannot answer, so they are blocked. Only a human at the keyboard can allow these.
- Every agent prompt says: never merge, never approve, never push to `main`.
- Branch protection is NOT available: GitHub requires Pro or a public repository for private repos (HTTP 403).

## Target setup (needs the maintainer; agents cannot create accounts)
1. Create three machine users (or GitHub Apps): `opop-ticket-agent`, `opop-code-agent`, `opop-review-agent`. Add them as collaborators with **Write** (App equivalents: Contents RW, Pull requests RW, Issues RW, Checks R).
2. Give each agent only its own token (fine-grained PAT, repo-scoped, 90-day expiry). Store tokens in the orchestrator's secret store, never in the repo.
3. Make the repository public or upgrade to GitHub Pro, then apply protection to `main`:
   - Require a pull request; require 1 approving review from a code owner (CODEOWNERS = the human maintainer only); dismiss stale approvals.
   - Required status checks: `secrets-scan`, plus `scope-check`, `go`, `java` once T-002/T-022 fixes land.
   - Require linear history; no force-push; no deletion; resolve conversations before merge.
   - Bots are not code owners, so their approvals do not count and they cannot merge.
4. Run each agent with its own token: `GH_TOKEN=<agent token> claude ...`. The review agent's token must not be the code agent's.
5. Codify as `scripts/apply-branch-protection.sh` (ticket T-022).
