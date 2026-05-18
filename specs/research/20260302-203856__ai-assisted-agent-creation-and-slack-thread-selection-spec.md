# AI-Assisted Agent Creation and Multi-Agent Slack Thread Selection Spec

**Date:** 2026-03-02  
**Status:** Ready for implementation  
**Scope:** Web + BFF + Slack runtime behavior (no Temporal changes)

---

## 1. Objective

Implement a production-ready creation and routing flow where:

1. Users can create agents from plain-language intent with AI assistance.
2. Slack and GitHub integrations are auto-carried from existing org connections.
3. User only confirms/selects repo access during setup.
4. Raw JSON fields are moved to an advanced mode.
5. Multiple Slack-thread agents can coexist and be explicitly selected per thread.

---

## 2. Locked Decisions

1. AI support for agent creation: `yes`.
2. Agent templates: `no`.
3. Integration inheritance: `yes`, no confirmation for integration reuse.
4. Repo access assignment: explicit user selection is required.
5. JSON configuration editing: hidden behind advanced mode.
6. Field-level usage hint labels: out of scope for this cut.

---

## 3. Current Gaps to Resolve

1. New definitions start with empty Slack/GitHub integration bindings.
2. Activation currently enforces one active `codebase_qa` per Slack workspace.
3. Slack thread runtime resolves by workspace instead of thread-to-agent binding.
4. Agent creation UX expects manual free-form authoring of complex config fields.

---

## 4. Target Product Behavior

## 4.1 Agent Creation Flow (Default)

1. User enters:
   - agent name
   - plain-language intent
   - runtime mode
2. User clicks `Generate Config`.
3. Backend AI generation returns a structured draft:
   - objective
   - instructions
   - output contract
   - output delivery defaults
   - tooling profile defaults
   - permission profile defaults
4. UI shows editable typed fields for this draft.
5. Slack workspace and GitHub installation are auto-bound from org defaults.
6. User must choose repo scope from the inherited GitHub installation.
7. User saves draft and runs activation preflight.
8. Activation is allowed only after integration and repo requirements pass.

## 4.2 Advanced Mode

1. Default mode uses typed fields and guardrails.
2. Advanced mode exposes raw JSON editors for:
   - output contract
   - output delivery
   - tooling profile
   - permission profile
3. Switching between default and advanced mode keeps values synchronized.
4. Invalid JSON blocks save/activation and returns field-level errors.

## 4.3 Multi-Agent Slack Thread Selection

1. New strict thread-start command:
   - `@App start <agent>`
2. Thread binding behavior:
   - command binds the thread to exactly one agent.
   - all subsequent thread messages route to the bound agent.
3. Root mention behavior when no binding exists:
   - if exactly one eligible Slack-thread agent exists, auto-bind and proceed.
   - if multiple exist, bot asks user to pick via `@App start <agent>`.
4. Termination behavior:
   - `@App terminate` ends runtime session and removes thread binding.
5. Existing background command remains:
   - `@App trigger run <agent>`

---

## 5. Data Model Changes

1. Remove workspace-level single-active-agent enforcement for Slack thread mode.
2. Add `thread_agent_bindings` table:
   - `id`
   - `workspace_id`
   - `channel_id`
   - `thread_ts`
   - `agent_definition_id`
   - `status` (`active`, `closed`)
   - `bound_by_user_id`
   - `created_at`
   - `updated_at`
   - `closed_at`
3. Add unique constraint for active binding:
   - unique on (`workspace_id`, `channel_id`, `thread_ts`) where `status='active'`.
4. Keep existing runtime session tables; session lookup should read binding first.

---

## 6. API and Contract Changes

## 6.1 Agent Setup APIs

1. Add `GenerateAgentConfigDraft` RPC:
   - input: `name`, `intent`, `runtime_mode`, optional repo hints.
   - output: structured draft object for editable fields.
2. Extend create/update definition response payloads with inherited integration status.
3. Add `GetOrgDefaultIntegrations` RPC:
   - returns best candidate Slack workspace + GitHub installation for current org.
