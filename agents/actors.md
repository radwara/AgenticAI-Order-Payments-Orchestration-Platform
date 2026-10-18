# Actors: separate identities that check each other

The code agent and the review agent are **two distinct actors**, like two engineers on a team. They never share an identity, session, memory or credentials. Each one's work is checked by the other, and both are checked by a human at the gates.

| | Code agent ("author") | Review agent ("reviewer") | Human (maintainer) |
|---|---|---|---|
| GitHub identity | Own bot account / GitHub App `opop-code-agent` | Own bot account / GitHub App `opop-review-agent` | Real account |
| Credentials | Own token: write to `agent/*` branches, issues, PRs | Own token: read code, write PR reviews and comments only | Full |
| Instructions | `agents/code-agent.md` | `agents/review-agent.md` | – |
| Session / memory | Own; no access to reviewer's | Own; no access to author's reasoning | – |
| Model / prompt | Configurable independently; **different model or prompt version from the reviewer recommended** | Same | – |
| Can merge | No | No | **Yes** |

## How they check each other

1. **Reviewer checks author:** reviews the PR against the ticket, specs and guard rails (see `review-agent.md`).
2. **Author checks reviewer:** for every finding the author must independently verify it (reproduce with a test or reasoning) and respond with one of:
   - `accept` – fix pushed, commit linked;
   - `dispute` – evidence that the finding is wrong or out of scope (failing test, spec citation, code path).
   A fix without verification, or silently ignoring a finding, is not allowed.
3. **Reviewer adjudicates disputes:** in the next round the reviewer must either withdraw the finding or restate it with new evidence. If a dispute survives a second round, it goes to the human as `escalated`; neither agent has the last word.
4. **Cross-audit of the reviewer:** the human's agree/disagree decisions on findings are logged. A reviewer with a high false-positive rate or missed-defect rate (found by the human after approval) gets its threshold raised or its prompt reworked. The author's dispute success rate is tracked the same way.

## Separation rules (enforced by config, not trust)
- Reviewer's token has no push permission; author's token has no review-approval permission.
- Branch protection requires a human approval; bot approvals do not count toward it.
- Neither bot may edit the other's comments, labels `status:approved`, or the ticket body.
- The reviewer never receives the author's plan/reasoning comments; it sees only diff, ticket, specs, CI, and the author's replies to findings.
- Review comments and replies are the only channel between the two. No side channels.
