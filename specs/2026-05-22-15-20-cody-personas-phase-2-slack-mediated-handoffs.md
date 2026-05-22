# Cody Personas Phase 2 Conversational Slack Handoff Spec

## Status

Implementation-ready shortcut spec.

This replaces the earlier Slack-mediated design that encoded handoff payloads,
signatures, and loop metadata into Slack messages. Keep
`specs/2026-05-22-14-30-cody-personas-phase-2-handoffs-implementation.md` as the
longer-term Kelos-native design. This shortcut is deliberately unrelated to
that API-heavy path.

## Summary

Cody should hand off like a person would in Slack.

Each persona posts a normal final response with the work it completed, the
important links, and any caveats. If another persona should continue, Cody adds
one clear next-command line to the response:

```text
@cody !dev please implement the ALPM-123 ticket described above and open a PR.
```

or:

```text
@cody !review please review the PR above.
```

The next persona reads the Slack thread context, which Kelos already fetches
for thread replies. We do not put machine metadata, signatures, parent task
fields, loop state, or hidden payloads into the Slack message.

The only Kelos shortcut change is optional auto-continuation: allow Cody's own
Slack replies to trigger the next persona when, and only when, the reply
contains one intentional handoff command line.

Without that Kelos change, the same approach still works as a manual handoff:
the user copies or replies with Cody's suggested `@cody !...` line.

## Goals

- Keep Slack messages human-readable.
- Avoid `TaskSpawner.spec.handoffs`.
- Avoid internal `!handoff` commands.
- Avoid signed payloads, embedded JSON, and visible orchestration metadata.
- Reuse existing user-facing persona routes: `!ticket`, `!dev`, `!review`, and
  new `!babysit`.
- Use Slack thread context as the handoff state.
- Make failure modes visible and easy for a human to recover from.

## Non-Goals

- No router persona.
- No GitHub triggers.
- No Kelos workflow engine.
- No Kubernetes-level parent/child Task dependency.
- No exact-once handoff guarantee.
- No hidden state store for prompts or loop metadata.
- No channel-level Slack whitelist.
- No service account or RBAC split in this phase.

## Current Kelos Behavior

Existing Slack behavior already supports most of this:

- Phase 1 has explicit Slack persona routes for `@cody !ticket`,
  `@cody !dev`, and `@cody !review`.
- For thread replies, `internal/slack/handler.go` fetches full Slack thread
  context and places it in the Task body.
- Task names are derived from channel plus Slack message timestamp, so retrying
  the same Slack event is naturally idempotent.
- Cody's own Slack bot messages are currently ignored before TaskSpawner
  matching. This prevents bot loops, but also prevents automatic handoff from a
  Cody reply.

Therefore, the shortcut only needs a narrow self-message exception for
intentional handoff lines.

## Handoff Message Contract

A Cody handoff is a normal final response plus one command line.

### Ticket to dev

```text
I created ALPM-123 for the portfolio report export bug.

Summary:
- The export fails when report filters include closed accounts.
- The ticket includes reproduction steps and the expected behavior.
- Jira: https://wgen4.atlassian.net/browse/ALPM-123

Next step:
@cody !dev please implement ALPM-123 using the ticket details above and open a PR.
```

### Dev to review

```text
I implemented the export fix and opened a PR.

PR: https://github.com/donchev7/kelos/pull/123
Notes:
- Added a regression test for closed-account report filters.
- Kept the change scoped to export query construction.

Next step:
@cody !review please review the PR above for correctness, regression risk, and missing tests.
```

### Review to dev fix inside babysitting

```text
I reviewed the PR and found two issues that need changes.

Findings:
- The regression test does not cover the empty-filter case.
- The export query still includes closed accounts when multiple filters are combined.

Next step:
@cody !dev please fix the review findings above on the same PR. Babysit fix attempt 1 of 2.
```

### Dev fix to re-review inside babysitting

```text
I pushed fixes to the same PR branch.

Changes:
- Added empty-filter regression coverage.
- Fixed combined-filter closed-account handling.

Next step:
@cody !review please re-review the PR above. Babysit fix attempt 1 of 2.
```

### Clean or blocked terminal states

When no follow-up should happen, Cody does not include a next-command line.

```text
I reviewed the PR and found no blocking issues. No further Cody handoff is needed.
```

```text
I cannot safely fix this automatically because the expected product behavior is ambiguous.
No further Cody handoff is needed until a human clarifies the requirement.
```

## Handoff Line Rules

The auto-continuation path only considers a line a handoff command when it is
outside code fences and matches one of these forms:

```text
@cody !ticket ...
@cody !dev ...
@cody !review ...
@cody !babysit ...
<@CODY_BOT_USER_ID> !ticket ...
<@CODY_BOT_USER_ID> !dev ...
<@CODY_BOT_USER_ID> !review ...
<@CODY_BOT_USER_ID> !babysit ...
```

Rules:

- The command must be on its own line.
- The command line must start with `@cody` or the Slack mention token.
- Only one handoff command line is allowed per Cody response.
- Lines inside fenced code blocks are ignored.
- Inline examples are ignored unless they are on their own line.
- The command line is routed through the existing persona TaskSpawners.
- The next Task body remains the full Slack thread context, so the receiving
  persona sees the prior Cody response and all earlier user context.

This lets Cody speak naturally while still giving Kelos a precise line to route.

## Manual Versus Automatic Continuation

### Manual mode

No Kelos code change is required.

Cody posts the next-command line. A human decides whether to continue and sends
that line as a reply:

```text
@cody !dev please implement ALPM-123 using the ticket details above and open a PR.
```

This is the safest first operating mode, but it is not a true automatic handoff.

### Automatic mode

Kelos treats Cody's own final Slack reply as an eligible Slack event when the
reply contains exactly one valid handoff command line.

This is the shortcut implementation recommended for Phase 2 because it gives
automatic handoff while preserving normal Slack readability.

## Minimal Kelos Change

Add a narrow self-handoff path in the Slack server.

### Config

Add a flag:

```text
SLACK_SELF_HANDOFF_ENABLED=false
SLACK_SELF_HANDOFF_MAX_PER_THREAD=6
```

Defaults:

- `SLACK_SELF_HANDOFF_ENABLED=false`
- `SLACK_SELF_HANDOFF_MAX_PER_THREAD=6`

The feature must be off by default so the image can ship before GitOps opts in.

### Self-handoff detection

Today, `shouldProcess(...)` rejects Cody's own messages. Keep that default.

If the message is from Cody's own Slack bot and
`SLACK_SELF_HANDOFF_ENABLED=true`, run self-handoff detection before rejecting.

Accept the message only when:

- the message is a thread reply
- the text contains exactly one valid handoff command line
- the command target is one of `ticket`, `dev`, `review`, or `babysit`
- the thread has fewer than `SLACK_SELF_HANDOFF_MAX_PER_THREAD` prior
  Cody-authored handoff command lines

If accepted:

1. Normalize `@cody` into the real Slack mention token, for example
   `<@CODY_BOT_USER_ID> !dev ...`.
2. Build `SlackMessageData.Text` from only the command line.
3. Fetch full thread context as usual and use it for `SlackMessageData.Body`.
4. Mark the message as bot-authored with Cody's own `bot_id`.
5. Route through normal TaskSpawner matching.

If rejected:

- Do not create a Task.
- Log the reason.
- Do not post another Slack message for normal rejections. The original Cody
  message remains visible and a human can manually continue if needed.

### TaskSpawner matching

Use the existing `allowedBotIDs` field.

For persona TaskSpawners that may receive Cody self-handoffs, add Cody's Slack
bot ID:

```yaml
when:
  slack:
    allowedBotIDs:
      - <cody-slack-bot-id>
```

This should be added only to the explicit persona routes, not to the stable
debugger route.

Required routes:

- `cody-ticket-slack` if ticket handoff is allowed
- `cody-dev-slack`
- `cody-pr-reviewer-slack`
- `cody-pr-babysitter-slack`

Do not add Cody's bot ID to `cody-debug-slack`.

### Why no new internal TaskSpawners

There is no separate `!handoff` route. The whole point of this shortcut is that
Cody posts the same command a person would post.

That means:

- humans and Cody use the same visible commands
- the same AgentConfigs run either way
- Slack thread context carries the state
- there is less Kelos-specific behavior to explain

## Cody Persona Behavior

### Shared instruction

All personas should follow this rule:

When another Cody persona should continue, end the response with one `Next
step:` section containing exactly one `@cody !...` command line. Do not include
more than one command line. Do not put command examples on standalone lines
unless they are intended to run.

### Ticket creator

If the user only asked for a ticket:

- create or update the ticket
- summarize the ticket
- do not include a next-command line

If the user asked to implement after ticket creation:

```text
Next step:
@cody !dev please implement <ticket-key> using the ticket details above and open a PR.
```

### Dev

Normal dev mode:

- implement the requested change
- open a PR when appropriate
- summarize branch, tests, and PR
- if PR review should run automatically:

```text
Next step:
@cody !review please review the PR above for correctness, regression risk, and missing tests.
```

