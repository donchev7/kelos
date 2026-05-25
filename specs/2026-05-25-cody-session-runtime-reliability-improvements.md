# Cody Session Runtime Reliability Improvements

Date: 2026-05-25

Status: implementation spec

Worktree: `cody/session-runtime-improvements-spec`

## Context

Two Cody Slack session runs from 2026-05-24 exposed gaps in the new
`AgentSession` / `AgentTurn` runtime:

- `cody-session-slack-sess-04a2a56d4f39-t-0001`
  - Kubernetes Job succeeded.
  - The session Job ran from `2026-05-24T09:13:13Z` to
    `2026-05-24T13:15:26Z`.
  - Loki showed repeated Codex router errors:
    `write_stdin failed: stdin is closed for this session; rerun exec_command with tty=true to keep stdin open`.
- `cody-session-slack-sess-ab39739a35db-t-0001`
  - Kubernetes Job succeeded.
  - The session Job ran from `2026-05-24T11:52:26Z` to
    `2026-05-24T13:51:48Z`.
  - Loki only showed the startup bubblewrap warning in the Cody pod.
  - `cody-tools` had transient Atlassian MCP `context canceled` logs around
    the same time, but later Atlassian calls succeeded.

The visible user symptom was a Slack failure/reconnect message while the
Kubernetes Jobs themselves completed. The current implementation makes this
hard to reason about because the session runner does not persist enough Codex
App Server event detail, and the Slack reporter only posts accepted/terminal
turn messages.

## Goals

- Use Codex App Server terminal events correctly so non-terminal status changes
  do not fail a turn, and documented failure notifications preserve useful
  failure context until the terminal turn event arrives.
- Persist enough structured event summaries to debug long Cody sessions from
  Loki and Kubernetes status without needing raw app-server protocol dumps.
- Update the Slack progress message during long `AgentTurn`s so users see
  useful activity instead of a stale "Working..." reply.
- Make interactive-stdin tool misuse obvious and actionable.
- Stop Kyverno privilege-escalation warnings on generated Cody session Jobs.

## Non-Goals

- Do not change the public Slack `!session` UX.
- Do not change one-shot non-session Cody tasks.
- Do not add new credentials or broaden Cody runtime RBAC.
- Do not introduce a new persistence backend.
- Do not solve operator read access to `AgentSession` / `AgentTurn` in this
  change. That is useful follow-up RBAC work, but separate from these runtime
  reliability fixes.

## Current Implementation Anchors

- `cmd/kelos-session-runner/main.go`
  - Starts `codex app-server --listen stdio://`.
  - Sends `thread/start` and `turn/start`.
  - `waitForTurn` consumes app-server events from `app.events`.
  - Any app-server `error` event currently fails the turn immediately.
  - Completed command and MCP items can update `AgentTurn.status.activity`.
- `internal/reporting/slack_turn.go`
  - Posts an accepted Slack reply for queued/running turns.
  - Updates that accepted reply once with the terminal result.
  - Does not update in-progress activity snapshots.
- `internal/slack/session.go`
  - Creates `AgentSession` and `AgentTurn`.
  - Builds per-turn transcript context.
- `internal/controller/job_builder.go`
  - `BuildSessionRunner` reuses the normal Job builder, then changes the
    container command to `/kelos-session-runner`.
  - Session Jobs currently inherit whatever security context the normal task
    template produces.

## Codex App Server Protocol Findings

The Codex app-server integration should be specified from the latest public
OpenAI docs, not from a developer's local Codex install. Local installs can lag
or lead the image actually deployed to Cody.

The public docs say clients can generate a TypeScript schema or JSON Schema
bundle from the CLI, and that generated artifacts are specific to the Codex
version that produced them:

```bash
codex app-server generate-json-schema --out ./schemas
```

Use generated schemas as an implementation/CI guardrail for the deployed image,
not as the source of functional requirements in this spec.

Relevant protocol findings from the public docs:

- `turn/start` returns a turn id, then the app server emits notifications for
  turn and item lifecycle events.
- `turn/completed` is the authoritative terminal notification for a turn. It
  carries the final turn status: `completed`, `interrupted`, or `failed`.
