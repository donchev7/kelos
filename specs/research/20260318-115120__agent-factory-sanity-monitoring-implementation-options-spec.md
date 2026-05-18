# Agent Factory Spec: Sanity-Check-Style Monitoring Agents

Date: 2026-03-18  
Status: Draft (decision-backed)  
Owner: Moontide  
Goal: Define how Moontide Agent Factory can create and run proactive, always-on agents that continuously monitor repositories/services and post ongoing updates to Slack.

## 1. Decision Snapshot

Based on your selections and clarification:

1. Packaging model: use current `background_triggered` model in Agent Factory first (not a separate dedicated agent type in v1).
2. Audit depth: dynamic/executable checks (start app, call flows), not just static code review.
3. Repo-specific execution contract: typed Moontide config (with optional import/migration path from `.sanity-check.md`).
4. Output channels: UI + Slack required in v1; PR + webhook as selectable add-ons.
5. Autonomy: fully autonomous execution inside sandbox policy (no per-run approvals).
6. Monitoring behavior: agent should proactively run and keep posting status/findings updates to Slack.

Assumption to unblock rollout planning:

1. Rollout scope is controlled by selected repositories in agent configuration, with tenant-level safety controls and quotas.

## 2. Product Intent

Moontide should support "Monitoring/Audit agents" as first-class blueprints in Agent Factory:

1. Watch selected repos/services continuously.
2. Execute runnable health/setup/flow checks in isolated sandboxes.
3. Produce structured Engineer + QA reports (two-role model).
4. Publish incremental and final updates to Slack and UI.
5. Optionally write to GitHub (PR comments and/or report commits) and outbound webhooks.

## 3. Must Have vs Optional by Capability

## 3.1 Core capability

Must Have:

1. Long-running background agent based on existing `background_triggered` runtime.
2. Manual + scheduled triggers.
3. Two-role report generation (`engineer`, `qa`) per run.
4. Slack updates during run + final summary.
5. Persisted run timeline, artifacts, and diagnostics in existing runtime visibility surface.

Optional / Nice-to-Have:

1. Event-driven triggers from GitHub webhook events (e.g., PR opened/synchronize).
2. "Change-only" posting mode (post only on regressions/deltas).
3. Adaptive scheduling based on prior run stability.

## 3.2 Delivery channels

Must Have:

1. UI mission control visibility.
2. Slack delivery target support.

Optional / Nice-to-Have:

1. GitHub PR comment delivery.
2. GitHub report commit/PR branch delivery.
3. Generic webhook JSON delivery.

## 3.3 Configuration model

Must Have:

1. Typed monitoring config in Agent Factory (not freeform only).
2. Per-repo setup/run/check contract fields.
3. Tool/permission/network policy linked to existing typed runtime policy.

Optional / Nice-to-Have:

1. Import assistant that converts `.sanity-check.md` to typed config.
2. Config presets for common stacks (Node API, Next.js app, Python service, monorepo).

## 4. What to Reuse from `sanity-check` vs What to Do Differently

## 4.1 Reuse directly (patterns)

Must Have:

1. Prompt-pipeline pattern:
   1. run audit role(s)
   2. produce concise executive summary
2. Multi-channel delivery pattern:
   1. always persist stdout-equivalent artifact
   2. optional Slack/PR/webhook fanout
3. Repo-level setup guidance pattern:
   1. explicit setup and focus instructions per repo
4. Report shape discipline:
   1. severity-structured output
   2. trends vs previous runs

Optional / Nice-to-Have:

1. PR idempotency pattern (update existing branch/PR for a date scope).
2. Slack truncation strategy (summary + critical-only fallback when large output).

## 4.2 Deliberately different in Moontide

Must Have:

1. No direct `claude` CLI orchestration in production path.
2. Use existing Moontide runtime stack:
   1. E2B sandbox
   2. OpenCode execution
   3. existing queue/lock/scheduler model
3. No host-level `--dangerously-skip-permissions`; autonomy constrained by sandbox runtime policy.
4. Use typed config and Connect/gRPC services instead of local CLI-only UX.

Optional / Nice-to-Have:

1. External engine adapter interface (future support for Claude Code print-mode agents as pluggable backend).

