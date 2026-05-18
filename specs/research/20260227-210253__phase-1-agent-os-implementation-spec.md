# Moontide AI - Phase 1 Agent OS Implementation Spec

**Date:** 2026-02-27  
**Status:** Draft (Scope-Locked for Phase 1)  
**Source:** `docs/20260227-161653__agent-first-product-ideas-draft.md`

---

## 1. Business Objective

Ship the first real version of the agent operating system with:

1. A user-facing Agent Factory that can define and run custom agents.
2. Event-driven execution via GitHub PR-opened and scheduled triggers.
3. Mission Control visibility for all triggered and interactive runs.
4. A strict Phase 1 focus on single-agent execution (no multi-agent teams yet).

---

## 2. Phase 1 Scope

1. Agent Factory v1 (create/edit/activate/deactivate user-defined agents).
2. Runtime support for:
   - Slack thread-scoped interaction mode.
   - Background trigger-driven mode.
3. Trigger Engine v1:
   - GitHub `pull_request` opened.
   - Cron-style scheduled trigger.
   - Slack on-demand trigger.
4. Mission Control v1:
   - runs list
   - run detail timeline
   - artifacts/reports
   - failure status
5. Two validated user-created analyzer agents:
   - Functional Test Coverage Analyzer
   - Technical Architecture Adherence Analyzer

---

## 3. Out of Scope (Deferred)

1. Multi-agent team orchestration and coordinator/worker topologies.
2. Memory system productization (thread/agent/org memory controls).
3. Persistent background "employee" identity semantics beyond standard triggered runs.
4. Full production hardening program (moved to later phase).

---

## 4. Phase 1 MVP Validation Criteria

1. With Agent Factory, user can create the same Slack thread-scoped Q&A agent behavior that exists today.
2. With Agent Factory + triggers, user can create and run:
   - one Functional Test Coverage Analyzer
   - one Technical Architecture Adherence Analyzer
3. Trigger types required for MVP:
   - PR opened
   - scheduled
   - Slack on-demand

---

## 5. Product Behavior Contract

## 5.1 Agent Factory

1. Users define agents via explicit configuration, not fixed built-in agent types.
2. Agent definition includes:
   - `owner_user_id`
   - `config_version` (increment on every config change)
   - `status` (`draft`, `active`, `inactive`)
   - `name`
   - `objective`
   - `instructions`
   - `integration bindings` (GitHub, Slack)
   - `repo scope` (allowlisted repos)
   - `runtime mode` (`slack_thread`, `background_triggered`)
   - `output contract` (required report schema/format)
   - `output delivery` (Mission Control only and/or external destinations such as Slack/GitHub)
   - `tooling profile`
   - `permission profile`
3. Users can activate/deactivate agents.

## 5.2 Triggered Runs

1. PR-opened trigger:
   - GitHub webhook event enqueues a run for eligible agents.
   - run analyzes repositories in scope.
   - run produces a structured report artifact.
2. Scheduled trigger:
   - scheduler enqueues runs on cron schedule using the Phase 1 cron profile (Section 13).
   - run produces a structured report artifact.
3. Slack-triggered background run:
   - users can explicitly trigger an agent run from Slack.
   - run is created as a background run with Slack metadata in trigger payload.
4. Multiple trigger rules per agent are allowed (including multiple rule types).
5. Each matching trigger rule creates an independent run candidate with deterministic dedupe.
6. Triggered runs are asynchronous and visible in Mission Control.
7. If a run is already active for the same agent, new runs queue (FIFO per agent).

## 5.3 Interactive Slack Runs

1. Existing thread-scoped Q&A behavior remains supported via Factory-created agent config.
2. Slack mention starts/reuses thread session as it does today.

## 5.4 Outputs

1. Every run has a final normalized artifact:
   - summary
   - key findings
   - evidence references
   - recommended actions
2. Mission Control always shows final run outcome (`completed`, `failed`, `terminated`).
3. External delivery targets are determined by agent config (`output delivery`), not global defaults.

## 5.5 Tooling and Permission Enforcement

