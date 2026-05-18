# Moontide AI - Prompt and Tooling E2E Product Spec

**Date:** 2026-03-06  
**Status:** Draft  
**Type:** Product spec (behavior contract, not implementation tasks)

---

## 1. Objective

Define how Moontide should work end-to-end for:

1. Prompt creation and prompt lifecycle across agent sessions.
2. Tooling and permission enforcement as typed product behavior.
3. Slack thread and background run behavior with clear runtime contracts.

This spec is meant to keep the product flexible (user-defined agents) while making runtime behavior deterministic and operable.

---

## 2. Product Principles

1. User-defined agents first.
2. Typed config first; raw JSON is internal.
3. Session memory first; prompt replay as recovery path.
4. Runtime enforcement in code, not prompt-only.
5. Fast user interaction, async execution, full observability.

---

## 3. Core Concepts

1. `Agent Definition`
- User-authored configuration for behavior, scope, tooling, permissions, and delivery.

2. `Runtime Session`
- Stateful conversation context for an agent in a thread (or equivalent context for background runs).

3. `Agent Run`
- One execution unit triggered by a Slack message or background trigger.
- For Slack-thread agents, each relevant user message creates a run tied to the same runtime session.

4. `Thread Binding`
- Mapping from Slack thread to exactly one active agent definition for that thread.

5. `Action Registry`
- Canonical typed list of runtime actions (mapped to tool calls/events) used for policy enforcement.

---

## 4. End-to-End User Flows

## 4.1 Onboarding

1. User signs up/logs in.
2. User connects Slack and GitHub integrations.
3. System validates integration health and visible scopes.
4. User can proceed to Agent Factory.

## 4.2 Agent Creation

1. User enters name + intent.
2. AI config assistant generates a first draft config.
3. User edits typed fields (objective, instructions, repo scope, runtime mode, tooling, permissions, delivery).
4. User activates agent only if validation passes.

## 4.3 Slack Thread Run

1. User mentions app in a root message.
2. App resolves target agent:
- If thread already bound: use bound agent.
- If not bound and multiple candidates exist: require explicit selection.
3. System creates/reuses runtime session and executes run.
4. App posts response in the same thread.
5. Follow-up thread messages route to the same bound agent.

## 4.4 Background Run

1. Trigger event arrives (PR opened, schedule, or Slack on-demand).
2. Matching active agents enqueue runs.
3. Worker executes runs asynchronously.
4. Results are delivered per typed delivery targets and visible in Mission Control.

---

## 5. Prompt System Product Contract

## 5.1 Prompt Layers

Each run uses layered prompt composition:

1. `Platform Layer` (stable, versioned)
- Core runtime rules and output safety constraints.

2. `Agent Layer` (user-defined)
- Objective + instructions + configured behavior.

3. `Run Context Layer` (system-assembled)
- Trigger metadata, repository scope, relevant integration context.

4. `Turn Layer` (latest user input)
- Current message content, sanitized and normalized.

## 5.2 Session Bootstrap vs Follow-up

1. First turn in a new runtime session includes full layers.
2. Follow-up turns should send only:
- Latest sanitized user message.
- Minimal run metadata required by runtime.
3. Do not replay full transcript/policy every turn in normal path.

## 5.3 Re-bootstrap Conditions

Full re-bootstrap is allowed only when:

1. Runtime session is lost/recreated.
2. Agent config version changes.
3. Explicit operator/user reset action is executed.

## 5.4 Prompt Versioning and Auditability

1. Compiled prompt metadata is versioned.
2. Each run stores:
- Prompt version reference.
- Agent config version.
- Effective policy snapshot reference.
3. Mission Control can show which prompt/policy version produced a result.

## 5.5 Output Behavior

1. No rigid required output section templates in prompt.
2. Default output is concise, Slack-appropriate, evidence-driven.
3. If agent needs structured artifacts, structure is defined by typed runtime artifact schema, not fragile prompt prose.

---

## 6. Tooling and Permission Product Contract