- `error` is documented as a notification emitted when a turn fails; the server
  then emits `turn/completed` with turn status `failed`. Kelos must record the
  error and still wait for the terminal `turn/completed` or stream/process
  failure.
- Progress and observability events that Kelos currently ignores include
  `turn/started`, `item/started`, `item/updated`,
  `item/commandExecution/outputDelta`, `turn/plan/updated`,
  `thread/status/changed`, `item/commandExecution/requestApproval`,
  `item/fileChange/requestApproval`, `item/tool/requestUserInput`,
  `item/tool/call`, and `serverRequest/resolved`.
- The public `turn/start` examples use text input items shaped as
  `{ type: "text", text: "..." }`. Fields beyond the public examples must be
  validated against the deployed image schema during implementation before they
  are kept.

Primary reference: <https://developers.openai.com/codex/app-server>

## Improvement 1: Use Turn Completion as the Terminal Signal

### Problem

`waitForTurn` currently treats every app-server event with method `error` as
terminal:

```go
case "error":
    if message := extractString(msg.Params, "error", "message"); message != "" {
        return "", errors.New(message)
    }
    return "", fmt.Errorf("Codex App Server reported an error")
```

That exits the event loop before Kelos sees the final `turn/completed` event.
Per the public app-server docs, an `error` notification is part of the turn
failure flow and should be followed by `turn/completed` with status `failed`.
Kelos should preserve the error detail, keep reading the stream, and let
`turn/completed` be the terminal decision point.

The observed Slack text was:

```text
Something went wrong: Error: Reconnecting... 2/5
```

If this text arrived through a documented app-server `error` notification, it
should be preserved as failure detail and the turn should fail once the matching
failed `turn/completed` arrives. Similar reconnect/status text from
non-terminal status channels, such as `thread/status/changed` or stderr
diagnostics, should update activity but not fail the turn by itself.

### Functional Requirements

- `turn/completed` is the terminal notification for a turn. Treat
  `status=completed` as success, `status=failed` as failure, and
  `status=interrupted` as a distinct terminal cancellation/interruption.
- An app-server `error` notification must be stored as structured error detail
  and must not cause the runner to return before the matching `turn/completed`.
- A `turn/completed` status of `failed` must fail the `AgentTurn`, using the
  most specific available failure message.
- Non-terminal status/activity events must not fail the `AgentTurn` by
  themselves.
- The runner must continue to fail when:
  - the app-server process exits;
  - the event stream closes;
  - the context is canceled;
  - `turn/completed` reports a failed status;
  - no terminal turn event arrives before a configured hard turn timeout.
- Non-terminal status/activity events must be reflected in
  `AgentTurn.status.activity` and
  structured logs.

### Implementation Shape

In `waitForTurn`:

- on `error`:
  - parse and store `lastAppServerError`;
  - log `codex_app_server_event` with `event_type=error`;
  - patch `AgentTurn.status.activity` to a concise value like
    `Codex app server reported an error; waiting for terminal turn status`;
  - continue waiting.
- on `thread/status/changed`:
  - patch activity to a safe summary, for example
    `Codex session status: active`;
  - do not infer turn terminality from the thread status payload without
    `turn/completed` or stream closure.
- on `turn/completed`:
  - if successful, return the final assistant response as today;
  - if failed, return an error using:
    1. stored app-server error text;
    2. turn error detail from the completed turn payload;
    3. a generic hard error if neither is present.
  - if interrupted, mark the `AgentTurn` as interrupted/canceled rather than
    succeeded.
- on app-server process exit, stream close, context cancellation, or hard turn
  timeout before `turn/completed`, fail the turn and include the stored
  `lastAppServerError` when present.

Add an upper-bound turn timeout to avoid sessions hanging forever after a
non-terminal status event. This can be a constant first, for example `4h`, because the
current user-facing issue is already multi-hour turns. A later PR can promote
this to a TaskSpawner session setting if needed.

### Tests

- Unit-test `waitForTurn` with a fake event channel:
  - `error` followed by failed `turn/completed` fails once, using the stored
    app-server error text;
  - `error` without terminal `turn/completed` fails on stream close or timeout
    with the stored error text;
  - `thread/status/changed` updates activity but does not fail the turn by
    itself;
  - successful `turn/completed` succeeds;
  - event channel close fails;
  - failed `turn/completed` fails.

