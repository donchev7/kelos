# Cody Aikido TaskSpawner Source Implementation Spec

Status: Draft
Date: 2026-05-27
Owner: Cody / Kelos

## Summary

Add a first-class scheduled Aikido source to `TaskSpawner`:

```yaml
spec:
  when:
    aikido:
      schedule: "0 6 * * *"
      repositories:
        - order-service
      statuses:
        - open
      severities:
        - critical
        - high
```

This keeps the implementation aligned with the existing Kelos mental model:

- controllers reconcile Kubernetes runtime resources;
- TaskSpawners define source triggers, filters, and task templates;
- source adapters own external API mechanics;
- AgentConfigs define Cody behavior after a Task starts.

The Aikido integration should not be a hard-coded Aikido controller for one
service. It should be a reusable source type where each TaskSpawner instance
can express its own schedule, repository filters, severity filters, and task
prompt.

## API Facts Checked

The Aikido public API supports the required read path:

- `GET /repositories/code`
  - Lists active code repositories.
  - Supports `page`, `per_page`, `include_inactive`, `filter_name`, and
    `filter_branch`.
  - `per_page` supports `10` to `200`.
- `GET /open-issue-groups`
  - Lists issue groups as shown in Aikido's feed, sorted descending by
    priority.
  - Supports `page`, `per_page`, `filter_code_repo_id`,
    `filter_external_code_repo_id`, `filter_code_repo_name`,
    `filter_container_repo_id`, `filter_team_id`, `filter_issue_type`, and
    `filter_status`.
  - `per_page` supports `10` to `20`.
  - `filter_status` values are `open`, `closed`, `snoozed`, and `ignored`.
  - `filter_issue_type` values include `open_source`, `leaked_secret`,
    `cloud`, `sast`, `iac`, `docker_container`, `cloud_instance`,
    `surface_monitoring`, `malware`, `eol`, `mobile`, `scm_security`,
    `ai_pentest`, and `license`.
  - Response objects include stable issue group IDs and severity fields.
- `GET /issues/groups/{issue_group_id}`
  - Fetches detail for one issue group.
- `GET /issues/{issue_id}`
  - Fetches detail for one issue.
- `GET /issues/export`
  - Broader issue export endpoint.
  - Supports `filter_status`, `filter_code_repo_name`, `filter_issue_type`,
    and `filter_severities`.
  - This is useful later for reporting-style workflows, but should not be the
    first trigger source because it can return a much larger result set.

This spec intentionally does not expose Aikido issue type as a v1 TaskSpawner
filter. The source should still pass the issue type into the spawned task as
context so the agent can classify the finding and choose the right remediation
path.

Kelos already has a read-only Aikido proxy in `cmd/cody-tools`:

```text
http://cody-tools.kelos-system.svc.cluster.local:8080/aikido
```

That proxy injects Aikido auth server-side, supports only `GET`, forwards to
`https://app.aikido.dev/api/public/v1`, and keeps Aikido credentials out of
Cody task pods. The new TaskSpawner source should reuse this proxy rather than
introducing Aikido credentials into spawner pods or task pods.

References:

- <https://apidocs.aikido.dev/reference/listcoderepos>
- <https://apidocs.aikido.dev/reference/listopenissuegroups>
- <https://apidocs.aikido.dev/reference/getissuegroupdetails>
- <https://apidocs.aikido.dev/reference/getissuedetail>
- <https://apidocs.aikido.dev/reference/exportissues>
- `specs/2026-05-21-21-08-cody-aikido-api-proxy-phase-1.md`

## Goals

- Trigger Cody tasks from scheduled Aikido issue group discovery.
- Allow a TaskSpawner to limit discovery to one service/repository.
- Keep Aikido filtering deterministic and outside prompts.
- Keep Aikido credentials only in `cody-tools`.
- Create one Kelos `Task` per matching Aikido issue group.
- For fixable findings, create or update one Jira ticket for the Aikido issue
  group and open PRs in every repo needed to remediate the finding.
- Preserve normal TaskSpawner controls such as `suspend`, `maxConcurrency`,
  `maxTotalTasks`, TTL behavior, and prompt templating.
- Make no Slack-specific assumptions. Aikido-triggered tasks should work even
  if no Slack interface exists.

## Non-Goals

- Do not add Aikido write operations.
- Do not add an Aikido API key, OAuth client secret, or bearer token to Cody
  task pods.