1. Every active agent must define `tooling_profile` and `permission_profile`.
2. `tooling_profile` includes:
   - `sandbox_template`
   - `enabled_tools` (explicit allowlist)
   - optional network mode (`none`, `github_only`, `restricted`)
3. `permission_profile` includes:
   - `default_mode` (`allow`, `ask`, `deny`)
   - per-tool/per-action overrides
4. Activation guardrails:
   - agent cannot activate without valid tooling + permission profiles
   - agent cannot activate without repo scope and required integrations
5. Runtime guardrails:
   - tools/actions not in `enabled_tools` are blocked
   - unspecified tool/action uses `default_mode`
   - `ask` routes to approval in the configured Slack approval channel
6. Approval behavior:
   - any workspace member can approve/deny permission requests
   - approved permissions are persisted per agent for future runs
   - persisted grants do not expire by default in Phase 1
   - if approval channel delivery fails, runtime fails closed for that action

---

## 6. Functional Requirements

### FR-01 Agent Definition Model

1. Persist Agent Factory definitions in durable DB schema.
2. Support agent lifecycle state (`draft`, `active`, `inactive`).
3. Track `owner_user_id` and `config_version`.
4. Enforce integration prerequisites before activation.

### FR-02 Repo Scope Enforcement

1. Agent config stores deterministic repo allowlist.
2. Triggered and Slack runs can only access allowlisted repos.
3. GitHub installation tokens remain scoped to selected repository IDs.

### FR-03 Trigger Rules

1. New trigger rules table supports:
   - `github_pr_opened`
   - `schedule`
   - `slack_on_demand`
2. Rule config stores:
   - enabled/disabled state
   - target agent
   - filter/config (org/repo filters or cron expression)
   - deterministic dedupe key strategy
3. Webhook/scheduler must be idempotent with dedupe keying.
4. Phase 1 scheduled trigger profile:
   - 5-field cron only (`minute hour day-of-month month day-of-week`)
   - UTC timezone only
   - minute-level granularity (no seconds field)
   - no extended cron syntax (`L`, `W`, `#`, `?`, `@reboot`, nicknames)

### FR-04 Trigger Execution Runtime

1. Trigger events enqueue background run jobs.
2. Worker processes jobs and writes stage events.
3. Retry behavior for transient failures (bounded attempts).
4. Terminal failures are visible in Mission Control with error reason.
5. Matching logic:
   - one run candidate per matching trigger rule
   - dedupe key includes `agent_definition_id`, `trigger_rule_id`, and external event identity when present
6. Concurrency behavior:
   - per-agent FIFO queue when overlapping runs occur
   - configurable per-agent queue max length; overflow drops oldest queued run with explicit error event

### FR-05 Mission Control v1

1. Runs list API with filters:
   - agent
   - trigger type
   - status
   - time window
2. Run detail API:
   - timeline events
   - trigger payload metadata
   - final artifact
   - error details
3. Optional action API:
   - retry failed run
   - terminate active run

### FR-06 Analyzer Agent Output Contracts

1. Functional Test Coverage Analyzer output must include:
   - tested functional areas found
   - suspected gaps
   - confidence per gap
   - suggested next tests
2. Technical Architecture Adherence Analyzer output must include:
   - architecture rules checked
   - violations/mismatches
   - affected files/components
   - remediation suggestions

### FR-07 Slack Q&A Parity via Factory

1. A Factory-created thread-scoped Q&A agent must pass current E2E thread behavior.
2. No regression in mention->answer flow for existing setup.

### FR-08 Tooling Profile

1. Persist `tooling_profile` in agent definition contract.
2. Runtime receives only allowed tool configuration for each run.
3. Sandbox bootstrap enforces the profile before task execution.

### FR-09 Permission Profile

1. Persist `permission_profile` in agent definition contract.
2. Runtime enforces `default_mode` + overrides for every tool/action.
3. `ask` mode emits approval requests and blocks action until resolved.

### FR-10 Persistent Permission Grants

1. Persist permission decisions keyed by:
   - `workspace_id`
   - `agent_definition_id`
   - `action`
   - `resource_pattern`
