# Moontide AI - E2E Thread Agent Production Readiness Proposal

**Date:** 2026-02-23  
**Status:** Proposal

---

## 1. Objective

Make the current Slack -> BFF -> E2B -> OpenCode E2E flow production ready for real team usage by:

1. Eliminating known breakpoints (timeouts, retries, duplicate handling, session drift).
2. Making runtime state visible beyond console logs.
3. Improving Slack thread UX so users see one coherent agent conversation.

---

## 2. Current State Snapshot (as implemented)

1. Slack webhook acknowledges quickly and processes async in-process.
2. Sessioning is thread-scoped (`workspace:channel:root_ts`) and persisted.
3. Runtime queue/permission memory is process-local (`Map`) in the BFF instance.
4. E2B sandbox is reused per thread session when possible.
5. OpenCode server is started in sandbox and driven through session/event APIs.
6. Streaming uses frequent `chat.update` from runtime output chunks.
7. Dedupe is persisted (`runtime_slack_events`) but retry/reclaim loop is incomplete.

---

## 3. Production Risk Register

## P0 (must fix before wider rollout)

1. **Timeout mismatch across layers**
   - `AGENT_SESSION_IDLE_MINUTES` defaults to 30 min.
   - `E2B_SANDBOX_TIMEOUT_MS` defaults to 5 min (300000).
   - Result: runtime session may be marked active while sandbox already expired.

2. **In-memory coordination is not multi-instance safe**
   - `threadQueues` and `pendingThreadPermissions` are in-process maps.
   - On restart or horizontal scale, ordering and permission state can break.

3. **Dedupe path can hide real DB failures**
   - `reserveSlackEvent` treats insert failure as duplicate by fallback read.
   - Non-unique-constraint DB failures risk being misclassified as dedupe.

4. **No durable retry/reclaim worker**
   - Failed events are marked, but bounded retry scheduler and stale-processing reclaim are not complete.
   - Temporary E2B/OpenCode/Slack errors can become dropped answers.

## P1 (high)

1. **Streaming can produce noisy or duplicated user-visible text**
   - Intermediate deltas and final response can conflict.
   - Prompt/tool noise can leak into Slack output if events are not filtered strictly.

2. **Rate limit resilience is incomplete**
   - Slack `chat.postMessage` / `chat.update` retries on `429` and `Retry-After` are not centrally managed.

3. **Limited failure taxonomy**
   - Errors are mostly surfaced as generic thread errors; no consistent user-facing classes.

## P2 (medium)

1. **Observability is log-centric**
   - Good structured logs exist, but there is no operational dashboard, metrics, traces, or run timeline UI.

2. **Manual debugging depends on raw logs and direct sandbox access**
   - Slower triage and harder support handoff.

---

## 4. Proposed Runtime Hardening Design

## 4.1 Reliability Control Plane

1. Keep webhook path as fast ack only.
2. Persist each accepted event as a durable job record.
3. Process jobs in worker loop with per-session serialization via durable lock.
4. Move permission-request state from memory map to DB table keyed by thread session and request id.
5. Enforce single active run per thread using DB lock or status transition guard.

## 4.2 Timeout and Lifecycle Policy

Define explicit budgets per stage and enforce them with typed error codes:

1. Slack ack budget: <= 1s target, hard <= 3s.
2. Queue wait budget: 30s warning, 120s hard fail.
3. Sandbox acquire budget: 120s.
4. Repo materialization budget: 180s per repo with capped parallelism.
5. OpenCode server boot budget: 180s.
6. Prompt-to-idle run budget: 480s default.
7. Session idle timeout: 30m default.

Lifecycle rules:

1. Call `sandbox.setTimeout(...)` on every new turn and every meaningful runtime heartbeat.
2. Update `runtime_sessions.last_activity_at` and `runtime_slack_events.last_heartbeat_at` at bounded intervals.
3. Idle means:
   - no queued turns
   - no active generation
   - no tool execution in progress
   - no unresolved permission request blocking execution
4. If sandbox is gone but session is active, mark session degraded and recreate sandbox on next turn with explicit user notice.

## 4.3 Retry, Idempotency, and Reclaim

1. Persist and honor `attempt_count`, `next_retry_at`, `last_error_at`, `last_error_message`.
2. Retry only transient classes (`network`, `rate_limit`, `timeout`, `provider_unavailable`) with exponential backoff + jitter.
3. Mark terminal classes immediately (`invalid_auth`, `permissions`, `bad_request`).
4. Reclaim stale `processing` events when heartbeat age exceeds threshold.
5. Distinguish duplicate from storage failure by checking concrete DB error code for unique constraint violation.

