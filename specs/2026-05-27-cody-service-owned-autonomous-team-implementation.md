# Notification-Service Daily Aikido Cody Job Implementation Spec

Status: Draft
Date: 2026-05-27
Owner: Cody / Kelos / Platform

## Summary

Implement one daily Cody run for `notification-service` that retrieves Aikido
context through Kelos `contextSources`, filters findings inside the Cody run,
creates or updates one ALPM Jira ticket per matching finding, and opens scoped
fix PRs where safe.

This version intentionally avoids a first-class Aikido TaskSpawner source. It
uses existing Kelos primitives:

- one service: `notification-service`;
- one trigger: scheduled Kelos `TaskSpawner` using `when.cron`;
- one Kelos Task per repo per day;
- Aikido retrieval through `taskTemplate.contextSources`;
- per-finding durable state in Jira tickets and GitHub PRs;
- no Slack self-handoff;
- no new workflow engine.

The tradeoff is explicit: Kelos will not create one Task per Aikido finding.
The daily Cody run owns pagination, critical/high filtering, per-finding Jira
dedupe, and safe fix PR creation.

## Goals

- Run one daily Aikido triage-and-fix Cody task for `notification-service`.
- Retrieve initial Aikido issue-group context through:

```text
http://cody-tools.kelos-system.svc.cluster.local:8080/aikido
```

- Filter inside Cody to open critical/high findings for the Aikido code
  repository named `notification-service`.
- Create or update one ALPM Jira ticket per Aikido issue group.
- Avoid Jira duplicates across repeated daily runs.
- Classify each finding as dependency, code vulnerability, leaked secret,
  container/base image, configuration/IaC, or unknown/manual triage.
- For safe dependency, base-image, code, or configuration fixes, open scoped PRs
  linked to the Jira ticket.
- Treat leaked secrets as rotation/escalation items. Do not silently patch,
  close, or mark resolved.
- Keep Aikido credentials only in `cody-tools`.

## Non-Goals

- Do not onboard `order-service`.
- Do not add broader autonomous team roles.
- Do not add PR review, architecture drift, standards review, CI doctor,
  release verifier, docs drift, or security verifier workflows.
- Do not add Slack triggers.
- Do not add Aikido write operations.
- Do not use `when.aikido` or any other Aikido-specific Kelos source for this
  notification-service workflow. A separate `when.aikido` source can coexist in
  Kelos and may be used by other service workflows.
- Do not mount Aikido credentials into spawner pods or Cody task pods.

## Current Facts

- Kelos already supports scheduled TaskSpawners through `when.cron`.
- Kelos already supports `taskTemplate.contextSources`, including HTTP GET
  requests whose responses are exposed as `.Context.<name>` in the prompt.
- `contextSources` run in the `kelos-spawner` pod before the Cody Task is
  created. Because the Aikido context source calls `cody-tools`, that spawner
  pod must be allowed through the `cody-tools` NetworkPolicy.
- `taskTemplate.podOverrides.labels` applies to spawned Cody task pods, not to
  the spawner CronJob pod. The Cody task pod still needs the
  `cody.alpheya.com/tools-client: "true"` label so it can fetch additional
  pages and issue details from `cody-tools`.
- `cody-tools` already exposes a read-only Aikido proxy at `/aikido`.
- `cody-tools` injects Aikido auth server-side and rejects non-GET requests.
- `k8s-platform-gitops/non-prod/kelos` already contains:
  - `deployment-cody-tools.yaml`;
  - `service-cody-tools.yaml`;
  - `networkpolicy-cody-tools.yaml`;
  - `external-secret-cody-aikido-api.yaml`;
  - `agentconfig-cody-atlassian-mcp.yaml`;
  - `agentconfig-cody-base.yaml`.
- `notification-service` already has service-local docs and package scripts
  that Cody can use when a safe fix requires code changes.

## Architecture

```text
notification-service/cody/
  -> platform-owned Flux GitRepository + Kustomization
  -> restricted Flux ServiceAccount applies TaskSpawner/AgentConfig
  -> Kelos when.cron TaskSpawner runs once per day
  -> kelos-spawner fetches initial Aikido context through contextSources
  -> Cody receives one repo-level daily task with raw Aikido context
  -> Cody filters to open critical/high notification-service findings
  -> Cody creates/updates one ALPM Jira ticket per finding
  -> Cody opens scoped PRs for safe fixes after Jira exists
  -> Jira and GitHub store durable per-finding state
```