2. Approval/denial decisions can be submitted by any workspace member.
3. Grant lookup is evaluated before emitting a new `ask` approval request.
4. Grants are agent-scoped and do not expire by default in Phase 1.
5. If approval-channel routing fails, action defaults to deny/fail-closed.

### FR-11 Config Snapshot and Queue Safety

1. Every enqueued run binds to immutable agent config snapshot/version at enqueue time.
2. Later config edits do not mutate queued/running runs.
3. Queue ordering is FIFO per agent.
4. Queue metadata is visible in Mission Control timeline/events.

### FR-12 Retention Defaults (Phase 1)

1. Retention defaults:
   - run metadata/events: 90 days
   - run artifacts: 180 days
   - raw stream/transcript logs: 30 days
2. Purge jobs must run automatically and be idempotent.
3. Mission Control must not show expired payload data after retention windows elapse.
4. Phase 1 treats these as system defaults (no per-agent overrides).

---

## 7. Data Model Additions (Conceptual)

1. `agent_definitions` (extend existing as needed)
   - owner_user_id
   - config_version
   - objective/instructions/runtime_mode/output_contract/output_delivery
   - tooling_profile_json
   - permission_profile_json
2. `agent_trigger_rules`
   - id, agent_definition_id, trigger_type, config_json, enabled, created_at, updated_at
3. `agent_runs`
   - id, agent_definition_id, trigger_type, trigger_rule_id, trigger_key, config_version, status, started_at, finished_at, error_code, error_message
4. `agent_run_events`
   - id, run_id, stage, status, payload_json, created_at
5. `agent_run_artifacts`
   - id, run_id, artifact_type, content_markdown/content_json, created_at
6. `agent_permission_grants`
   - id, workspace_id, agent_definition_id, action, resource_pattern, decision, granted_by_user_id, created_at, updated_at
7. `agent_run_queue_state` (or equivalent fields in `agent_runs`)
   - queue_position, enqueued_at, dequeued_at, dropped_reason

Implementation note:
1. Existing `runtime_run_events` can be reused/extended instead of creating parallel tables if contracts remain clear.

---

## 8. API Surface (Conceptual)

## 8.1 Control Plane APIs

1. `CreateAgentDefinition`
2. `UpdateAgentDefinition`
3. `ListAgentDefinitions`
4. `SetAgentDefinitionStatus`
5. `ValidateAgentDefinitionActivation` (preflight validation for required config)
6. `CreateTriggerRule`
7. `UpdateTriggerRule`
8. `ListTriggerRules`
9. `DeleteTriggerRule`
10. `ListAgentPermissionGrants`
11. `UpsertAgentPermissionGrant`
12. `DeleteAgentPermissionGrant`

## 8.2 Mission Control APIs

1. `ListAgentRuns`
2. `GetAgentRun`
3. `ListAgentRunEvents`
4. `GetAgentRunArtifact`
5. `RetryAgentRun`
6. `TerminateAgentRun`

## 8.3 Ingress/Webhooks

1. `POST /webhooks/github/events`
   - handle `pull_request` opened
2. Scheduler loop
   - scans due scheduled trigger rules and enqueues runs
3. Slack trigger ingress
   - handles explicit Slack command/mention format to enqueue background agent runs

---

## 9. Architecture and Execution Flow

1. Control plane writes agent definitions and trigger rules.
2. GitHub webhook and scheduler normalize trigger events.
3. Trigger events enqueue background runs.
4. Runtime worker executes run in sandbox with repo-scoped token.
5. Worker writes run timeline and final artifact.
6. Mission Control reads these records for operator visibility.

---

## 10. Milestones

## M1 - Agent Factory Core

1. Agent definition schema and APIs.
2. Activation/deactivation rules.
3. Thread-scoped Q&A parity through Factory config.

## M2 - Trigger Engine

1. PR-opened webhook ingestion and dedupe.
2. Scheduled trigger loop and enqueue path.
3. Slack-triggered background enqueue path.
4. Background run worker path with timeline events + per-agent queueing.

## M3 - Mission Control + Analyzer Validation