## 4.4 Multi-instance Safety

1. Replace in-memory thread queue with durable queue partitioned by `session_key`.
2. Replace in-memory pending permission map with persisted `pending_runtime_permissions`.
3. Make command handling (`terminate`, `approve`, `deny`) operate purely on persisted state.

---

## 5. Observability Plan (Beyond BFF Logs)

## 5.1 Structured Event Model

Persist run timeline events with correlation IDs:

1. `workspace_id`
2. `slack_event_id`
3. `session_key`
4. `runtime_session_id`
5. `sandbox_id`
6. `opencode_session_id`
7. `run_id`
8. `stage`
9. `status`
10. `duration_ms`
11. `error_code`
12. `error_message`

## 5.2 Metrics

Add counters/histograms/gauges:

1. `slack_events_received_total`
2. `slack_events_deduped_total`
3. `slack_events_failed_total`
4. `thread_run_duration_ms`
5. `sandbox_start_duration_ms`
6. `repo_clone_duration_ms`
7. `opencode_run_duration_ms`
8. `permission_requests_total`
9. `permission_wait_duration_ms`
10. `active_sessions`
11. `active_sandboxes`
12. `retry_queue_depth`

## 5.3 Tracing

1. Add trace/span propagation from webhook ingest to final Slack reply.
2. Key spans: verify signature, dedupe reserve, queue wait, sandbox create/connect, repo sync, OpenCode prompt, stream bridge, Slack post/update.

## 5.4 Operator Surfaces

1. Minimal ops page for:
   - live sessions
   - failed events
   - stuck processing jobs
   - pending permissions
2. Per-thread timeline view with direct links to sandbox id and run id.
3. Alerting:
   - high failure rate
   - repeated timeout failures
   - stale processing backlog
   - Slack 429 spikes

---

## 6. Slack UX Improvements

1. Use one canonical bot message per turn:
   - create once
   - stream controlled updates
   - finalize once
2. Do not stream raw tool internals by default.
3. Stream only user-safe progress states:
   - "Starting sandbox"
   - "Syncing repositories"
   - "Thinking"
   - "Waiting for approval"
4. Ensure final assistant answer replaces progress text, not appended as duplicate.
5. Never include system prompt text in Slack output.
6. Permission requests stay explicit and actionable (`@App approve` / `@App deny`).
7. `@App terminate` returns deterministic teardown result and closes thread session.

---

## 7. Proposed Phased Delivery

## Phase 1 - Reliability Baseline

1. Durable queue + retries + stale reclaim.
2. Timeout budget enforcement and typed error codes.
3. Sandbox/session drift handling.

Exit criteria:

1. No dropped events in transient-failure simulations.
2. Deterministic behavior under duplicate/retry deliveries.

## Phase 2 - Visibility and Operations

1. Runtime timeline persistence and correlation IDs.
2. Metrics + dashboards + alerting.
3. Minimal ops view for failed/stuck items.

Exit criteria:

1. Any failed thread can be diagnosed without raw host logs.
2. On-call can identify failing stage in under 5 minutes.

## Phase 3 - UX Quality

1. Canonical streaming/finalization behavior.
2. Progress messaging and error messaging standards.
3. Permission and termination UX polish.

Exit criteria:

1. No duplicated bot turns in normal thread flow.
2. No prompt/tool leakage in Slack responses.

---

## 8. Manual E2E Gate (Production Readiness)

1. Root mention starts thread session and replies once.
2. Follow-up messages by multiple users stay in same sandbox/session.
3. Permission request appears; approve/deny command works.
4. `@App terminate` ends sandbox/session deterministically.
5. Timeout simulation produces graceful recovery message and session remains usable.
6. Retry simulation (forced transient failure) eventually succeeds once.
7. Operator can inspect run timeline and error reason without tailing process logs.

---

## 9. External Reference Notes

1. Slack Events API expects ack within 3 seconds and retries failed deliveries with backoff; includes retry headers (`x-slack-retry-num`, `x-slack-retry-reason`).
2. Slack messaging APIs are rate-limited and should respect `429` + `Retry-After`; message posting guidance is roughly 1 message/sec per channel.
3. E2B sandbox timeout defaults are short unless explicitly set/extended, and can be extended with `setTimeout`.
4. E2B OpenCode guide supports running `opencode serve` in background and using session APIs for programmatic prompts.

References:

1. https://docs.slack.dev/apis/events-api/
2. https://docs.slack.dev/apis/web-api/rate-limits/
3. https://e2b.dev/docs/sandbox
4. https://e2b.dev/docs/commands/background
5. https://e2b.dev/docs/agents/opencode
