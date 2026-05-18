# OpenClaw SaaS Platform — Architecture Tech Spec

**Version:** 1.0 · **Date:** February 2026 · **Status:** Draft  
**Scope:** Multi-tenant managed platform for the OpenClaw open-source AI agent

---

## 1. Executive Summary

OpenClaw is a self-hosted, local-first AI agent gateway with 175K+ GitHub stars. It connects messaging platforms (WhatsApp, Telegram, Slack, Discord, iMessage) to LLMs, enabling autonomous task execution through shell commands, browser control, file operations, cron scheduling, and a composable skills system.

The self-hosted model presents significant barriers: Docker/Node.js setup, API key management, WhatsApp pairing, network security configuration, and ongoing patching. A SecurityScorecard report found 135,000+ exposed instances, with 63% classified as vulnerable. The existing OpenClawd managed hosting service addresses basic deployment, but lacks the multi-tenant isolation, skill sandboxing, and enterprise controls needed for a true SaaS platform.

This spec defines the architecture for a production-grade, multi-tenant SaaS platform that wraps the full OpenClaw stack with enterprise isolation, security, billing, and operational tooling — informed by state-of-the-art patterns from Ramp's Inspect, Stripe's Minions, E2B, and Modal.

---

## 2. State of the Art: Lessons from Industry

### 2.1 Ramp Inspect — Closed-Loop Agent Sandboxing

Ramp's internal coding agent demonstrates the gold standard for sandboxed agent execution at scale:

- **Modal-based sandboxes:** Each agent session runs in an isolated sandbox with near-instant startup and filesystem snapshots for fast iteration cycles.
- **Cloudflare Durable Objects** for session state: Each coding session gets a persistent Durable Object with embedded SQLite, WebSocket hub, and event stream — enabling multiplayer observation of agent actions.
- **Closed-loop verification:** The agent doesn't just generate output — it runs tests, checks monitoring dashboards, queries databases, and visually verifies frontend changes. This "verification gap" closure is what makes it production-ready.
- **Full tool access within the sandbox:** The agent has the same context and tooling as a human engineer — CI/CD, Sentry, Datadog, GitHub, feature flags — all accessible from within the sandboxed environment.
- **Unlimited concurrency:** Multiple engineers run separate agent instances simultaneously with no resource contention.

**Key takeaway for OpenClaw SaaS:** Agents need rich, sandboxed environments with real tool access — not just code execution. The sandbox must include the full OpenClaw runtime (gateway, skills engine, messaging integrations) while isolating each tenant's execution context.

### 2.2 Stripe Minions — Unattended One-Shot Agents

Stripe's coding agents produce 1,000+ merged PRs per week with zero human-written code per PR:

- **Slack-triggered, fully unattended:** An engineer tags a Slack thread, the minion reads the full thread context plus linked resources, produces code, runs CI, and creates a review-ready PR with no intermediate interaction.
- **Deep monorepo integration:** Minions operate within Stripe's massive Ruby/Sorbet codebase, with access to internal documentation systems built specifically for agent consumption.
- **Parallelization as a core primitive:** Engineers maintain dedicated Slack channels for minion work and routinely launch multiple minions in parallel for different tasks.
- **Human review as the single gate:** All code is human-reviewed before merge, making the review step the quality control mechanism rather than interactive oversight during generation.

**Key takeaway for OpenClaw SaaS:** The trigger-and-forget pattern maps directly to how OpenClaw users interact via messaging. The SaaS platform should optimize for unattended, long-running agent sessions triggered asynchronously from chat — not synchronous request/response patterns.

### 2.3 E2B — Firecracker MicroVM Sandboxing at Scale

E2B has become the open-source standard for agent sandboxes, used by 88% of Fortune 100:

- **Firecracker microVMs:** Each sandbox gets a dedicated Linux kernel, providing hardware-level isolation that prevents container-escape attacks. Startup in <200ms.
- **24-hour session support** with pause/resume from saved state.
- **Docker + E2B partnership:** E2B provides code execution isolation while Docker's MCP Gateway provides secure tool connectivity to 200+ external services.
- **Customizable templates:** Pre-built sandbox images tailored to specific workloads.

**Key takeaway for OpenClaw SaaS:** MicroVM isolation (Firecracker or Kata Containers) is the right baseline for OpenClaw, which executes arbitrary shell commands and browser automation. Container-level isolation (shared kernel) is insufficient for this threat model.

### 2.4 Modal — Serverless Sandboxes for Coding Agents

Modal powers Ramp's Inspect and offers the most developer-friendly sandbox primitives:

- **Snapshot/restore:** Save full sandbox state and resume instantly — critical for OpenClaw's persistent sessions and long-term memory.
- **Scale to 50,000+ concurrent sessions** with sub-second cold starts.
- **gVisor isolation** (syscall interception in userspace) — a middle ground between containers and full microVMs.
- **Built-in egress policies** for controlling what sandboxed agents can reach on the network.
- **Pay-per-CPU-cycle billing** — only charge for active compute.

**Key takeaway for OpenClaw SaaS:** Snapshot/restore is essential for OpenClaw's persistent memory model. A user's agent should hibernate when inactive and resume with full context in <1 second.

---

## 3. Design Principles

1. **Isolation is non-negotiable.** OpenClaw agents have shell access, browser control, and file system operations. Every tenant gets hardware-level isolation via microVMs. No shared-kernel designs.

2. **The sandbox IS the product.** Unlike simpler SaaS wrappers, each tenant's OpenClaw instance must be a fully functional runtime — Gateway, skills engine, messaging integrations, persistent memory — running inside its own isolated environment.

3. **Unattended-first.** Optimize for asynchronous, long-running agent sessions triggered from messaging platforms. The agent may run for hours or days without user interaction (cron jobs, monitoring, proactive outreach).

4. **Open-source compatibility.** The SaaS platform runs unmodified OpenClaw (or minimal patches). Tenants should be able to export their configuration, skills, and memory and self-host at any time. No vendor lock-in.

5. **Defense in depth.** Every layer — network, runtime, skill execution, LLM interaction, data storage — has independent security controls. Assume any single layer can be compromised.

6. **Scale-to-zero economics.** Most personal AI agents are idle 95%+ of the time. The platform must support hibernation with instant resume to make per-tenant economics viable.

---

## 4. High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                        EXTERNAL INTERFACES                          │
│  WhatsApp  Telegram  Slack  Discord  Signal  iMessage  WebChat      │
└──────────┬──────────────────────────────────────────────────────┬────┘
           │                                                      │
           ▼                                                      ▼
┌─────────────────────┐                            ┌──────────────────┐
│   MESSAGE INGRESS   │                            │   TENANT PORTAL  │
│   (Edge Gateway)    │                            │   (Web Dashboard)│
│                     │                            │                  │
│ • Webhook receivers │                            │ • Onboarding     │
│ • Phone # / token   │                            │ • Config editor  │
│ • Tenant resolution │                            │ • Skill market   │
│ • Rate limiting     │                            │ • Usage / billing│
│ • TLS termination   │                            │ • Audit logs     │
└──────────┬──────────┘                            └────────┬─────────┘
           │                                                 │
           ▼                                                 ▼