## Improvement 2: Persist Structured App-Server Event Summaries

### Problem

The runner currently consumes app-server events but mostly does not log them.
Loki only showed startup warnings and a few raw Codex stderr errors. That made
the investigation depend on indirect Slack reporter logs and surviving Job
metadata.

### Functional Requirements

- Every `AgentTurn` must produce structured logs that can be queried by:
  - `agent_session`;
  - `agent_turn`;
  - `codex_thread_id`;
  - `codex_turn_id`;
  - `event_type`;
  - tool kind when available.
- Logs must not include raw prompts, full Slack transcripts, tokens, secret
  values, command output, or raw MCP payloads.
- Logs should be concise but enough to reconstruct the run timeline.
- Logs and status should preserve enough app-server lifecycle sequence to
  distinguish Codex failure, Kelos runner failure, Slack reporter failure, and
  upstream MCP/tool failure.

### Implementation Shape

Add a session-runner logger helper:

```go
func logAppServerEvent(log logr.Logger, session, turn, threadID, codexTurnID string, msg rpcMessage)
```

Emit a structured line for:

- `turn/start` response received;
- `turn/started`;
- `thread/status/changed`;
- `turn/plan/updated`;
- `item/started`;
- `item/updated`;
- `item/completed`;
- `item/agentMessage/delta` only as aggregate progress, not every token;
- `item/commandExecution/outputDelta` only as aggregate byte/count metadata,
  not raw stdout/stderr;
- `item/commandExecution/requestApproval`;
- `item/fileChange/requestApproval`;
- `item/tool/requestUserInput`;
- `item/tool/call`;
- `serverRequest/resolved`;
- `error`;
- `turn/completed`;
- app-server process exit.

For `item/completed`, extract and log:

- `item.type`;
- `item.status`;
- command string only as a short redacted/summarized command name;
- MCP tool name;
- exit code when present;
- duration if the app-server payload exposes one.

For command strings:

- cap at 160 characters;
- redact obvious secrets:
  - `Authorization: Bearer ...`;
  - `token=...`;
  - `_authToken=...`;
  - `password=...`;
  - shell env assignments ending in `_TOKEN`, `_KEY`, `_SECRET`.

For streaming events:

- aggregate `item/agentMessage/delta` by item id and byte/count only;
- aggregate `item/commandExecution/outputDelta` by item id, stream, and
  byte/count only;
- do not persist raw assistant message deltas or command output deltas.

Also patch `AgentTurn.status.activity` for important completed items, but keep
full event detail in logs rather than CR status.

Add a protocol schema guardrail:

- generate the Codex app-server JSON schema from the exact Codex binary used in
  the session image;
- validate request-building helpers for `initialize`, `thread/start`,
  `thread/resume`, `turn/start`, and initialized notifications;
- either keep the `turn/start` input item shape to the public documented
  `{ type: "text", text: "..." }` form, or explicitly validate any extra fields
  against the deployed image schema before shipping;
- fail CI or the image build if request fields are not accepted by the generated
  schema, rather than depending on permissive parsing.

### Tests

- Unit-test redaction for bearer tokens, GitHub package tokens, and password
  patterns.
- Unit-test event summary extraction for:
  - command execution success/failure;
  - MCP tool call success/failure;
  - `thread/status/changed`;
  - `turn/plan/updated`;
  - command output delta aggregation without raw output;
  - turn completion;
  - app-server error.
- Verify log helper does not include raw prompt/transcript fields.

## Improvement 3: Live Slack Progress Updates for AgentTurn

### Problem

`SlackTurnReporter` posts:

1. accepted reply;
2. terminal result.

Long turns can run for tens of minutes or hours, so Slack shows stale progress.
The session runner already writes `AgentTurn.status.activity` from completed
items, but the reporter does not update the accepted Slack message while the
turn is running.

### Functional Requirements

- While an `AgentTurn` is running, update the accepted Slack progress message
  with current activity.
