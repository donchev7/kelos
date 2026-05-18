# Moontide Proposal: Observability -> Feedback Loop Agents in Agent Factory

Date: 2026-03-19  
Status: Draft proposal  
Owner: Moontide platform

## 1. Executive Summary

This proposal defines how Moontide can let users create a new class of proactive agents that:

1. Continuously ingest observability signals (alerts/logs/metrics events).
2. Correlate and prioritize incidents.
3. Run autonomous diagnostic loops in sandboxed runtime.
4. Post progressive updates and final outcomes to Slack/GitHub/UI.
5. Optionally execute bounded remediation actions and verify recovery.

### Recommendation

Use a phased approach anchored on the **existing `background_triggered` runtime** and queue/worker primitives:

1. **V1 (recommended now):** webhook-driven observability ingress (Alertmanager/Grafana/custom), incident correlation, diagnostic run loop, Slack updates, mission control visibility.
2. **V1.5:** GitHub check-run/comment delivery, richer incident timelines, stronger dedupe/suppression controls.
3. **V2:** optional automated remediation with approval policies and closed-loop verification.

This gives fast delivery with low architecture risk, while preserving a path to a dedicated control-loop runtime mode later.

## 2. Why This Matters

Moontide already supports proactive background runs via schedules and webhooks. What is missing is a first-class **observe -> reason -> act -> verify -> report** control loop.

Without this loop, users still need humans to:

1. Aggregate noisy alert streams.
2. Decide whether an alert is real/repeated/correlated.
3. Run diagnostics manually.
4. Publish updates to stakeholders.

The proposed agent type converts those manual steps into repeatable, policy-governed automation.

## 3. Current State in Moontide (Repo Analysis)

### 3.1 Existing strengths (reusable as-is)

1. Background orchestration foundation exists:
   1. In-process schedulers and workers started in bootstrap (`apps/bff/src/main.ts`).
   2. Background run queue, locks, statuses, events, artifacts, delivery (`apps/bff/src/agents/agent-run-core.ts`).
2. Typed Agent Factory config and policy compiler exists:
   1. Tooling/network/permission profiles (`apps/bff/src/agents/agent-config-schema.ts`).
3. Runtime visibility APIs exist:
   1. List runs/events/artifacts, retry/terminate (`apps/bff/src/runtime/runtime-visibility-core.ts`, `packages/proto/proto/moontide/runtime/v1/runtime_visibility.proto`).
4. Webhook inbox + async processing pattern exists:
   1. GitHub inbox claim/lease/retry/dead-letter (`apps/bff/src/webhooks/github-events.route.ts`).
5. Sandbox runtime already supports robust staged execution:
   1. Preflight health checks, stage timing/logging, timeout classification (`apps/bff/src/agents/code-search-runtime.ts`).

### 3.2 Current limitations for observability-control-loop use cases

1. Trigger types are limited to `github_pr_opened`, `schedule`, `slack_on_demand` (`apps/bff/src/agents/agent-definition-core.ts`).
2. Integrations are currently GitHub + Slack only (`packages/proto/proto/moontide/integrations/v1/integration_setup.proto`, `apps/bff/src/integrations/integration-core.ts`).
3. Background run artifacts are currently final-report oriented, not incident-lifecycle oriented.
4. No first-class incident model (correlation group, severity transitions, ack state, suppression windows).
5. No native observability signal normalization layer (alert payload contract abstraction).

## 4. External Research Findings (Design Inputs)

## 4.1 Agentic feedback loop pattern (OpenAI Harness Engineering)

Key pattern:

1. Make logs/metrics/traces directly legible to the agent.
2. Let the agent iterate in a loop: detect -> test -> fix -> re-test -> report.
3. Prefer repository-local artifacts and explicit run loops.

Implication for Moontide:

1. Provide first-class observability signal artifacts as run inputs.
2. Persist loop decisions as events/artifacts so future runs can reason over prior incident history.

Source: https://openai.com/index/harness-engineering/

## 4.2 Control-loop architecture (Kubernetes controllers/operator pattern)

Key pattern:

1. Non-terminating control loops compare desired vs current state.
2. Controllers reconcile continuously and report updated state.

Implication for Moontide:

1. Treat each observability agent as a controller:
   1. desired state = service healthy + SLO constraints + no unresolved critical incidents
   2. current state = incoming alerts + live checks + run outcomes
2. Reconciliation loop should be explicit, idempotent, and auditable.

Sources:

- https://kubernetes.io/docs/concepts/architecture/controller/
- https://kubernetes.io/docs/concepts/extend-kubernetes/operator/

## 4.3 Signal standards and telemetry pipeline (OpenTelemetry)

Key pattern:

1. Standard signals: traces, metrics, logs, baggage.
2. Collector supports receive/process/export model.
3. Agent deployment pattern supports per-service/per-host collection, then export.

Implication for Moontide:

1. Normalize incoming signal payloads into a common envelope, independent of source.
2. Keep signal-source adapters pluggable.
3. Use consistent attribute naming for cross-source correlation (service/env/region/severity/incident key).

Sources:

- https://opentelemetry.io/docs/concepts/signals/
- https://opentelemetry.io/docs/collector/components/processor/
- https://opentelemetry.io/docs/collector/deploy/agent/

## 4.4 Alert routing/noise controls (Prometheus + Alertmanager)

Key pattern:

1. Alert rules support anti-flap controls (`for`, `keep_firing_for`).
2. Alertmanager supports grouping and notification pacing (`group_wait`, `group_interval`, `repeat_interval`).
3. Generic webhook integrations are standard, with stable payload structure (receiver/status/alerts/common labels).

Implication for Moontide:

1. Introduce noise controls in agent config (cooldown, dedupe window, min-confidence period).
2. Prefer webhook ingestion first because enterprises already route alerts there.
3. Preserve source metadata for debugging and replay.

Sources:

- https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/
- https://prometheus.io/docs/alerting/latest/configuration/
- https://prometheus.io/docs/alerting/latest/notifications/

## 4.5 Delivery constraints (Slack + GitHub)

Key pattern:

1. Slack message delivery is rate-limited; `chat.postMessage` has method-specific behavior and channel/workspace limits.
2. Slack long messages can be truncated.
3. GitHub webhook best practices require fast acknowledgment and async queue processing.
4. GitHub installation tokens expire quickly (1 hour).

Implication for Moontide:

1. Must implement progressive update throttling and digesting for Slack.
2. Must keep existing async webhook queue model and idempotency.
3. Must mint installation tokens just-in-time during each diagnostic/remediation cycle.

Sources:

- https://docs.slack.dev/apis/web-api/rate-limits/
- https://api.slack.com/changelog/2018-04-truncating-really-long-messages
- https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks
- https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app

## 5. Product Scope

## 5.1 Must Have (V1)

1. New Agent Factory blueprint: **Observability Feedback Agent**.
2. Signal ingestion via webhook endpoints (Alertmanager/Grafana/custom-compatible payloads).
3. Correlation and dedupe into incident groups.
4. Autonomous diagnostic run in existing sandbox runtime.
5. Progressive + final Slack updates.
6. Mission control visibility for incident-driven runs.
7. Guardrails: budget limits, loop max duration/iterations, policy-bounded tools.

## 5.2 Optional / Nice-to-Have (V1.5+)

1. Native pull connectors (Prometheus query, Loki query, cloud monitor APIs).
2. GitHub check-run updates and PR comments for incident-linked repos.
3. Auto-remediation steps with approval gates.
4. Adaptive suppression based on alert history and confidence scoring.
5. Multi-agent coordination (observer + diagnoser + fixer roles).

## 6. Target Runtime Model

## 6.1 Core loop

The control loop for each observability incident:

1. **Ingest:** receive signal event from webhook.
2. **Normalize:** convert payload to `ObservedSignalEvent` envelope.
3. **Correlate:** attach event to existing open incident or open new incident.
4. **Prioritize:** apply severity + confidence + suppression rules.
5. **Diagnose:** enqueue background run with incident context + repo scope.
6. **Report:** post progress messages at bounded cadence.
7. **Verify:** run post-diagnostic checks; decide resolved/degraded/escalate.
8. **Close or Continue:** update incident state and schedule next reconciliation.

## 6.2 Proposed event/state model

Incident states:

1. `new`
2. `triaging`
3. `investigating`
4. `monitoring`
5. `resolved`
6. `suppressed`
7. `escalated`

Run outcome states (incident-specific):

1. `insight_only`
2. `likely_false_positive`
3. `verified_regression`
4. `candidate_fix_generated`
5. `remediation_applied`
6. `recovery_verified`

## 7. Agent Factory Design

## 7.1 New blueprint

Blueprint name: `observability_feedback`

### Must Have fields

1. `signal_sources[]`
   1. `kind` (`alertmanager_webhook`, `grafana_webhook`, `generic_webhook`)
   2. `source_id`
   3. optional shared secret
2. `correlation`
   1. `group_by_keys[]` (for example: `service`, `env`, `alertname`)
   2. `dedupe_window_seconds`
   3. `cooldown_seconds`
3. `triage_policy`
   1. `severity_threshold`
   2. `min_occurrences`
   3. `auto_ack_enabled`
4. `diagnostics`
   1. `repos[]`
   2. `commands[]`
   3. `checks[]`
   4. `hypothesis_prompts[]`