┌──────────────────────────────────────────────────────────────────────┐
│                         CONTROL PLANE                                │
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────────┐  │
│  │   Tenant     │  │   Session    │  │   Lifecycle               │  │
│  │   Registry   │  │   Router     │  │   Orchestrator            │  │
│  │              │  │              │  │                           │  │
│  │ • Identity   │  │ • Tenant →   │  │ • Provision / hibernate   │  │
│  │ • Config     │  │   sandbox    │  │ • Wake / resume           │  │
│  │ • Billing    │  │   mapping    │  │ • Scale / migrate         │  │
│  │ • Quotas     │  │ • Sticky     │  │ • Health checks           │  │
│  │ • API keys   │  │   sessions   │  │ • Rolling updates         │  │
│  └──────────────┘  └──────────────┘  └───────────────────────────┘  │
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────────┐  │
│  │   Secrets    │  │   Metering   │  │   Skill                   │  │
│  │   Vault      │  │   Pipeline   │  │   Registry                │  │
│  │              │  │              │  │                           │  │
│  │ • API keys   │  │ • LLM tokens │  │ • Curated marketplace     │  │
│  │ • OAuth      │  │ • Compute    │  │ • Security review         │  │
│  │ • Encrypted  │  │ • Messages   │  │ • Versioning              │  │
│  │   at rest    │  │ • Storage    │  │ • Dependency scanning     │  │
│  └──────────────┘  └──────────────┘  └───────────────────────────┘  │
│                                                                      │
└──────────────────────────────┬───────────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────────┐
│                          DATA PLANE                                  │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │                    TENANT SANDBOX (per tenant)                  │  │
│  │                    Firecracker MicroVM                          │  │
│  │                                                                │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────┐  │  │
│  │  │   OpenClaw   │  │   Skills     │  │   Tool              │  │  │
│  │  │   Gateway    │  │   Runtime    │  │   Sandbox           │  │  │
│  │  │              │  │              │  │   (nested)          │  │  │
│  │  │ • Agent loop │  │ • Installed  │  │                     │  │  │
│  │  │ • Sessions   │  │   skills     │  │ • Shell exec        │  │  │
│  │  │ • Memory     │  │ • Skill      │  │ • Browser           │  │  │
│  │  │ • Channel    │  │   discovery  │  │ • File I/O          │  │  │
│  │  │   adapters   │  │ • Hot reload │  │ • Network (filtered)│  │  │
│  │  └──────────────┘  └──────────────┘  └─────────────────────┘  │  │
│  │                                                                │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────┐  │  │
│  │  │   LLM Proxy  │  │   Persistent │  │   Observability     │  │  │
│  │  │              │  │   Storage    │  │   Sidecar           │  │  │
│  │  │ • Model      │  │              │  │                     │  │  │
│  │  │   routing    │  │ • Sessions   │  │ • Structured logs   │  │  │
│  │  │ • Token      │  │ • Memory     │  │ • Action audit      │  │  │
│  │  │   metering   │  │ • Skills     │  │ • Metrics export    │  │  │
│  │  │ • Failover   │  │ • Config     │  │ • Anomaly detection │  │  │
│  │  └──────────────┘  └──────────────┘  └─────────────────────┘  │  │
│  │                                                                │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐      │
│  │Tenant A │ │Tenant B │ │Tenant C │ │Tenant D │ │   ...   │      │
│  │ microVM │ │ microVM │ │ microVM │ │  (hiber- │ │         │      │
│  │ (active)│ │ (active)│ │ (active)│ │  nating) │ │         │      │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘      │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 5. Control Plane — Detailed Design

### 5.1 Tenant Registry

The central source of truth for all tenant state. Stores:

- **Identity:** User/org account, authentication credentials, SSO configuration (enterprise tier).
- **Configuration:** The tenant's `openclaw.json` equivalent — agent config, model selection, channel bindings, tool policies, skill manifest. Stored encrypted, versioned with full change history.
- **Subscription tier:** Free / Pro / Team / Enterprise, with associated quota limits.
- **Integration credentials:** References to Secrets Vault entries for API keys, OAuth tokens, WhatsApp Business credentials.
- **Sandbox metadata:** Current sandbox state (active/hibernating/provisioning), assigned host, snapshot ID, last activity timestamp.

**Storage:** PostgreSQL with row-level security (RLS). Each tenant's data is isolated at the database level via RLS policies keyed on `tenant_id`. Enterprise tier optionally gets schema-per-tenant or dedicated database instances.

### 5.2 Message Ingress (Edge Gateway)

The public-facing entry point for all inbound messages from chat platforms. This is the most latency-sensitive component — messages from WhatsApp or Telegram need sub-second routing to the correct tenant sandbox.

**Responsibilities:**

- **Webhook reception:** Each messaging platform delivers messages via webhooks. The Edge Gateway hosts webhook endpoints for all supported platforms.
- **Tenant resolution:** Maps inbound messages to tenants. The mapping key depends on the platform:
  - WhatsApp: phone number → tenant
  - Telegram: bot token → tenant
  - Slack: workspace + bot ID → tenant
  - Discord: bot token + guild → tenant
  - Signal: phone number → tenant
- **Wake-on-message:** If the target tenant's sandbox is hibernating, triggers a wake via the Lifecycle Orchestrator. The message is queued in a durable buffer (NATS JetStream) until the sandbox is ready.
- **Rate limiting:** Per-tenant and per-platform rate limits to prevent abuse and manage costs.
- **TLS termination and DDoS protection:** Runs behind Cloudflare or equivalent edge network.

**Implementation:** Deployed as a stateless service across multiple availability zones. Uses consistent hashing for tenant-to-pod affinity (optimizes for hot routing tables) with fallback to the Tenant Registry for cold lookups.

### 5.3 Session Router

Maps active conversations to sandbox endpoints:

- **Sticky routing:** Once a tenant's sandbox is active, all messages for that tenant route to the same sandbox instance. The router maintains an in-memory mapping (backed by Redis) of `tenant_id → sandbox_endpoint`.
- **Failover:** If a sandbox becomes unhealthy, the router triggers re-provisioning and replays buffered messages from the durable queue.
- **Multi-agent routing passthrough:** OpenClaw's internal multi-agent routing (personal agent, work agent, etc.) operates within the sandbox. The Session Router only needs to route to the correct tenant — intra-tenant agent routing is handled by the OpenClaw Gateway inside the sandbox.
- **Durable session state (alternative architecture):** Following the Ramp Inspect / Open-Inspect pattern, session routing state can be modeled as Cloudflare Durable Objects (or equivalent stateful actors) with embedded SQLite per session. This eliminates the Redis dependency for the hot path and provides stronger consistency guarantees for individual session state, at the cost of requiring a Cloudflare Workers deployment. A separate EventBus Durable Object handles real-time event broadcasting to connected dashboard clients with user-tagged WebSocket connections. See **Appendix A** for the full Inspect reference architecture.

### 5.4 Lifecycle Orchestrator

Manages the full lifecycle of tenant sandboxes:

- **Provision:** On tenant signup, builds a sandbox image from the OpenClaw base template + tenant config. Injects secrets via Vault sidecar. Starts the microVM.
- **Hibernate:** After configurable idle timeout (default: 15 minutes of no messages), snapshots the full microVM state (memory + filesystem) to object storage and terminates the VM. This is the critical cost optimization — most agents are idle most of the time.
- **Wake/Resume:** On inbound message to a hibernating tenant, restores from snapshot. Target: <1 second resume latency (following Modal's snapshot/restore pattern). The pre-warmed snapshot includes the full OpenClaw process state — no cold boot of Node.js, no re-establishment of sessions.
- **Scale:** Horizontal scaling of the sandbox fleet via Kubernetes + Karpenter (or equivalent auto-provisioner). Each physical host runs multiple microVMs via Firecracker.
- **Rolling updates:** When a new OpenClaw version is released, the orchestrator builds new base images and migrates tenants on their next wake cycle. Active tenants are migrated during their next idle window.
- **Health checks:** Continuous liveness/readiness probes to each active sandbox. Unhealthy sandboxes are automatically re-provisioned from the latest snapshot.

**State machine:**

```
PROVISIONING → ACTIVE ⇄ HIBERNATING → TERMINATED
                 ↓
             UNHEALTHY → RE-PROVISIONING → ACTIVE
```

### 5.5 Secrets Vault

Manages all sensitive credentials:

- **Tenant API keys:** LLM provider keys (Anthropic, OpenAI, Google), messaging platform credentials, third-party service OAuth tokens.
- **Platform-managed keys:** For tenants who opt into platform-provided LLM access (no BYOK), the platform's own API keys are used with per-tenant metering.
- **Injection model:** Secrets are injected into the sandbox as environment variables at boot time via a Vault sidecar (or init container equivalent for microVMs). They never touch the filesystem in plaintext — addressing the #1 security issue with self-hosted OpenClaw deployments.
- **Rotation:** Automatic rotation support with zero-downtime re-injection.

**Implementation:** HashiCorp Vault (or AWS Secrets Manager for cloud-native deployments) with tenant-scoped policies.

### 5.6 Metering Pipeline

Captures all billable events for usage-based pricing:

- **LLM tokens:** The LLM Proxy inside each sandbox emits token-level usage events (model, input tokens, output tokens, cache hits) on every API call.
- **Compute time:** Sandbox active time metered per second.
- **Message volume:** Inbound/outbound messages counted per platform.
- **Storage:** Persistent storage usage (sessions, memory, skills, files).
- **Skill executions:** Number and duration of skill invocations.

**Pipeline:** Events are emitted from sandboxes via a lightweight agent → Kafka/NATS topic → aggregation service → Stripe Billing (for metered billing) + data warehouse (for analytics dashboards).

### 5.7 Skill Registry (Marketplace)

The most significant value-add over self-hosted OpenClaw. A curated, vetted skill marketplace:

- **Submission:** Developers submit skills as versioned packages (following OpenClaw's SKILL.md format).
- **Security review pipeline:**
  1. **Static analysis:** Automated scanning for known malicious patterns (data exfiltration, credential harvesting, prompt injection payloads). Informed by Cisco's research on malicious OpenClaw skills.
  2. **Sandboxed execution testing:** Each skill is executed in an isolated sandbox against a battery of test scenarios to verify it only performs its declared operations.
  3. **Dependency audit:** All external dependencies scanned for known vulnerabilities (Snyk/Trivy).
  4. **Human review:** High-risk skills (those requesting shell access, network access, or filesystem writes) require manual security review.
- **Installation:** One-click install from the tenant dashboard or via chat command. The skill is pulled into the tenant's sandbox and hot-reloaded by the OpenClaw runtime.
- **Permissions model:** Each skill declares required capabilities (e.g., `network:egress`, `filesystem:write`, `shell:execute`). Tenants must explicitly approve each capability. Skills cannot escalate privileges beyond their declared scope.

---

## 6. Data Plane — Detailed Design

### 6.1 Tenant Sandbox Architecture

Each tenant gets a dedicated Firecracker microVM containing the full OpenClaw stack:

**Why Firecracker microVMs (not containers):**

OpenClaw agents execute arbitrary shell commands, control browsers, read/write files, and can autonomously write and install new skills. This is fundamentally untrusted code execution. Container isolation (shared kernel) is insufficient because:

- Container escape vulnerabilities are discovered regularly and would allow cross-tenant access.
- OpenClaw's shell execution could exploit shared-kernel attack surfaces.
- Compliance requirements (SOC 2, GDPR) increasingly mandate hardware-level isolation for multi-tenant systems processing user data.

Firecracker provides dedicated-kernel isolation with <200ms startup — comparable to container performance for the snapshot/resume path that dominates steady-state operations.

**Sandbox contents:**

| Component | Purpose |
|---|---|
| OpenClaw Gateway process | Core agent loop, session management, channel adapters |
| OpenClaw Skills runtime | Skill discovery, injection, and hot-reload |
| Docker-in-VM (optional) | For skills that require their own containerized execution |
| LLM Proxy sidecar | Model routing, token metering, failover |
| Observability sidecar | Log collection, metrics export, action audit |
| Persistent volume mount | Sessions, memory, skills, workspace files |
| Network policy agent | Egress filtering, DNS control |

### 6.2 Nested Sandboxing: Tool Execution

OpenClaw's tools (shell exec, browser, file I/O) run inside the tenant's microVM, but high-risk operations get an additional layer of isolation:

**Tier 1 — Low risk (no additional sandboxing):**
- Read-only file access within workspace
- Session/memory read operations
- LLM API calls (proxied through metering layer)
- Calendar/email reads (via OAuth with read-only scopes)

**Tier 2 — Medium risk (process-level sandboxing):**
- Shell command execution: Runs in a seccomp-filtered, namespaced subprocess with restricted syscalls. No access to the Gateway process memory or configuration.
- File write operations: Confined to the workspace directory via mount namespaces.
- Web fetch/search: Proxied through the network policy agent with URL allowlisting.

**Tier 3 — High risk (nested container/Deno isolate):**
- Browser automation: Chromium runs in a nested container with its own network namespace. No access to the host microVM filesystem.
- Custom skill execution: Untrusted or community skills run in Deno isolates (V8 sandbox) with explicit capability grants. Cannot access the filesystem, network, or environment variables unless the skill's permission manifest declares the need and the tenant approves.
- Self-authored skills: When the agent autonomously writes a new skill, it executes in the most restricted Tier 3 environment until the tenant explicitly promotes it.

This nested model follows the defense-in-depth principle: even if a prompt injection attack compromises the LLM's behavior, the blast radius is contained first by the tool sandbox tier, then by the microVM boundary, then by the network policy layer.

### 6.3 LLM Proxy

A lightweight sidecar inside each sandbox that mediates all LLM API calls:

- **Model routing:** Supports the tenant's configured model selection (primary + fallbacks), matching OpenClaw's native model-selection logic.
- **Token metering:** Captures input/output token counts, model used, cache hit status, and latency for every call. Emits structured events to the Metering Pipeline.
- **BYOK + Platform keys:** If the tenant provides their own API keys, the proxy uses those (injected from Vault). If using platform-provided access, the proxy authenticates with the platform's pooled keys and meters usage for billing.
- **Rate limiting:** Enforces per-tenant token budgets and requests-per-minute limits based on subscription tier.
- **Prompt injection detection:** Optionally runs a lightweight classifier on outbound prompts to detect prompt injection patterns in tool outputs (web search results, email content, etc.) before they reach the primary model. This addresses OpenClaw's documented vulnerability to injection via untrusted content the agent reads.

### 6.4 Persistent Storage

OpenClaw stores state as Markdown/YAML files under `~/.openclaw/`. The SaaS platform must preserve this model (for export compatibility) while adding durability and backup:

- **Workspace volume:** Each sandbox mounts a persistent volume containing the tenant's workspace — sessions, memory, skills, config files. This volume persists across hibernate/wake cycles.
- **Backing store:** Volumes are backed by distributed block storage (e.g., AWS EBS, Ceph RBD) with automatic snapshots.
- **Backup/export:** Nightly automated backups to object storage. Tenants can trigger on-demand exports that produce a tarball of their entire `~/.openclaw/` directory — ready to drop into a self-hosted deployment.
- **Encryption:** All persistent volumes encrypted at rest with tenant-specific keys (envelope encryption via Vault).

### 6.5 Network Policy

Each sandbox operates with strict network controls:

- **Default deny egress:** All outbound connections are blocked unless explicitly allowed.
- **Allowlisted destinations:**
  - LLM API endpoints (api.anthropic.com, api.openai.com, etc.)
  - Messaging platform APIs (WhatsApp Business API, Telegram Bot API, etc.)
  - Tenant-configured external services (via skill permissions)
  - Skill Registry (for skill installation)
- **DNS filtering:** All DNS queries from within the sandbox route through a policy-aware DNS resolver that enforces the allowlist.
- **No lateral movement:** Sandboxes cannot communicate with each other. There is no inter-tenant network path.
- **Egress logging:** All outbound connections are logged with destination, port, bytes transferred, and initiating process — available in the tenant's audit log.

---

## 7. Messaging Integration Architecture

The hardest operational challenge in this platform. Each messaging platform has different integration models:

### 7.1 WhatsApp (Most Complex)

- **WhatsApp Business API** requires a dedicated phone number per business account. For the SaaS platform, this means provisioning phone numbers for each tenant.
- **Options:**
  - **Shared number with tenant routing:** A pool of platform-owned numbers, with tenants identified by conversation context. Simplest operationally but provides a weaker user experience (the tenant doesn't "own" their number).
  - **Dedicated number per tenant:** Each tenant gets a provisioned number via Twilio/MessageBird. Better UX, higher cost. Required for Pro+ tiers.
  - **BYOP (Bring Your Own Phone):** Enterprise tenants connect their existing WhatsApp Business account. The platform acts as a webhook relay.
- **Implementation:** A dedicated WhatsApp Bridge service manages the connection pool, number provisioning, message routing, and session state for the WhatsApp Business API. This sits between the Edge Gateway and the Message Ingress layer.

### 7.2 Telegram / Discord / Slack

These platforms use bot tokens, making multi-tenancy straightforward:

- Each tenant creates their own bot and provides the token (stored in Vault).
- The platform registers webhook endpoints per tenant-bot.
- Inbound webhooks route through the Edge Gateway → Session Router → tenant sandbox.

### 7.3 Connection Lifecycle Management

- **Health monitoring:** Continuous checks that each tenant's messaging connections are alive. Automatic reconnection on failure.
- **Credential refresh:** OAuth tokens (Slack, Google) are refreshed automatically before expiry.
- **Rate limit management:** Per-platform rate limits are tracked globally and per-tenant to avoid platform-level throttling.

---

## 8. Security Architecture

### 8.1 Threat Model

| Threat | Vector | Mitigation |
|---|---|---|
| Cross-tenant data access | VM escape, shared resource exploitation | Firecracker microVM isolation (dedicated kernel per tenant) |
| Prompt injection | Malicious content in emails, web pages, messages the agent reads | Prompt injection classifier in LLM Proxy; tool output sanitization |
| Malicious skills | Community-submitted skills performing data exfiltration | Skill review pipeline; Deno isolate execution; capability-based permissions |
| Credential theft | Agent accesses API keys or OAuth tokens | Secrets injected as env vars, never on filesystem; skill sandbox cannot read env vars |
| Lateral movement | Compromised sandbox probes internal network | Default-deny egress; no inter-sandbox connectivity; DNS filtering |
| Resource exhaustion | Single tenant consumes disproportionate compute/network | Per-tenant CPU/memory limits on microVM; rate limiting at Edge Gateway; token budgets |
| Data exfiltration via LLM | Agent sends sensitive data to the model provider | Tenant-configurable data classification; optional PII redaction in LLM Proxy |
| Admin plane compromise | Attacker gains access to control plane | Control plane in separate VPC; zero-trust access; audit logging on all admin operations |

### 8.2 Security Boundaries

```
Internet ─── [Edge Gateway] ─── [Control Plane VPC] ─── [Data Plane VPC]
                                        │                       │
                                   (no direct access)     (per-tenant microVMs)
                                        │                       │
                                   Admin access only      No inter-VM traffic
                                   via VPN + MFA          Network policies enforced
                                                          at hypervisor level
```

### 8.3 Audit and Compliance

- **Action audit log:** Every tool invocation, shell command, file operation, network request, and skill execution is logged with timestamp, tenant ID, agent ID, session ID, and full parameters. Stored immutably in append-only storage.
- **Tenant-accessible audit dashboard:** Tenants can review all actions their agent has taken. Filterable by time, action type, risk level.
- **SOC 2 Type II:** Architecture designed to support SOC 2 compliance. Encryption at rest and in transit, access controls, audit trails, incident response procedures.
- **GDPR / data residency:** Tenant data stays in the configured region. Sandbox placement is region-aware. Data export and deletion APIs support right-to-erasure requests.

---

## 9. Observability

### 9.1 Platform-Level (Operator)

- **Infrastructure metrics:** Per-host CPU/memory/disk, microVM count, sandbox startup/resume latency (P50/P95/P99), snapshot sizes.
- **Message pipeline:** End-to-end latency (message received → agent response sent), queue depth, dead-letter counts.
- **Sandbox fleet health:** Active vs. hibernating vs. unhealthy sandboxes, wake latency distribution, snapshot success rate.
- **Cost tracking:** Per-tenant compute cost, LLM token cost, storage cost — feeding into margin analysis.

### 9.2 Tenant-Level (Customer Dashboard)

- **Agent activity:** Messages processed, skills executed, tools invoked — broken down by channel and agent (for multi-agent tenants).
- **LLM usage:** Token consumption by model, cost estimate, cache hit rate.
- **Error log:** Failed tool executions, skill errors, model failovers, with enough context to debug.
- **Performance:** Agent response latency distribution, time spent in tool execution vs. LLM inference.

### 9.3 Implementation

OpenTelemetry instrumentation throughout, exporting to:
- **Metrics:** Prometheus → Grafana (operator) / embedded dashboards (tenant)
- **Logs:** Structured JSON → Loki or Elasticsearch, with tenant_id as a first-class label
- **Traces:** Distributed traces spanning message ingress → routing → sandbox → tool execution → LLM call → response

---

## 10. Billing Model

### 10.1 Tier Structure

| Tier | Target | Key Limits |
|---|---|---|
| **Free** | Hobbyists, evaluation | 1 agent, 100 messages/day, 1 channel, community skills only, shared LLM quota |
| **Pro** ($29/mo) | Individual power users | 3 agents, 2,000 messages/day, all channels, dedicated WhatsApp number, BYOK LLM, 10GB storage |
| **Team** ($99/mo) | Small teams, freelancers | 10 agents, 10,000 messages/day, team workspace sharing, approval workflows, priority support |
| **Enterprise** (custom) | Organizations | Unlimited agents, SSO/SAML, dedicated infrastructure, custom data residency, SLA, audit exports |

### 10.2 Usage-Based Components

On top of the base subscription:
- **LLM tokens** (if using platform-provided keys): Pass-through at cost + 20% margin, metered per token.
- **Compute overage:** Base tier includes N hours/month of active compute. Overage billed per minute.
- **Storage overage:** Base tier includes N GB. Overage per GB/month.
- **Premium skills:** Marketplace skills from third-party developers may carry per-use or subscription fees (revenue split: 70% developer / 30% platform).

---

## 11. Deployment Architecture

### 11.1 Infrastructure

| Component | Technology | Rationale |
|---|---|---|
| Sandbox runtime | Firecracker microVMs on bare metal | Hardware-level isolation, <200ms startup, snapshot/restore |
| Orchestration | Kubernetes (control plane) + custom scheduler (data plane) | K8s for stateless services; custom scheduler for microVM placement and bin-packing |
| Edge | Cloudflare Workers or AWS CloudFront + Lambda@Edge | Global webhook reception with minimal latency |
| Message queue | NATS JetStream | Lightweight, durable queuing for message buffering during sandbox wake |
| State store | PostgreSQL (Citus for horizontal scaling) | Control plane state with row-level security |
| Secrets | HashiCorp Vault | Tenant-scoped secret management with audit logging |
| Object storage | S3-compatible (AWS S3 / MinIO) | Sandbox snapshots, backups, skill packages |
| Block storage | EBS / Ceph RBD | Persistent volumes for active sandbox workspaces |
| Billing | Stripe | Subscription management, metered billing, invoicing |
| Observability | OpenTelemetry → Prometheus + Loki + Tempo → Grafana | Full-stack observability with tenant-aware dashboards |

### 11.2 Regional Deployment

- **Initial launch:** US-East + EU-West (two regions covers the majority of early adopters).
- **Data residency:** Tenant data (config, sessions, memory, snapshots) stays in their configured region. LLM API calls may cross regions (depending on provider endpoints).
- **Edge Gateway:** Globally distributed via Cloudflare. Webhooks are received at the nearest edge location and routed to the correct regional data plane.

### 11.3 Capacity Planning

Key metrics to model:
- **Concurrent active sandboxes:** A function of total tenants × active percentage (expect 5-10% concurrent during peak).
- **Snapshot storage:** ~500MB–2GB per tenant (compressed microVM snapshot). At 100K tenants: 50–200TB snapshot storage.
- **Wake latency budget:** P99 resume from snapshot must be <2 seconds (including volume re-attach). This constrains snapshot format and storage throughput.
- **Message throughput:** OpenClaw processes messages sequentially per session. At 100K tenants with 5% concurrent, and average 1 message/minute per active session: ~5,000 messages/minute platform-wide. The Edge Gateway and message queue are not the bottleneck — sandbox wake latency is.

---

## 12. Migration Path: Self-Hosted → SaaS

A critical adoption driver. Users with existing self-hosted OpenClaw installations should be able to migrate seamlessly:

1. **Export:** User runs `openclaw export` on their self-hosted instance, producing a tarball of `~/.openclaw/` (config, sessions, memory, skills, workspace).
2. **Import:** User uploads the tarball via the SaaS tenant portal. The platform provisions a sandbox, injects the exported state, and validates configuration.
3. **Channel re-binding:** Messaging platform credentials are re-configured to point webhooks at the SaaS Edge Gateway instead of the user's self-hosted instance.
4. **Validation:** Platform runs a health check confirming all channels are connected, all skills are loaded, and the agent responds correctly.

Reverse migration (SaaS → self-hosted) follows the same export/import pattern. No vendor lock-in.

---

## 13. Differentiation from OpenClawd

OpenClawd provides basic managed hosting — one-click deployment with hardened defaults. This spec goes significantly further:

| Capability | OpenClawd | This Spec |
|---|---|---|
| Isolation | Likely container-level | Firecracker microVM (hardware-level) |
| Skill sandboxing | None (inherits OpenClaw defaults) | Nested Deno isolates with capability-based permissions |
| Skill marketplace | None | Curated marketplace with security review pipeline |
| Hibernate/resume | Unknown | <1s snapshot restore with scale-to-zero economics |
| Prompt injection defense | None | LLM Proxy classifier on tool outputs |
| Team features | None | Shared workspaces, RBAC, approval workflows |
| Audit logging | None | Full action audit with tenant-accessible dashboard |
| Enterprise features | None | SSO/SAML, dedicated infrastructure, data residency, SLA |
| Export/portability | Unknown | Full bidirectional migration with self-hosted OpenClaw |
| Multi-region | Unknown | US-East + EU-West with region-aware data residency |

---

## 14. Open Questions and Risks

1. **Upstream compatibility:** OpenClaw is actively developed and the project is transitioning to an open-source foundation. Breaking changes in the Gateway protocol or skills format could require significant platform adaptation. Mitigation: pin to specific OpenClaw versions per tenant; decouple upgrade cadence.

2. **WhatsApp Business API costs:** Dedicated phone numbers per tenant add significant COGS. Meta's pricing for WhatsApp Business conversations is per-conversation, which needs to be passed through or absorbed. This is the single largest variable cost for personal-use tiers.

3. **Snapshot size growth:** OpenClaw's persistent memory grows unboundedly over time. Large snapshots increase hibernate/resume latency and storage costs. Mitigation: implement memory compaction/summarization; tier storage limits; incremental snapshots.

4. **Prompt injection remains unsolved:** No amount of sandboxing fully prevents an LLM from being manipulated by injected instructions. The defense layers reduce blast radius but cannot eliminate the fundamental risk. Transparency with users about this limitation is essential.

5. **Regulatory uncertainty:** Autonomous AI agents that can send emails, make purchases, and interact with external services on behalf of users may face emerging regulatory requirements. The audit logging and approval workflow features are designed to support compliance, but the regulatory landscape is evolving.

---

## 15. Implementation Phases

### Phase 1: Foundation (Months 1–3)
- Control plane core: tenant registry, auth, billing integration
- Firecracker sandbox runtime: provision, start, stop, snapshot, restore
- Edge Gateway: webhook reception for WhatsApp + Telegram
- Basic tenant dashboard: onboarding wizard, config editor, usage view
- LLM Proxy with token metering
- Single region deployment (US-East)

### Phase 2: Production Hardening (Months 4–6)
- Hibernate/resume with <1s wake latency
- Full network policy enforcement
- Nested tool sandboxing (Tier 2 + 3)
- Audit logging and tenant dashboard
- Secrets Vault integration
- All messaging platform support
- Second region (EU-West)

### Phase 3: Marketplace and Enterprise (Months 7–9)
- Skill marketplace with security review pipeline
- Team features: shared workspaces, RBAC, approval workflows
- SSO/SAML for enterprise
- Export/import for self-hosted migration
- Dedicated infrastructure option for enterprise
- Prompt injection detection in LLM Proxy

### Phase 4: Scale and Differentiation (Months 10–12)
- Multi-agent orchestration features
- Hybrid deployment (customer VPC data plane + managed control plane)
- Advanced observability: anomaly detection, cost optimization recommendations
- Premium skill developer program
- SOC 2 Type II certification

---

## Appendix A: Ramp Inspect / Open-Inspect Reference Architecture (Detailed)

The following is a comprehensive documentation of the Ramp Inspect architecture as implemented in the open-source Open-Inspect (Background Agents) project. This serves as the primary reference implementation for sandboxed, multi-tenant agent platforms and directly informs several design decisions in this spec. The architecture is decomposed into seven major subsystems, each color-coded in the reference diagram.

---

### A.1 System Overview

The Inspect architecture follows a clean three-tier separation:

1. **Presentation tier:** A React frontend that embeds sandboxed environments via iFrames and communicates with the backend via HTTP RPC, WebSocket subscriptions, and prompt submission.
2. **Control tier:** A Cloudflare Worker (`agent-api`) backed by Durable Objects and D1, responsible for session lifecycle, real-time event distribution, REST API routing, and sandbox spawn orchestration.
3. **Execution tier:** Modal Sandboxes containing the full development environment — agent runtime, code-server IDE, VNC-accessible browser, and repository code — managed by a Modal Backend service.

The key insight is that the control plane (Cloudflare) and the data plane (Modal) are on entirely separate infrastructure stacks, communicating via authenticated WebSocket tunnels and HTTP APIs. This separation allows each to scale independently and provides a natural security boundary.

---

### A.2 Cloudflare Worker — Control Plane (`agent-api`)

The Cloudflare Worker serves as the API gateway and stateful session coordinator. It hosts:

#### A.2.1 REST API Routes (`/api/sessions/*`)

The primary HTTP interface for the frontend and external integrations. Handles:

- Session CRUD (create, list, get, delete)
- Session configuration and parameter updates
- File upload routing
- Authentication and authorization checks

All REST routes are prefixed under `/api/sessions/*` and are stateless — they delegate to Durable Objects for any stateful operations.

#### A.2.2 SessionAgent Durable Object

The core stateful primitive. **One Durable Object instance exists per active session**, providing:

- **Per-session state:** Complete session metadata, configuration, and current status. Stored durably — survives Worker restarts and redeployments.
- **Messages & Parts:** The full conversation history between the user and the agent, stored as an ordered sequence of message parts (text, code, tool calls, tool results, images). This is the equivalent of OpenClaw's session transcript.
- **Question handling:** When the agent inside the sandbox needs to ask the user a clarifying question (equivalent to OpenClaw's interactive prompts), the Durable Object holds the pending question state and routes the user's response back to the sandbox.
- **Durable SQLite:** Each Durable Object instance has its own embedded SQLite database (Cloudflare's D1-on-DO feature). This provides ACID transactions for session state without an external database dependency. Schema includes tables for messages, tool invocations, file references, and session metadata.

The SessionAgent DO communicates with the Modal Sandbox via authenticated WebSocket connections. When a user sends a prompt, the flow is:

```
Frontend → REST API → SessionAgent DO → WebSocket → Modal Sandbox → Agent
```

When the agent produces output or asks a question:

```
Agent → Modal Sandbox → WebSocket → SessionAgent DO → EventBus DO → Frontend
```

#### A.2.3 EventBus Durable Object

A dedicated Durable Object for real-time event distribution:

- **Real-time broadcasting:** Pushes agent events (new messages, status changes, screenshots, file updates) to all connected frontend clients observing a given session.
- **User-tagged connections:** Each WebSocket connection is tagged with the user's identity, enabling:
  - Multiplayer: multiple users can observe the same agent session simultaneously.
  - Filtered broadcasts: events can be targeted to specific users (e.g., only the session owner sees certain administrative events).
- **Notifications:** Generates push notifications for session completion, errors, or agent questions that require human input.

The EventBus pattern decouples event production (from the sandbox) from event consumption (by the frontend), allowing the frontend to disconnect and reconnect without missing events. Missed events are replayed from the SessionAgent DO's durable state.

#### A.2.4 D1 Database

A Cloudflare D1 (SQLite-based) database for global platform state that spans across sessions:

- **Users & Auth:** User accounts, authentication tokens, team memberships, permissions.
- **Integrations:** Connected external services (GitHub App installations, Slack workspaces, Linear teams) with their credentials and configuration.
- **Agent memories:** Cross-session knowledge that the agent accumulates over time — coding patterns, project context, user preferences. This is the equivalent of OpenClaw's long-term memory store.

D1 is read-heavy and eventually consistent, which is appropriate for this data. Session-critical state lives in the Durable Objects (strongly consistent).

#### A.2.5 Sandbox Spawn Trigger

When a new session is created or an existing session needs a fresh sandbox, the Cloudflare Worker issues an `eTrigger: spawn sandbox` call to the Modal Backend. This trigger includes:

- Session ID (for routing the sandbox back to the correct Durable Object)
- Repository URL and branch
- Image template to use (e.g., webapp, mobile, backend)
- Environment configuration (environment variables, secrets references)
- Agent configuration (model selection, system prompt overrides)

---

### A.3 Modal Backend — Sandbox Orchestration

The Modal Backend is a Python service that manages the lifecycle of sandbox environments on Modal's infrastructure:

#### A.3.1 Session Manager (`session.py`)

Handles session-level resources that exist outside the sandbox itself:

- **Screenshots:** Captures and stores VNC screenshots from active sandboxes for visual verification. These are surfaced to the user in the frontend's session view.
- **File uploads:** Manages file transfer between the user's browser and the sandbox filesystem. Files are staged in the Session Manager before being synced into the sandbox's Runner Volume.
- **Assets:** Static assets associated with a session (generated images, exported documents, build artifacts) that need to persist independently of the sandbox lifecycle.

#### A.3.2 Sandbox Manager (`sandboxes.py`)

The core orchestration logic for Modal Sandbox instances:

- **Modal Queue / Prompt queue:** Inbound prompts from the Cloudflare Worker are queued here before being dispatched to the appropriate sandbox. This decouples the control plane's request rate from the sandbox's processing rate and provides backpressure.
- **Sandbox lifecycle:** Handles create, start, stop, snapshot, and destroy operations on Modal Sandboxes. Implements retry logic, health checking, and graceful shutdown.
- **Scaling decisions:** Determines when to create new sandboxes (on session creation) and when to tear them down (on session completion or idle timeout).

#### A.3.3 Modal Dict

A distributed key-value store (Modal's native primitive) used for:

- **Session locks:** Prevents concurrent modifications to the same session. When a sandbox is processing a prompt, the session lock prevents duplicate dispatches.
- **Image store:** Caches metadata about available sandbox images (templates, versions, build status) to avoid redundant image builds.

#### A.3.4 Image Definition System

A modular, composable system for building sandbox images. Each image is defined as a Python file that layers capabilities onto a base:

- **`base.py`** — The foundation image containing:
  - Common development tools (git, curl, build-essential, etc.)
  - `code-server` — VS Code in the browser, providing the IDE experience inside the sandbox
  - `ttyd` — Terminal emulation over HTTP/WebSocket
  - Claude CLI (or equivalent agent CLI) for direct agent interaction from within the sandbox

- **`webapp.py`** — Web application development layer:
  - Dev server configuration (Next.js, Vite, webpack dev server, etc.)
  - Deploy scripts for preview environments
  - Browser testing tools

- **`core.py`** — Infrastructure services layer:
  - PostgreSQL (for applications that need a database during development)
  - Redis (for caching/queuing in the dev environment)
  - Temporal (for workflow orchestration testing)
  - RabbitMQ (for message queue testing)

- **`android.py`, `ios.py`, `elixir.py`, etc.** — Platform-specific layers:
  - Android SDK and emulator
  - iOS simulator (requires macOS host — noted as a constraint)
  - Language-specific toolchains (Elixir/Erlang OTP, Rust toolchain, Go, etc.)

Images are built incrementally: `base.py` is the foundation, and specialized layers are composed on top using Modal's image layering system (analogous to Docker multi-stage builds but with Modal's snapshot/caching for faster iteration). This means a webapp sandbox doesn't include the Android SDK, and vice versa — keeping image sizes manageable and startup times fast.

**Relevance to OpenClaw SaaS:** This image templating pattern maps directly to OpenClaw skill-specific sandbox templates. Different tenants may need different base capabilities (a tenant using browser automation skills needs Chromium; a tenant focused on calendar/email skills doesn't). Composable image layers allow right-sized sandboxes per tenant profile.

---

### A.4 Modal Sandbox — Execution Environment

The sandbox is a Modal compute instance (gVisor-isolated container with near-instant startup) containing the full development environment. It is composed of several subsystems:

#### A.4.1 VNC Subsystem (Virtual Display)

A complete graphical desktop environment running inside the sandbox, used for visual verification of frontend changes:

- **Xvfb (X Virtual Framebuffer):** Creates a virtual display (no physical monitor required). This is the root of the graphical stack — all GUI applications render to this virtual framebuffer.
- **Fluxbox Window Manager:** A lightweight X11 window manager that provides basic window management (title bars, resize, minimize) for GUI applications running in the sandbox.
- **Chromium + DevTools:** A full Chromium browser instance running on the virtual display. The agent uses this to:
  - Navigate to the dev server URL to visually verify frontend changes
  - Open DevTools to inspect console errors, network requests, and DOM structure
  - Take screenshots for visual comparison
- **x11vnc:** Exposes the Xvfb framebuffer as a VNC server, allowing remote clients to view and interact with the virtual desktop.
- **websockify:** Translates the VNC protocol (RFB over TCP) to WebSocket, enabling the frontend's VNC iFrame (noVNC) to connect directly from the browser.
- **Dev Server (repo-specific):** The application's development server (e.g., `next dev`, `vite dev`) running on the virtual display, serving the application under development at a local URL.
- **Dev URL:** The local URL where the dev server is accessible. The Chromium browser navigates to this URL for visual verification.

**Data flow for visual verification:**
```
Agent decides to verify → captures screenshot via VNC API → 
screenshot stored by Session Manager → surfaced in frontend Session UI
```

Or for live observation by the user:
```
x11vnc → websockify (TCP→WS) → Modal Tunnel (authenticated) → 
Frontend VNC iFrame (noVNC renderer)
```

#### A.4.2 VS Code Subsystem

A full VS Code editing environment inside the sandbox:

- **code-server:** The open-source VS Code server (cdr/code-server) running inside the sandbox. Provides the full VS Code experience (extensions, terminal, source control) accessible via HTTP.
- **Auth Proxy:** An authentication proxy that sits in front of code-server, validating JWT tokens from the frontend before granting access. Prevents unauthorized access to the IDE even if the sandbox's network is reachable.
- **Read/Write** to repository code: code-server has full read/write access to the repository code mounted in the Runner Volume.
- **`@opencode-ui/web`:** A custom VS Code extension or web UI component that integrates the OpenCode agent interface directly into the code-server environment. This provides an in-IDE chat interface for interacting with the agent while viewing code.

#### A.4.3 OpenCode Agent Runtime

The AI coding agent running inside the sandbox:

- **`opencode serve`:** The OpenCode agent process, running in server mode. It exposes:
  - An HTTP API for receiving prompts and returning responses
  - WebSocket events for streaming real-time agent output (thinking, tool calls, code edits)
- **Runner Volume:** A persistent volume containing the cloned repository code and any generated artifacts. Shared between code-server (for IDE access) and the agent (for code reading/writing).
- **Repository Code:** The git-cloned repository that the agent operates on. Cloned during sandbox provisioning or restored from a snapshot.

**Agent interaction flow:**
```
Prompt arrives (from Cloudflare DO via WebSocket) →
OpenCode agent processes prompt →
Agent reads code from Runner Volume →
Agent writes/modifies code in Runner Volume →
Agent runs tests (via shell in sandbox) →
Agent verifies visually (via Chromium on VNC) →
Agent creates PR (via GitHub API) →
Response sent back to SessionAgent DO
```

#### A.4.4 Event Stream

A structured event bus within the sandbox that captures all agent activity:

- **Messages:** Agent output messages (text, code blocks, reasoning)
- **Session events:** Session start/stop, model switches, error states
- **Question events:** When the agent needs user input (clarification, approval for destructive actions)

Events flow from the sandbox → Cloudflare Worker via WebSocket → SessionAgent DO (for persistence) → EventBus DO (for real-time broadcast to frontend).

---

### A.5 Runner — Agent Orchestration

The Runner is the process that bootstraps and manages the agent within the sandbox:

- **Runner CLI (`bin.ts`):** The entry point for the agent process. Accepts configuration (model, system prompt, tools, repository) and starts the OpenCode agent in server mode.
- **Config:** Runtime configuration passed from the Sandbox Manager, including:
  - Model selection and API keys
  - Repository details (URL, branch, commit)
  - Tool permissions and restrictions
  - Session context (previous messages, memory)

#### A.5.1 Proxy Factory

Handles authentication and protocol translation at the sandbox boundary:

- **JWT Auth:** Validates JWT tokens on all incoming connections to the sandbox. Tokens are issued by the Cloudflare Worker and scoped to a specific session.
- **WS Proxying:** Proxies WebSocket connections from the Cloudflare Worker to the OpenCode agent's WebSocket interface. Handles reconnection, buffering, and protocol translation.

#### A.5.2 AgentClient

The WebSocket client that connects the sandbox to the Cloudflare Durable Object:

- Establishes and maintains a persistent WebSocket connection from the sandbox to the SessionAgent DO.
- Serializes agent events (messages, tool calls, file changes) into the DO's message format.
- Receives inbound prompts and question responses from the DO and routes them to the OpenCode agent.

#### A.5.3 Prompt Handler (`prompt.ts`)

Manages the prompt lifecycle:

- Receives prompts from the AgentClient
- Formats them with session context (previous messages, file state, tool results)
- Dispatches to the OpenCode agent
- Handles streaming responses and emits events to the Event Stream

---

### A.6 Frontend — React Application

The user-facing web application:

#### A.6.1 Embedded Environments (iFrames)

The frontend embeds three sandbox subsystems via authenticated iFrames:

- **VNC iFrame:** Renders the sandbox's virtual desktop using noVNC (JavaScript VNC client). Connected via `websockify → Modal Tunnel → JWT Auth → Frontend`. Users can observe the agent navigating websites, running the dev server, and visually verifying changes in real time.
- **VSCode iFrame:** Embeds code-server, providing a full VS Code editing experience. Connected via `code-server → Auth Proxy → Modal Tunnel → JWT Auth → Frontend`. Users can browse, edit, and review code alongside the agent.
- **Terminal iFrame:** Provides direct terminal access to the sandbox shell. Connected via `ttyd → Modal Tunnel → JWT Auth → Frontend`. Users can run commands, inspect logs, and interact with the sandbox environment directly.

All three iFrames connect through **Modal Tunnels** — authenticated, encrypted tunnels that expose sandbox-internal services to the frontend without opening public ports. Each tunnel requires JWT authentication, ensuring only authorized users can access a sandbox's internal services.

#### A.6.2 Session Management

- **Session UI (`SessionListPage`):** Lists all sessions with their status (active, completed, errored), last activity, and quick actions (resume, delete, view PR).
- **React Query Cache:** Client-side state management with optimistic updates. Cache is updated both from HTTP RPC responses and real-time WebSocket events.
- **Cache updates:** When the EventBus DO pushes a new event (e.g., agent produced a new message), the React Query cache is updated in real time, providing an instant UI update without polling.

#### A.6.3 Communication Channels

The frontend uses three communication patterns:

- **HTTP RPC:** For request/response operations (create session, submit prompt, fetch session list). Routes through the Cloudflare Worker's REST API.
- **WebSocket Subscriptions:** For real-time event streaming. Connected to the EventBus DO for live updates on agent activity, new messages, and status changes.
- **Prompt submission:** A dedicated flow for submitting user prompts. Goes through the REST API → SessionAgent DO → WebSocket → sandbox, with the response streamed back via the EventBus WebSocket.

---

### A.7 External Services

The sandbox has outbound access to external APIs, mediated by the sandbox's network configuration:

- **GitHub API:** For repository operations — clone, push, create PRs, read issues. Authenticated via a GitHub App installation scoped to the organization's repositories. (Note: in the open-source version, all users share the same GitHub App credentials — a key limitation for multi-tenant deployments.)
- **Slack API:** For notifications — posting session updates, PR links, and agent questions to Slack channels. Also the primary trigger interface (users tag the agent in Slack to start tasks).
- **Linear API:** For issue tracking integration — reading issue details, updating status, linking PRs to issues.
- **Anthropic (Claude) / OpenAI (GPT) / Google (Gemini):** LLM providers for the agent's reasoning. The agent runtime supports multiple providers with model selection at session creation time.

---

### A.8 Web Layer

A web-facing layer that handles public HTTP traffic:

- **Auth Proxy:** Authenticates incoming HTTP requests before routing to internal services. Validates session tokens, enforces CORS policies, and handles OAuth flows.
- **httpd:** A lightweight HTTP server that serves the frontend static assets and proxies API requests to the Cloudflare Worker.

---

### A.9 Key Architectural Patterns and Implications for OpenClaw SaaS

#### A.9.1 Durable Objects as Session State Machines

The most distinctive pattern in this architecture. Rather than using a centralized database for session state, each session gets its own Durable Object with embedded SQLite. This provides:

- **Strong consistency per session:** All operations on a single session are serialized through its DO, eliminating race conditions without distributed locks.
- **Co-located state and logic:** The DO contains both the session data and the routing/lifecycle logic, reducing network hops.
- **Automatic scaling:** Cloudflare manages DO placement and migration — no capacity planning for the session layer.
- **Built-in WebSocket support:** DOs natively support WebSocket connections, making real-time event streaming trivial.

**For OpenClaw SaaS:** This pattern could replace our Redis-backed Session Router with a more robust solution. Each tenant-session could be a Durable Object, with the DO maintaining the WebSocket connection to the sandbox and broadcasting events to connected dashboard clients. The tradeoff is Cloudflare platform dependency.

#### A.9.2 Separation of Event Persistence and Event Broadcasting

The architecture uses two distinct Durable Objects: SessionAgent (persistence) and EventBus (broadcasting). This separation means:

- The SessionAgent DO can be optimized for write durability and consistency
- The EventBus DO can be optimized for fan-out and low-latency delivery
- A crash in the EventBus doesn't lose session state
- Frontend clients can reconnect to the EventBus and replay missed events from the SessionAgent

**For OpenClaw SaaS:** Our spec should add a dedicated real-time event system (analogous to the EventBus DO) for the tenant dashboard. Currently, the spec covers observability via OpenTelemetry but doesn't describe how real-time agent activity is pushed to the web dashboard.

#### A.9.3 Composable Image Templates

The modular image system (base.py + webapp.py + core.py + platform-specific layers) enables right-sized sandboxes without maintaining dozens of monolithic images. Key properties:

- **Layered composition:** Each layer adds capabilities without duplicating the base
- **Cached snapshots:** Modal caches intermediate layers, so adding a new platform-specific layer doesn't rebuild from scratch
- **Per-session image selection:** Different sessions can use different image templates based on the repository's requirements

**For OpenClaw SaaS:** OpenClaw tenants have diverse needs (some need browser automation, some need email/calendar integration, some need code execution). A composable template system would allow:
- `base` — OpenClaw Gateway + core agent runtime
- `browser` — adds Chromium for browser automation skills
- `developer` — adds code-server, language runtimes for coding-oriented agents
- `productivity` — adds calendar/email integration libraries
- `custom` — tenant-provided Dockerfile layered on top of any base

#### A.9.4 Multi-Boundary JWT Authentication

Authentication is enforced at four distinct boundaries:

1. **Cloudflare Worker → REST API:** User session token validates at the API gateway
2. **Frontend → Modal Tunnel:** JWT token authenticates iFrame connections to sandbox services
3. **Proxy Factory → Sandbox services:** JWT validates all inbound connections to the sandbox
4. **Auth Proxy → code-server:** Additional auth layer specifically for the IDE

This defense-in-depth approach means that even if one auth boundary is compromised, the others still protect internal services.

**For OpenClaw SaaS:** Our spec should be more explicit about internal authentication between the control plane and data plane. Specifically:
- Edge Gateway → Sandbox: Mutual TLS + per-tenant JWT
- Dashboard → Sandbox (for real-time observation): Short-lived JWTs scoped to specific sessions
- Control Plane → Modal/Firecracker API: Service-to-service authentication with rotated credentials

#### A.9.5 Limitations of the Open-Source Implementation

The open-source Open-Inspect implementation has documented limitations that our spec must address:

- **Single-tenant only:** All users share the same GitHub App credentials, meaning any user can access any repo the App is installed on. Multi-tenant deployments require per-tenant credential isolation — which our Secrets Vault design provides.
- **No skill sandboxing:** The agent has full access to the sandbox filesystem and network. There is no nested sandboxing for untrusted tool execution — which our Tier 1/2/3 nested sandboxing model addresses.
- **Cloudflare dependency:** The Durable Objects pattern is Cloudflare-specific. A cloud-agnostic implementation would need an equivalent stateful actor framework (e.g., Microsoft Orleans, Akka, or custom implementation on top of a distributed database).
- **No hibernate/resume:** Sandboxes are active for the duration of a session and destroyed on completion. There is no snapshot/restore for cost optimization — which our Lifecycle Orchestrator addresses via Firecracker snapshots.