1. Runs list/detail/event/artifact APIs + web views.
2. Functional Test Coverage Analyzer validation run.
3. Technical Architecture Adherence Analyzer validation run.
4. End-to-end MVP signoff against Section 4 criteria.

---

## 11. Testing Strategy (Strict TDD)

For each feature area:

1. Write failing tests first.
2. Implement minimal passing code.
3. Refactor with test suite green.
4. Add regression tests for every found bug.

Required suites:

1. Agent Factory CRUD/lifecycle validation.
2. Trigger ingestion and dedupe (GitHub + schedule).
3. Worker execution state transitions.
4. Mission Control query correctness.
5. Analyzer output contract validation.
6. Q&A parity regression tests for Slack thread mode.
7. Tooling/permission enforcement tests (`allow`, `ask`, `deny`, blocked tool).

---

## 12. Manual E2E Gate (Phase 1)

1. Create thread-scoped Q&A agent in Factory and verify Slack thread Q&A works end-to-end.
2. Create Functional Test Coverage Analyzer with PR-opened trigger and validate run appears in Mission Control with final artifact.
3. Create Technical Architecture Adherence Analyzer with scheduled trigger and validate scheduled run appears with final artifact.
4. Trigger an analyzer via Slack command and validate background run is enqueued and completed.
5. Verify failures are visible and retry action works for at least one forced transient error.
6. Verify repo scope boundaries are enforced across all three agents.
7. Verify a disallowed tool/action is blocked by runtime policy.
8. Verify an `ask`-mode action produces approval flow and only proceeds after approval.
9. Verify an approved permission is reused in later runs of the same agent without another approval prompt.
10. Verify overlapping triggers queue in FIFO order and execute sequentially.

---

## 13. Locked Phase 1 Defaults

1. Retention defaults:
   - run metadata/events: 90 days
   - run artifacts: 180 days
   - raw stream/transcript logs: 30 days
2. Scheduled trigger cron profile:
   - 5-field cron only (`minute hour day-of-month month day-of-week`)
   - UTC timezone only
   - minute-level granularity
   - no extended cron syntax (`L`, `W`, `#`, `?`, `@reboot`, nicknames)

---

## 14. External Docs Research Notes (Implementation Grounding)

This section captures external behavior contracts that should directly shape implementation.

### 14.1 OpenCode (latest docs + SDK/server spec)

1. Server and SDK runtime contract
- `opencode serve` exposes API docs at `/doc` and supports session/event APIs.
- For production runtime in Phase 1, prefer SDK-first integration (`@opencode-ai/sdk`) with:
  - `session.create()`
  - `session.prompt()`
  - `event.subscribe()`
- Keep the raw HTTP endpoints as fallback/debug path only.

2. Permission model is the primary enforcement path
- OpenCode permission actions are `allow`, `ask`, `deny`.
- Permission rules are evaluated in order where later matching rules override earlier rules.
- OpenCode docs mark legacy `tools` booleans as deprecated in favor of permission policy as of v1.1.1.
- Phase 1 implication: `permission_profile` is the source of truth; `tooling_profile` should compile into permission policy, not diverge.

3. Agent config semantics
- Agent config supports explicit model/provider plus mode (`primary`, `subagent`, `all`) and policy/tool overrides.
- We remain single-agent in Phase 1 but must keep definition schema forward-compatible for subagent/team mode in later phases.

4. Event stream semantics
- SDK event subscription can emit heterogeneous bus events; do not rely on one hardcoded event subtype set.
- Runtime should parse typed events, persist normalized events, and only publish user-facing messages from assistant output events (not internal logs).

### 14.2 E2B (latest sandbox + command + security docs)

1. Sandbox lifecycle and timeout
- Default sandbox timeout is 5 minutes unless overridden.
- `setTimeout` resets timeout from "now" and should be used as run heartbeat extension.
- Tier limits matter: max continuous runtime differs by plan.

2. Reuse and identity
- `Sandbox.connect()` is the canonical way to reuse running sandboxes.
- Metadata is first-class and should be used to tag sandbox with `agent_definition_id`, `run_id`, and trigger source for diagnostics.

