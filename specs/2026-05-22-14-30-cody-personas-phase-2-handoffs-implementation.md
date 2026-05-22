# Cody Personas Phase 2 Handoff Implementation Spec

## Status

Implementation-ready Phase 2 spec.

Scope:

- Kelos code changes for result-driven Task handoffs.
- Cody GitOps follow-up after the Kelos image and CRDs are released.
- Slack persona handoffs only.
- Explicit PR babysitting with a bounded dev/review loop.

Out of scope:

- No router persona.
- No GitHub triggers.
- No channel-level Slack whitelist.
- No new Cody service accounts or RBAC split.

This spec builds on:

- `specs/2026-05-22-12-00-cody-personas-and-handoffs.md`
- `specs/2026-05-22-13-06-cody-personas-phase-1-implementation.md`

Phase 1 is assumed to be complete: Cody has explicit Slack persona entrypoints
for `@cody !ticket`, `@cody !dev`, and `@cody !review`, while normal
`@cody ...` debugger behavior remains unchanged.

Phase 2 adds one new explicit Slack entrypoint:

- `@cody !babysit <PR URL or PR context>`

## Summary

Phase 2 adds automatic Slack-thread handoffs between Cody personas.

A persona Task can finish with structured results such as:

```text
handoff.target: dev
handoff.reason: Ticket ALPM-123 is ready for implementation.
handoff.prompt: Implement ALPM-123 and open a PR.
ticket: ALPM-123
```

Kelos then evaluates handoff rules configured on the parent `TaskSpawner`. If a
rule matches the parent Task phase and results, Kelos creates exactly one child
Task from a handoff `taskTemplate`.

The child Task:

- uses the target persona AgentConfig stack
- reports back to the same Slack thread
- carries lineage labels and annotations
- depends on the parent Task by default
- is subject to the same `TaskSpawner` task limits

The intent is to support practical Cody flows such as:

- `@cody !ticket ...` creates or updates a Jira ticket, then hands off to dev
  when implementation is requested.
- `@cody !dev ALPM-123 ...` opens a PR, then hands off to PR review when a PR
  URL is emitted.
- `@cody !babysit <PR>` starts a bounded PR babysitter loop: review the PR,
  hand findings to dev for focused fixes, re-review, and stop when clean,
  blocked, or max fix attempts are reached.

Automatic reviewer-to-dev fix loops only run inside the explicit babysitter
route. One-shot `@cody !dev ...` and `@cody !review ...` remain one-pass routes
unless a later GitOps change opts them into babysitting.

## Design Principles

- Preserve Phase 1 behavior unless a `TaskSpawner` explicitly configures
  `spec.handoffs`.
- Keep handoff policy declarative and near the source persona route.
- Let agents request handoff only through structured task outputs, not through
  Kubernetes API access.
- Reuse existing Task primitives: `taskTemplate`, metadata templating,
  `dependsOn`, `status.results`, Slack reporting annotations, and
  `maxConcurrency` / `maxTotalTasks`.
- Make handoffs auditable through labels, annotations, owner references,
  Kubernetes events, and parent/child task status.
- Keep this narrower than a workflow engine. Phase 2 only creates child Tasks
  when a parent Task reaches a configured terminal phase and result predicate.
- Keep automatic loops bounded by explicit loop metadata, max attempts, and
  same-PR constraints.

## Current Kelos Behavior

Kelos already supports most of the required pieces:

| Capability | Current behavior | Phase 2 use |
| --- | --- | --- |
| `TaskSpawner` | Creates Tasks from source events. | Owns persona trigger and handoff rules. |
| `TaskTemplate` | Defines type, prompt, AgentConfigs, workspace, pod overrides, labels, annotations, and TTL. | Reused for child handoff Tasks. |
| `Task.status.results` | Parsed from `key: value` lines emitted by `kelos-capture`. | Drives handoff matching and child prompt templates. |
| `dependsOn` | Downstream Tasks can wait on previous Tasks and read dependency outputs through `.Deps`. | Child handoff Tasks depend on the parent by default. |
| Slack reporting annotations | Slack-origin Tasks report status and final responses in the Slack thread. | Child Tasks inherit these annotations to continue in the same thread. |
| TaskSpawner labels | Spawned Tasks receive `kelos.dev/taskspawner`. | Child Tasks use the same label so limits and audit queries include them. |

The missing pieces are:

- a safe way for agents to emit additional result keys
- a CRD field to declare dynamic handoff rules
- a controller path that creates child Tasks when rules match
- Cody GitOps configuration that wires persona-to-persona flows

## Non-Goals

- Do not introduce a router persona.
- Do not add GitHub webhook, GitHub PR polling, GitHub comment, or GitHub label
  triggers.
- Do not let agents create arbitrary Kubernetes Tasks directly.
- Do not add a generic DAG/workflow API.
- Do not automatically create reviewer-to-dev remediation loops outside the
  explicit `@cody !babysit` route.
- Do not change behavior for TaskSpawners that omit `spec.handoffs`.
- Do not change the stable debugger route.
- Do not split Cody service accounts in this phase.
- Do not rely on channel-level Slack filtering.
- Do not let the PR babysitter change unrelated PR scope or switch to a
  different branch/PR mid-loop.

## Proposed API

Add `handoffs` to `TaskSpawnerSpec`.

```go
type TaskSpawnerSpec struct {
    When TaskSpawnerWhen `json:"when"`
    TaskTemplate TaskTemplate `json:"taskTemplate"`
    PollInterval *metav1.Duration `json:"pollInterval,omitempty"`
    MaxConcurrency *int32 `json:"maxConcurrency,omitempty"`
    Suspend bool `json:"suspend,omitempty"`
    MaxTotalTasks *int32 `json:"maxTotalTasks,omitempty"`

    // Handoffs declares child Tasks to create when a Task spawned by this
    // TaskSpawner reaches a configured phase and result predicate.
    // +optional
    // +kubebuilder:validation:MaxItems=8
    Handoffs []TaskHandoff `json:"handoffs,omitempty"`
}
```

Add the handoff types:

```go
type TaskHandoff struct {
    // Name identifies this handoff rule and is copied to child metadata.
    // +kubebuilder:validation:MinLength=1
    // +kubebuilder:validation:MaxLength=40
    // +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
    Name string `json:"name"`

    // TerminalPhases limits which parent phases may trigger this handoff.
    // Defaults to ["Succeeded"].
    // +optional
    // +kubebuilder:validation:MaxItems=2
    TerminalPhases []TaskPhase `json:"terminalPhases,omitempty"`

    // When contains result predicates that must all match.
    // Empty means phase-only matching.
    // +optional
    When TaskHandoffWhen `json:"when,omitempty"`

    // Inherit controls metadata copied from the parent Task.
    // +optional
    Inherit TaskHandoffInherit `json:"inherit,omitempty"`

    // Loop controls bounded handoff loops such as PR babysitting.
    // +optional
    Loop TaskHandoffLoop `json:"loop,omitempty"`

    // TaskTemplate is rendered and created as the child Task when this
    // handoff matches.
    TaskTemplate TaskTemplate `json:"taskTemplate"`
}

type TaskHandoffWhen struct {
    // Results are ANDed together.
    // +optional
    // +kubebuilder:validation:MaxItems=16
    Results []TaskResultMatch `json:"results,omitempty"`
}

type TaskResultMatch struct {
    // Key is the status.results key to inspect.
    // +kubebuilder:validation:MinLength=1
    // +kubebuilder:validation:MaxLength=80
    // +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`
    Key string `json:"key"`

    // Operator defaults to Exists.
    // +optional
    Operator TaskResultOperator `json:"operator,omitempty"`

    // Value is used by Equals and NotEquals.
    // +optional
    Value string `json:"value,omitempty"`

    // Values is used by In and NotIn.
    // +optional
    // +kubebuilder:validation:MaxItems=32
    Values []string `json:"values,omitempty"`
}

type TaskResultOperator string

const (
    TaskResultExists    TaskResultOperator = "Exists"
    TaskResultEquals    TaskResultOperator = "Equals"
    TaskResultNotEquals TaskResultOperator = "NotEquals"
    TaskResultIn        TaskResultOperator = "In"
    TaskResultNotIn     TaskResultOperator = "NotIn"
)

type TaskHandoffInherit struct {
    // Labels copies selected parent labels by exact key.
    // +optional
    Labels []string `json:"labels,omitempty"`

    // Annotations copies selected parent annotations by exact key.
    // +optional
    Annotations []string `json:"annotations,omitempty"`

    // SlackThread copies the Slack reporting label and thread annotations.
    // +optional
    SlackThread bool `json:"slackThread,omitempty"`

    // DependsOnParent defaults to true. When true, the child Task depends on
    // the parent Task and can read parent outputs through dependency handling.
    // +optional
    DependsOnParent *bool `json:"dependsOnParent,omitempty"`

    // Lineage defaults to true. When true, Kelos writes parent/root/depth
    // labels and annotations.
    // +optional
    Lineage *bool `json:"lineage,omitempty"`
}