## 5. Architecture Options

## Option A: Agent Factory Blueprint on Existing Background Runtime (Recommended)

Summary:

1. Add a new "Monitoring / Sanity Check" blueprint that compiles to current `background_triggered` definitions.
2. Reuse `agent_runs`, `agent_run_events`, `agent_run_artifacts`, trigger schedulers, and delivery pipeline.

Must Have:

1. Blueprint UI + typed config compiler.
2. New prompt pack and artifact schema for two-role outputs.
3. Extended delivery targets beyond Slack (`github_pr_comment`, `github_pr_commit`, `webhook`) with feature flags.

Optional / Nice-to-Have:

1. Dedicated timeline widgets in existing launch/runs page.
2. Blueprint cloning/version templates.

Pros:

1. Lowest implementation risk and fastest path.
2. Uses already-deployed schedulers and runtime visibility APIs.
3. Minimizes new infra and operational overhead.

Cons:

1. Some abstractions in current runtime are optimized for repo-QA answers, not continuous monitoring semantics.

## Option B: Dedicated Monitoring Agent Runtime Mode

Summary:

1. Add a first-class runtime mode (e.g., `monitoring_audit`) with specialized state machine and delivery semantics.

Must Have:

1. New runtime-mode branch in orchestration.
2. Dedicated report event schema and posting cadence controls.

Optional / Nice-to-Have:

1. Distinct worker process and queue partitions.

Pros:

1. Cleaner long-term domain model.
2. Better separation of concerns.

Cons:

1. Higher upfront engineering cost.
2. More migration and maintenance burden now.

## Option C: External Runner Adapter (Claude-CLI-like Sidecar)

Summary:

1. Keep Moontide orchestration but call an external runner process (Claude-style) inside sandbox or sidecar.

Must Have:

1. Adapter contract and strict output schema.
2. Secure secret injection and command policy.

Optional / Nice-to-Have:

1. Multi-engine A/B (OpenCode vs external runner).

Pros:

1. Maximum tool parity with existing `sanity-check`.

Cons:

1. Extra operational complexity and attack surface.
2. Drift from Moontide's native runtime stack.

## Recommendation

Must Have:

1. Implement Option A now.
2. Design interfaces so Option B is possible later without migration pain.

Optional / Nice-to-Have:

1. Keep Option C as experimental backend only.

## 6. Recommended V1 Design (Option A)

## 6.1 Agent blueprint: "Sanity Monitoring"

Must Have:

1. New blueprint in Agent Factory wizard that emits:
   1. `runtime_mode = background_triggered`
   2. trigger rules for schedule/manual
   3. typed monitoring config payload in `config_json`
2. Default output delivery includes Slack channel target.
3. Default enabled actions include safe audit set (`read`, `list`, `glob`, `grep`, `bash`) with network mode aligned to repo/service needs.

Optional / Nice-to-Have:

1. Additional blueprint variants:
   1. "PR-focused"
   2. "Service uptime"
   3. "Regression guard"

## 6.2 Typed monitoring config schema

Must Have:

1. Schema fields (proposed):
   1. `repositories[]`
   2. `setup.bootstrap_commands[]`
   3. `setup.start_commands[]`
   4. `setup.health_checks[]`
   5. `flows[]` (name, steps, expected assertions)
   6. `focus_paths[]`
   7. `exclude_paths[]`
   8. `role_prompts.engineer`
   9. `role_prompts.qa`
   10. `posting` (progress cadence, severity threshold)
2. Validation in backend before activation.
3. Storage as typed JSON object; avoid markdown-only contract.

Optional / Nice-to-Have:

1. Importer from `.sanity-check.md`.
2. Lint tool for monitoring config quality.

## 6.3 Trigger model

Must Have:

1. Manual trigger:
   1. from UI
   2. optional Slack slash/control command
2. Scheduled trigger:
   1. UTC cron via existing trigger scheduler
   2. per-agent jitter support to avoid synchronization spikes

Optional / Nice-to-Have:

1. GitHub event trigger:
   1. on `pull_request` opened/synchronize/reopened
2. Trigger dedupe windows by `(repo, branch, trigger_source, time_bucket)`.

## 6.4 Execution lifecycle (per run)