Slack is not involved. GitHub is used only when a safe code/config fix is
needed.

## Service-Owned Files

Add the following files to `notification-service`:

```text
notification-service/
  cody/
    kustomization.yaml
    service-context.yaml
    taskspawner-daily-aikido.yaml
```

### `cody/kustomization.yaml`

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - service-context.yaml
  - taskspawner-daily-aikido.yaml
```

### `cody/service-context.yaml`

```yaml
apiVersion: kelos.dev/v1alpha1
kind: AgentConfig
metadata:
  name: cody-notification-service-context
  namespace: kelos-system
  labels:
    cody.alpheya.com/service-owned: "true"
    cody.alpheya.com/service: notification-service
    cody.alpheya.com/repo: quantum-wealth/notification-service
spec:
  agentsMD: |
    ## notification-service context

    Service: notification-service
    Repository: quantum-wealth/notification-service
    Aikido code repository name: notification-service
    Jira project: ALPM

    notification-service is a NestJS service for multi-channel notification
    delivery. It accepts notification requests via ConnectRPC, orchestrates
    delivery through Temporal workflows, and manages user notification
    preferences. In-app inbox is production-ready; email is partially
    implemented.

    Daily Aikido work is allowed to create/update Jira tickets and open scoped
    PRs for safe fixes. Prefer one PR per Jira ticket/finding. Combine fixes
    only when the same dependency or code change necessarily resolves multiple
    Aikido issue groups, and link every affected Jira ticket.

    Useful local validation commands:
    - npm run lint
    - npm run test
    - npm run build
    - npm run test:integration
    - npm run test:e2e
```

### `cody/taskspawner-daily-aikido.yaml`

```yaml
apiVersion: kelos.dev/v1alpha1
kind: TaskSpawner
metadata:
  name: cody-notification-service-daily-aikido
  namespace: kelos-system
  labels:
    cody.alpheya.com/service-owned: "true"
    cody.alpheya.com/service: notification-service
    cody.alpheya.com/repo: quantum-wealth/notification-service
    cody.alpheya.com/workflow: aikido-security-triage-fix
