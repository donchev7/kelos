# Slack Thread Runtime Reliability and Citation Integrity Spec

Date: 2026-04-01  
Status: Draft v1  
Owner: Moontide  
Related docs:
1. [Prompt Runtime Implementation Spec](/Users/shan/Documents/sandbox/starlight/moontide_ai/docs/archived/20260306-161742__prompt-runtime-implementation-spec.md)
2. [Runtime Session Liveness and Heartbeat Spec](/Users/shan/Documents/sandbox/starlight/moontide_ai/docs/archived/20260309-103726__runtime-session-liveness-and-heartbeat-spec.md)
3. [Opencode Timeout Stage Remediation Spec](/Users/shan/Documents/sandbox/starlight/moontide_ai/docs/archived/20260309-153943__opencode-timeout-stage-remediation-spec.md)

## 1. Problem statement

Two production-like thread behaviors need hardening:

1. Thread runtime reliability degradation:
1. User-visible failures include:
1. `opencode /session/<id>/message timed out after 20000ms`
2. `[deadline_exceeded] ... timeoutMs ...`
3. `terminated`
2. Automatic retries and new user messages can pile onto the same thread session, creating a perception of loops and non-deterministic recovery.
3. Multiple `runtime_slack_events` can remain in `processing` for the same `session_key`, making operational state hard to reason about.

2. Citation integrity degradation:
1. Some answers contain malformed citation blocks and mixed/incorrect repo-path-line mappings.
2. Citation rendering can include duplicated or conflicting citation sections.
3. Stored `citations_json` can drift from human-readable answer text quality.

## 2. Goals and non-goals

### 2.1 Must-have goals

1. Eliminate avoidable thread failures caused by short finalize-time fetch timeouts.
2. Prevent thread-level retry storms and stale retry replay behavior.
3. Ensure at most one actively executing runtime event per thread session.
4. Normalize user-facing runtime failures into concise, actionable messages.
5. Make citations deterministic, valid, and render-safe for Slack responses.
6. Add strict TDD coverage for timeout/retry/session orchestration and citation correctness.

### 2.2 Optional (nice-to-have)

1. Add richer runtime diagnostics fields for stage/endpoint-level failure attribution.
2. Add thread-level runtime health UI surface (last event state + retry reason).
3. Add admin repair tooling for superseding stale failed events in bulk.

### 2.3 Non-goals

1. No redesign of Slack agent UX flows (`start`, `terminate`, permission prompts).
2. No changes to onboarding flows or memory bootstrap flows.
3. No large refactor of OpenCode transport stack beyond required hardening.

## 3. Scope

### 3.1 In scope (must-have)

1. [apps/bff/src/agents/code-search-runtime.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/agents/code-search-runtime.ts)
2. [apps/bff/src/agents/codebase-qa-runtime-core.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/agents/codebase-qa-runtime-core.ts)
3. [apps/bff/src/runtime/runtime-reliability.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/runtime/runtime-reliability.ts)
4. [apps/bff/src/runtime/runtime-slack-retry-worker.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/runtime/runtime-slack-retry-worker.ts)
5. [apps/bff/src/runtime/runtime-session-lock.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/apps/bff/src/runtime/runtime-session-lock.ts)
6. [packages/db/src/schema/e2e_slice.ts](/Users/shan/Documents/sandbox/starlight/moontide_ai/packages/db/src/schema/e2e_slice.ts) (only if schema additions are required by selected implementation variant)
7. Runtime tests in `apps/bff/src/agents` and `apps/bff/src/runtime`

### 3.2 Out of scope

1. Agent factory/background-run orchestration redesign.
2. Slack installation/oauth changes.
3. Frontend redesign beyond minimal status/error text improvements.

## 4. Root-cause analysis

### 4.1 Runtime reliability root causes

1. Finalization depends on `GET /session/<id>/message` with a 20s default timeout.
1. `listOpenCodeSessionMessages` uses `fetchOpenCodeJson(... timeoutMs ?? 20000)`.
2. This path is used in both timeout reconciliation and normal finalize.
3. Long-running or large sessions can exceed this budget and fail after compute already happened.

2. Endpoint consumption overlap:
1. `/session/<id>/message` is consumed from:
1. timeout reconciliation path
2. finalize path
2. Failures on this endpoint can cascade as runtime failure despite successful stream progress.