Must Have:

1. For each selected repository:
   1. clone repo with installation token
   2. run engineer role prompt
   3. run QA role prompt
2. Combine role outputs into one report artifact.
3. Generate run summary artifact.
4. Emit run timeline events for each major stage.

Optional / Nice-to-Have:

1. Cross-repo synthesis pass ("global risk summary").
2. Incremental stream publishing as each repo completes.

## 6.5 Reporting model

Must Have:

1. Artifact JSON must include:
   1. role outputs
   2. severity buckets
   3. evidence references
   4. assumptions/not-tested
2. Artifact Markdown must include:
   1. summary
   2. engineer section
   3. QA section
   4. trends/delta vs previous successful run

Optional / Nice-to-Have:

1. Machine-readable checklist output for downstream automation.
2. SARIF-style export for code-scanning UIs.

## 6.6 Delivery design

Must Have:

1. Slack updates:
   1. start notification
   2. progress updates at stage boundaries
   3. final summary + link to full artifact in UI
2. UI run artifact availability via existing runtime visibility APIs.

Optional / Nice-to-Have:

1. GitHub PR comment mode:
   1. post/update one sticky comment per run context
2. GitHub PR commit mode:
   1. write markdown report file to branch and open/update PR
3. Webhook mode:
   1. JSON payload for SIEM/ops pipelines.

## 6.7 Permission and safety model

Must Have:

1. Fully autonomous in sandbox, but constrained by runtime policy:
   1. allowed tool actions
   2. network allowlist
   3. repo-scoped credentials
2. Hard timeout budget per role and per run.
3. Resource caps:
   1. max repos per run
   2. max runtime duration
   3. max message size for Slack delivery

Optional / Nice-to-Have:

1. Command allow/deny regexes for `bash` tool categories.
2. Auto-redaction layer for secrets in posted artifacts.

## 6.8 Observability and operations

Must Have:

1. Track:
   1. run success/failure rates
   2. per-stage duration
   3. sandbox failure reasons
   4. delivery failures by channel
2. Retry model:
   1. transient retry on sandbox/network/provider errors
   2. idempotent delivery attempt tracking

Optional / Nice-to-Have:

1. SLO dashboard for monitoring agent fleet.
2. Auto-disable agent after repeated hard failures.

## 7. Proposed API / Schema Changes

## 7.1 Agent setup and trigger APIs

Must Have:

1. Extend trigger `config_json` contract for monitoring blueprint:
   1. schedule/manual metadata
   2. repo targets
   3. run cadence and posting cadence
2. Add typed validation endpoint for monitoring config.

Optional / Nice-to-Have:

1. Dedicated `CreateMonitoringDefinition` endpoint for cleaner UX contracts.

## 7.2 Output delivery targets

Must Have:

1. Extend delivery target kinds beyond current `slack_thread|slack_channel` to include:
   1. `github_pr_comment`
   2. `github_pr_commit`
   3. `webhook`
2. Update parser/validator and run-delivery executor accordingly.

Optional / Nice-to-Have:

1. `github_check_run` delivery target for native Checks UI.

## 7.3 Artifacts

Must Have:

1. Add artifact types:
   1. `monitoring_report_final`
   2. `monitoring_report_engineer`
   3. `monitoring_report_qa`
   4. `monitoring_delta`

Optional / Nice-to-Have:

1. Binary artifact attachment support for logs/screenshots.

## 8. Phased Delivery Plan

## Phase 1 (MVP, 2-3 weeks)

Must Have:

1. Monitoring blueprint using existing background runtime.
2. Typed config v1.
3. Schedule + manual triggers.
4. Two-role report generation.
5. Slack + UI delivery.

Optional / Nice-to-Have:

1. Config importer from `.sanity-check.md`.

## Phase 2 (V1 hardening, 2-4 weeks)

Must Have:

1. Trend/delta comparison with baseline runs.
2. Stronger retries and failure classification.
3. Cost controls and per-agent quotas.

Optional / Nice-to-Have:

1. GitHub PR comment delivery.
2. Webhook delivery.

## Phase 3 (V2 expansion)

Must Have:

1. GitHub writeback modes (`comment` + `commit`) selectable.

Optional / Nice-to-Have:

1. `github_check_run` delivery.
2. Optional dedicated runtime mode (`monitoring_audit`).
3. Multi-engine backend abstraction.

## 9. Risks and Mitigations

## 9.1 False positives / noisy updates

Must Have:

1. Severity threshold and dedupe before posting.
2. "only changed findings" mode per Slack thread.

Optional / Nice-to-Have:

1. Confidence scoring model.

## 9.2 Cost blow-ups

Must Have:

1. Hard run timeout and repo count limits.
2. Schedule frequency guardrails.
3. Per-agent/token budget telemetry.

Optional / Nice-to-Have:

1. Dynamic backoff when no changes detected.

## 9.3 Delivery channel failures

Must Have:

1. Delivery attempt table and retry policy by channel.
2. Non-blocking delivery failure handling (run success independent from delivery).

Optional / Nice-to-Have:

1. Secondary fallback channel routing.

## 10. Why This Fits Moontide Better Than Porting `sanity-check` As-Is

Must Have:

1. Aligns with existing Moontide primitives:
   1. trigger scheduler
   2. run queue + lock
   3. runtime visibility APIs
   4. Slack and GitHub integration stack
   5. E2B/OpenCode execution
2. Keeps product experience in Agent Factory instead of external CLI workflows.

Optional / Nice-to-Have:

1. External runner adapter for niche cases.

## 11. External Docs / Research Notes

Used to validate delivery/trigger/tool constraints:

1. OpenCode CLI supports non-interactive runs, JSON output, and headless serve/attach patterns:
   1. https://opencode.ai/docs/cli/
2. Claude Code CLI references `-p` and `--dangerously-skip-permissions` (relevant to parity analysis, not recommended as primary runtime path):
   1. https://code.claude.com/docs/en/cli-reference
3. Slack incoming webhook behavior and constraints (channel binding, thread replies, error semantics):
   1. https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/
4. Slack message sizing guidance (`chat.postMessage`: best around 4,000 chars, truncation beyond 40,000):
   1. https://docs.slack.dev/reference/methods/chat.postMessage
5. GitHub webhook event coverage and payload constraints:
   1. https://docs.github.com/en/webhooks/webhook-events-and-payloads
6. GitHub Actions schedule caveats (if using workflow-based triggers in future):
   1. https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows
7. GitHub App installation token lifecycle (1-hour expiry and scoped repository/permission options):
   1. https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app
8. GitHub REST endpoints for PR comments and Checks integration options:
   1. https://docs.github.com/en/rest/issues/comments
   2. https://docs.github.com/en/rest/checks/runs

## 12. Implementation Readiness Checklist

Must Have:

1. Confirm typed monitoring schema.
2. Confirm Slack channel UX for proactive posting cadence.
3. Confirm role prompt packs and report schema.
4. Implement delivery-target extensions and validators.
5. Add runtime and delivery observability counters.

Optional / Nice-to-Have:

1. `.sanity-check.md` importer.
2. GitHub writeback modes.
3. Delta-only posting mode.

## 13. Appendix A: Runtime Logic of the `sanity-check` CLI

This appendix explains how `sanity-check` is actually built and what happens at runtime when triggered.

## 13.1 Runtime entrypoints

`sanity-check` can be triggered in two primary ways:

1. Local CLI run:
   1. `sanity-check [flags]`
   2. executes `src/cli.ts`
2. GitHub Action run:
   1. workflow executes Docker action (`action.yml` -> `Dockerfile` -> `entrypoint.sh`)
   2. `entrypoint.sh` converts action inputs into CLI flags
   3. runs `bun run /app/src/cli.ts ...` inside checked-out repo workspace

## 13.2 Top-level control flow (`src/cli.ts`)

When triggered, control flow is:

1. Parse CLI flags with Commander:
   1. `--slack`
   2. `--pr`
   3. `--webhook <url>`
   4. `--config <path>`
   5. `--timeout <seconds>`
2. Resolve cwd and date stamp.
3. Load config:
   1. default `.sanity-check.md` is optional
   2. explicit `--config` path is required (errors if missing)
4. Gather git metadata:
   1. repo name from `git remote get-url origin`
   2. short SHA from `git rev-parse --short HEAD`