- Updates must be throttled to avoid Slack API spam.
- Terminal reporting remains the source of truth for final answer delivery.
- If progress update fails, do not fail the `AgentTurn`.
- Progress messages must not expose raw command output or credentials.

### Implementation Shape

Extend `SlackTurnReporter.ReportTurnStatus`:

- If phase is running/queued and `SlackProgressMessageTS` is empty, post the
  accepted message as today.
- If phase is running and `SlackProgressMessageTS` is set:
  - derive a progress message from `turn.Status.Activity` or
    `turn.Status.Message`;
  - update the existing progress Slack message using `UpdateMessage`;
  - skip the update if the message content has not changed since last update.

Add status fields only if needed:

- Prefer no CRD change initially.
- If dedupe/throttle needs state, add an annotation or status field:
  - `status.slackProgressHash`;
  - `status.lastProgressReportedAt`.

The lower-risk first implementation can update at the cadence of the existing
Slack reporting loop and rely on content hash dedupe. If a CRD change is
needed, update:

- `api/v1alpha1/agentturn_types.go`;
- generated deepcopy/client code;
- Helm CRD template;
- install CRD manifest.

Progress message content:

```text
:hourglass_flowing_sand: Working on your request...
Current activity: <sanitized activity>
Task: <turn name>
```

Examples:

- `Current activity: command failed: gh pr checks`
- `Current activity: MCP call succeeded: getJiraIssue`
- `Current activity: Codex app server started turn`
- `Current activity: Codex session status: active`
- `Current activity: Codex app server reported an error; waiting for terminal turn status`

### Tests

- Unit-test reporter behavior:
  - accepted message posted once;
  - running activity update edits existing message;
  - identical activity does not re-update when hash/status is unchanged;
  - terminal result still updates the accepted message.
- Existing Slack reporter fake should assert update calls and status patches.

## Improvement 4: Make Interactive stdin / TTY Tool Misuse Actionable

### Problem

The first session had repeated Codex router errors:

```text
write_stdin failed: stdin is closed for this session; rerun exec_command with tty=true to keep stdin open
```

This is a tool-use problem: the model tried to write to a command session that
was not opened with a TTY / persistent stdin. Today this only appears in pod
stderr, so the user sees slow or confusing behavior.

### Functional Requirements

- When this error happens, the turn should expose an actionable progress
  message instead of silently drifting.
- Repeated identical stdin errors should be deduplicated.
- The session prompt should instruct Cody to start interactive commands with
  `tty=true` when follow-up stdin is expected.
- This should not make all command failures terminal. The agent should still
  be able to recover by rerunning the command correctly.

### Implementation Shape

There are two layers:

1. Session prompt guidance.
2. Runtime event recognition.

Prompt guidance:

- Update `renderTurnPrompt` in `cmd/kelos-session-runner/main.go` to include:

```text
For shell commands that require follow-up stdin, an interactive prompt, or a
long-running session you need to write to later, start the command with a TTY.
If a previous command reports that stdin is closed, rerun it as an interactive
TTY session instead of retrying write_stdin.
```

Runtime recognition:

- Add a helper:

```go
func classifyToolRouterError(message string) (activity string, recoverable bool)
```

- Match the exact `write_stdin failed: stdin is closed` phrase.
- Patch `AgentTurn.status.activity` once per turn with:
  `Tool session stdin closed; rerun the command with tty=true if interaction is required`.
- Log structured fields:
  - `event_type=tool_router_error`;
  - `recoverable=true`;
  - `error_kind=stdin_closed`.

The runtime does not need to parse all Codex stderr. If the app-server forwards
the error as an app-server `error` event, handle it there. If it only appears
on stderr, consider teeing child stderr through a scanner in `startAppServer`
that recognizes only known safe operational error lines and logs them with
session context. Do not parse arbitrary stderr into status without redaction.

The current runner attaches app-server stderr directly to pod stderr. If this
improvement needs Slack-visible activity for stderr-only router errors, replace
that direct assignment with a stderr pipe plus tee: Kubernetes logs still get
the original line, while Kelos can classify known safe operational errors and
patch `AgentTurn.status.activity`.

### Tests

- Unit-test prompt rendering contains the TTY guidance.
- Unit-test classifier recognizes the stdin-closed error.
- Unit-test duplicate stdin-closed events do not spam status updates.