type TaskHandoffLoop struct {
    // Name identifies the bounded loop this handoff participates in.
    // Empty means this handoff is not part of a loop.
    // +optional
    // +kubebuilder:validation:MaxLength=40
    // +kubebuilder:validation:Pattern=`^$|^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
    Name string `json:"name,omitempty"`

    // MaxAttempts is the maximum number of fix attempts for this loop.
    // For PR babysitting, this is the maximum number of reviewer-to-dev-fix
    // handoffs, not the maximum number of total Tasks.
    // +optional
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:Maximum=5
    MaxAttempts *int32 `json:"maxAttempts,omitempty"`

    // IncrementAttempt increments the loop attempt counter on the child Task.
    // For PR babysitting, set this on reviewer-to-dev-fix handoffs only.
    // +optional
    IncrementAttempt bool `json:"incrementAttempt,omitempty"`

    // SubjectResultKey binds the loop to one external subject, such as pr.url.
    // If set, the parent Task must have this result key and future loop
    // handoffs must carry the same value.
    // +optional
    // +kubebuilder:validation:MaxLength=80
    // +kubebuilder:validation:Pattern=`^$|^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`
    SubjectResultKey string `json:"subjectResultKey,omitempty"`
}
```

### Validation

CRD validation must enforce:

- `handoffs[].name` is a DNS-label-safe value.
- `handoffs[].terminalPhases`, when set, only contains `Succeeded` or
  `Failed`.
- `handoffs[].when.results[].operator`, when set, is one of `Exists`,
  `Equals`, `NotEquals`, `In`, or `NotIn`.
- `Exists` must not set `value` or `values`.
- `Equals` and `NotEquals` must set `value` and must not set `values`.
- `In` and `NotIn` must set at least one `values` entry and must not set
  `value`.
- `handoffs[].loop.name`, when set, is DNS-label-safe.
- `handoffs[].loop.maxAttempts`, when set, is between 1 and 5.
- `handoffs[].loop.subjectResultKey`, when set, follows the result-key
  pattern.
- `handoffs` is optional and defaults to no behavior.

The existing validation that requires `workspaceRef` for GitHub-backed source
types must remain unchanged.

## Agent Result Output Helper

Add a small helper in the reference agent image so agents can emit additional
Kelos result keys without hand-editing capture markers.

Proposed binary name: `kelos-output`.

Usage:

```bash
kelos-output set handoff.target dev
kelos-output set handoff.reason "Ticket ALPM-123 is ready for implementation."
kelos-output set-file handoff.prompt /tmp/handoff-prompt.txt
kelos-output set-file handoff.prompt.base64 /tmp/handoff-prompt.txt --base64
```

Implementation contract:

- Append line-delimited `key: value` entries to
  `/tmp/kelos-extra-outputs`.
- Use atomic append semantics.
- Reject reserved built-in keys:
  - `branch`
  - `pr`
  - `commit`
  - `base-branch`
  - `cost-usd`
  - `input-tokens`
  - `output-tokens`
  - `response`
- Accept keys matching:
  `^[a-z0-9]([a-z0-9.-]{0,78}[a-z0-9])?$`
- Reject keys longer than 80 characters.
- Reject values containing newlines for `set`.
- Limit `set` values to 16 KiB.
- Limit `set-file` values to 64 KiB after optional base64 encoding.
- Return a non-zero exit code with a clear stderr message on validation
  failure.

`kelos-output` is a convenience and safety boundary. Agents can be instructed
to use it, and `kelos-capture` remains responsible for publishing outputs into
the Task status.

### Capture Changes

Update `internal/capture/capture.go`:

1. Continue emitting existing built-in outputs exactly as today.
2. If `/tmp/kelos-extra-outputs` exists, validate each line as `key: value`.
3. Append validated extra output lines after built-ins.
4. Reject extra output lines that use reserved built-in keys.
5. Ignore a missing extra output file.
6. Surface malformed extra output lines in capture stderr, but do not hide the
   primary agent response.

Because reserved built-in keys are rejected, appending extras after built-ins
will not let an agent override `branch`, `pr`, `commit`, token usage, cost, or
the captured response.

## Handoff Reconciliation

Add handoff reconciliation to the Task controller or a small adjacent
reconciler that watches `Task` status updates.

### Trigger Conditions

For each Task update:

1. The Task must have label `kelos.dev/taskspawner`.
2. The Task phase must be terminal: `Succeeded` or `Failed`.
3. The referenced `TaskSpawner` must exist in the same namespace.
4. The `TaskSpawner` must have at least one `spec.handoffs` entry.
5. Each handoff entry must match:
   - parent terminal phase
   - all configured result predicates
   - handoff safety checks

Default `terminalPhases` is `Succeeded`.

### Result Predicate Semantics

All predicates in a handoff rule are ANDed.

| Operator | Match behavior |
| --- | --- |
| unset or `Exists` | `status.results[key]` exists and is not empty. |
| `Equals` | result exists and equals `value`. |
| `NotEquals` | result is missing or does not equal `value`. |
| `In` | result exists and equals one of `values`. |
| `NotIn` | result is missing or not in `values`. |

String comparison is exact and case-sensitive.

### Child Task Identity

Child Task names must be deterministic and Kubernetes-safe:

```text
<parent-task-name>-<handoff-name>-<hash>
```

If that exceeds 63 characters, truncate the parent segment and keep the suffix
stable. The hash input should include:

- parent Task namespace
- parent Task name
- parent Task UID
- handoff name

This makes re-reconciliation idempotent while avoiding collisions after parent
name reuse.

### Duplicate Prevention

Before creating a child Task, the controller must check for an existing Task in
the namespace with all of:

- `kelos.dev/parent-task=<parent name>`
- `kelos.dev/handoff=<handoff name>`
- `kelos.dev/taskspawner=<parent taskspawner name>`

If one exists, do not create another child.

Also tolerate create conflicts by re-reading the expected child name and
treating an existing child as success when labels match.

### Metadata

Every child Task created by a handoff must include:

Labels:

```yaml
kelos.dev/taskspawner: <source taskspawner name>
kelos.dev/parent-task: <parent task name>
kelos.dev/handoff: <handoff name>
kelos.dev/lineage-root: <root task name>
kelos.dev/lineage-depth: "<depth>"
```

Annotations:

```yaml
kelos.dev/parent-task-uid: <parent task uid>
kelos.dev/handoff: <handoff name>
```

When `loop.name` is set on the handoff or inherited from the parent loop, also
write:

Labels:

```yaml
kelos.dev/loop-name: <loop name>
kelos.dev/loop-attempt: "<attempt>"
kelos.dev/loop-max-attempts: "<max attempts>"
kelos.dev/loop-subject-key: <subject result key>
kelos.dev/loop-subject-hash: <subject value hash>
```

Annotations:

```yaml
kelos.dev/loop-name: <loop name>
kelos.dev/loop-attempt: "<attempt>"
kelos.dev/loop-max-attempts: "<max attempts>"
kelos.dev/loop-subject-key: <subject result key>
kelos.dev/loop-subject: <subject value>
```

Only write `loop-subject-*` metadata when `loop.subjectResultKey` is set or
the parent Task already has loop subject metadata.

When `inherit.slackThread: true`, copy:

Labels:

```yaml
kelos.dev/slack-reporting: enabled
```

Annotations:

```yaml
kelos.dev/slack-reporting: enabled
kelos.dev/slack-channel: <parent value>
kelos.dev/slack-thread-ts: <parent value>
kelos.dev/slack-user-id: <parent value>
```

Only copy Slack annotations that exist on the parent. Missing Slack thread
metadata should not block non-Slack handoffs, but Cody Phase 2 GitOps should set
`inherit.slackThread: true` only for Slack persona handoffs.

When `inherit.labels` or `inherit.annotations` is set, copy only exact listed
keys that exist on the parent. Do not support wildcards in Phase 2.

The handoff `taskTemplate.metadata` is rendered and applied after inherited
metadata. If the same key is present in both inherited metadata and rendered
template metadata, rendered template metadata wins except for reserved Kelos
lineage keys.

Reserved lineage keys cannot be overridden:

- `kelos.dev/taskspawner`
- `kelos.dev/parent-task`
- `kelos.dev/handoff`
- `kelos.dev/lineage-root`
- `kelos.dev/lineage-depth`
- `kelos.dev/parent-task-uid`
- `kelos.dev/loop-name`
- `kelos.dev/loop-attempt`
- `kelos.dev/loop-max-attempts`
- `kelos.dev/loop-subject-key`
- `kelos.dev/loop-subject-hash`
- `kelos.dev/loop-subject`

### Owner References

The child Task should use the same TaskSpawner owner reference behavior as
normal spawned Tasks.

Do not set the parent Task as a Kubernetes owner reference in Phase 2. Parent
Tasks commonly have TTL cleanup, and a parent owner reference would risk
garbage-collecting useful child Tasks. Parentage is tracked through labels and
annotations instead.

### `dependsOn`

`inherit.dependsOnParent` defaults to true.

When true, append the parent Task name to the child `spec.dependsOn` list unless
it is already present. This gives the child Task access to parent outputs
through existing dependency prompt templating and preserves an explicit runtime
relationship.

When false, do not add an implicit dependency. This should be rare and is not
needed for Cody Phase 2.

### Prompt Template Variables

Child handoff templates need parent context without forcing all information
through `.Deps`.

When rendering a handoff child `TaskTemplate`, add:

```text
.Upstream.Name
.Upstream.Namespace
.Upstream.Phase
.Upstream.Message
.Upstream.Outputs
.Upstream.Results
.Upstream.Labels
.Upstream.Annotations
.Lineage.Root
.Lineage.Parent
.Lineage.Depth
.Handoff.Name
.Loop.Name
.Loop.Attempt
.Loop.MaxAttempts
.Loop.SubjectKey
.Loop.Subject
.Loop.SubjectHash
```

Existing source-event template variables must continue to work unchanged for
normal TaskSpawner-created Tasks.

Template rendering should continue to use `missingkey=error`. GitOps handoff
templates should use `index` for optional result keys:

```gotemplate
PR: {{ index .Upstream.Results "pr" }}
Reason: {{ index .Upstream.Results "handoff.reason" }}
```

### Loop Semantics

Loop support is only a bounded counter and metadata model. It does not turn
Kelos into a workflow engine.

For a handoff with `loop.name`:

1. If the parent Task already has `kelos.dev/loop-name`, the child must keep the
   same loop name unless the handoff explicitly starts a new loop from a
   non-loop parent.
2. The current attempt is read from parent label `kelos.dev/loop-attempt`.
   Missing means `0`.
3. The max attempts value is read from `handoff.loop.maxAttempts` when set;
   otherwise from parent label `kelos.dev/loop-max-attempts`.
4. If `handoff.loop.incrementAttempt=true`, the child attempt is parent attempt
   plus 1.
5. If `handoff.loop.incrementAttempt=false`, the child attempt is the parent
   attempt.
6. If incrementing would make the child attempt greater than max attempts,
   block child creation and emit `HandoffBlocked`.
7. If `loop.subjectResultKey` is set, read that key from
   `parent.status.results`. Missing or empty subject values block child
   creation.
8. On the first loop handoff, write the subject key, subject value annotation,
   and subject hash label to the child.
9. On later loop handoffs, require the parent subject hash to match the hash of
   the current subject value. A mismatch blocks child creation.

For PR babysitting, only reviewer-to-dev-fix handoffs increment the attempt.
Dev-fix-to-review handoffs keep the same attempt number.

The babysitter prompts must also tell the reviewer to stop without emitting a
handoff when the current attempt is already at max. The controller check is the
safety net, not the primary user experience.

### Safety Checks

The handoff reconciler must block:

- lineage depth greater than 8
- loop attempt increments that would exceed `loop.maxAttempts`
- loop handoffs that would switch `kelos.dev/loop-name` mid-loop
- loop handoffs whose subject result is missing or differs from the existing
  loop subject
- a handoff whose target template would create a Task with the same handoff name
  and same AgentConfig stack as the parent, unless explicitly allowed in a
  future API
- child creation when the source `TaskSpawner` is suspended
- child creation when `maxTotalTasks` would be exceeded
- child creation when `maxConcurrency` would be exceeded

Concurrency and total-task counting must include both source-event Tasks and
handoff child Tasks because they share the same `kelos.dev/taskspawner` label.

When blocked, record a Kubernetes event on the parent Task and the TaskSpawner.
Do not mutate the parent Task phase.

### Kubernetes Events

Emit events for:

| Event reason | Object | When |
| --- | --- | --- |
| `HandoffCreated` | parent Task and TaskSpawner | A child Task is created. |
| `HandoffSkipped` | parent Task | A rule does not match or an idempotent child already exists. |
| `HandoffBlocked` | parent Task and TaskSpawner | A safety, suspend, concurrency, or total limit prevents creation. |
| `HandoffFailed` | parent Task and TaskSpawner | Child rendering or create fails unexpectedly. |

Event messages should include handoff name, child task name when available, and
the blocking reason.

## Slack Reporting Behavior

No new Slack API behavior is required for the minimum viable Phase 2.

When `inherit.slackThread: true`, the child Task inherits the Slack reporting
label and thread annotations. The existing Slack reporter will then post child
Task accepted/running/final responses in the same Slack thread as the parent.

Expected user experience:

1. User invokes `@cody !ticket ...`.
2. Ticket persona replies in the Slack thread with its final response.
3. If it emitted a matching handoff result, Kelos creates the dev child Task.
4. Dev persona status and final response appear in the same thread.
5. If dev opens a PR and emits a matching handoff result, Kelos creates the PR
   reviewer child Task.
6. Reviewer status and final response appear in the same thread.

For `@cody !babysit <PR>`, the same Slack thread should contain the full
babysitter sequence:

1. Babysitter normalizes the PR URL/context and starts the loop.
2. Reviewer checks the PR.
3. If clean, the loop stops.
4. If changes are requested and attempts remain, dev-fix runs on the same PR
   branch.
5. Reviewer checks the updated PR.
6. The loop stops when clean, blocked, or max fix attempts are reached.

Optional later polish: include `kelos.dev/handoff` or persona labels in Slack
status blocks so users can visually distinguish parent and child personas. This
is not required for Phase 2.

## Cody Result Contract

Persona AgentConfigs should instruct Cody to emit these result keys through
`kelos-output` when handoff is appropriate.

### Common handoff keys

| Key | Required | Purpose |
| --- | --- | --- |
| `handoff.target` | Yes | Target persona route, such as `dev` or `pr-reviewer`. |
| `handoff.reason` | Recommended | Short reason for audit and child prompt context. |
| `handoff.prompt` | Recommended | Human-readable child prompt. Keep under 16 KiB. |
| `handoff.prompt.base64` | Optional | Larger or multiline child prompt, base64 encoded. |
| `pr.url` | Optional | PR URL supplied by the user or discovered by a babysitter. Use this when the built-in `pr` result is unavailable. |
| `loop.status` | Optional | Loop state such as `continue`, `clean`, `blocked`, or `max-attempts`. |

Prefer `handoff.prompt` when the prompt is one line or can be compact. Use
`handoff.prompt.base64` when preserving multiline formatting matters.

### Ticket creator outputs

| Key | Purpose |
| --- | --- |
| `ticket` | Jira issue key such as `ALPM-123`. |
| `ticket.url` | Jira issue URL when available. |
| `handoff.target=dev` | Emit only when implementation should start automatically. |

Ticket creator should not hand off to dev for requests that only ask for ticket
creation, refinement, or backlog grooming.

### Dev outputs

| Key | Purpose |
| --- | --- |
| `branch` | Existing built-in output when a branch was created or used. |
| `commit` | Existing built-in output when a commit exists. |
| `pr` | Existing built-in output when a PR was opened. |
| `handoff.target=pr-reviewer` | Emit only when the PR is ready for automated review. |

Dev should not emit reviewer handoff when no PR was opened.

### PR reviewer outputs

| Key | Purpose |
| --- | --- |
| `review.findings` | Optional compact summary count or category. |
| `review.result` | Optional value such as `clean`, `changes-requested`, or `blocked`. |
| `handoff.target=dev-fix` | Emit only inside the PR babysitter loop when changes are requested and another fix attempt is allowed. |

One-shot reviewer Tasks must not emit `handoff.target=dev-fix`. Only reviewer
Tasks created inside the PR babysitter loop may emit that target. If a one-shot
review finds issues, it should tell the user how to invoke `@cody !dev ...` or
`@cody !babysit ...`.

### PR babysitter outputs

| Key | Purpose |
| --- | --- |
| `pr.url` | PR URL to babysit. |
| `handoff.target=pr-reviewer` | Starts or continues review. |
| `handoff.reason` | Short audit reason for the next child Task. |

The babysitter persona should normalize the initial Slack request into a PR URL
and start the first review handoff. It should not modify code directly.

### Dev-fix outputs

| Key | Purpose |
| --- | --- |
| `fix.pushed=true` | Dev successfully pushed a focused fix to the existing PR branch. |
| `fix.result=blocked` | Dev could not safely apply the requested fix. |
| `handoff.target=pr-reviewer` | Emit only after pushing fixes that should be re-reviewed. |

Dev-fix mode is narrower than normal dev mode. It must only address reviewer
findings for the existing PR and must not re-scope the original ticket.

## Cody GitOps Follow-Up

After Kelos Phase 2 is released, update the Cody Slack persona TaskSpawners in
`k8s-platform-gitops/non-prod/kelos`.

This GitOps follow-up is intentionally separate from the Kelos code PR because
it depends on the new CRD and controller behavior being deployed.

Required GitOps additions:

- Add `agentconfig-cody-pr-babysitter.yaml`.
- Add `taskspawner-cody-pr-babysitter-slack.yaml`.
- Add `!babysit` to the stable debugger exclusion list so normal debugger
  routing does not also answer babysitter requests.
- Keep `mentionOptional` unset. Users must type `@cody !babysit ...`.

### PR babysitter route

Add a new Slack TaskSpawner for:

```text
@cody !babysit <PR URL or PR context>
```

The source Task should use `cody-pr-babysitter` and only normalize the request
into structured outputs. It should not review or edit code directly.

Recommended source Task behavior:

```text
pr.url: https://github.com/<org>/<repo>/pull/<number>
handoff.target: pr-reviewer
handoff.reason: User asked Cody to babysit this PR until clean, blocked, or max attempts are reached.
```

Configure the babysitter TaskSpawner with all three loop handoffs below.

#### Babysitter to reviewer

```yaml
handoffs:
  - name: babysit-to-review
    terminalPhases:
      - Succeeded
    when:
      results:
        - key: handoff.target
          operator: Equals
          value: pr-reviewer
        - key: pr.url
          operator: Exists
    inherit:
      slackThread: true
      dependsOnParent: true
      lineage: true
    loop:
      name: pr-babysit
      maxAttempts: 2
      subjectResultKey: pr.url
    taskTemplate:
      type: codex
      credentials:
        type: oauth
        secretRef:
          name: cody-codex-credentials
      image: docker.io/alpheya/codex:main
      ttlSecondsAfterFinished: 3600
      agentConfigRefs:
        - name: cody-base
        - name: cody-pr-reviewer
        - name: cody-atlassian-mcp
      metadata:
        labels:
          cody.alpheya.com/persona: pr-reviewer
          cody.alpheya.com/source: slack-babysit-handoff
      promptTemplate: |
        Cody PR babysitter review pass.

        Parent task: {{ .Upstream.Name }}
        PR: {{ index .Upstream.Results "pr.url" }}
        Loop: {{ .Loop.Name }}
        Fix attempt: {{ .Loop.Attempt }} of {{ .Loop.MaxAttempts }}
        Handoff reason: {{ index .Upstream.Results "handoff.reason" }}

        Review this PR. If it is clean, emit review.result=clean and do not
        emit a handoff target. If changes are required and fix attempts remain,
        emit review.result=changes-requested and handoff.target=dev-fix with a
        concise handoff.prompt describing the exact required fixes. When
        emitting a handoff, also emit pr.url exactly as shown above. If blocked
        or max attempts have been reached, emit review.result=blocked and do
        not emit a handoff target.
