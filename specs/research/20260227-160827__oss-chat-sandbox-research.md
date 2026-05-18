# OSS Research: Chat Interfaces + Remote Sandboxes

## Scope

Research question: how open source systems implement Slack/Discord/text interfaces backed by remote sandboxes or isolated runtimes, and what we should adopt.

Date: 2026-02-27

## Repositories Reviewed

### 1) RhysSullivan/claude-sandbox-bot (Discord + Vercel Sandbox)
- URL: https://github.com/RhysSullivan/claude-sandbox-bot
- From README:
  - Thread-per-conversation model.
  - New message starts a thread + Claude session; replies continue same session.
  - Streams system/assistant/user/result events back to thread.
  - Uses isolated `@vercel/sandbox`.
- Why relevant:
  - Closest interaction pattern to our target thread-scoped runtime.

### 2) e2b-dev/claude-code-fastapi (Text API + E2B Sandbox)
- URL: https://github.com/e2b-dev/claude-code-fastapi
- From README:
  - Session management across requests.
  - Endpoint model supports new session and resume-by-session-id.
  - Runs Claude Code in E2B sandbox templates.
- Why relevant:
  - Demonstrates session reuse contract with remote sandbox lifecycle.

### 3) ghostwriternr/claude-code-containers (GitHub interface + Cloudflare Containers)
- URL: https://github.com/ghostwriternr/claude-code-containers
- From README:
  - Uses Cloudflare Containers + Durable Objects.
  - Focuses on isolated execution and durable orchestration state.
- Why relevant:
  - Strong example of durable control-plane + isolated runtime model.

### 4) AnandChowdhary/claude-code-slack-bot (Slack + async orchestration)
- URL: https://github.com/AnandChowdhary/claude-code-slack-bot
- Discoverability source: https://anandchowdhary.com/open-source/2025/claude-code-slack-bot
- From README excerpt on listing page:
  - Mention creates issue and async progress loop.
  - Cloudflare KV for thread/issue mappings (with TTL).
  - Cloudflare Queues for polling/progress updates.
- Why relevant:
  - Useful durable Slack thread mapping + queue-based processing pattern.

### 5) context-machine-lab/sleepless-agent (Slack + persistent queue + isolated workspaces)
- URL: https://github.com/context-machine-lab/sleepless-agent
- From README:
  - Slack commands feed SQLite-backed task queue.
  - Daemon executor model.
  - Isolated workspace per task.
- Why relevant:
  - Good reference for durable queue/worker flow and explicit task isolation.

### 6) mpociot/claude-code-slack-bot (Slack + thread/session streaming)
- URL: https://github.com/mpociot/claude-code-slack-bot
- From README:
  - Thread support with conversation continuity.
  - Streaming responses and real-time updates.
- Why relevant:
  - Useful Slack UX/runtime message update patterns (even without remote sandbox emphasis).

## Shared Patterns Across Implementations

1. Stable conversation key
- Map thread/channel context to one runtime session identity and reuse it.

2. Durable control plane
- Store thread/session state in DB/KV/DO, not in-process memory only.

3. Queue/worker orchestration
- Keep webhook ack path fast.
- Run heavy work in async worker.

4. Isolated runtime per conversation/job
- Sandbox/container/workspace per unit of work.
- Reuse where possible; recreate on stale/unhealthy runtime.

5. Streaming UX with a single mutable message target
- Stream incremental output to one chat message (update/edit), then finalize once.

6. Explicit lifecycle and time boundaries
- Status states (queued/running/awaiting_input/failed/completed).
- Timeouts and reclaim/retry logic with bounded attempts.

7. Debuggability and operator controls
- Keep run IDs, timeline events, and retry/terminate controls.

## Implications for Our Implementation

### Keep
1. Thread-scoped session concept.
2. Sandbox reuse with recreate-on-failure behavior.
3. Run timeline + diagnostics APIs.

### Change
1. Replace long-held per-thread DB advisory lock with queue + lease worker
- Current pattern serializes correctly but risks throughput collapse when lock scope includes long I/O.
- Target pattern:
  - webhook inserts event with `queued` status
  - worker claims via short transaction and lease (`locked_until`, `worker_id`)
  - processing happens outside DB transaction
  - lease heartbeats every N seconds

2. Add explicit HTTP timeouts on all external network calls
- Slack/OpenCode/GitHub HTTP calls should use bounded timeouts + classified retries.

3. Make heartbeat truly periodic during long runs
- Update runtime event heartbeat while cloning, running, streaming, waiting for permissions.
- Stale reclaim should also handle `last_heartbeat_at IS NULL`.

4. Strengthen exactly-once outbound Slack semantics
- Persist outbound message id/ts as an outbox step to avoid duplicate final posts.

5. Separate permission-wait state from active-run state
- Explicit `awaiting_permission` status with expiry and deterministic user feedback.

6. Formalize session lifecycle state machine
- Suggested states:
  - `queued`
  - `starting`
  - `running`
  - `awaiting_permission`
  - `finalizing`
  - `failed`
  - `completed`
  - `terminated`

## Temporal Evaluation

### Should we use Temporal?

Short answer: yes, as a targeted spike first.

Temporal fits this workload because we have:
1. Long-running, stateful thread workflows.
2. External side effects with failure-prone boundaries (Slack, GitHub, E2B, OpenCode).
3. Human-in-the-loop control events (`approve`, `deny`, `terminate`).
4. A need for durable execution history and better operational visibility.

### What Temporal would replace in our current design

1. Per-thread DB advisory lock orchestration
- Replace with one workflow per thread key (`workspace_id + channel_id + thread_ts`).

2. Custom retry/reclaim scheduler logic
- Replace with Activity-level retry/timeouts/heartbeats and Workflow-level state transitions.

3. Ad-hoc control-command routing
- Replace with workflow signals/updates for permission decisions and termination.

### Recommended adoption path (incremental)

1. Spike (narrow)
- Implement one workflow for thread lifecycle:
  - start on root mention
  - signal on follow-up message
  - signal on `@App terminate`
  - activity heartbeat while sandbox work runs

2. Validate before wider migration
- Confirm:
  - no lock contention/pool starvation under load
  - deterministic replay constraints are manageable
  - visibility and debugging improve for real incidents

3. Decide migration scope
- If spike succeeds, move webhook processing to enqueue/signal only and migrate executor paths behind Temporal workers.

### Sources

- Temporal workflows: https://docs.temporal.io/workflows
- Temporal activities: https://docs.temporal.io/activities
- Temporal TypeScript failure detection (timeouts/retries/heartbeats): https://docs.temporal.io/develop/typescript/failure-detection
- Temporal TypeScript message passing (signals/queries/updates): https://docs.temporal.io/develop/typescript/message-passing
- Temporal TypeScript observability: https://docs.temporal.io/develop/typescript/observability
- Temporal Cloud: https://temporal.io/get-cloud/aws-marketplace

## Practical Next Step Plan

1. Introduce `runtime_job_queue` + worker lease model (P0).
2. Move Slack webhook processing to enqueue-only path (P0).
3. Port current run logic into worker executor with periodic heartbeats (P0).
4. Add global timeout wrappers for all external HTTP calls (P0).
5. Implement outbox table for Slack sends/updates (P1).
6. Add operator actions: retry job, terminate session, dead-letter inspection (P1).

## Notes

- Findings are based on publicly available project README/documentation and repository metadata, not deep source code audits for every project.