## Improvement 5: Stop Privilege-Escalation Policy Warnings

### Problem

Kubernetes events for session Jobs showed Kyverno policy warnings:

```text
policy disallow-privilege-escalation/autogen-privilege-escalation fail:
spec.containers[*].securityContext.allowPrivilegeEscalation must be set to false
```

The Jobs still ran, but these warnings add noise and will become a hard failure
if the policy is tightened.

### Functional Requirements

- Generated session runner Jobs must set
  `securityContext.allowPrivilegeEscalation: false` on the Cody container.
- Do not remove or overwrite user-provided security context fields.
- Preserve existing TaskTemplate `podOverrides`.
- Normal one-shot tasks should not regress.

### Implementation Shape

In `JobBuilder.BuildSessionRunner`, after selecting the first container:

- ensure `c.SecurityContext` exists;
- if `AllowPrivilegeEscalation` is nil, set it to `false`;
- leave it unchanged if the user explicitly set a value.

Depending on current normal Job builder behavior, consider applying the same
default to all agent Jobs in `JobBuilder.Build`, not only sessions. The safer
session-specific change is acceptable for this PR because the observed warnings
were session Jobs.

Also consider setting:

- `runAsNonRoot: true` only if current image/user already satisfies it;
- `readOnlyRootFilesystem` only after validating Codex/session runtime writes.

Do not set those extra fields in this PR unless tests and runtime image
behavior prove they are safe.

### Tests

- Add/extend `internal/controller/job_builder_test.go`:
  - session runner container gets `allowPrivilegeEscalation=false` by default;
  - existing user-provided `allowPrivilegeEscalation` value is not overwritten;
  - other security context fields survive.

## End-to-End Verification Plan

Use a non-prod `!session` test thread after image/chart deployment:

1. Trigger a session:

   ```text
   @cody !session please run a tiny investigation and tell me what you checked
   ```

2. Confirm:
   - one `AgentSession`;
   - one runner Job;
   - one first `AgentTurn`;
   - no duplicate accepted Slack replies.

3. Trigger a follow-up:

   ```text
   @cody continue and check one more thing
   ```

4. Confirm:
   - same `AgentSession`;
   - new `AgentTurn`;
   - accepted Slack message is updated with activity before terminal result.

5. Query Loki:

   ```logql
   {namespace="kelos-system", job="kelos-system/<session-job>"}
   ```

   Confirm structured event summaries include:
   - session name;
   - turn name;
   - codex thread/turn IDs;
   - event types;
   - command/MCP summaries without raw credentials.
   - no raw prompts, Slack transcripts, command output, or token values.

6. Confirm Kubernetes events for the session Job no longer include
   `disallow-privilege-escalation` warnings.

7. Confirm non-terminal session status events, if reproduced, update activity
   and do not mark the turn failed unless `turn/completed` fails.

8. Confirm a documented app-server `error` event, if reproduced, is preserved
   and the runner waits for the terminal `turn/completed` before deciding
   whether the `AgentTurn` succeeds or fails.

## Rollout

1. Implement in Kelos.
2. Run unit tests for:
   - session runner;
   - Slack turn reporter;
   - JobBuilder.
3. Build and push a new `docker.io/alpheya/codex:session` image if
   `/kelos-session-runner` changed inside the Codex image.
4. Build and publish the Kelos chart if CRDs or controller/slack-server code
   changed.
5. Update GitOps only if image tags, chart version, or TaskSpawner values need
   to change.
6. Test in `!session` route before enabling any broader behavior.

## Acceptance Criteria

- A non-terminal app-server status/activity event does not by itself fail an
  `AgentTurn`.
- A documented app-server `error` notification is preserved as failure detail
  and paired with the final failed `turn/completed`.
- A terminal app-server failure still produces a failed `AgentTurn` and Slack
  failure message.
- Long-running session turns update their Slack progress message with sanitized
  activity.
- Loki has useful structured session/turn event logs.
- Interactive stdin misuse is surfaced as actionable activity.
- Session Jobs no longer generate privilege-escalation policy warnings.
- No changes to non-session Cody behavior are required for this work.