5. Initialize debug log path under `.sanity-check/debug-YYYY-MM-DD.log`.
6. Build main prompt:
   1. load `src/prompts/base.md`
   2. append repo config content if present
7. Run first Claude session (main findings).
8. Build summary prompt (`src/prompts/summary.md` + first output).
9. Run second Claude session (summary synthesis).
10. Assemble markdown report:
    1. header (repo/date/commit)
    2. summary
    3. findings block
11. Deliver:
    1. stdout always
    2. PR optional
    3. Slack optional
    4. webhook optional
12. Exit code 0 on normal completion path.

## 13.3 Prompting pipeline

Implemented prompt files in runtime:

1. `base.md`:
   1. long-form instruction set for autonomous repo sanity execution
   2. includes setup/testing/report rules
2. `summary.md`:
   1. asks for 2-3 line executive summary from generated findings

How prompts are assembled:

1. Main run: `base.md` + optional repository-specific markdown from `.sanity-check.md`.
2. Summary run: `summary.md` + main run output text.

## 13.4 Runner internals (`src/runner.ts`)

The runner wraps `claude` process execution:

1. Spawn command:
   1. `claude -p <prompt> --output-format text --max-turns 200 --dangerously-skip-permissions`
2. Stream handling:
   1. capture stdout/stderr
   2. append stderr to debug log
3. Timeout:
   1. optional per-run timeout via `--timeout`
   2. sends SIGTERM then SIGKILL after grace period
4. Result parse:
   1. if exit 0 -> success
   2. otherwise preserve partial output and surface stderr/error reason
5. Cleanup:
   1. recursively finds descendant PIDs (`pgrep -P`)
   2. sends SIGTERM to descendants to avoid orphaned app/test processes

## 13.5 Delivery logic

## 13.5.1 Stdout

1. Always prints full report to stdout.

## 13.5.2 PR delivery (`src/delivery/pr.ts`)

1. Determine default branch.
2. Build deterministic branch name `sanity-check/<date>` with suffix fallback.
3. If open PR exists for that branch:
   1. rewrite branch from base
   2. update report file
   3. force push
4. Else:
   1. create branch
   2. add report file
   3. commit/push
   4. open PR via `gh pr create`
5. Restore original local branch best-effort.

## 13.5.3 Slack delivery (`src/delivery/slack.ts`)

1. Requires `SLACK_WEBHOOK_URL`.
2. Converts markdown to Slack mrkdwn.
3. Truncates long payloads:
   1. prioritize Summary + Critical content
4. Posts webhook payload with timeout guard.

## 13.5.4 Webhook delivery (`src/delivery/webhook.ts`)

1. POST JSON payload:
   1. `repo`
   2. `date`
   3. `commit`
   4. `report`
2. Uses fetch timeout guard and error logging.

## 13.6 Practical runtime sequence (end-to-end)

Simplified sequence:

1. Trigger fires (CLI or Action).
2. CLI loads config + git metadata.
3. Main Claude run executes repo sanity workflow.
4. Summary Claude run compresses output.
5. Report assembled.
6. Report delivered to selected channels.
7. Child processes cleaned up.

## 13.7 Current implementation quirks relevant to Moontide design

Important for parity planning:

1. Role-model drift:
   1. public docs/readme mention engineer + QA split
   2. current implementation executes one main run + one summary run
2. Action/CLI contract mismatch:
   1. action entrypoint forwards `--role`
   2. CLI currently does not define `--role`
3. Stall handling drift:
   1. docs mention auto-writing "continue" on idle
   2. runner currently logs idle but uses `stdin: ignore`
4. Summary run uses same high max-turn profile as main run.

These quirks are exactly why Moontide should adopt patterns, not clone implementation literally.

## 13.8 Why this appendix matters for Moontide

For Moontide Agent Factory design, this runtime analysis shows:

1. What to reuse:
   1. prompt pipeline
   2. report synthesis
   3. multi-channel delivery
2. What to avoid:
   1. contract drift between docs/action/CLI
   2. host-level unbounded autonomy model
3. What to replace with Moontide-native primitives:
   1. queued background runs
   2. typed config validation
   3. runtime visibility + policy-bound sandbox execution