spec:
  maxConcurrency: 1
  when:
    cron:
      schedule: "0 5 * * *"
  taskTemplate:
    type: codex
    credentials:
      type: oauth
      secretRef:
        name: cody-codex-credentials
    image: docker.io/alpheya/codex:main
    agentConfigRefs:
      - name: cody-base
      - name: cody-atlassian-mcp
      - name: cody-notification-service-context
    contextSources:
      - name: aikidoOpenIssueGroupsPage0
        failurePolicy: Fail
        http:
          method: GET
          url: "http://cody-tools.kelos-system.svc.cluster.local:8080/aikido/open-issue-groups?filter_code_repo_name=notification-service&filter_status=open&per_page=20&page=0"
          timeoutSeconds: 30
          maxResponseBytes: 131072
    promptTemplate: |
      Daily Aikido triage-and-fix for notification-service.

      Cron tick:
      - Time: {{.Time}}
      - Schedule: {{.Schedule}}

      Scope:
      - Service: notification-service
      - Repository: quantum-wealth/notification-service
      - Aikido code repository name: notification-service
      - Jira project: ALPM
      - Only open critical/high findings are in scope.
      - This is one repo-level daily run. You must process each matching
        finding inside this task.

      Aikido access:
      - Use only the internal read-only proxy:
        http://cody-tools.kelos-system.svc.cluster.local:8080/aikido
      - Use GET requests only.
      - Do not call https://app.aikido.dev directly.
      - Do not inspect environment variables or files for Aikido credentials.
      - Do not send Authorization headers; cody-tools injects auth.

      Initial Aikido context from Kelos contextSources:
      {{ index .Context "aikidoOpenIssueGroupsPage0" }}

      Retrieval and filtering:
      - Parse the initial context as Aikido open issue-group results.
      - If the first page contains 20 items, fetch additional pages from:
        /open-issue-groups?filter_code_repo_name=notification-service&filter_status=open&per_page=20&page=<n>
        until a page returns fewer than 20 items.
      - Filter to severity `critical` or `high`. Aikido's open issue-groups
        endpoint does not provide a documented severity query parameter, so you
        must verify severity client-side.
      - If severity, issue type, location, or remediation evidence is missing,
        fetch detail through:
        /issues/groups/<issue_group_id>
      - Skip non-critical/high findings and mention the skip count in the final
        response.

      Jira duplicate rule:
      - Search ALPM before creating anything.
      - First search for existing unresolved issues containing the exact
        Aikido issue group ID.
      - Also search for labels:
        cody-security, aikido, service-notification-service.
      - If one matching unresolved issue exists, update it.
      - If no matching unresolved issue exists, create one.
      - If multiple possible matches exist, do not create a duplicate. Update
        the most likely issue or record ambiguity in the final response.
      - Do not open a code PR for a finding until a Jira ticket exists.

      Jira ticket content for each finding:
      - Summary: "[Aikido][<severity>] notification-service: <finding title>"
      - Include:
        - Aikido issue group ID
        - Severity and score when available
        - Issue type
        - Classification
        - Affected package/file/image/resource/location
        - Current Aikido status
        - Why this affects notification-service
        - Recommended owner/action
        - Whether it is a safe patch candidate
        - Exact Aikido proxy paths used as evidence

      Classification:
      - dependency
      - code vulnerability
      - leaked secret
      - container/base image
      - configuration/IaC
      - unknown/manual triage

      Fix behavior:
      - For safe dependency, base-image, code, or configuration fixes, clone
        the required repo, create a branch, implement the smallest safe fix,
        run relevant validation, and open a merge-ready PR.
      - Prefer one PR per Jira ticket/finding.
      - If one code change necessarily resolves multiple Aikido issue groups,
        one PR may link all affected Jira tickets.
      - PR title and branch should include the ALPM key.
      - PR body must include the Aikido issue group ID, Jira ticket, evidence,
        verification, and rollback notes.
      - If validation cannot run, explain the exact blocker in both Jira and
        the final response.

      Leaked secret rule:
      - Treat leaked secrets as security rotation/escalation work.
      - Do not mark them patched or resolved.
      - Include explicit rotation guidance and note that removing code alone is
        not sufficient evidence of remediation.
      - Only open a PR if a safe code/config cleanup is separately required.

      Labels to apply when supported:
      - cody-security
      - aikido
      - service-notification-service
      - severity-critical or severity-high
      - type-dependency, type-code, type-secret, type-container,
        type-config, or type-manual-triage
      - cody-security-patch-candidate when a safe automated fix exists
      - cody-security-secret-rotation-required for leaked secrets

      Final response:
      - Summarize how many Aikido findings were fetched.
      - Summarize how many findings matched critical/high.
      - List Jira tickets created/updated.
      - List PRs opened.
      - List skipped findings and why.
      - State any manual follow-up required.
    metadata:
      labels:
        cody.alpheya.com/workflow: aikido-security-triage-fix
        cody.alpheya.com/service: notification-service
    ttlSecondsAfterFinished: 86400
    podOverrides:
      labels:
        cody.alpheya.com/tools-client: "true"
      serviceAccountName: cody-debugger
      env:
        - name: CODY_TOOLS_GITHUB_BASE_URL
          value: http://cody-tools.kelos-system.svc.cluster.local:8080/github
```

## Platform-Owned GitOps Files

Add a small platform-owned registration under
`k8s-platform-gitops/non-prod/kelos/service-owned-cody/`:

```text
service-owned-cody/
  rbac-cody-service-flux-applier.yaml
  networkpolicy-cody-tools-service-owned-spawners.yaml
  gitrepository-notification-service-cody.yaml
  kustomization-notification-service-cody.yaml
```

Register the directory from:

```text
k8s-platform-gitops/non-prod/kelos/kustomization.yaml
```

### Flux Applier RBAC

Create a Flux applier that can apply only the Kelos resources needed by this
job:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cody-service-config-applier
  namespace: flux-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cody-service-config-applier
  namespace: kelos-system
rules:
  - apiGroups: ["kelos.dev"]
    resources:
      - agentconfigs
      - taskspawners
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: cody-service-config-applier
  namespace: kelos-system
subjects:
  - kind: ServiceAccount
    name: cody-service-config-applier
    namespace: flux-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: cody-service-config-applier
```