- Do not replace the existing `cody-tools` Aikido proxy.
- Do not implement Aikido webhooks in this pass.
- Do not implement container-image or VM-wide source filters in the first pass.
  The first source targets code-repository findings. Shared image findings can
  be added later with `containerRepositories` or an export-backed source mode.
- Do not add a generic workflow engine as part of this change.

## Component Model

### TaskSpawner Controller

The controller should recognize `spec.when.aikido.schedule` as a scheduled
source and reconcile a Kubernetes `CronJob`, just like `spec.when.cron`.

The controller should not call Aikido. It should only:

- classify the TaskSpawner as scheduled;
- build/update/delete the spawner CronJob;
- set the TaskSpawner status fields it already owns;
- preserve stale Deployment/CronJob cleanup behavior when the source type
  changes.

### TaskSpawner Spec

The TaskSpawner owns the schedule and filters:

- `schedule`: when to run discovery;
- `repositories`: optional exact Aikido code repository names;
- `statuses`: optional Aikido status filters, defaulting to `open`;
- `severities`: optional severity filters;
- `taskTemplate`: prompt, image, credentials, workspace, AgentConfig refs, and
  task metadata.

This is the right place to limit a cron to a single service:

```yaml
repositories:
  - asset-service
```

For platform-owned manifests, this list lives in the TaskSpawner YAML in
GitOps. For service-owned Cody manifests later, the same field can live in the
service repo's `cody/` TaskSpawner YAML and be reconciled by the platform-owned
Flux registration. Values should match Aikido code repository names exactly.

### Aikido Source Adapter

The source adapter should live under `internal/source` and own:

- Aikido proxy URL construction;
- repository validation;
- API query construction;
- pagination;
- response decoding;
- severity filtering when Aikido does not expose a server-side filter on the
  chosen endpoint;
- duplicate issue-group collapse across filter combinations;
- conversion from Aikido issue groups to `source.WorkItem`.

### cody-tools

`cody-tools` remains the only component with Aikido credentials. The source
adapter talks to:

```text
GET http://cody-tools.kelos-system.svc.cluster.local:8080/aikido/...
```

No new Aikido secret should be mounted into the spawner CronJob or the spawned
Cody task.

`cody-tools` should also own the GitHub App private key and package registry
token. Spawned Codex task pods should receive only the broker base URL:

```text
CODY_TOOLS_GITHUB_BASE_URL=http://cody-tools.kelos-system.svc.cluster.local:8080/github
```

The Codex image's Git, `gh`, `npm`, and `pnpm` helper scripts use that URL to
request short-lived tokens from `cody-tools`. Do not mount `GITHUB_APP_*`
secrets into the spawned Cody task pod.

### AgentConfig

AgentConfig owns the security triage-and-fix behavior, not the TaskSpawner
source. A security AgentConfig can instruct Cody to:

- inspect the Aikido finding;
- inspect the repository;
- decide whether a code fix is appropriate;
- create or update one Jira ticket for the Aikido issue group;
- open PRs in every affected repo needed to remediate the finding;
- treat leaked secrets as incident/security rotation work rather than silently
  patching and closing;
- produce an RCA or exception recommendation when no safe automated fix exists.

## Proposed API

Add `Aikido` to `When`:

```go
type When struct {
    // existing fields...

    // Aikido discovers security issue groups from Aikido on a schedule.
    // +optional
    Aikido *Aikido `json:"aikido,omitempty"`
}
```

Add a minimal Aikido source config:

```go
type Aikido struct {
    // Schedule is a cron expression for Aikido discovery.
    // +kubebuilder:validation:Required
    Schedule string `json:"schedule"`

    // Repositories filters by exact Aikido code repository name. When empty,
    // discovery is account-wide for code-repository issue groups.
    // +optional
    // +kubebuilder:validation:MaxItems=25
    Repositories []string `json:"repositories,omitempty"`

    // Statuses filters by Aikido issue group status. Defaults to ["open"].
    // +optional
    // +kubebuilder:validation:Items:Enum=open;closed;snoozed;ignored
    // +kubebuilder:validation:MaxItems=4
    Statuses []string `json:"statuses,omitempty"`

    // Severities filters by Aikido severity. When empty, all severities match.
    // +optional
    // +kubebuilder:validation:Items:Enum=critical;high;medium;low
    // +kubebuilder:validation:MaxItems=4
    Severities []string `json:"severities,omitempty"`
}
```