5. `feedback_delivery`
   1. `slack_channel_target`
   2. `progress_interval_seconds`
   3. `max_progress_updates`
6. `safety`
   1. `max_runtime_minutes`
   2. `max_loop_iterations`
   3. `token_budget_per_incident`
   4. `tool_policy_profile`

### Optional fields

1. `remediation`
   1. `enabled`
   2. `approval_mode` (`never`, `on_high_severity`, `always`)
   3. `allowed_actions[]` (for example: rollback command, feature flag flip)
2. `github_feedback`
   1. `check_run_enabled`
   2. `pr_comment_enabled`
3. `quiet_hours`
4. `change_only_mode`

## 7.2 UX behavior in Agent Factory

### Must Have

1. Wizard path for observability blueprint with typed forms.
2. Validation warnings before activation (missing source secret, no repos, no delivery target).
3. Preview of normalized signal payload and expected correlation key.

### Optional

1. “Replay sample alert payload” sandbox test button.
2. Template presets for common stacks (Node API, Kubernetes service, Next.js app).

## 8. Backend Architecture Changes

## 8.1 Ingestion and scheduling

### Must Have

1. Add observability webhook routes, parallel to GitHub webhook route pattern.
2. Add durable inbox table for observability events with lease/retry/dead-letter.
3. Add scheduler `startObservabilityInboxScheduler()` in bootstrap.
4. Map normalized incidents to `background_triggered` run enqueues.

### Optional

1. Shared generic inbox abstraction across GitHub + observability providers.
2. Pull-based collectors (periodic queries) as additional trigger source.

## 8.2 Data model additions (proposed)

### Must Have

1. `observability_signal_inbox`
   1. provider, delivery_id, payload, status, attempt_count, next_attempt_at, lock lease fields.
2. `observability_incidents`
   1. incident_id, agent_definition_id, state, severity, correlation_key, first_seen_at, last_seen_at, resolved_at.
3. `observability_incident_events`
   1. event timeline (ingested, deduped, run_enqueued, escalation, resolved).
4. `agent_runs` link extensions
   1. optional `incident_id` foreign key.

### Optional

1. `observability_remediation_actions` for bounded action audit logs.
2. `signal_fingerprints` cache table for high-volume dedupe.

## 8.3 API/proto changes

### Must Have

1. Extend `agent_setup.proto` V2 typed config model with observability blueprint payload.
2. Extend `runtime_visibility.proto` with:
   1. `ListIncidents`
   2. `GetIncident`
   3. `ListIncidentEvents`
   4. `RetryIncident`
3. Extend `integration_setup.proto` for observability source credentials and health status.

### Optional

1. Incident subscription stream API for real-time UI updates.

## 9. Runtime Execution Design

## 9.1 Diagnostic run prompt contract

### Must Have

Inject a deterministic context object into each run prompt:

1. Incident summary (severity, labels, annotations, first/last seen).
2. Correlated prior failures and last known healthy signal.
3. Selected repos and suspect commit ranges (if known).
4. Required output sections:
   1. hypothesis
   2. evidence
   3. confidence
   4. next action
   5. verification status

### Optional

1. Two-pass agent decomposition (`triage pass`, `deep diagnostics pass`).

## 9.2 Feedback delivery strategy

### Must Have

1. Post one “incident started” message.
2. Post bounded periodic updates (throttled).
3. Post one final structured summary with explicit status:
   1. resolved
   2. mitigated
   3. unresolved/escalated

### Optional

1. Threaded update compaction (edit last status message instead of posting many messages).
2. Additional outputs to GitHub checks/comments.

## 10. Guardrails and Safety

## 10.1 Must Have

1. Tooling policy stays bounded to existing runtime permission model.
2. No unbounded loops:
   1. hard max duration
   2. hard max iterations
3. Incident-level budget caps (tokens/time/commands).
4. Idempotency keys for every ingest and run enqueue.
5. Dead-letter and operator replay path for failed signal processing.

## 10.2 Optional

1. Approval gates for specific remediation actions.
2. Blast-radius policy (allowed repo/env/action matrix).
3. Model fallback policy for reliability tiers.

## 11. Rollout Plan

## Phase 0: Foundations (1 sprint)

### Must Have

1. Define typed config schema and validation.
2. Add observability inbox table + scheduler + retry semantics.
3. Add minimal incident tables and incident linking to runs.

### Optional

1. Shared inbox abstraction refactor.

## Phase 1: Usable V1 (1-2 sprints)

### Must Have

1. Agent Factory observability blueprint UI.
2. Alertmanager/Grafana/generic webhook ingestion adapters.
3. Incident correlation + run enqueue path.
4. Slack progressive/final feedback delivery.
5. Runtime visibility incident screens (list/detail/timeline).