3. Retry/event orchestration is event-centric, not thread-centric:
1. Retries can continue for older prompts even after newer thread messages arrive.
2. This creates stale replay behavior and repeated “error chatter” in one Slack thread.

4. User-facing error formatting is too raw:
1. Provider internal messages are sent verbatim to Slack users.
2. `terminated` and long provider diagnostics are not normalized into actionable copy.

### 4.2 Citation integrity root causes

1. Thread mode citation extraction relies on regex over free-form assistant text (`parseCitationReferencesFromAnswer`), which is brittle with markdown/bullets/backticks.
2. Answers can already include ad-hoc “Citations:” text while runtime appends canonical citations again, causing duplication/misalignment.
3. Citation payloads are not strictly validated against in-scope repositories and path/line sanity before persistence/rendering.

## 5. Proposed solution

## 5.1 Runtime reliability hardening

### 5.1.1 Finalize-path timeout strategy (must-have)

1. Introduce endpoint-specific timeout envs:
1. `OPENCODE_SESSION_MESSAGES_TIMEOUT_MS` (default `90000`)
2. Optional `OPENCODE_SESSION_STATUS_TIMEOUT_MS` (default `30000`)
2. Do not fail a run solely because message-list fetch timed out if a non-empty streamed answer already exists.
3. In finalize:
1. Prefer stream result first.
2. Use message-list fetch as reconciliation/quality path, not mandatory success path.
4. If stream content exists and message fetch fails:
1. return processed answer
2. record a `finalize_degraded` runtime event
3. keep citations empty or fallback-safe

### 5.1.2 Thread-level retry orchestration (must-have)

1. Enforce one active execution per `session_key` in behavior and state transitions.
2. Retry sweep should process at most one due failed event per `session_key` per tick.
3. When a newer event exists in the same session:
1. older failed events should be marked `superseded` (or equivalent terminal status)
2. they should not continue automatic retries
4. Manual retry should fail fast if a same-session event is already active.
5. Keep retry budget bounded and explicit:
1. retain `RUNTIME_RETRY_MAX_ATTEMPTS`
2. stop retries on terminal non-retryable classifications

### 5.1.3 Error classification + user messaging (must-have)

1. Expand runtime classification to explicitly handle:
1. `terminated`-style provider errors
2. `deadline_exceeded` variants
3. session message fetch timeout errors
2. Map to user-safe messages:
1. short action text
2. no raw provider stack/details
3. include suggested next action (`retry`, `start new root mention`, etc.)
3. Persist full diagnostic detail in runtime event logs, not user Slack messages.

### 5.1.4 Optional reliability enhancements

1. Add jitter to retry scheduling to avoid synchronized retry bursts.
2. Add a per-thread cooldown after N consecutive transient failures.
3. Add a “thread reset recommended” heuristic after repeated same-stage failures.

## 5.2 Citation integrity hardening

### 5.2.1 Structured response contract for thread mode (must-have)

1. Thread prompts must require strict JSON payload (same principle as background mode), including:
1. `answer: string`
2. `needs_clarification: boolean`
3. `clarification_question: string | null`
4. `citations: Array<{ repo?: string, path: string, line_start: number, line_end: number }>`
2. Parse citations from structured payload, not regex over prose.
3. Keep regex extraction only as guarded fallback and disable it by default for thread mode.

### 5.2.2 Citation validation + normalization (must-have)

1. Validate each citation before persistence/render:
1. repo resolves to an in-scope repository
2. path non-empty and normalized (`./` stripped)
3. `line_start >= 1`
4. `line_end >= line_start`
2. Drop invalid citations; do not render malformed ones.
3. Deduplicate by normalized `(repo, path, line_start, line_end)`.
4. If all citations invalid:
1. return answer without citation block
2. record `citation_validation_failed` diagnostic event

### 5.2.3 Canonical rendering policy (must-have)

1. Render citations only from validated structured citations.
2. Do not append canonical citation block if output already includes unmanaged ad-hoc citation text unless sanitization is enabled.
3. Preferred policy:
1. strip model-emitted ad-hoc “Citations:” sections
2. append single canonical block generated by server

### 5.2.4 Optional citation enhancements