Do not add broad "exactly one source" CEL validation in this PR unless it
already exists in generated CRDs. That would tighten an older contract and is
not needed for this source.

Do not require `taskTemplate.workspaceRef` for Aikido at the CRD level. Some
Aikido workflows are reporting-only and do not need a repository checkout. A
TaskSpawner that should produce code changes can still set `workspaceRef`.

## Discovery Behavior

On each scheduled run:

1. Load the TaskSpawner.
2. Build an `AikidoSource` from `spec.when.aikido`.
3. If `repositories` is configured, validate each repository name against
   `/repositories/code?filter_name=<name>&per_page=20`.
   - Treat exact name matches as valid.
   - If a configured repository has no exact active match, fail discovery with
     an explicit error instead of returning zero tasks.
4. Query `/open-issue-groups` for each repository/status combination.
   - Omit a query parameter when the corresponding filter list is empty.
   - Default `statuses` to `open`.
   - Use `per_page=20`.
   - Iterate pages starting from `page=0`.
   - Cap pages with an internal constant, initially `10` pages per query.
5. Apply severity filtering client-side, because `/open-issue-groups` does not
   document a severity query parameter.
6. Deduplicate by Aikido issue group ID.
7. Convert each issue group into one `WorkItem`.
8. Let existing TaskSpawner dedupe and limits decide which Tasks are created.

If a page returns `20` items at the page cap, return an error that asks the
operator to narrow filters. Silent partial discovery would be worse than a
failed scheduled run.

If no issue groups match, discovery succeeds and creates no tasks.

## WorkItem Mapping

Use stable Kubernetes-name-safe IDs so an open Aikido issue group does not
spawn duplicate active tasks every schedule tick:

```text
WorkItem.ID = "aikido-group-<safe_issue_group_id>"
```

If the Aikido issue group ID is long, shorten it with a deterministic hash
suffix. Keep the original Aikido issue group ID in metadata.

Map fields as:

```text
Kind     = "AikidoIssueGroup"
Number   = issue_group_id
Title    = issue_group.title
URL      = issue_group.url when Aikido returns one, otherwise empty
Labels   = ["aikido", "severity:<severity>", "status:<status>", "type:<type>", "repo:<repo>"]
Body     = concise markdown summary built by the source adapter
Metadata = required machine-readable Aikido metadata map
```

The body should include:

- issue group ID;
- title and description;
- severity and severity score;
- status;
- issue type;
- code repository names from locations;
- package/CVE/file/line/fix hints when present and safe;
- a note telling Cody to use the internal Aikido proxy for deeper read-only
  context if needed.

The body should not include raw unbounded JSON by default. For leaked-secret
findings, redact possible secret values and include only identifiers,
locations, commit/file/line metadata, and Aikido IDs.

The source must populate `WorkItem.Metadata` for every Aikido work item. Use
annotation-style keys so the spawner can copy them directly onto spawned Tasks:

```text
aikido.kelos.dev/issue-group-id = <issue_group_id>
aikido.kelos.dev/severity       = <severity or unknown>
aikido.kelos.dev/status         = <status or unknown>
aikido.kelos.dev/issue-type     = <issue_type or unknown>
aikido.kelos.dev/repositories   = comma-separated repository names
aikido.kelos.dev/url            = Aikido issue group URL when known
```

The metadata map is non-optional for Aikido. If an Aikido response lacks a
non-critical value, populate the key with `unknown` or an empty string rather
than omitting the key. Missing issue group ID is fatal because the source cannot
build a stable dedupe key without it.

Expose metadata to prompt, branch, and task metadata templates as
`{{ index .Metadata "aikido.kelos.dev/issue-group-id" }}`. Existing variables
such as `{{.ID}}`, `{{.Number}}`, `{{.Kind}}`, `{{.Title}}`, `{{.Body}}`,
`{{.URL}}`, and `{{.Labels}}` remain available.

## Example TaskSpawner