Dev fix mode inside babysitting:

- only fix reviewer findings from the thread
- push to the same PR branch
- do not change unrelated scope
- after pushing fixes:

```text
Next step:
@cody !review please re-review the PR above. Babysit fix attempt <n> of <max>.
```

### PR reviewer

One-shot review mode:

- review the PR
- report findings
- do not include a dev-fix next-command line unless the user invoked
  babysitting

Babysitter review mode:

- if clean, report clean and stop
- if blocked, report why and stop
- if changes are required and attempts remain:

```text
Next step:
@cody !dev please fix the review findings above on the same PR. Babysit fix attempt <next> of <max>.
```

The reviewer decides whether attempts remain from the visible thread context.
The Kelos safety cap is only a generic backstop.

### PR babysitter

Add a user-facing `@cody !babysit` route.

The babysitter persona does not review or edit code directly. It normalizes the
request and starts the first review:

```text
I will babysit this PR for up to 2 Cody fix attempts.

PR: https://github.com/donchev7/kelos/pull/123

Next step:
@cody !review please review the PR above. This is babysit review pass 0 of 2.
```

## GitOps Changes

### Add babysitter route

Add:

- `agentconfig-cody-pr-babysitter.yaml`
- `taskspawner-cody-pr-babysitter-slack.yaml`

Trigger:

```text
@cody !babysit <PR URL or PR context>
```

### Stable debugger exclusion

Add `!babysit` to the stable debugger exclusion list:

```yaml
excludePatterns:
  - '^!(alpha|exp)\b'
  - '^!(ticket|dev|review|babysit)\b'
```

### Allow Cody self-handoff into persona routes

After the Kelos image supports self-handoff detection, add Cody's Slack bot ID
to `allowedBotIDs` for the persona routes that may be continued by Cody:

```yaml
when:
  slack:
    allowedBotIDs:
      - <cody-slack-bot-id>
```

Do not add this to the catch-all debugger.

### Enable the feature

Set:

```text
SLACK_SELF_HANDOFF_ENABLED=true
SLACK_SELF_HANDOFF_MAX_PER_THREAD=6
```

No new Slack app, signing secret, internal TaskSpawner, or handoff CRD is
required.

## End-To-End Flows

### Ticket to dev to review

1. User sends:

   ```text
   @cody !ticket create a ticket for X and implement it if clear
   ```

2. Ticket creator posts normal ticket summary plus:

   ```text
   @cody !dev please implement ALPM-123 using the ticket details above and open a PR.
   ```

3. Kelos extracts that one line from Cody's thread reply and routes it to
   `cody-dev-slack`.
4. Dev reads the thread, implements, opens a PR, and posts:

   ```text
   @cody !review please review the PR above for correctness, regression risk, and missing tests.
   ```

5. Kelos routes that line to `cody-pr-reviewer-slack`.
6. Reviewer reads the thread and posts final findings.

### Babysit PR

1. User sends:

   ```text
   @cody !babysit https://github.com/donchev7/kelos/pull/123
   ```

2. Babysitter posts a normal response plus:

   ```text
   @cody !review please review the PR above. This is babysit review pass 0 of 2.
   ```

3. Reviewer reviews.
4. If clean or blocked, reviewer stops with no next-command line.
5. If changes are needed, reviewer posts:

   ```text
   @cody !dev please fix the review findings above on the same PR. Babysit fix attempt 1 of 2.
   ```

6. Dev fixes and posts:

   ```text
   @cody !review please re-review the PR above. Babysit fix attempt 1 of 2.
   ```

7. Loop continues until clean, blocked, or the persona decides the max attempt
   has been reached. The Kelos per-thread cap prevents runaway chains if the
   prompt logic fails.

## Unhappy Paths And Fallbacks

| Situation | Behavior |
| --- | --- |
| Cody omits the next-command line | Parent task succeeds; no handoff occurs. Human can invoke the next persona manually. |
| Cody includes two next-command lines | Kelos ignores self-handoff to avoid fanout. Human can choose one manually. |
| Cody puts a command example in a code block | Ignored by self-handoff detection. |
| Cody's message is not a thread reply | Ignored by self-handoff detection. |
| Cody's bot ID is not configured in `allowedBotIDs` | The self-handoff line is detected but no persona route accepts it. Human can send the line manually. |
| Persona TaskSpawner is at max concurrency | Existing Slack handler drops the event. The visible command line remains in Slack for manual retry. |
| Slack event delivery misses Cody's reply | The visible command line remains in Slack for manual retry. |
| Babysitter loop goes wrong | Per-thread self-handoff cap stops runaway chains. Human can continue manually if needed. |
| Reviewer says blocked | No next-command line is included, so the loop stops. |