```

#### Reviewer to dev-fix

Add a second handoff entry to the same babysitter TaskSpawner:

```yaml
  - name: review-to-dev-fix
    terminalPhases:
      - Succeeded
    when:
      results:
        - key: handoff.target
          operator: Equals
          value: dev-fix
        - key: review.result
          operator: Equals
          value: changes-requested
        - key: pr.url
          operator: Exists
    inherit:
      slackThread: true
      dependsOnParent: true
      lineage: true
    loop:
      name: pr-babysit
      maxAttempts: 2
      incrementAttempt: true
      subjectResultKey: pr.url
    taskTemplate:
      type: codex
      credentials:
        type: oauth
        secretRef:
          name: cody-codex-credentials
      image: docker.io/alpheya/codex:main
      ttlSecondsAfterFinished: 3600
      agentConfigRefs:
        - name: cody-base
        - name: cody-dev
        - name: cody-atlassian-mcp
      metadata:
        labels:
          cody.alpheya.com/persona: dev
          cody.alpheya.com/mode: pr-babysitter-fix
          cody.alpheya.com/source: slack-babysit-handoff
      promptTemplate: |
        Cody PR babysitter fix pass.

        Parent review task: {{ .Upstream.Name }}
        PR: {{ index .Upstream.Results "pr.url" }}
        Loop: {{ .Loop.Name }}
        Fix attempt: {{ .Loop.Attempt }} of {{ .Loop.MaxAttempts }}
        Review result: {{ index .Upstream.Results "review.result" }}
        Review findings:
        {{ index .Upstream.Results "handoff.prompt" }}

        Apply only the requested fixes to the existing PR branch. Do not change
        unrelated scope. Push the fix to the same PR branch. If fixes were
        pushed, emit fix.pushed=true, pr.url exactly as shown above, and
        handoff.target=pr-reviewer. If the fix cannot be applied safely, emit
        fix.result=blocked and do not emit a handoff target.