3. Command execution behavior
- `commands.run()` supports `background`, `timeoutMs`, `requestTimeoutMs`, `cwd`, `envs`, `onStdout`, `onStderr`.
- SDK command timeout defaults are short for long-running agent tasks (for example 60s in command references), so every long command path must set explicit timeout budgets.

4. Network and secure access
- Internet access is configurable (`allowInternetAccess`).
- Secure access is default in modern SDK versions and should stay enabled for production behavior.

5. Template reliability
- Custom template build/readiness model is snapshot-based; runtime assumptions should match template capabilities.
- Phase 1 should pin and validate template/tooling versions in setup validation.

### 14.3 GitHub (latest app + webhook docs)

1. PR-opened trigger source
- Trigger should consume `pull_request` webhook with `action=opened`.
- GitHub App must have required permissions (at least pull request read access for this event domain).

2. Webhook security
- Validate `X-Hub-Signature-256` with HMAC-SHA256 over raw UTF-8 payload.
- Use constant-time comparison, reject on signature mismatch.

3. Installation token scoping
- Installation tokens expire in 1 hour.
- Token creation supports `repository_ids` and optional reduced `permissions`.
- We should mint scoped tokens per run/turn and never issue broad "all repo" tokens when allowlist is known.

### 14.4 OSS Agent Factory Patterns (for product and runtime decisions)

1. Dify trigger architecture
- Trigger-first workflow model supports schedule + event/plugin/webhook sources.
- Trigger source is surfaced in execution context/history.
- Multiple triggers per workflow are practical and common.

2. CrewAI config style
- Agent and task definitions are treated as declarative config with explicit inputs/outputs.
- Flows emphasize state-driven execution with persistent IDs and event routing.
- This reinforces our "Agent Factory as declarative contract + run state machine" direction.

3. Flowise orchestration split
- Clear split between chatflow and agentflow with checkpoint/restart and human-in-the-loop controls.
- Reinforces our phase split: single-agent Phase 1, orchestration and HITL expansion in Phase 2+.

4. LangGraph durable execution pattern
- Durable execution centers on persistence/checkpointing, thread/run identifiers, and resumability.
- This validates our requirement for durable run/event records and deterministic retry boundaries.

### 14.5 Phase 1 Implementation Implications (locked from research)

1. `permission_profile` is the authoritative runtime policy and must map directly to OpenCode `allow/ask/deny` + ordered patterns.
2. `tooling_profile` must compile to OpenCode-compatible policy constraints and runtime template/tool prerequisites.
3. Trigger execution records must include explicit trigger source/type in Mission Control.
4. GitHub PR-opened path must enforce webhook signature verification and installation-token repo scoping.
5. E2B runs must set explicit command/sandbox timeout budgets and heartbeat timeout extensions.
6. Runtime must prefer SDK event streams and publish only end-user-safe output events to Slack.
7. Run/sandbox/session IDs must be correlation-first across BFF logs, Mission Control timeline, and sandbox metadata.

### 14.6 Concrete Implementation Mapping (to remove ambiguity before coding)

1. Agent definition to OpenCode mapping
- `permission_profile.default_mode` -> default permission action.
- `permission_profile.overrides[]` -> ordered permission rules (latest rule wins).
- `tooling_profile.enabled_tools[]` -> generated permission rules that deny all non-listed tools/actions.
- `tooling_profile.sandbox_template` -> E2B template ID selection.
- `runtime_mode` -> either Slack thread executor path or trigger worker path.
- `output_delivery` -> Mission Control persistence plus optional external publisher adapters (Slack/GitHub).

2. Trigger event normalization
- Normalize GitHub PR-opened and schedule events into one `AgentTriggerEvent` contract:
  - `trigger_type`
  - `trigger_rule_id`
  - `trigger_key` (dedupe key)
  - `workspace_id`
  - `agent_definition_id`
  - source payload metadata (repo/org/pr number or schedule id/time)
- Persist both raw payload hash and normalized fields for idempotency and diagnostics.

3. Run lifecycle state contract (Phase 1)
- `queued -> starting -> running -> completed|failed|terminated`
- `awaiting_permission` substate is required when OpenCode emits `ask`.
- Every transition writes a timeline event with correlation IDs.