4. On create, backend auto-attaches inherited integration IDs when available.

## 6.2 Slack Runtime APIs/Behavior

1. Add thread binding resolver/manager functions:
   - create binding
   - lookup active binding
   - close binding on terminate
2. Update Slack event ingress classification to:
   - parse `start` command
   - parse `terminate` command
   - resolve binding-based routing
3. Keep compatibility with existing permission approval commands in-thread.

---

## 7. Runtime Behavior Changes

1. Replace "active definition by workspace" routing with:
   - `active thread binding -> agent definition`.
2. Remove assumption that one active agent exists per workspace.
3. Enforce repo allowlist exactly as today after agent selection.
4. Keep one sandbox/session per thread binding.
5. On `terminate`, perform:
   - sandbox/session cleanup
   - permission pending expiry
   - binding close transition.

---

## 8. Validation Rules

1. Activation preflight requires:
   - Slack connected
   - GitHub connected
   - at least one selected repo
   - valid tooling profile
   - valid permission profile
2. If permission default mode is `ask` and approval channel is missing:
   - activation fails with explicit error.
3. AI generation output is validated server-side before persistence.
4. Repo selection must belong to inherited/attached installation.

---

## 9. Migration and Backward Compatibility

1. Existing agents remain valid and editable.
2. Existing threads without binding:
   - first new message follows new routing logic and may prompt for agent selection.
3. Legacy behavior fallback:
   - if only one eligible Slack-thread agent exists, auto-bind to preserve low friction.
4. Keep old commands working where safe; prioritize strict command parsing for new behavior.

---

## 10. Testing Strategy (Strict TDD)

1. Write failing tests first for each change.
2. Implement minimum behavior to pass.
3. Refactor with tests green.
4. Add regression tests for each discovered edge case.

Required suites:

1. Agent creation AI-draft generation contract tests.
2. Integration inheritance tests on create.
3. Repo selection enforcement tests with inherited installation.
4. Advanced mode serialization/parsing tests.
5. Slack command parse tests:
   - `start`
   - `trigger run`
   - `terminate`
6. Thread binding lifecycle tests:
   - create
   - resolve
   - close
7. Multi-agent workspace routing tests.
8. Activation preflight rule tests including ask-mode channel dependency.

---

## 11. Manual E2E Validation

1. Create two Slack-thread agents in same workspace.
2. Confirm both can be active simultaneously.
3. In a new thread, run `@App start <agent-a>` and verify responses come from agent A.
4. Continue thread messages without mentions and verify agent A remains selected.
5. Run `@App terminate` and verify session cleanup.
6. In another thread, run `@App start <agent-b>` and verify agent B routing.
7. Create a new agent using plain-language intent and `Generate Config`.
8. Verify Slack/GitHub integrations auto-carry on create.
9. Verify repo scope still requires explicit selection.
10. Save and activate from default mode.
11. Switch to advanced mode, edit JSON, and verify validation errors on invalid payloads.
12. Verify background command still works: `@App trigger run <agent>`.

---

## 12. Risks and Mitigations

1. Risk: ambiguous agent names in `@App start <agent>`.
   - Mitigation: deterministic matching order and disambiguation response.
2. Risk: stale bindings after process restarts.
   - Mitigation: durable DB binding table and cleanup hooks on terminate.
3. Risk: AI-generated config can be malformed or unsafe.
   - Mitigation: strict schema validation and safe defaults.
4. Risk: integration inheritance picks wrong org connection.
   - Mitigation: deterministic org-scoped default selection rules and audit event logging.

---

## 13. Implementation Order

1. Data model and migration for thread bindings.
2. Remove single-active-workspace constraint.
3. Slack runtime binding-first routing and commands.
4. Integration inheritance on create.
5. `GenerateAgentConfigDraft` backend + web flow.
6. Advanced mode UX for raw JSON.
7. Final hardening and E2E test pass.