```

#### Dev-fix to reviewer

Add a third handoff entry to the same babysitter TaskSpawner:

```yaml
  - name: dev-fix-to-review
    terminalPhases:
      - Succeeded
    when:
      results:
        - key: handoff.target
          operator: Equals
          value: pr-reviewer
        - key: fix.pushed
          operator: Equals
          value: "true"
        - key: pr.url
          operator: Exists
    inherit:
      slackThread: true
      dependsOnParent: true
      lineage: true
    loop:
      name: pr-babysit
      maxAttempts: 2
      subjectResultKey: pr.url
    taskTemplate:
      type: codex
      credentials:
        type: oauth
        secretRef:
          name: cody-codex-credentials
      image: docker.io/alpheya/codex:main
      ttlSecondsAfterFinished: 3600
      agentConfigRefs:
        - name: cody-base
        - name: cody-pr-reviewer
        - name: cody-atlassian-mcp
      metadata:
        labels:
          cody.alpheya.com/persona: pr-reviewer
          cody.alpheya.com/source: slack-babysit-handoff
      promptTemplate: |
        Cody PR babysitter re-review pass.

        Parent fix task: {{ .Upstream.Name }}
        PR: {{ index .Upstream.Results "pr.url" }}
        Loop: {{ .Loop.Name }}
        Fix attempt: {{ .Loop.Attempt }} of {{ .Loop.MaxAttempts }}

        Re-review the same PR after Cody dev pushed fixes. If clean, emit
        review.result=clean and do not emit a handoff target. If changes are
        still required and another fix attempt remains, emit
        review.result=changes-requested and handoff.target=dev-fix with a
        concise handoff.prompt. When emitting a handoff, also emit pr.url
        exactly as shown above. If blocked or max attempts have been reached,
        emit review.result=blocked and do not emit a handoff target.