## 6.1 Typed Tooling Profile

Agent definition must provide:

1. `sandbox_template`
2. `enabled_actions[]` from canonical action registry
3. `network_mode`:
- `github_only` (recommended default)
- `allowlisted`
4. `network_allowlist_hosts[]` when `allowlisted`

## 6.2 Typed Permission Profile

Agent definition must provide:

1. `default_mode`: `allow` | `ask` | `deny`
2. `overrides[]`: action-specific mode overrides
3. `approval_channel_id` when any action may resolve to `ask`

## 6.3 Enforcement Order (Deterministic)

For each attempted action:

1. If action not in `enabled_actions`: deny.
2. Else resolve permission mode (override > default).
3. If mode is `allow`: execute.
4. If mode is `deny`: block and log.
5. If mode is `ask`: request Slack approval, await decision, persist result for that agent/action scope, then proceed/deny.

Enforcement is runtime code policy, never prompt-only.

## 6.4 Approval UX Contract

1. Approval requests post to configured Slack channel.
2. Any workspace member can approve/deny (Phase 1 policy).
3. Decision and approver are auditable in run timeline.

---

## 7. Delivery Contract

## 7.1 Delivery Targets

Delivery is typed by target kind:

1. `slack_thread`
- Always reply in triggering thread.

2. `slack_channel`
- Always post top-level in configured channel.

No separate thread-strategy field.

## 7.2 Runtime Events vs User Messages

1. Internal runtime events (tool progress, raw stderr, low-level stream noise) are not posted verbatim by default.
2. User-facing updates should be coalesced and meaningful.
3. Final answer/artifact is always explicit and terminal for the run.

---

## 8. State and Reliability Contract

## 8.1 Idempotency and Dedupe

1. Slack/GitHub events are deduped by source event identity.
2. Retry-safe processing required for webhook retries.

## 8.2 Queueing and Concurrency

1. Per-thread serialization for thread-bound runs.
2. Per-agent queueing for background runs.
3. New runs enqueue if conflicting run is active.

## 8.3 Failure Behavior

1. Slack webhook path acks fast and processes async.
2. Transient failures retry with bounded policy.
3. Terminal failures produce clear user-facing failure message and full timeline diagnostics.

---

## 9. Observability Contract

## 9.1 Mission Control Must Show

1. Trigger source and event metadata.
2. Run lifecycle timeline (queued, started, waiting approval, completed/failed).
3. Effective prompt/policy version references.
4. Action approvals/denials and actor identity.
5. Delivery outcomes per target.

## 9.2 Runtime Logs

1. Structured logs with correlation IDs:
- workspace, channel, thread, run, session, sandbox.
2. Errors include stage and policy context.

---

## 10. Minimal Typed Surface for Prompt + Tooling

These fields must be explicit and typed in product APIs/UI:

1. `objective`
2. `instructions`
3. `runtime_mode`
4. `repository_scope`
5. `sandbox_template`
6. `enabled_actions[]`
7. `network_mode`
8. `network_allowlist_hosts[]`
9. `permission_default_mode`
10. `permission_overrides[]`
11. `approval_channel_id`
12. `delivery_targets[]`

Advanced JSON remains read-only diagnostics, not editable source of truth.

---

## 11. Open Product Decisions (To Lock)

1. Thread agent selection syntax when multiple slack-thread agents are active (required vs optional mention syntax).
2. Policy grant scope for `ask` approvals:
- per run, per thread, or persisted per agent/action/workspace.
3. User-facing progress streaming level:
- final-only, stage-level, or configurable.
4. Background artifact shape:
- freeform markdown vs typed schema families by agent purpose.

---

## 12. Acceptance Criteria for This Spec

1. New thread bootstraps once; follow-ups are lightweight and session-native.
2. Tool access is blocked/allowed strictly by typed runtime policy.
3. Permission `ask` flow works end-to-end via Slack and is auditable.
4. Delivery behavior is deterministic by target kind.
5. Mission Control can explain exactly why a run behaved as it did.