4. Timeout budgeting
- Distinguish:
  - sandbox lifetime budget (`Sandbox.create/connect` timeout + sandbox TTL)
  - command execution budget (`commands.run timeoutMs`)
  - SDK request budget (`requestTimeoutMs`)
  - run overall budget (worker-level guardrail)
- Budget values must be explicit config, not SDK defaults.

5. Slack output safety rule
- Never mirror raw prompt/system scaffolding.
- Stream only assistant-user-visible text and explicit permission prompts.
- Persist final artifact separately from stream transcript.

6. Approval and persistence rule
- `ask` approvals are posted to a configured Slack approval channel.
- Any workspace member can approve/deny.
- Approved decisions persist per agent/action/resource pattern and apply to future runs.

7. Queue and config snapshot rule
- Overlapping runs for an agent queue FIFO.
- Each run executes against immutable config snapshot captured at enqueue time.

### 14.7 References

1. OpenCode Intro: https://opencode.ai/docs
2. OpenCode Server: https://opencode.ai/docs/develop/server
3. OpenCode SDK: https://opencode.ai/docs/develop/sdk
4. OpenCode Permissions: https://opencode.ai/docs/config/permissions
5. OpenCode Agents: https://opencode.ai/docs/config/agents
6. OpenCode Tools: https://opencode.ai/docs/config/tools
7. OpenCode source types/events: https://raw.githubusercontent.com/anomalyco/opencode/dev/packages/opencode/src/index.ts
8. E2B Quickstart (timeouts/lifecycle): https://e2b.dev/docs/quickstart
9. E2B sandbox create/connect: https://e2b.dev/docs/sdk-reference/js-sdk/v2.8.4/sandbox/create
10. E2B command execution options: https://e2b.dev/docs/sdk-reference/js-sdk/v2.8.4/commands/command-start
11. E2B template concepts: https://e2b.dev/docs/sandbox-template
12. GitHub webhook signature validation: https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries
13. GitHub pull request events/payloads: https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request
14. GitHub installation token generation (`repository_ids`): https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app
15. Dify trigger docs: https://docs.dify.ai/en/guides/workflow/node/trigger
16. CrewAI flows: https://docs.crewai.com/en/concepts/flows
17. Flowise Agentflow docs: https://docs.flowiseai.com/using-flowise/agentflow
18. LangGraph durable execution: https://docs.langchain.com/oss/javascript/langgraph/durable-execution

---

## 15. Implementation-Readiness Additions (Pre-Implementation Locks)

These details are added to remove ambiguity before full implementation.

### 15.1 Tenant and Authorization Contract

1. Every control-plane and Mission Control read/write operation must be scoped by `workspace_id`.
2. `owner_user_id` indicates creator, but all authenticated workspace members can:
   - view active agent definitions,
   - trigger runs from allowed channels,
   - review run history,
   - respond to permission requests.
3. Activation, deactivation, and destructive actions (delete/archive) require a workspace-level write role gate.
4. Cross-workspace data access must hard-fail even if IDs are guessed.

### 15.2 Trigger Eligibility Rules

1. A trigger rule is eligible only when:
   - agent status is `active`,
   - required integrations are connected and healthy,
   - repo scope is non-empty,
   - trigger rule is `enabled`.
2. If eligibility fails, no run is enqueued; write a validation event with reason.
3. Trigger matching for GitHub PR-opened is repository-scoped: event repository must be in the agent allowlist.

### 15.3 Idempotency and Dedupe Keys

1. GitHub ingress dedupe key: `workspace_id + provider + delivery_id`.
2. Trigger-run dedupe key: `agent_definition_id + trigger_rule_id + external_event_identity`.
3. Scheduler dedupe key: `agent_definition_id + trigger_rule_id + scheduled_fire_at_utc`.
4. Slack on-demand dedupe key: `workspace_id + channel_id + thread_ts + normalized_command + time_bucket`.
5. Dedupe collisions return deterministic status (`deduped`) with no side effects.

### 15.4 Retry and Failure Classification