```

Use the same runtime fields as the Phase 1 dev and reviewer TaskSpawners,
including `podOverrides`, GitHub App env, JWT env, and service account.

### Ticket to dev handoff

Add a `handoffs` entry to `cody-ticket-slack`:

```yaml
handoffs:
  - name: ticket-to-dev
    terminalPhases:
      - Succeeded
    when:
      results:
        - key: handoff.target
          operator: Equals
          value: dev
        - key: ticket
          operator: Exists
    inherit:
      slackThread: true
      dependsOnParent: true
      lineage: true
    taskTemplate:
      type: codex
      credentials:
        type: oauth
        secretRef:
          name: cody-codex-credentials
      image: docker.io/alpheya/codex:main
      ttlSecondsAfterFinished: 3600
      agentConfigRefs:
        - name: cody-base
        - name: cody-dev
        - name: cody-atlassian-mcp
      metadata:
        labels:
          cody.alpheya.com/persona: dev
          cody.alpheya.com/source: slack-handoff
      promptTemplate: |
        Cody dev handoff from ticket creator.

        Parent task: {{ .Upstream.Name }}
        Ticket: {{ index .Upstream.Results "ticket" }}
        Ticket URL: {{ index .Upstream.Results "ticket.url" }}
        Handoff reason: {{ index .Upstream.Results "handoff.reason" }}

        User request for implementation:
        {{ index .Upstream.Results "handoff.prompt" }}
```

Use the same runtime fields as the Phase 1 dev TaskSpawner, including
`podOverrides`, GitHub App env, JWT env, and service account.

### Dev to PR reviewer handoff

Add a `handoffs` entry to `cody-dev-slack`:

```yaml
handoffs:
  - name: dev-to-review
    terminalPhases:
      - Succeeded
    when:
      results:
        - key: handoff.target
          operator: Equals
          value: pr-reviewer
        - key: pr
          operator: Exists
    inherit:
      slackThread: true
      dependsOnParent: true
      lineage: true
    taskTemplate:
      type: codex
      credentials:
        type: oauth
        secretRef:
          name: cody-codex-credentials
      image: docker.io/alpheya/codex:main
      ttlSecondsAfterFinished: 3600
      agentConfigRefs:
        - name: cody-base
        - name: cody-pr-reviewer
        - name: cody-atlassian-mcp
      metadata:
        labels:
          cody.alpheya.com/persona: pr-reviewer
          cody.alpheya.com/source: slack-handoff
      promptTemplate: |
        Cody PR review handoff from dev.

        Parent task: {{ .Upstream.Name }}
        PR: {{ index .Upstream.Results "pr" }}
        Branch: {{ index .Upstream.Results "branch" }}
        Commit: {{ index .Upstream.Results "commit" }}
        Handoff reason: {{ index .Upstream.Results "handoff.reason" }}

        Review this PR using the PR reviewer persona. Focus on correctness,
        regression risk, missing tests, security concerns, and whether the
        implementation satisfies the ticket or Slack request.
