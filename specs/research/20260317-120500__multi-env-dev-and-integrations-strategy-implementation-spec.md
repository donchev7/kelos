# Multi-Environment Dev + External Integrations Strategy Implementation Spec

Date: 2026-03-17
Status: Proposed
Owner: App + Platform

## 1. Context

Moontide needs a development model that supports:

1. Fast local iteration (`localhost`) for product and code changes.
2. Stable end-to-end testing for OAuth + webhooks with GitHub and Slack.
3. Safe separation between development and production integrations/data.

The current app already exposes:

1. GitHub webhook: `/webhooks/github/events`
2. Slack webhook: `/webhooks/slack/events`
3. GitHub OAuth callback: `/integrations/github/callback`
4. Slack OAuth callback: `/integrations/slack/callback`

It also runs in-process schedulers, so each environment must be treated as a single-worker runtime unless and until scheduler concurrency is redesigned.

## 2. Problem Statement

Pure localhost dev is great for coding speed, but external providers require stable HTTPS callbacks and webhook endpoints. Ad-hoc tunnels are workable for occasional debugging but fragile for day-to-day team integration testing.

Without explicit environment separation, common failure modes are:

1. Dev traffic accidentally hitting prod integrations.
2. OAuth callback drift (wrong URL in provider settings).
3. Cross-environment token or webhook secret confusion.
4. Deploy risk from testing integration changes directly in prod.

## 3. Goals

1. Keep localhost as the primary inner-loop workflow.
2. Add a stable shared dev environment for integration testing.
3. Hard-isolate integration identities (GitHub App + Slack App) between dev and prod.
4. Define clear runbooks for config, deploy, validation, and incident response.
5. Keep the model simple enough for a small team to operate.

## 4. Non-Goals

1. No requirement for per-PR ephemeral integration environments in this phase.
2. No scheduler architecture rewrite in this spec.
3. No change to provider platform fundamentals (GitHub/Slack behavior).

## 5. External Platform Constraints (Research Summary)

### Necessary

1. Slack Events API uses a single Request URL per app config and requires public HTTPS verification.
2. Slack requires quick acknowledgement for event delivery (3-second expectation).
3. GitHub App webhook delivery targets one configured webhook URL per app.
4. GitHub webhook receivers should respond quickly (10-second guidance).
5. GitHub App supports multiple callback URLs for OAuth (up to configured limit), but webhook URL is still a single endpoint per app identity.

### Optional / Nice-to-have

1. Slack Socket Mode can reduce webhook-url dependence for local debugging, if future architecture supports it.
2. Provider config automation can reduce manual drift (manifest-driven Slack setup, scripted checklists for GitHub App settings).

## 6. Environment Topology

### Necessary

Adopt three lanes immediately:

| Lane | Purpose | Infra | URL | GitHub App | Slack App | Database |
|---|---|---|---|---|---|---|
| `local` | coding/testing inner loop | local process + local postgres | `http://localhost:3030` and `http://localhost:5173` | Dev App credentials (only when needed) | Dev App credentials (only when needed) | local |
| `dev` (shared) | stable integration and QA | Fly app `moontide-app-dev` | `https://moontide-app-dev.fly.dev` | Dedicated Dev GitHub App | Dedicated Dev Slack App | dedicated dev DB |
| `prod` | user traffic | Fly app `moontide-app-prod` | `https://moontide-app-prod.fly.dev` (or custom domain) | Dedicated Prod GitHub App | Dedicated Prod Slack App | dedicated prod DB |

### Optional / Nice-to-have

1. `preview` lane per PR (deploy-only, no provider callbacks).
2. Dedicated custom domains per lane (for example `dev.app.company.com`, `app.company.com`).

## 7. Integration Identity Strategy

### 7.1 GitHub App Strategy

#### Necessary

1. Create two separate GitHub Apps:
   1. `Moontide Dev`
   2. `Moontide Prod`
2. Set each app with environment-specific:
   1. Callback URL
   2. Setup URL
   3. Webhook URL
   4. Webhook secret
3. Install Dev app only on sandbox/test orgs or test repos.
4. Install Prod app only on production orgs/repos.
5. Keep private keys and client secrets environment-scoped (never shared across lanes).

#### Optional / Nice-to-have

1. Separate GitHub organizations for dev and prod validation.
2. Periodic automated verification script that checks configured URLs for both apps.

### 7.2 Slack App Strategy

#### Necessary

1. Create two separate Slack Apps:
   1. `Moontide Dev`
   2. `Moontide Prod`
2. Set environment-specific:
   1. OAuth redirect URL(s)
   2. Event Subscriptions Request URL
   3. Signing secret + client secret
