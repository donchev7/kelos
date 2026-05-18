# Agent GitHub Write and `gh` CLI Capability Spec

Date: 2026-04-01  
Status: Draft v1  
Owner: Moontide

## 1. Purpose

Define an implementation plan to let each Moontide agent be configured with one explicit capability toggle:

1. Edit code.
2. Use `gh` CLI for branch/commit/push/PR/comment workflows.

This spec keeps your simplification: one permission switch, not a full matrix.

## 2. Product decision

## 2.1 Must-have

Use one agent-level capability:

1. `github_write_automation_enabled` (boolean, default `false`).

Behavior:

1. `false`: agent cannot edit files and cannot execute `gh`-based GitHub write workflows.
2. `true`: agent can edit files and can run full `gh` CLI workflows for repos granted to that agent’s GitHub App installation token.

## 2.2 Optional (later)

1. Split this into finer capabilities (`edit_only`, `pr_comment_only`, `push_only`) after baseline is stable.

## 3. Current repo baseline (what already exists)

1. GitHub App installation-token minting is already implemented in [github.client.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/integrations/github.client.ts).
2. OpenCode permission config is already generated and injected (`permission` map) in [code-search-runtime.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/agents/code-search-runtime.ts).
3. Agent config pipeline already supports typed tooling + permission profiles in:
1. [agent-config-schema.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/agents/agent-config-schema.ts)
2. [agent-definition-core.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/agents/agent-definition-core.ts)
4. Factory UI already edits tooling profiles in:
1. [agent-create.tsx](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/web/src/pages/factory/agent-create.tsx)
2. [agent-editor.tsx](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/web/src/pages/factory/agent-editor.tsx)

## 4. Scope and non-goals

## 4.1 Must-have scope

1. Per-agent write capability toggle.
2. Runtime enforcement in OpenCode permission policy.
3. GitHub App token wiring for `gh` CLI.
4. Branch/commit/push + PR create/update + PR comment/review-response loop.

## 4.2 Out of scope

1. Full multi-step policy engine for all tool categories.
2. Non-GitHub providers (GitLab/Bitbucket).
3. Human-approval workflow redesign.

## 5. Config model changes

## 5.1 Must-have: tooling profile extension

Extend tooling profile JSON with:

```json
{
  "sandbox_template": "opencode-q-and-a",
  "enabled_tools": ["read", "list", "glob", "grep", "bash"],
  "network_mode": "github_only",
  "network_allowlist_hosts": [],
  "github_write_automation_enabled": false
}
```

Rules:

1. Default `false`.
2. Exposed as one checkbox/toggle in Factory UI.
3. Validation requires GitHub installation configured on the definition.

## 5.2 Must-have: compiled runtime policy extension

Add to compiled tooling policy:

1. `githubWriteAutomationEnabled: boolean`.

No DB migration required if stored in existing JSON config and typed columns remain unchanged.

## 6. Enforcement model (OpenCode + credential gate)

## 6.1 Must-have: OpenCode policy is the primary enforcement

Generate OpenCode permission config with command-pattern rules for `bash` plus `edit` action state.

When `github_write_automation_enabled=false`:

1. `edit: "deny"`.
2. `bash` denies `gh *` and mutating git commands at minimum:
1. `git add *`
2. `git commit *`
3. `git push *`
4. `git checkout -b *`
5. `git switch -c *`
6. `git merge *`
7. `git rebase *`
8. `git reset --hard *`
9. `git clean *`

When `github_write_automation_enabled=true`:

1. `edit: "allow"`.
2. `bash` allows:
1. `gh *` (full GH CLI as requested)
2. required git lifecycle commands for branch/commit/push.

Important implementation note:

1. Current runtime builds only scalar permission modes.
2. Must extend permission config generation to support OpenCode object-syntax rules for `bash` (pattern -> mode) so this is enforceable inside OpenCode, not just in app logic.

## 6.2 Must-have: credential gate as defense-in-depth

1. Only inject `GH_TOKEN`/`GITHUB_TOKEN` when `github_write_automation_enabled=true`.
2. For disabled agents, no GitHub write token is injected.
3. Even if a command slips through, lack of token prevents repository writes.

## 6.3 Optional: server-side command precheck

1. Add an extra guard in runtime approval handler: deny write-class `bash` commands if capability is off.
2. Keep this secondary to OpenCode policy, not primary.

## 7. GitHub auth and token flow

## 7.1 Must-have

1. Keep GitHub App as the only credential authority.
2. Mint short-lived installation token per run, scoped to selected repository IDs.
3. Pass token to sandbox runtime as env:
1. `GH_TOKEN`
2. `GITHUB_TOKEN`
4. Do not pass app private key into sandbox.
5. Do not persist installation token in DB logs or artifacts.

## 7.2 Runtime behavior

1. `gh` commands rely on `GH_TOKEN` env, not interactive `gh auth login`.
2. Git remote auth uses installation token over HTTPS:
1. `https://x-access-token:${GH_TOKEN}@github.com/<owner>/<repo>.git`

## 8. Execution model for write-enabled agents

## 8.1 Must-have run sequence

1. Resolve agent + trigger + repo allowlist.
2. Mint installation token for allowed repo(s).
3. Start sandbox and OpenCode with write capability policy.
4. Agent edits files and runs `gh`/`git` commands.
5. Push branch.
6. Create or update PR.
7. Read PR comments/reviews and post responses/fixes.
8. Persist run artifacts and status.

