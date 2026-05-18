# Runtime Stuck States and Fallbacks

## Potential Stuck / Dead-End Scenarios

1. Session lock can block a whole thread when an in-flight task hangs.
- Lock is held for the full async task via PG advisory lock: `apps/bff/src/runtime/runtime-session-lock.ts:4`.

2. Several network calls do not set explicit request timeouts.
- OpenCode HTTP calls use `fetch(...)` without `AbortController` timeout wrappers: `apps/bff/src/agents/code-search-runtime.ts:630`, `apps/bff/src/agents/code-search-runtime.ts:661`.
- Slack message APIs also use `fetch(...)` without explicit timeout wrappers: `apps/bff/src/integrations/slack.client.ts:118`, `apps/bff/src/integrations/slack.client.ts:153`.

3. Heartbeat updates are not continuous during long processing.
- Heartbeat is set at reserve/final status update, not periodically while running: `apps/bff/src/agents/codebase-qa-runtime-core.ts:307`, `apps/bff/src/agents/codebase-qa-runtime-core.ts:178`.
- Stale-processing reclaim can mark a legitimately long run as failed and schedule retry, creating duplicate work: `apps/bff/src/runtime/runtime-slack-retry-worker.ts:47`.

4. Rows with `lastHeartbeatAt = NULL` are not reclaimed by stale-processing logic.
- Reclaim predicate requires `lastHeartbeatAt < staleBefore`; SQL `NULL` will not match this condition: `apps/bff/src/runtime/runtime-slack-retry-worker.ts:47`.

5. Some failures intentionally become terminal unless manually retried.
- Non-transient classification does not auto-retry (`nextRetryAt = null`), so event remains failed until manual action: `apps/bff/src/agents/codebase-qa-runtime-core.ts:807`, `apps/bff/src/runtime/runtime-reliability.ts:17`, `apps/bff/src/runtime/runtime-visibility-core.ts:468`.

## Existing Fallbacks

1. Sandbox fallback: reconnect-or-create.
- If sandbox connect fails, code creates a fresh sandbox: `apps/bff/src/agents/code-search-runtime.ts:1248`.

2. OpenCode session fallback: reuse-or-create.
- If prior OpenCode session is stale, code creates a new one: `apps/bff/src/agents/code-search-runtime.ts:818`.

3. Assistant answer fallback.
- If final message list has no assistant text, runtime falls back to streamed text: `apps/bff/src/agents/code-search-runtime.ts:873`.

4. Permission routing fallback.
- Permission reply sandbox resolution falls back from pending record to active runtime session state: `apps/bff/src/agents/codebase-qa-runtime-core.ts:1070`.

5. Auto-retry fallback for transient failures.
- Transient failures are backoff-retried using runtime retry worker: `apps/bff/src/agents/codebase-qa-runtime-core.ts:807`, `apps/bff/src/runtime/runtime-slack-retry-worker.ts:176`.

6. Manual retry fallback.
- Failed events can be manually re-queued via runtime visibility API (with limits): `apps/bff/src/runtime/runtime-visibility-core.ts:468`.

7. Idempotent teardown fallback.
- Sandbox kill failures are intentionally swallowed to keep termination idempotent: `apps/bff/src/agents/code-search-runtime.ts:1362`.

8. Rate limit backend fallback mode.
- Rate limiting supports either in-memory or DB storage mode via config selection: `apps/bff/src/security/distributed-rate-limit.ts:61`.