The main fallback is intentionally human: because the handoff is just a normal
Slack command, anyone can reply with the same command line later.

## Known Tradeoffs

- This is less deterministic than Kelos-native handoffs.
- Loop state is conversational, not controller-enforced.
- Same-PR discipline is prompt-enforced, not Kubernetes-enforced.
- Max attempts rely on persona instructions, with only a generic per-thread cap
  as backup.
- There is no parent/child Task relationship beyond Slack thread context.
- Slack delivery is part of the workflow.

These are acceptable for the shortcut because the implementation is small and
the Slack thread stays understandable.

## Implementation Plan

### 1. Add self-handoff detection

Files:

- `internal/slack/handler.go`
- `internal/slack/filter.go`

Acceptance criteria:

- Cody self messages remain ignored by default.
- With `SLACK_SELF_HANDOFF_ENABLED=true`, Cody self thread replies with exactly
  one valid handoff line are eligible for routing.
- Non-thread self messages are ignored.
- Messages with zero or multiple handoff lines are ignored.
- Lines in fenced code blocks are ignored.
- The routed message text is only the normalized command line.
- The Task body remains full Slack thread context.

### 2. Add per-thread handoff cap

Files:

- `internal/slack/handler.go`
- `internal/slack/thread.go`

Acceptance criteria:

- self-handoff routing counts prior Cody-authored handoff command lines in the
  thread
- default cap is 6
- cap can be configured with `SLACK_SELF_HANDOFF_MAX_PER_THREAD`
- cap rejection creates no Task and logs a clear reason

### 3. GitOps

Files in `k8s-platform-gitops/non-prod/kelos`:

- add `cody-pr-babysitter` AgentConfig
- add `cody-pr-babysitter-slack` TaskSpawner
- add `!babysit` to debugger excludes
- add Cody bot ID to `allowedBotIDs` for persona routes that may receive
  self-handoffs
- set `SLACK_SELF_HANDOFF_ENABLED=true`

### 4. AgentConfig updates

Update the persona prompts so each persona knows when to include one
next-command line and when to stop.

## Tests

Unit tests:

- extract one `@cody !dev` line
- extract one Slack mention-token `!review` line
- ignore lines inside fenced code blocks
- ignore messages with no handoff line
- ignore messages with multiple handoff lines
- normalize literal `@cody` to the bot mention token
- enforce per-thread self-handoff cap

Slack handler tests:

- self message ignored when feature flag is off
- self thread reply with one valid line routes when flag is on
- self non-thread message is ignored
- human `@cody !dev` behavior is unchanged
- Cody bot ID must be allowlisted on the target persona spawner
- stable debugger does not receive self-handoff messages

Manual tests:

Ticket only:

```text
@cody !ticket create a test-only ticket and do not implement it
```

Expected: ticket creator responds; no next-command line; no handoff.

Ticket to dev:

```text
@cody !ticket create a test-only ticket and implement it if clear
```

Expected: ticket creator posts a normal response plus one `@cody !dev ...`
line; dev runs from the same thread.

Dev to review:

```text
@cody !dev make a tiny docs-only test PR
```

Expected: dev opens a PR and posts one `@cody !review ...` line; reviewer runs.

Babysit:

```text
@cody !babysit https://github.com/donchev7/kelos/pull/<test-pr>
```

Expected: babysitter starts review; review/dev-fix/re-review continues only
while Cody includes next-command lines.

Multiple command guard:

Have Cody post two standalone `@cody !...` lines in a test thread.

Expected: no self-handoff Task is created.

## Rollback

Fast rollback:

```text
SLACK_SELF_HANDOFF_ENABLED=false
```

GitOps rollback:

- remove Cody's bot ID from persona `allowedBotIDs`
- remove `cody-pr-babysitter-slack` if babysitting is problematic
- keep Phase 1 `!ticket`, `!dev`, and `!review` routes

Manual operation continues to work: Cody can still suggest next-command lines,
and humans can send them explicitly.

## Acceptance Criteria

This shortcut is complete when:

- Cody can post normal final responses with one next-command line.
- Kelos can optionally auto-route Cody's own handoff line to the next persona.
- No hidden Slack payload, signature, or internal `!handoff` command is needed.
- `@cody !babysit` exists and can run a bounded conversational review/fix loop.
- Existing human-triggered Phase 1 behavior is unchanged.
- Disabling `SLACK_SELF_HANDOFF_ENABLED` returns the system to manual handoff
  suggestions only.