3. Restrict Dev app installation to a dev workspace (or strictly sandbox channels in the same workspace if separate workspace is not possible).
4. Keep bot tokens and signing secrets environment-scoped.

#### Optional / Nice-to-have

1. App manifests in repo for reproducible Slack app config.
2. Separate dedicated dev Slack workspace for cleaner safety boundaries.

## 8. URL and Callback Mapping

### Necessary

Use fixed mappings per environment.

Dev:

1. `BETTER_AUTH_URL=https://moontide-app-dev.fly.dev`
2. `CORS_ORIGINS=https://moontide-app-dev.fly.dev`
3. `WEB_APP_URL=https://moontide-app-dev.fly.dev`
4. `PUBLIC_WEBHOOK_BASE_URL=https://moontide-app-dev.fly.dev`
5. `GITHUB_REDIRECT_URI=https://moontide-app-dev.fly.dev/integrations/github/callback`
6. `SLACK_REDIRECT_URI=https://moontide-app-dev.fly.dev/integrations/slack/callback`
7. `GitHub webhook URL=https://moontide-app-dev.fly.dev/webhooks/github/events`
8. `Slack request URL=https://moontide-app-dev.fly.dev/webhooks/slack/events`

Prod:

1. Same pattern on prod domain (`moontide-app-prod.fly.dev` or custom domain).

Local:

1. Keep localhost URLs in local `.env`.
2. Use tunnel only for temporary local webhook/OAuth debugging.

### Optional / Nice-to-have

1. Custom domains per lane.
2. DNS + TLS automation checks in CI.

## 9. Secrets and Config Partitioning

### Necessary

1. Separate Fly apps and secrets per lane (`moontide-app-dev`, `moontide-app-prod`).
2. Never copy prod secrets into dev/local.
3. Keep non-sensitive config in `fly.toml [env]`; keep sensitive values in Fly secrets.
4. Maintain a documented env matrix:
   1. Variable name
   2. local value source
   3. dev value source
   4. prod value source
5. Rotate dev and prod secrets independently.

### Optional / Nice-to-have

1. Secret manager integration (for example 1Password/Vault) as source of truth.
2. Automated drift detection for Fly secrets vs expected variable inventory.

## 10. Local Dev Workflow

### Necessary

1. Default mode: local-only development without external provider dependency.
2. For integration work:
   1. Switch to Dev provider credentials in local `.env`.
   2. Start tunnel.
   3. Temporarily repoint Dev app callback/webhook URLs to tunnel endpoint.
   4. Revert Dev app URLs back to shared dev URL after local test.
3. Never repoint Prod app URLs to a local tunnel.

### Optional / Nice-to-have

1. Script helpers for tunnel URL update + restore.
2. One-command local integration profile switcher.

## 11. Shared Dev Environment Workflow

### Necessary

1. Deploy integration-impacting changes to `moontide-app-dev` first.
2. Validate on shared dev:
   1. `/api/health`
   2. GitHub OAuth install flow
   3. Slack OAuth install flow
   4. GitHub webhook signature failure and success paths
   5. Slack request signature failure and success paths
3. Promote only after dev lane passes.

### Optional / Nice-to-have

1. Scheduled synthetic integration tests (hourly/daily).
2. Dashboards/alerts specific to dev integration endpoints.

## 12. CI/CD and Release Strategy

### Necessary

1. Two deployment targets:
   1. Dev app
   2. Prod app
2. `main` deploy flow must include:
   1. test/lint/typecheck gates
   2. deploy
   3. post-deploy smoke checks
3. Keep machine count explicitly controlled for scheduler safety:
   1. dev single machine
   2. prod single machine (until scheduler concurrency model changes)

### Optional / Nice-to-have

1. `develop` branch auto-deploy to shared dev.
2. PR preview apps without provider callbacks.
3. Progressive rollout/blue-green once runtime model supports safe parallel scheduler execution.

## 13. Safety Guardrails in Application Config

### Necessary

Add explicit environment guardrails:

1. Add `APP_ENV` (`local|dev|prod`) in env schema.
2. Add startup assertions:
   1. In `prod`, block obvious dev URLs/IDs.
   2. In `dev`, block obvious prod URLs/IDs where feasible.
3. Add runtime logging banner on startup:
   1. app env
   2. web domain
   3. provider app IDs (non-secret identifiers)
4. Add CI check that `fly.toml` for each lane has no placeholder values.

### Optional / Nice-to-have

1. Hard fail if provider app slug/id does not match expected env allowlist.
2. Integration heartbeat endpoint checking provider credential validity.

## 14. Data and Access Isolation

### Necessary