1. Retries are bounded and exponential backoff with jitter.
2. Retryable classes (default): transient network errors, provider 5xx, sandbox infra failures.
3. Non-retryable classes (default): auth failure, permission denied, invalid config, schema validation failure.
4. Each failure event must include:
   - `error_code`,
   - `retryable` boolean,
   - `attempt`,
   - `next_retry_at` (if scheduled).

### 15.5 Scheduler Semantics

1. Scheduler ticks every minute (UTC).
2. Missed tick policy during downtime: catch up only for the last 15 minutes.
3. Catch-up runs still use scheduler dedupe keys to prevent duplicate enqueue.
4. If a rule remains invalid (for example bad cron), persist `invalid` state and surface in Mission Control.

### 15.6 Output Delivery Contract

1. Mission Control artifact persistence is mandatory for all runs.
2. External delivery is best-effort with retries and explicit terminal failure event.
3. Delivery destinations for Phase 1:
   - Slack thread message,
   - Slack channel message,
   - GitHub PR comment (for PR-triggered runs).
4. Delivery payload must include a stable `run_id` reference for traceability.

### 15.7 Mission Control Query Contract

1. List endpoints must support:
   - pagination (`cursor` + `page_size`),
   - deterministic sort (`created_at desc`, secondary `id desc`),
   - filters (`agent`, `trigger_type`, `status`, `time window`).
2. Detail endpoint must include:
   - run header,
   - normalized trigger metadata,
   - event timeline,
   - artifact summary,
   - delivery attempts.
3. Timeline/event payload must avoid secrets and raw credential material.

### 15.8 Activation Validation Contract

1. Activation preflight must validate:
   - required integrations,
   - repo scope,
   - trigger rule validity,
   - output contract schema presence,
   - tooling/permission profile validity.
2. Activation returns structured validation errors keyed by field path.
3. Activation is atomic: either fully active or unchanged.

### 15.9 Phase 1 Operational Limits

1. Default per-agent queue max length: 20.
2. Default concurrent active runs per agent: 1.
3. Default run hard timeout: 20 minutes.
4. Default sandbox TTL must be >= run hard timeout.
5. Oversized artifacts (beyond configured limit) must be truncated with explicit marker.

### 15.10 Observability and Audit Minimums

1. Every run/event/delivery record carries correlation IDs:
   - `run_id`,
   - `agent_definition_id`,
   - `trigger_rule_id` (if any),
   - `sandbox_id` (if any),
   - `opencode_session_id` (if any).
2. Metrics required in Phase 1:
   - runs started/completed/failed/terminated,
   - queue depth by agent,
   - trigger ingest accepted/deduped/rejected,
   - delivery success/failure.
3. Security-sensitive actions (connect/disconnect, activation, terminate, permission grant) must emit audit events.

---

## 16. Resolved Product Decisions (Locked 2026-03-02)

1. Roles and permissions model:
   - All workspace members can activate/deactivate/delete agents in Phase 1.
2. Slack on-demand command format:
   - Lock to strict command: `@App trigger run <agent>`.
3. PR comment delivery:
   - Configurable per trigger rule.
   - Default is `on` when not explicitly set.
4. Scheduler catch-up window:
   - 15 minutes.
5. Artifact size limit:
   - Max artifact payload size is 512 KB in Phase 1.
6. Retry budget:
   - Run retries: 3 attempts with exponential backoff (`10s`, `30s`, `90s`) + jitter.
   - Delivery retries: 8 attempts with exponential backoff capped at 15 minutes + jitter.
7. Termination authority:
   - Any workspace member can terminate active runs.
8. Visibility scope:
   - All runs are visible to all workspace members.
9. Queue overflow policy:
   - Reject newest enqueue request with explicit backpressure event.
10. Approval channel policy:
   - Activation fails if `ask` mode is configured without an approval channel.

---

## 17. Appendix A - OSS Product Research Synthesis (No Scope Lock Yet)

This appendix captures relevant product/runtime patterns from adjacent OSS systems.  
It is intentionally informational and does not alter the implementation contract above.

### 17.1 Sources Reviewed