No `Workspace` permission is needed for this first version unless we decide to
make repository checkout a Kelos `Workspace` concern later. Cody can clone
repos directly using the existing `cody-tools` GitHub broker.

### cody-tools NetworkPolicy

The daily Task needs two cody-tools access paths:

- the spawned Cody task pod, handled by `podOverrides.labels`:

```yaml
cody.alpheya.com/tools-client: "true"
```

- the spawner CronJob pod, because it fetches `contextSources` before creating
  the Task.

The spawner pod already carries `kelos.dev/taskspawner:
cody-notification-service-daily-aikido`. Add an additive `NetworkPolicy`, or
patch the existing `cody-tools` NetworkPolicy, to admit that exact spawner
label. The first implementation should prefer an additive policy so the
existing Cody task-pod access rule stays untouched:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: cody-tools-service-owned-spawner-access
  namespace: kelos-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: cody-tools
  policyTypes:
    - Ingress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              kelos.dev/taskspawner: cody-notification-service-daily-aikido
      ports:
        - port: 8080
          protocol: TCP
```

If we onboard many service-owned cron context jobs, replace this exact label
rule with a stable platform label for service-owned spawners.

### Flux Registration

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: cody-notification-service
  namespace: flux-system
spec:
  interval: 5m
  timeout: 5m
  url: https://github.com/quantum-wealth/notification-service.git
  ref:
    branch: main
  secretRef:
    name: github-creds
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: cody-notification-service
  namespace: flux-system
spec:
  interval: 5m
  retryInterval: 5m
  timeout: 5m
  prune: true
  wait: true
  path: ./cody
  targetNamespace: kelos-system
  serviceAccountName: cody-service-config-applier
  sourceRef:
    kind: GitRepository
    name: cody-notification-service
    namespace: flux-system
```

## Jira Behavior

The job should use the existing `cody-atlassian-mcp` AgentConfig for Jira.

Issue type:

- Preferred type: `Bug`.
- If ALPM rejects `Bug`, inspect allowed issue types once and use the nearest
  available defect/security/work item type.
- If no suitable issue type is available, fail explicitly and report the Jira
  configuration blocker in the Kelos task result.

Duplicate marker:

Each Jira issue body/comment must include this stable marker:

```text
Aikido issue group ID: <issue_group_id>
Service: notification-service
Repository: quantum-wealth/notification-service
```

This marker is the primary duplicate-prevention key.

## Implementation Steps

### 1. notification-service PR

- Add `cody/kustomization.yaml`.
- Add `cody/service-context.yaml`.
- Add `cody/taskspawner-daily-aikido.yaml`.
- Use `when.cron`, not `when.aikido`.
- Add a `contextSources` HTTP GET for the first Aikido open issue-groups page.
- Keep the repo-specific prompt and AgentConfig in the service repo.

### 2. k8s-platform-gitops PR

- Add `service-owned-cody/rbac-cody-service-flux-applier.yaml`.
- Add `service-owned-cody/networkpolicy-cody-tools-service-owned-spawners.yaml`
  or equivalent patch to allow the exact spawner CronJob pod to reach
  `cody-tools`.
- Add `service-owned-cody/gitrepository-notification-service-cody.yaml`.
- Add `service-owned-cody/kustomization-notification-service-cody.yaml`.
- Include the service-owned Cody directory from
  `non-prod/kelos/kustomization.yaml`.
- Confirm existing `cody-tools` Aikido env and `external-secret-cody-aikido-api`
  are still present.

### 3. Kelos PR

No Kelos API/source change is required for this approach.

Required existing capabilities:

- `when.cron` reconciles to a CronJob.
- `taskTemplate.contextSources` supports HTTP GET.
- context values are available in prompt templates as `.Context.<name>` or
  `{{ index .Context "<name>" }}`.
- task pod labels from `taskTemplate.podOverrides.labels` are applied to the
  spawned Cody task.

## Validation

### Static

- `notification-service/cody` renders with `kustomize build`.
- Rendered service-owned resources are only:
  - `AgentConfig`
  - `TaskSpawner`
- Rendered resources are in `kelos-system`.
- The TaskSpawner uses `when.cron`, not `when.aikido`.
- The TaskSpawner includes one `contextSources` entry named
  `aikidoOpenIssueGroupsPage0`.