### Optional

1. GitHub PR comment output.

## Phase 2: Closed-loop hardening (1-2 sprints)

### Must Have

1. Verification checks post-diagnosis.
2. Suppression/cooldown/noise policy tuning controls.
3. SLO dashboards and replay tooling.

### Optional

1. Auto-remediation with approval.
2. GitHub check-run integration.

## 12. Testing and Reliability Strategy

## 12.1 Must Have

1. Unit tests:
   1. payload normalization
   2. correlation keying
   3. dedupe and cooldown logic
2. Integration tests:
   1. webhook ack + inbox persistence
   2. sweep retry/dead-letter behavior
   3. incident -> run enqueue path
3. End-to-end tests:
   1. synthetic alert -> run -> Slack final update
   2. duplicate alert suppression
4. Chaos tests:
   1. webhook bursts
   2. partial outage in sandbox runner

## 12.2 Optional

1. Shadow mode in production where runs execute but no external action is posted.
2. Replay harness for historical incidents.

## 13. Success Metrics

## 13.1 Must Have

1. Time to first agent update (TTFU) after alert ingestion.
2. Time to incident triage classification.
3. Duplicate alert suppression rate.
4. Percentage of incidents with complete final report.
5. Delivery success rate (Slack/GitHub).

## 13.2 Optional

1. Mean time to validated recovery.
2. Human intervention rate per 100 incidents.
3. Auto-remediation success rate.

## 14. Concrete Repo Impact (Expected)

Likely change areas:

1. `apps/bff/src/main.ts`
   1. register/start observability ingress scheduler.
2. `apps/bff/src/agents/agent-definition-core.ts`
   1. add/validate observability trigger config and activation checks.
3. `apps/bff/src/agents/agent-config-schema.ts`
   1. add typed blueprint config sections.
4. `apps/bff/src/agents/agent-run-core.ts`
   1. incident-aware enqueue + artifact/report format.
5. `apps/bff/src/runtime/runtime-visibility-core.ts`
   1. incident APIs.
6. `apps/bff/src/integrations/integration-core.ts`
   1. observability source credentials + health snapshots.
7. `packages/db/src/schema/e2e_slice.ts`
   1. incident/inbox tables and indexes.
8. `packages/proto/proto/moontide/agents/v1/agent_setup.proto`
9. `packages/proto/proto/moontide/runtime/v1/runtime_visibility.proto`
10. `packages/proto/proto/moontide/integrations/v1/integration_setup.proto`
11. `apps/web/src/pages/factory/*` and `apps/web/src/pages/mission-control/*`
   1. blueprint config + incident UI.

## 15. Risks and Mitigations

1. Risk: Alert storms flood run queue.
   1. Mitigation: strict dedupe, cooldown windows, inbox batching, per-agent queue caps.
2. Risk: Noisy/low-quality payloads create false incidents.
   1. Mitigation: source-specific normalizers + confidence threshold + correlation policy.
3. Risk: Slack spam or truncation.
   1. Mitigation: throttled progress updates, digest summaries, message length guards.
4. Risk: Long incident loops consume excessive budget.
   1. Mitigation: hard per-incident budget and max-loop controls.
5. Risk: Unsafe auto-remediation.
   1. Mitigation: keep remediation optional and policy-gated; start diagnostic-only.

## 16. Decision

Implement **V1 on existing `background_triggered` runtime** with a new observability ingress + incident correlation layer.

This is the fastest, safest path to the desired observability-feedback-loop agent while leveraging current Moontide architecture and keeping future extensibility to a dedicated runtime mode.

## 17. References

1. OpenAI Harness Engineering (agent observability and feedback loops): https://openai.com/index/harness-engineering/
2. Kubernetes controllers (control loop model): https://kubernetes.io/docs/concepts/architecture/controller/
3. Kubernetes operator pattern: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/
4. OpenTelemetry signals: https://opentelemetry.io/docs/concepts/signals/
5. OpenTelemetry Collector processors: https://opentelemetry.io/docs/collector/components/processor/
6. OpenTelemetry Collector agent deployment pattern: https://opentelemetry.io/docs/collector/deploy/agent/
7. Prometheus alerting rules (`for`, `keep_firing_for`): https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/
8. Alertmanager routing and webhook config: https://prometheus.io/docs/alerting/latest/configuration/
9. Alertmanager notification/webhook data structures: https://prometheus.io/docs/alerting/latest/notifications/
10. Slack Web API rate limits: https://docs.slack.dev/apis/web-api/rate-limits/
11. Slack long message truncation: https://api.slack.com/changelog/2018-04-truncating-really-long-messages
12. GitHub webhook best practices: https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks
13. GitHub installation access token generation and expiry: https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app