1. Add commit SHA resolution quality flags (`exact`, `unknown`, `inferred`).
2. Add hard cap on citation count per response for Slack readability.
3. Add minimal citation confidence scoring for UI diagnostics.

## 6. Data model and migration impact

### 6.1 Must-have baseline (no migration variant)

1. Keep existing `runtime_slack_events` shape.
2. Implement supersede/active behavior via existing `status` text values and existing indexes.
3. Persist additional diagnostics in runtime run events without schema changes.

### 6.2 Optional migration variant

If stronger auditability is desired now, add:

1. `runtime_slack_events.superseded_by_event_id` (nullable text)
2. `runtime_slack_events.failure_stage` (text)
3. `runtime_slack_events.failure_endpoint` (text)

This variant is optional and should be deferred unless required by observability goals.

## 7. API and behavior contract updates

### 7.1 Must-have behavioral updates

1. Slack thread answers should no longer contain malformed citation permutations from regex parsing.
2. Retry flow should prioritize newest user intent in a thread and avoid stale retry replay.
3. Timeout errors should surface as concise, user-safe guidance.

### 7.2 Optional API additions

1. Add runtime visibility fields for `superseded` counts per thread.
2. Add endpoint/stage filters for runtime run event queries.

## 8. Test plan (strict TDD)

## 8.1 Runtime reliability tests (must-have)

1. `code-search-runtime` tests:
1. finalize succeeds when streamed answer exists and `/session/<id>/message` times out
2. finalize fails only when both stream and reconciliation fail
3. endpoint-specific timeout envs are honored
2. `runtime-reliability` tests:
1. `terminated` and `deadline_exceeded` classification paths
2. user-facing normalization coverage
3. `runtime-slack-retry-worker` tests:
1. only one due event per session_key is claimed per tick
2. older failed events are superseded when newer event exists
3. retry does not continue after superseded status
4. `codebase-qa-runtime-core` tests:
1. manual retry rejected when same-session execution is active
2. no duplicate user-visible failure spam for superseded events

## 8.2 Citation integrity tests (must-have)

1. Structured payload parse success with valid citations.
2. Invalid citations are dropped (repo/path/range validation).
3. Duplicate citations are collapsed.
4. Malformed free-form citation text does not pollute `citations_json`.
5. Final rendered Slack output contains at most one canonical `Citations:` block.

## 8.3 Regression tests (must-have)

1. Existing successful thread Q&A path remains green.
2. Existing permission request flow remains green.
3. Existing runtime heartbeat/stale-reclaim tests remain green.
4. Full BFF lint/typecheck/test suite remains green.

## 9. Observability requirements

### 9.1 Must-have

1. Runtime run events must capture stage-level failures for:
1. `stream_run`
2. `finalize`
3. `reconcile_timeout`
2. Log when events are superseded with:
1. superseded event id
2. replacement/newer event id
3. session key
3. Log citation validation drop counts per response.

### 9.2 Optional

1. Metrics:
1. `runtime_thread_superseded_events_total`
2. `runtime_finalize_degraded_total`
3. `runtime_citation_invalid_total`

## 10. Rollout plan

Single-pass rollout (no phased feature flagging required):

1. Ship runtime timeout + finalize fallback hardening.
2. Ship retry/session coalescing and supersede logic.
3. Ship structured citation parsing + canonical rendering.
4. Run full automated suite.
5. Perform manual validation in Slack thread with synthetic timeout and malformed citation prompts.

## 11. Acceptance criteria

1. A thread run with valid streamed output is not marked failed only because `/session/<id>/message` exceeded 20s.
2. Same-session stale failed events do not keep replaying once newer user events exist.
3. At most one same-session runtime event is actively executing at a time.
4. User-facing Slack error text no longer includes raw provider diagnostic blobs.
5. Citation block output is deterministic, deduplicated, and structurally valid.
6. `citations_json` does not contain malformed repo/path swaps from free-form regex parsing.

## 12. Implementation readiness checklist

Must-have before coding:

1. Confirm timeout defaults to adopt (`OPENCODE_SESSION_MESSAGES_TIMEOUT_MS`, optional status timeout).
2. Confirm whether superseded events should be visibly surfaced in runtime visibility API now.
3. Confirm whether optional DB migration variant is deferred.

Current recommendation:

1. Proceed with no-migration baseline first.
2. Add migration fields only if runtime visibility requirements demand it immediately.