```

Use the same runtime fields as the Phase 1 reviewer TaskSpawner.

### AgentConfig updates

Add `cody-pr-babysitter` instructions:

- Parse the Slack request and identify the PR URL or exact PR context.
- If no PR is identifiable, ask for a PR URL and do not emit handoff keys.
- Emit `pr.url`.
- Emit `handoff.target=pr-reviewer`.
- Emit `handoff.reason`.
- Do not review code directly.
- Do not edit code directly.

Update `cody-ticket-creator` instructions:

- Use `kelos-output set ticket <KEY>` after creating or updating a Jira ticket.
- Use `kelos-output set ticket.url <URL>` when a URL is available.
- Emit `handoff.target=dev` only when the user explicitly asked for
  implementation after ticket creation or the ticket text clearly requires
  immediate implementation.
- Emit `handoff.prompt` with a concise implementation brief.

Update `cody-dev` instructions:

- Use the existing PR creation workflow.
- Emit `handoff.target=pr-reviewer` only after a PR is opened and ready for
  review.
- Emit `handoff.reason` with a short reason.
- Let the built-in capture output provide `branch`, `commit`, and `pr`.

Update `cody-pr-reviewer` instructions:

- In one-shot review mode, do not emit `handoff.target=dev-fix`.
- In PR babysitter mode, emit `review.result=clean` and no handoff target when
  the PR is clean.
- In PR babysitter mode, emit `review.result=changes-requested`,
  `handoff.target=dev-fix`, `pr.url`, and a concise `handoff.prompt` when
  changes are needed and another fix attempt remains.
- In PR babysitter mode, emit `review.result=blocked` and no handoff target
  when blocked or when max attempts are reached.
- If one-shot review finds issues, tell the user the exact `@cody !dev ...` or
  `@cody !babysit ...` command to invoke.

Update `cody-dev` instructions for babysitter fix mode:

- Detect `cody.alpheya.com/mode: pr-babysitter-fix` or the babysitter fix
  prompt.
- Only address reviewer findings for the existing PR.
- Do not change unrelated scope.
- Push to the existing PR branch only.
- Emit `fix.pushed=true`, `pr.url`, and `handoff.target=pr-reviewer` after a
  fix is pushed.
- Emit `fix.result=blocked` and no handoff target when the fix cannot be
  applied safely.

## Example End-to-End Flows

### Ticket to dev to review

User message:

```text
@cody !ticket create a ticket for the portfolio report export bug and implement it if the ticket is clear
```

Expected sequence:

1. `cody-ticket-slack` creates a ticket Task.
2. Ticket persona creates `ALPM-123`.
3. Ticket persona emits:

   ```text
   ticket: ALPM-123
   ticket.url: https://wgen4.atlassian.net/browse/ALPM-123
   handoff.target: dev
   handoff.reason: User asked to implement after ticket creation.
   handoff.prompt: Implement ALPM-123. Reproduce the portfolio report export bug, add a focused fix and tests, then open a PR.
   ```

4. Handoff reconciler creates a dev child Task in the same Slack thread.
5. Dev persona implements the fix and opens a PR.
6. Dev capture emits built-in `branch`, `commit`, and `pr`; dev also emits:

   ```text
   handoff.target: pr-reviewer
   handoff.reason: PR is open and ready for review.
   ```

7. Handoff reconciler creates a PR reviewer child Task in the same Slack thread.
8. Reviewer posts findings in the same Slack thread.

### PR babysitter loop

User message:

```text
@cody !babysit https://github.com/donchev7/kelos/pull/123
```

Expected sequence:

1. `cody-pr-babysitter-slack` creates a babysitter Task.
2. Babysitter emits:

   ```text
   pr.url: https://github.com/donchev7/kelos/pull/123
   handoff.target: pr-reviewer
   handoff.reason: User asked Cody to babysit this PR.
   ```

3. Handoff reconciler creates a reviewer child Task with
   `kelos.dev/loop-name=pr-babysit`, `kelos.dev/loop-attempt=0`, and
   `kelos.dev/loop-max-attempts=2`.
4. If reviewer finds no issues, it emits `review.result=clean` and no handoff
   target. The loop stops.
5. If reviewer finds issues, it emits:

   ```text
   review.result: changes-requested
   pr.url: https://github.com/donchev7/kelos/pull/123
   handoff.target: dev-fix
   handoff.prompt: Fix the missing assertion in the handoff controller test.
   ```

6. Handoff reconciler creates a dev-fix child Task with
   `kelos.dev/loop-attempt=1`.
7. Dev-fix pushes focused fixes and emits:

   ```text
   fix.pushed: true
   pr.url: https://github.com/donchev7/kelos/pull/123
   handoff.target: pr-reviewer
   ```

8. Handoff reconciler creates another reviewer child Task with the same attempt
   number.
9. The loop continues until clean, blocked, or the next reviewer-to-dev-fix
   handoff would exceed max attempts.

## Implementation Plan

### 1. Add `kelos-output`

Files to add or modify:

- reference agent image scripts or binaries
- image packaging files
- tests for helper validation

Acceptance criteria:

- `kelos-output set key value` appends a valid output line.
- Reserved keys are rejected.
- Invalid keys are rejected.
- Newlines in `set` values are rejected.
- Size limits are enforced.
- `set-file --base64` writes base64 output.

### 2. Extend capture

Files to modify:

- `internal/capture/capture.go`
- capture tests

Acceptance criteria:

- Existing built-in capture output is unchanged.
- Missing `/tmp/kelos-extra-outputs` is ignored.
- Valid extra outputs appear in `Task.status.outputs` and
  `Task.status.results`.
- Reserved key attempts are rejected and cannot override built-ins.
- Malformed extra output lines are surfaced clearly.

### 3. Add API fields and CRD generation

Files to modify:

- `api/v1alpha1/taskspawner_types.go`
- generated deepcopy files
- CRD manifests
- API docs if present

Acceptance criteria:

- `TaskSpawner.spec.handoffs` is optional.
- `TaskSpawner.spec.handoffs[].loop` is optional.
- Existing TaskSpawner YAML remains valid.
- Validation rejects invalid operators and invalid result match shapes.
- Validation rejects invalid loop names and invalid max attempts.
- Validation rejects invalid loop subject result keys.
- Generated CRDs include the new schema.

### 4. Implement handoff matching and child creation

Recommended files:

- `internal/controller/task_handoff.go`
- `internal/controller/task_handoff_test.go`
- small changes in `internal/controller/task_controller.go`
- reuse `internal/taskbuilder/builder.go`

Acceptance criteria:

- A matching terminal parent creates exactly one child Task.
- Reconciliation is idempotent.
- Non-matching result predicates create no child.
- Failed parents only trigger rules that include `Failed`.
- Child Task inherits Slack reporting when configured.
- Child Task has lineage metadata.
- Child Task depends on parent by default.
- Child prompt template can read `.Upstream.Results`.
- Child prompt template can read `.Loop` values for loop handoffs.
- Loop attempts increment only on handoffs with
  `loop.incrementAttempt=true`.
- Loop handoffs are blocked when the next attempt would exceed max attempts.
- Loop handoffs are blocked when the subject changes mid-loop.
- `maxConcurrency`, `maxTotalTasks`, and `suspend` block child creation.
- Handoff blocked/created/failed events are emitted.

### 5. Documentation and examples

Files to add or modify:

- example TaskSpawner YAML
- user-facing handoff docs if Kelos has a docs area
- Cody persona docs if kept in Kelos

Acceptance criteria:

- Docs show `handoffs` YAML.
- Docs show `kelos-output` usage.
- Docs explain Slack thread inheritance and lineage labels.

### 6. Release and GitOps follow-up

After the Kelos PR merges:

1. Build and publish a Kelos controller image that includes the handoff
   reconciler and `kelos-output` support in the reference agent image.
2. Apply CRDs through the existing deployment path.
3. Update `k8s-platform-gitops/non-prod/kelos` with Cody handoff rules.
4. Wait for Flux to apply the GitOps PR.
5. Run manual Slack tests listed below.

## Test Plan

### Unit tests

- `kelos-output` accepts valid keys and values.
- `kelos-output` rejects reserved keys.
- `kelos-output` rejects malformed keys.
- `kelos-output set-file --base64` produces a single-line base64 value.
- Capture appends valid extra outputs.
- Capture rejects reserved extra output keys.
- Result matcher covers `Exists`, `Equals`, `NotEquals`, `In`, and `NotIn`.
- Handoff child naming is deterministic and at most 63 characters.
- Handoff template vars include `.Upstream`, `.Lineage`, `.Handoff`, and
  `.Loop`.
- Loop attempt calculation covers start, increment, carry-forward, and
  max-attempt blocking.
- Loop subject binding hashes and compares stable subject values.

### Controller tests

- Parent `Succeeded` plus matching `handoff.target` creates a child.
- Parent `Running` creates no child.
- Parent `Failed` creates no child unless configured.
- A repeated reconcile does not create a duplicate child.
- Existing child with matching lineage labels is treated as already created.
- Slack annotations are copied when `inherit.slackThread=true`.
- Custom listed labels and annotations are copied.
- Rendered child metadata overrides non-reserved inherited metadata.
- Reserved lineage keys cannot be overridden.
- Child has `spec.dependsOn` containing the parent by default.
- `dependsOnParent=false` skips implicit dependency.
- `suspend=true` blocks child creation.
- `maxConcurrency` blocks child creation.
- `maxTotalTasks` blocks child creation.
- lineage depth greater than 8 blocks child creation.
- PR babysitter reviewer-to-dev-fix increments loop attempt.
- PR babysitter dev-fix-to-review keeps the same loop attempt.
- PR babysitter max attempts blocks the next dev-fix child.
- PR babysitter loop metadata is copied through reviewer and dev-fix children.
- PR babysitter subject mismatch blocks child creation.

### Manual Slack tests after GitOps rollout

Use low-risk test requests in a non-prod Slack channel.

Ticket only:

```text
@cody !ticket create a Jira ticket for a test-only docs cleanup request. Do not implement it.
```

Expected:

- ticket persona runs
- Jira ticket is created or updated
- no dev handoff occurs

Ticket to dev:

```text
@cody !ticket create a small test-only docs cleanup ticket and implement it if the scope is clear.
```

Expected:

- ticket persona runs
- dev child Task appears in the same Slack thread if ticket persona emits
  `handoff.target=dev`
- no duplicate dev child appears

Dev only:

```text
@cody !dev make a no-op docs-only test change in the Kelos fork, open a PR, and keep it clearly marked as test-only.
```

Expected:

- dev persona runs
- PR is opened
- reviewer child Task appears only after `pr` and `handoff.target=pr-reviewer`
  exist

Review only:

```text
@cody !review https://github.com/donchev7/kelos/pull/<test-pr>
```

Expected:

- reviewer persona runs independently
- no automatic dev handoff occurs

Babysit PR:

```text
@cody !babysit https://github.com/donchev7/kelos/pull/<test-pr>
```

Expected:

- babysitter persona normalizes the PR and starts review
- reviewer child appears in the same Slack thread
- if reviewer emits `review.result=changes-requested` and
  `handoff.target=dev-fix`, dev-fix child appears in the same thread
- dev-fix pushes only focused fixes to the existing PR branch
- re-review child appears after `fix.pushed=true`
- loop stops when clean, blocked, or max attempts are reached
- no more than the configured max number of dev-fix attempts occurs

Negative routing:

```text
@cody debug why the word dev appears in this sentence
```

Expected:

- stable debugger route handles the request
- no dev persona route runs

Unmentioned prefix:

```text
!dev do not run this
```

Expected:

- no Cody Task is created

Unmentioned babysitter prefix:

```text
!babysit https://github.com/donchev7/kelos/pull/<test-pr>
```

Expected:

- no Cody Task is created

## Rollback

Kelos code rollback:

- Revert the controller image to a version before `spec.handoffs` support.
- Existing TaskSpawners without `handoffs` are unaffected.
- TaskSpawners with `handoffs` require the newer CRD. Remove `handoffs` before
  rolling CRDs back.

GitOps rollback:

- Remove `handoffs` entries from Cody TaskSpawners.
- Remove `cody-pr-babysitter-slack` if the babysitter route itself is causing
  issues.
- Keep Phase 1 persona routes intact.
- Flux should converge without deleting existing completed Tasks until their
  TTL expires.

Operational kill switches:

- Set `spec.suspend: true` on a Cody TaskSpawner to stop both source-event and
  handoff child creation for that route.
- Lower `maxConcurrency` or `maxTotalTasks` to contain a problematic route.
- Remove or change the relevant `handoffs` entry to stop only that handoff.

## Open Questions

1. Should `handoff.prompt.base64` be decoded by Kelos before rendering, or
   should AgentConfigs avoid base64 handoff prompts until a later release?
   Recommendation: keep Phase 2 simple and do not decode automatically.
2. Should Slack reporter include persona and handoff labels in status messages?
   Recommendation: defer. Thread continuity is enough for Phase 2.
3. Should failed parent Tasks support handoff to a remediation persona?
   Recommendation: keep the API capable of `Failed`, but do not configure Cody
   failed-task handoffs in the first GitOps rollout.

## Acceptance Criteria

Phase 2 is complete when:

- Kelos supports optional `TaskSpawner.spec.handoffs`.
- Kelos supports optional bounded loop metadata for handoff chains.
- Agents can emit safe custom result keys through `kelos-output`.
- Capture publishes those keys into `Task.status.results`.
- Matching handoff rules create exactly one child Task.
- Loop handoffs enforce max attempts and cannot run forever.
- Child Tasks report in the same Slack thread when configured.
- Cody GitOps can configure ticket-to-dev and dev-to-review handoffs without
  adding GitHub triggers or a router persona.
- Cody GitOps can configure `@cody !babysit` to run a bounded review/dev-fix
  loop on one PR.
- Existing Cody behavior remains unchanged for routes without handoffs.