1. Builderz Labs Mission Control (GitHub):
   - https://github.com/builderz-labs/mission-control
2. Nous Hermes Agent (GitHub):
   - https://github.com/NousResearch/hermes-agent
3. X post reference (not retrievable in this environment due access constraints):
   - https://x.com/ashpreetbedi/status/2028176285575594465

### 17.2 Observed Patterns - Mission Control

1. Product focus is control-plane visibility and orchestration UX:
   - runs/tasks/panels, real-time updates, role-based access, review gates.
2. Strong operational ergonomics:
   - outbound webhook history + retry tracking,
   - scheduler visibility,
   - structured logging and quality gate commands.
3. Gateway abstraction pattern:
   - supports multiple gateways and direct CLI integrations.
4. Notable maturity caveat:
   - project self-labels as alpha; roadmap still includes webhook robustness/security items.

### 17.3 Observed Patterns - Hermes Agent

1. Unified daemon pattern:
   - one gateway process handles messaging sessions + cron + delivery.
2. Messaging guardrails:
   - per-platform allowlists (`*_ALLOWED_USERS`) and optional global bypass.
3. Persistent operator context:
   - memory files, skill storage, cron data, logs persisted under home state directory.
4. Runtime safety patterns:
   - explicit exec approvals in chat channels,
   - sandbox backend flexibility (local/docker/ssh/singularity/modal),
   - documented container hardening defaults.
5. Multi-agent delegation controls:
   - subagent depth limits and blocked tool classes for child agents.

### 17.4 Cross-Project Synthesis for Moontide Phase 1

1. Control-plane + runtime split is validated:
   - Mission Control-like run visibility should remain separate from execution internals.
2. Durable event history is a product feature, not only an ops tool:
   - timeline, attempts, delivery outcomes, and review state should be first-class.
3. Approval and command guardrails must be policy-driven:
   - allowlists + role gates + explicit approval flows are recurring patterns.
4. Scheduler reliability needs explicit duplicate-prevention semantics:
   - lock/dedupe/catch-up behavior should be visible and auditable.
5. Runtime backend abstraction is useful even in single-agent phase:
   - keeps future migration paths open (provider/runtime changes without product rewrite).

### 17.5 Candidate Patterns to Potentially Incorporate

1. Quality review gate on background analyzers before marking run as "final".
2. Direct CLI/worker registration + heartbeat endpoint for worker health visibility.
3. Run attempts table semantics surfaced in Mission Control UI.
4. Explicit delivery attempt history for Slack/GitHub outputs.
5. Optional per-agent Slack user/channel allowlists for control commands.
6. Saved Mission Control views/filters for operator workflows.
7. Subagent guardrail schema placeholders (forward-compat, disabled in Phase 1).

---

## 18. Appendix B - Questions to Incorporate Research Findings

These are the remaining decisions needed to convert Appendix A into implementation scope.

1. Quality gate behavior:
   - Should analyzer runs require manual "approve" before being considered completed, or ship as informational only in Phase 1?
2. Command authorization:
   - For Slack-triggered background commands, should we add explicit per-agent allowed user/channel lists now?
3. Worker heartbeat visibility:
   - Do you want explicit "worker online/offline/last heartbeat" surfaced in Mission Control in Phase 1?
4. Delivery telemetry:
   - Should delivery-attempt history be customer-visible in Mission Control detail, or operator-only for now?
5. Attempt model:
   - Do we expose `run + attempts` as separate UI concepts immediately, or keep attempts nested only in timeline events?
6. Scheduler diagnostics:
   - Should we expose scheduler lock/catch-up/dedupe events in Mission Control, or keep scheduler internals log-only?
7. Runtime backend abstraction:
   - Keep E2B-only hard lock for Phase 1, or add an internal adapter interface now (single implementation)?
8. Subagent forward-compat:
   - Add disabled config fields for delegation limits now, or defer entirely to Phase 2?
9. Role model granularity:
   - Keep "all members" for all actions in Phase 1, or introduce minimal admin-only actions now (delete/archive)?
10. X-post alignment:
   - Can you share screenshots/text from the referenced X thread so we can map specific ideas precisely into this appendix?
