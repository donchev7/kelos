# Runtime Session Liveness and Heartbeat Implementation Spec

**Date:** 2026-03-09  
**Status:** Ready for implementation  
**Scope:** Slack-thread runtime lifecycle in BFF (`apps/bff`)  
**Supersedes:** runtime-processing-heartbeat implementation draft (removed)

**References:**

1. OpenCode server/session/event API: https://opencode.ai/docs/server/
2. OpenCode plugin event handling (`session.idle`): https://opencode.ai/docs/plugins/
3. E2B sandbox lifecycle and timeout control: https://e2b.dev/docs/sandbox
4. E2B command timeout semantics: https://e2b.dev/docs/sdk-reference/js-sdk/v2.2.2/commands

---

## 1. Objective

Define and implement a single, explicit liveness model so:

1. Active runs are never reclaimed as stale while work is still in progress.
2. Session idle timeout starts only after OpenCode reports completion (`session.idle` / `session.status=idle`).
3. Follow-up thread messages reuse the same session/sandbox while within idle TTL.
4. Stale reclaim still works for truly abandoned runs.

---

## 2. Functional Model

## 2.1 Two Different Lifecycles (Must Stay Separate)

1. **Run lifecycle** (per Slack event in `runtime_slack_events`):
   - `processing` -> `processed` or `failed`
2. **Session lifecycle** (per thread session in `runtime_sessions`):
   - `active` while sandbox/session is usable
   - `ended` once terminated/expired/closed

Run heartbeat protects `processing` rows from false stale reclaim.  
Session TTL controls whether thread follow-ups can continue in the same sandbox.

## 2.2 Desired Thread Behavior

1. Root mention starts or reuses a session, runs agent.
2. While agent is running, the event is protected by heartbeat.
3. When OpenCode reports idle, run completes and session enters idle-wait period.
4. Any follow-up user message before expiry resumes work in same session.
5. If no activity until expiry, session is ended and sandbox is terminated.

## 2.3 End-to-End Example A (Normal Thread Reuse)

1. User posts root mention in Slack thread: `@App review this module`.
2. BFF reserves runtime event row as `processing` and starts heartbeat loop.
3. Sandbox boots, OpenCode session starts, prompt is sent.
4. While OpenCode is working, heartbeat ticks keep:
   - event row fresh (`runtime_slack_events.lastHeartbeatAt`)
   - session alive (`runtime_sessions.expiresAt` extended repeatedly)
5. OpenCode emits `session.idle`.
6. BFF marks event `processed`, posts response, stops heartbeat loop.
7. BFF sets session idle window: `expiresAt = now + AGENT_SESSION_IDLE_MINUTES`.
8. User sends a follow-up message in same thread after 2 minutes.
9. Same session/sandbox is reused; heartbeat loop starts again for the new run.

## 2.4 End-to-End Example B (Crash + Recovery)

1. User triggers a run; event is `processing`.
2. BFF process crashes mid-run; heartbeats stop.
3. Retry worker sweep sees stale processing row (`lastHeartbeatAt` older than threshold).
4. Worker marks old attempt failed and schedules retry (existing retry policy).
5. Retry worker claims due retry and reprocesses the same Slack event.
6. User gets eventual response (or final failure message) without manual intervention.

## 2.5 Lifecycle Timeline (Functional)

1. `t0`: root mention received -> event `processing`, heartbeat `on`.
2. `t0..tN`: OpenCode active -> heartbeat updates event + session.
3. `tN`: OpenCode `session.idle` -> event terminal, heartbeat `off`.
4. `tN`: session idle timer starts (`expiresAt = tN + idleTTL`).
5. `tN < t < expiresAt`: follow-up message -> session reused.
6. `t >= expiresAt`: no active session; follow-up without fresh root-mention is rejected/ignored per routing rules.

## 2.6 What Counts as "Active" vs "Idle"

1. **Active:** we are waiting on OpenCode work for a specific event (`runtime_slack_events.status=processing`).
2. **Idle:** last run completed (`session.idle` observed) and we are only waiting for the next user message.
3. **Stale:** active run appears abandoned because heartbeat has not moved past threshold.

---

## 3. Current Gaps

1. `runtime_slack_events.lastHeartbeatAt` is not refreshed continuously through long runs.
2. Reclaimer can mark valid long-running events stale.
3. Session expiry extension is mostly tied to session create/reuse and end-of-run update, not continuous active progress.
4. Reclaimer threshold is not validated against runtime timeouts, so config can be logically unsafe.

---

## 4. Target State Machine

## 4.1 Run State (`runtime_slack_events`)

1. `processing` (heartbeat active)
2. `processed` (terminal; response persisted)
3. `failed` (terminal for this attempt; may schedule retry)

Rules:

1. Only `processing` rows are eligible for stale reclaim.
2. Reclaim requires missing heartbeat beyond threshold.
3. Terminal transitions always stop heartbeat loop.

## 4.2 Session State (`runtime_sessions`)

1. `active` with `expiresAt` representing idle cutoff.
2. `ended` after explicit terminate/deactivate/expiry cleanup.

Rules:

1. On active work start: session is active and expiry is extended.
2. During active work: expiry gets refreshed periodically with heartbeat.
3. On OpenCode idle completion: set idle expiry (`now + AGENT_SESSION_IDLE_MINUTES`).
4. Follow-up before expiry: reuse session.
5. Follow-up after expiry: reject/no active session unless new root mention starts new session.

---

## 5. Technical Design

## 5.1 Heartbeat Controller (per processing event)

Create a focused controller used by `askQuestionFromSlack(...)`:

1. Start after event reservation (`status=processing`).
2. Tick every `RUNTIME_PROCESSING_HEARTBEAT_SECONDS`.
3. On each tick:
   - update `runtime_slack_events.lastHeartbeatAt` and `updatedAt`
   - update `runtime_sessions.lastActivityAt` and `expiresAt` (keep alive while active)
4. Also trigger immediate heartbeats on milestones:
   - session created/reused
   - allowlist/token ready
   - OpenCode stream activity callbacks
   - permission request callbacks
5. Stop in `finally` for all exits (success/failure/throw/early return).

Implementation location:

1. `apps/bff/src/agents/codebase-qa-runtime-core.ts`
2. Helper methods for event/session heartbeat writes.

## 5.2 Completion Semantics from OpenCode

Completion should be tied to OpenCode session state events:

1. `session.idle`
2. `session.status` where status type is `idle`
3. `session.error` => failure path

At completion:

1. Stop run heartbeat.
2. Persist run result (`processed` or `failed`).
3. Set session idle expiry (`expiresAt = now + idle minutes`).

No stale reclaim should run against this completed event.

## 5.3 Stale Reclaim Guardrails

Keep existing worker reclaim flow but enforce safe thresholds:

1. `RUNTIME_PROCESSING_STALE_SECONDS` must be greater than:
   - `3 * RUNTIME_PROCESSING_HEARTBEAT_SECONDS`
   - and `OPENCODE_RUN_TIMEOUT_MS / 1000` plus buffer
2. Add explicit startup assertion with actionable error.

Required default:

1. `RUNTIME_PROCESSING_HEARTBEAT_SECONDS=10`
2. `RUNTIME_PROCESSING_STALE_SECONDS=420`  
   (enough for long prompt runs + infra jitter; tune after metrics)

## 5.4 Follow-up Classification Safety

Current follow-up routing checks active session via `expiresAt`.  
During long runs, expiry must not lapse.

Required behavior:

1. Active run heartbeat refreshes `runtime_sessions.expiresAt`.
2. Thread follow-up during long runs remains routable as active session.

---

## 6. Data Semantics

## 6.1 `runtime_slack_events.lastHeartbeatAt`

Meaning:

1. Last known proof that this run is still alive.

Updated by:

1. reservation start
2. heartbeat ticks
3. milestone beats
4. terminal writeback

## 6.2 `runtime_sessions.expiresAt`

Meaning:

1. Idle timeout cutoff for reusing the session.

Updated by:

1. session create/reuse
2. active run heartbeat ticks (keep alive while processing)
3. run completion (reset idle window)
4. terminate/deactivate (set ended + immediate expiry)

---

## 7. Config Changes

Add env:

1. `RUNTIME_PROCESSING_HEARTBEAT_SECONDS` (default `10`)

Update policy assertions (`runtime-policy.ts`):

1. stale >= 3 * heartbeat
2. stale >= ceil(OPENCODE_RUN_TIMEOUT_MS / 1000) + safety buffer (+60s)
3. invalid runtime liveness config must fail BFF startup (hard error), not warn-only.

No UI changes needed for this scope.

---

## 8. Logging and Visibility

Add structured runtime logs:

1. `heartbeat-start` (eventId/sessionKey)
2. `heartbeat-stop` (reason: completed|failed|aborted)
3. `heartbeat-tick-sampled` (sampled; not every tick)
4. `session-expiry-extended` (sampled)
5. existing reclaim logs remain (`reclaimed-stale-processing`, `retry-claimed`)

Goal: quickly distinguish:

1. real stuck run
2. false stale reclaim
3. OpenCode completion vs transport failure

---

## 9. Failure Handling Rules

1. If heartbeat write fails transiently, keep processing and retry on next tick.
2. If OpenCode stream fails but run may still be active, classify as retryable where appropriate.
3. Reclaimer must only act after stale threshold, never based on missing single tick.

---

## 10. Test Plan (TDD)

## 10.1 Unit: Runtime Core

1. heartbeat starts for processing event.
2. heartbeat stops on success.
3. heartbeat stops on failure.
4. heartbeat refresh updates both event row and session expiry.
5. completion sets session idle expiry from completion time.

## 10.2 Unit: Runtime Policy

1. reject stale < 3x heartbeat.
2. reject stale < run-timeout-derived minimum.
3. accept valid configuration.

## 10.3 Unit: Retry Worker

1. no reclaim for fresh-heartbeat processing rows.
2. reclaim for truly stale processing rows.
3. reclaimed rows get retry scheduling behavior unchanged.

## 10.4 Integration-ish Runtime Flow

1. long-running run (> previous stale window) does not get reclaimed.
2. follow-up in same thread during/after run within TTL is accepted.
3. post-expiry follow-up without mention is ignored as no active session.

---

## 11. Manual Validation

1. Trigger long root mention (cold sandbox + repo fetch).
2. Confirm logs show heartbeat start/ticks and no stale reclaim.
3. Confirm OpenCode idle leads to final Slack reply.
4. Send follow-up in same thread within idle window; verify reuse.
5. Wait past idle expiry; send non-mention follow-up; verify no active-session behavior.
6. Force crash/abandon during processing; verify stale reclaim + retry works.

---

## 12. Acceptance Criteria

1. No false stale reclaim for healthy in-flight runs.
2. Idle timer starts at run completion signal, not at run start.
3. Session remains reusable for configured idle window.
4. Retry worker still recovers truly abandoned processing rows.
5. Tests and quality gates are green.

---

## 13. Out of Scope

1. Temporal migration.
2. Multi-process distributed lease coordinator.
3. UI-level session status surfaces (separate spec).