```yaml
apiVersion: kelos.dev/v1alpha1
kind: TaskSpawner
metadata:
  name: cody-aikido-order-service
  namespace: kelos-system
spec:
  when:
    aikido:
      schedule: "0 6 * * *"
      repositories:
        - order-service
      statuses:
        - open
      severities:
        - critical
        - high
  maxConcurrency: 2
  taskTemplate:
    type: codex
    credentials:
      type: oauth
      secretRef:
        name: cody-codex-credentials
    image: docker.io/alpheya/codex:main
    agentConfigRefs:
      - name: cody-base
      - name: cody-dev
      - name: cody-atlassian-mcp
    metadata:
      labels:
        cody.alpheya.com/persona: security-fixer
        cody.alpheya.com/source: aikido
        cody.alpheya.com/aikido-severity: '{{ index .Metadata "aikido.kelos.dev/severity" }}'
        cody.alpheya.com/aikido-status: '{{ index .Metadata "aikido.kelos.dev/status" }}'
      annotations:
        aikido.kelos.dev/issue-group-id: '{{ index .Metadata "aikido.kelos.dev/issue-group-id" }}'
        aikido.kelos.dev/severity: '{{ index .Metadata "aikido.kelos.dev/severity" }}'
        aikido.kelos.dev/status: '{{ index .Metadata "aikido.kelos.dev/status" }}'
        aikido.kelos.dev/issue-type: '{{ index .Metadata "aikido.kelos.dev/issue-type" }}'
        aikido.kelos.dev/repositories: '{{ index .Metadata "aikido.kelos.dev/repositories" }}'
        aikido.kelos.dev/url: '{{ index .Metadata "aikido.kelos.dev/url" }}'
    ttlSecondsAfterFinished: 3600
    podOverrides:
      labels:
        cody.alpheya.com/tools-client: "true"
      serviceAccountName: cody-debugger
      env:
        - name: CODY_TOOLS_GITHUB_BASE_URL
          value: http://cody-tools.kelos-system.svc.cluster.local:8080/github
        - name: KUBERNETES_CLUSTER_NAME
          value: non-prod
        - name: ALPHEYA_TOKEN_SIGNING_KEY
          valueFrom:
            secretKeyRef:
              name: cody-jwt-signing
              key: key.pem
        - name: ALPHEYA_TOKEN_SIGNING_KEY_ID
          value: ca1858fd-4624-4524-be5b-8c4f265ada2a
        - name: ALPHEYA_TOKEN_SIGNING_ISSUER
          value: https://auth.qwlth.dev
        - name: ALPHEYA_TOKEN_SIGNING_AUDIENCE
          value: alpheya
        - name: ALPHEYA_TOKEN_SIGNING_DEFAULT_CLAIMS
          value: |-
            {
              "sub": "6ab6d10e-5f43-4e74-9dca-e99e7c7c73dd",
              "roles": [
                "all_access:int",
                "all_access:int2",
                "all_access:dq-dev",
                "all_access:performance",
                "head_of_wealth:integration-testing",
                "tenant_group_admin",
                "iam_admin"
              ],
              "email": "cody@alpheya.com",
              "name": "Cody Developer",
              "ext": {
                "sub": "3abc9f82-ca4b-49ad-b3d2-3fe9723ed2e5",
                "preferred_username": "cody@alpheya.com"
              }
            }
    promptTemplate: |
      Triage and fix this Aikido security finding.

      {{.Kind}} #{{.Number}}: {{.Title}}

      {{.Body}}

      Requirements:
      - create or update exactly one Jira ticket for this Aikido issue group
        before opening code PRs;
      - confirm whether the finding applies and classify it as dependency,
        code, leaked secret, container/base-image, or configuration;
      - for leaked secrets, do not silently patch; document
        rotation/escalation steps in the Jira ticket and stop before code PRs
        unless a safe code/config cleanup is also needed;
      - for safe dependency, base-image, code, or configuration fixes,
        implement the remediation and open merge-ready PRs in every repo
        needed to resolve the finding;
      - link every PR to the single Jira ticket and include Aikido issue group
        evidence;
      - if no safe fix is possible, update the Jira ticket with RCA-quality
        evidence and the human blocker.
```

## Code Changes

### API Types

Update:

- `api/v1alpha1/taskspawner_types.go`

Add:

- `When.Aikido`
- `type Aikido struct`

Run generated updates with the repo's existing target:

```text
make update
```

Expected generated updates include:

- `api/v1alpha1/zz_generated.deepcopy.go`
- CRDs under `internal/manifests/`
- Helm chart CRD templates if generated by `make update`

### Controller

Update:

- `internal/controller/taskspawner_controller.go`
- `internal/controller/taskspawner_deployment_builder.go`

Add helpers:

```go
func isScheduledSource(ts *kelosv1alpha1.TaskSpawner) bool {
    return ts.Spec.When.Cron != nil ||
        (ts.Spec.When.Aikido != nil && ts.Spec.When.Aikido.Schedule != "")
}

func taskSpawnerSchedule(ts *kelosv1alpha1.TaskSpawner) string {
    switch {
    case ts.Spec.When.Cron != nil:
        return ts.Spec.When.Cron.Schedule
    case ts.Spec.When.Aikido != nil:
        return ts.Spec.When.Aikido.Schedule
    default:
        return ""
    }
}
```

Use the schedule helper anywhere the CronJob currently reads
`ts.Spec.When.Cron.Schedule`.

Keep the existing resource model:

- scheduled sources get a CronJob;
- polling sources get a Deployment;
- webhook/socket sources get no per-spawner workload.

### Spawner Runtime

Update:

- `cmd/kelos-spawner/main.go`
- `cmd/kelos-spawner/reconciler.go`

Add runtime config:

```go
type spawnerRuntimeConfig struct {
    // existing fields...
    AikidoProxyURL string
}
```

Use a non-secret default:

```text
http://cody-tools.kelos-system.svc.cluster.local:8080/aikido
```

Allow override through a non-secret env var or flag, for local tests and
non-standard namespaces:

```text
KELOS_AIKIDO_PROXY_URL
--aikido-proxy-url
```

This value is not sensitive, so it is safe as a flag default. Do not read
Aikido credentials in `kelos-spawner`.

Because the scheduled spawner job calls `cody-tools`, the reconciled Aikido
spawner CronJob pod should use the same tools-client network label used by Cody
task pods:

```yaml
cody.alpheya.com/tools-client: "true"
```

In `buildSourceWithProxy`, add:

```go
if ts.Spec.When.Aikido != nil {
    return &source.AikidoSource{
        ProxyBaseURL: aikidoProxyURL,
        Repositories: ts.Spec.When.Aikido.Repositories,
        Statuses: ts.Spec.When.Aikido.Statuses,
        Severities: ts.Spec.When.Aikido.Severities,
        Client: httpClient,
    }, nil
}
```

`resolvedPollInterval` does not need Aikido support if Aikido is scheduled
through a CronJob.

### Source Adapter

Add:

- `internal/source/aikido.go`
- `internal/source/aikido_test.go`

Suggested internal constants:

```go
const (
    defaultAikidoProxyURL = "http://cody-tools.kelos-system.svc.cluster.local:8080/aikido"
    aikidoOpenIssueGroupsPerPage = 20
    aikidoMaxOpenIssueGroupPages = 10
    aikidoMaxBodyBytes = 64 * 1024
)
```

The source adapter should:

- use `http.Client` from runtime config, defaulting to `http.DefaultClient`;
- set `Accept: application/json`;
- never set Aikido auth headers;
- build paths relative to the proxy base URL;
- reject invalid proxy base URLs at source construction or first discovery;
- read error bodies with a small limit;
- return explicit errors for non-2xx responses;
- deduplicate by issue group ID;
- populate the required `WorkItem.Metadata` map for every emitted item;
- produce deterministic ordering based on Aikido response order after
  deduplication.

### Required Source Metadata

Add a `Metadata map[string]string` field to `source.WorkItem` in v1. This is a
generic field, but Aikido is the first required producer.

Update:

- `internal/source/source.go`
- `internal/source/prompt.go`
- `internal/taskbuilder/builder.go` or the spawner task creation path

Required behavior:

- `source.WorkItemToTemplateVars` exposes `Metadata`.
- `source.RenderTemplate` exposes `Metadata` for polling/scheduled sources.
- The spawner copies every `WorkItem.Metadata` entry to the spawned Task's
  annotations after TaskTemplate metadata rendering.
- Source metadata annotations win if a TaskTemplate tries to set the same
  annotation key. This keeps source identity stable and avoids a manifest
  accidentally corrupting dedupe/debug metadata.
- Aikido source tests must assert that emitted work items include the required
  metadata keys.

TaskTemplate metadata remains useful for extra labels and annotations, but it
is not the source of truth for required Aikido metadata:

```yaml
taskTemplate:
  metadata:
    labels:
      cody.alpheya.com/aikido-severity: '{{ index .Metadata "aikido.kelos.dev/severity" }}'
```

## Error Behavior

The implementation should fail loudly for integration errors and create zero
tasks only when the query succeeds with zero findings.

Handled paths:

- Aikido credentials missing in `cody-tools`: proxy returns an error; source
  returns an error; CronJob fails.
- Aikido API returns non-2xx: source returns an error with status and limited
  body.
- Proxy URL malformed: source returns a config error.
- Configured repository typo: source returns an explicit repository-not-found
  error after `/repositories/code` validation.
- Aikido response lacks issue group ID: source returns an explicit mapping
  error because stable task identity and required metadata cannot be built.
- Page cap reached with a full page: source returns an explicit "narrow
  filters" error.
- No matching issue groups: discovery succeeds with no tasks.
- Duplicate issue group returned through multiple filter combinations:
  one WorkItem is emitted.
- Existing Task already exists for an open issue group: current TaskSpawner
  dedupe prevents duplicate active tasks.
- Existing completed Task exists and the WorkItem has no newer trigger time:
  no retrigger occurs. This is acceptable for v1 because Aikido issue group
  tasks are stable remediation units.
- TaskSpawner suspended: existing suspend behavior prevents discovery.
- `maxConcurrency` reached: existing TaskSpawner behavior skips remaining
  new items for that cycle.
- `maxTotalTasks` reached: existing TaskSpawner behavior stops creating new
  tasks.

Unimplemented in v1:

- automatic retrigger when an Aikido issue group changes after a completed
  task;
- source-level "daily summary even when no findings";
- container repository filtering for shared base images;
- Aikido webhook-triggered tasks;
- Aikido writeback such as snooze, ignore, or task creation in Aikido.

## Testing Plan

Unit tests:

- `AikidoSource` builds `/open-issue-groups` requests with repository and
  status query params.
- `AikidoSource` validates configured repositories with `/repositories/code`.
- repository validation fails on no exact match.
- status defaults to `open`.
- severity filtering is applied client-side.
- duplicate issue groups collapse to one WorkItem.
- pagination continues while pages are full and stops when a short page is
  returned.
- page cap returns an explicit error when the capped page is full.
- non-2xx proxy responses return explicit errors with limited body text.
- WorkItem mapping produces stable `aikido-group-<id>` IDs and safe labels.
- WorkItem mapping always produces required `WorkItem.Metadata` keys.
- `WorkItem.Metadata` is exposed to prompt, branch, and metadata templates.
- spawned Tasks receive source metadata annotations even when TaskTemplate
  metadata does not repeat those annotations.
- leaked-secret-like payloads do not leak raw secret values into `Body`.

Controller/spawner tests:

- Aikido TaskSpawners are classified as scheduled and produce CronJobs.
- CronJob schedule is read from `spec.when.aikido.schedule`.
- switching from Aikido to polling source deletes stale CronJob.
- switching from polling source to Aikido deletes stale Deployment.
- `buildSourceWithProxy` returns `source.AikidoSource` for `when.aikido`.
- Aikido source does not require GitHub workspace credentials.
- generated CRD contains `spec.when.aikido`.

Manual smoke test after deployment:

```text
kubectl -n kelos-system get taskspawner cody-aikido-order-service -o yaml
kubectl -n kelos-system get cronjob cody-aikido-order-service
kubectl -n kelos-system create job --from=cronjob/cody-aikido-order-service aikido-order-service-manual-$(date +%s)
kubectl -n kelos-system logs job/<manual-job-name>
kubectl -n kelos-system get tasks -l kelos.dev/taskspawner=cody-aikido-order-service
```

## Rollout Plan

1. Implement Kelos API/source/controller/spawner changes.
2. Run `make update`.
3. Run focused unit tests for source, spawner, and controller.
4. Add or update a non-prod TaskSpawner manifest in GitOps only after the CRD
   and controller image are deployed.
5. Deploy the new Kelos controller/spawner image.
6. Apply the updated CRD.
7. Apply the Aikido TaskSpawner manifest.
8. Trigger one manual CronJob job for validation.
9. Confirm no Aikido credentials appear in spawner or Cody task pod env.

## Follow-Ups

- Add `containerRepositories` and `containerRepositoryIDs` filters for shared
  image workflows.
- Add an export-backed reporting mode for daily summaries and "no findings"
  confirmations.
- Add Aikido webhook support if Aikido webhooks can reliably deliver issue
  group events.
- Add source-level change detection so resolved or materially changed issue
  groups can retrigger completed tasks.
- Add stricter Jira duplicate matching, ownership, and transition policy once
  the triage-and-fix behavior is proven.