1. Use separate Postgres databases (or clusters) for dev and prod.
2. Do not allow prod repo installations in Dev GitHub app.
3. Do not allow prod workspace install for Dev Slack app.
4. Restrict who can change provider settings and Fly secrets.

### Optional / Nice-to-have

1. Row-level data tagging by `environment` for additional guardrails.
2. Audit log export for integration configuration changes.

## 15. Detailed Implementation Plan

### W1. Environment Baseline

#### Necessary

1. Create `moontide-app-dev` Fly app.
2. Create dev Postgres and attach.
3. Set dev secrets and non-secret env values.

#### Optional / Nice-to-have

1. Dedicated custom dev domain.

### W2. Provider Split

#### Necessary

1. Create Dev GitHub App and Dev Slack App.
2. Configure callback/webhook URLs to `moontide-app-dev`.
3. Store env-specific credentials in Fly secrets and local dev profile.

#### Optional / Nice-to-have

1. Slack manifest file in repo.

### W3. App Guardrails

#### Necessary

1. Add `APP_ENV` and assertions in config bootstrap.
2. Add startup diagnostics logs for env identity.

#### Optional / Nice-to-have

1. Provider ID allowlist enforcement at startup.

### W4. CI/CD Lanes

#### Necessary

1. Add workflow for deploy-to-dev and deploy-to-prod.
2. Add post-deploy smoke checks for both lanes.

#### Optional / Nice-to-have

1. Preview deployment workflow.

### W5. Runbooks and Documentation

#### Necessary

1. Create one operator runbook:
   1. rotate secrets
   2. rollback
   3. tunnel-based local integration test
2. Add provider settings checklist for Dev/Prod.

#### Optional / Nice-to-have

1. CLI scripts for repetitive provider/Fly checks.

## 16. Verification and Acceptance Criteria

### Necessary

Deployment and health:

1. Dev and prod each have exactly one running app machine.
2. `/api/health` is passing in both lanes.

Integration correctness:

1. Dev Slack app events reach only dev webhook endpoint.
2. Prod Slack app events reach only prod webhook endpoint.
3. Dev GitHub webhooks reach only dev webhook endpoint.
4. Prod GitHub webhooks reach only prod webhook endpoint.
5. OAuth flows complete in both lanes with lane-specific app identities.

Safety:

1. No placeholder values in deployed config.
2. No shared secrets between dev and prod lanes.

### Optional / Nice-to-have

1. Synthetic periodic integration probes with alerting.
2. Config drift report generated weekly.

## 17. Rollout Sequence

### Necessary

1. Implement W1 and W2.
2. Validate Dev end-to-end.
3. Implement W3 guardrails.
4. Implement W4 workflows.
5. Cut over team integration testing to shared dev.
6. Keep localhost for coding loop.

### Optional / Nice-to-have

1. Add preview lane after dev/prod are stable.

## 18. Rollback Plan

### Necessary

1. If dev split causes issues:
   1. freeze provider setting changes
   2. revert to last known good dev callback/webhook URLs
2. If prod deploy fails:
   1. rollback image release in Fly
   2. verify health checks
   3. keep provider endpoints unchanged unless endpoint integrity is implicated
3. Keep provider secrets/version history for quick reapplication.

### Optional / Nice-to-have

1. Automated rollback trigger based on smoke check failures.

## 19. Open Questions

### Necessary (answer before full rollout)

1. Will team maintain a dedicated dev Slack workspace?
2. Will team maintain separate dev GitHub org/repo sandbox?
3. Is `develop -> dev` auto-deploy desired or manual promote only?
4. Who owns provider config changes and secret rotation?

### Optional / Nice-to-have

1. Is Socket Mode desirable for local Slack debugging later?
2. Is preview-env support worth operational complexity now?

## 20. Public References

1. Slack request signing + 3-second acknowledgement guidance: https://api.slack.com/apis/connections/events-api
2. Slack HTTP API and retries overview: https://api.slack.com/apis/http
3. Slack OAuth v2 and redirect URL configuration: https://api.slack.com/authentication/oauth-v2
4. Slack app manifests: https://api.slack.com/concepts/manifests
5. Slack Socket Mode: https://api.slack.com/apis/connections/socket-implement
6. GitHub App webhooks: https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/using-webhooks-with-github-apps
7. GitHub callback URL behavior: https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/about-the-user-authorization-callback-url
8. GitHub webhook best practices (delivery timing/retries): https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks
9. Fly deploy behavior and flags: https://fly.io/docs/flyctl/deploy/
10. Fly app availability and machine count behavior: https://fly.io/docs/apps/app-availability/
11. Fly review apps guide: https://fly.io/docs/blueprints/review-apps-guide/