- The context source URL calls `cody-tools`, not `https://app.aikido.dev`.
- The spawned Cody task pod has:

```yaml
cody.alpheya.com/tools-client: "true"
```

- The cody-tools NetworkPolicy admits the spawner CronJob pod label:

```yaml
kelos.dev/taskspawner: cody-notification-service-daily-aikido
```

- The Flux Kustomization uses:

```yaml
serviceAccountName: cody-service-config-applier
```

### Runtime

After both PRs merge:

1. Reconcile platform GitOps.
2. Verify Flux source:

```bash
flux get sources git cody-notification-service -n flux-system
```

3. Verify Flux Kustomization:

```bash
flux get kustomizations cody-notification-service -n flux-system
```

4. Verify Kelos resources:

```bash
kubectl get agentconfig cody-notification-service-context -n kelos-system
kubectl get taskspawner cody-notification-service-daily-aikido -n kelos-system
```

5. Trigger the CronJob manually after Kelos creates it:

```bash
kubectl create job -n kelos-system \
  --from=cronjob/cody-notification-service-daily-aikido \
  cody-notification-service-daily-aikido-manual
```

6. Verify one Kelos Task is created for the repo-level daily run:

```bash
kubectl get tasks -n kelos-system \
  -l kelos.dev/taskspawner=cody-notification-service-daily-aikido
```

7. Verify ALPM Jira:

- critical/high Aikido findings create or update one ticket per finding;
- repeated run does not create duplicates;
- leaked secret findings contain rotation guidance;
- out-of-scope severities are skipped;
- safe fix candidates have linked PRs or explicit blockers.

8. Verify Aikido access behavior:

- `kelos-spawner` fetches initial Aikido context through `cody-tools`;
- Cody fetches any additional pages/details through `cody-tools`;
- nothing calls `https://app.aikido.dev` directly;
- no Aikido credentials are mounted into either pod.

## Failure Modes

| Failure | Expected behavior |
| --- | --- |
| `notification-service/cody` missing | Flux Kustomization fails; platform waits for service PR. |
| spawner cannot reach `cody-tools` | `contextSources` fails and no Cody task is created. Fix NetworkPolicy. |
| cody-tools unavailable | `contextSources` fails explicitly; no Jira tickets or PRs are created from stale guesses. |
| Aikido returns no findings | Cody reports zero matching findings and makes no Jira/PR changes. |
| Aikido returns more than 20 findings | Cody fetches additional pages from `cody-tools` until a short page. |
| Aikido severity missing | Cody fetches issue-group detail; if still unknown, skip the finding and report it as manual triage. |
| Jira unavailable | Cody reports Jira blocker and does not open PRs because Jira must exist first. |
| Jira duplicate search ambiguous | Cody does not create a duplicate; it updates the most likely issue or records ambiguity. |
| GitHub unavailable after Jira exists | Cody updates Jira/final response with the PR blocker and does not claim remediation. |
| Validation fails for a PR | Cody leaves PR open only if useful, reports failing checks, and updates Jira with evidence. |
| Secret finding detected | Cody creates/updates Jira with rotation guidance and does not mark resolved. |
| Aikido repo name differs | Cody reports zero findings; platform verifies the configured Aikido repo name through cody-tools. |
| Partial per-finding failure | Cody continues with independent findings where safe and reports the failed finding explicitly. |

## Acceptance Criteria

- Daily scheduled Kelos TaskSpawner exists for `notification-service`.
- The TaskSpawner uses `when.cron` with one run per repo per day.
- The TaskSpawner uses `contextSources` to retrieve initial Aikido context from
  `cody-tools`.
- The spawner CronJob pod and spawned Cody task pod can reach `cody-tools`
  without receiving Aikido credentials.
- No Aikido credentials are mounted into spawner pods or Cody task pods.
- Cody filters to open critical/high findings inside the daily run.
- Cody creates or updates one ALPM Jira ticket per matching Aikido issue group
  without duplicates.
- Safe dependency, base-image, code, or configuration fixes produce scoped PRs
  linked to the corresponding Jira tickets.
- Jira tickets include issue group ID, severity, type, affected location,
  evidence, recommended owner/action, and PR links when applicable.
- Secret findings explicitly require rotation/escalation and are not silently
  patched or marked resolved.