## 8.2 Must-have idempotency

1. Branch naming convention includes run ID.
2. Before `gh pr create`, check if open PR already exists for same head branch.
3. Reuse existing PR when present.

## 8.3 Optional safeguards

1. Enforce protected branch denylist (`main`, `master`, `release/*`) for direct pushes.
2. Auto-add PR label indicating Moontide-generated changes.

## 9. GitHub event and feedback loop coverage

## 9.1 Must-have events

1. `pull_request` (opened/synchronize/reopened) to run initial PR tasks.
2. `issue_comment` on PRs to capture human follow-ups.
3. `pull_request_review_comment` for line-level review feedback.

## 9.2 Must-have response loop

1. Fetch unresolved review comments via `gh api`.
2. Generate edits.
3. Push follow-up commits.
4. Reply in PR thread with summary and references.

## 10. Required app permissions and install settings

## 10.1 Must-have GitHub App repository permissions

1. `Contents: write` (push/update refs).
2. `Pull requests: write` (create/update PR, review flows).
3. `Issues: write` (PR issue-comments path).
4. `Metadata: read` (standard baseline).

## 10.2 Optional

1. `Checks: write` if agent will post check runs.
2. `Commit statuses: write` if status contexts are used.

## 11. Impacted code surfaces

## 11.1 Must-have backend

1. [agent-config-schema.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/agents/agent-config-schema.ts)
2. [agent-definition-core.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/agents/agent-definition-core.ts)
3. [agent-run-core.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/agents/agent-run-core.ts)
4. [code-search-runtime.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/agents/code-search-runtime.ts)
5. [codebase-qa-runtime-core.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/agents/codebase-qa-runtime-core.ts)
6. [github.client.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/integrations/github.client.ts) (token lifecycle helpers)
7. [github-events.route.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/webhooks/github-events.route.ts)

## 11.2 Must-have frontend

1. [typed-config.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/web/src/pages/factory/typed-config.ts)
2. [agent-create.tsx](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/web/src/pages/factory/agent-create.tsx)
3. [agent-editor.tsx](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/web/src/pages/factory/agent-editor.tsx)

## 12. TDD plan

## 12.1 Must-have tests

1. Config parsing:
1. defaults `github_write_automation_enabled=false`
2. round-trip create/edit/read serialization.
2. Permission compilation:
1. disabled mode blocks `edit` and `gh`/git-write patterns.
2. enabled mode allows `edit` + `gh *`.
3. Runtime credential gate:
1. no `GH_TOKEN` injected when disabled.
2. token injected and masked in logs when enabled.
4. Run orchestration:
1. PR create path succeeds with installation token.
2. review-comment follow-up path reads comments and posts response.
5. Webhook routing:
1. `pull_request`, `issue_comment`, `pull_request_review_comment` events map to correct run intents.

## 12.2 Optional tests

1. Long-run token expiry/refresh simulation.
2. Protected branch denylist behavior.

## 13. Observability and audit

## 13.1 Must-have

1. Run event stages:
1. `gh_token_minted`
2. `write_capability_policy_compiled`
3. `git_branch_created`
4. `git_push_completed`
5. `pr_created_or_updated`
6. `pr_feedback_processed`
2. Record command summaries and targets, never raw secrets.
3. Persist PR URL, branch name, and commit SHA list in artifacts.

## 13.2 Optional

1. Structured per-command telemetry for `gh` subcommands.

## 14. Security controls

## 14.1 Must-have

1. Installation-token scope limited to selected repositories only.
2. Token TTL honored; no reuse outside run lifecycle.
3. Secrets redaction in logs and artifacts.
4. Fail closed:
1. policy compile failure -> run blocked
2. token mint failure -> run blocked
3. missing GitHub installation -> run blocked

## 14.2 Optional

1. Org-level emergency kill switch to disable all write-enabled agents.

## 15. Acceptance criteria

1. Agent config has one toggle: enable/disable code edit + full `gh` CLI.
2. Disabled agent cannot edit files or perform GitHub write operations through OpenCode.
3. Enabled agent can create branch, commit, push, create PR, read PR comments, and post follow-up updates.
4. All GitHub writes are attributable to the GitHub App installation identity.
5. No long-lived GitHub user PATs or app private keys are exposed to sandboxes.

## 16. References

1. OpenCode permissions (object syntax and command pattern matching): https://opencode.ai/docs/permissions
2. OpenCode tools and `bash`/`edit` model: https://opencode.ai/docs/tools
3. E2B OpenCode usage patterns: https://e2b.dev/docs/agents/opencode
4. E2B restricted public traffic / traffic token: https://e2b.dev/docs/sandbox/internet-access
5. GitHub App installation auth and token usage: https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation
6. GitHub pull request REST API: https://docs.github.com/en/rest/pulls/pulls
7. GitHub pull request review comment REST API: https://docs.github.com/en/rest/pulls/comments
8. GitHub issue comment REST API (used for PR issue comments): https://docs.github.com/en/rest/issues/comments
9. GitHub webhook events and payloads: https://docs.github.com/en/webhooks/webhook-events-and-payloads
10. GitHub CLI authentication guidance (`GH_TOKEN`): https://cli.github.com/manual/gh_help_environment and https://cli.github.com/manual/gh_auth_login
