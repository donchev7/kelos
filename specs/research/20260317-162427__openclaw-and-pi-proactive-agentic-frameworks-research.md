# OpenClaw and Pi Research: Proactive, Always-On Agentic Frameworks

Date: 2026-03-17  
Research window: March 17, 2026  
Scope: OpenClaw and Pi (functionality, architecture, implementation patterns, and OpenClaw variants/evolution)

## 1. Executive Summary

OpenClaw and Pi are tightly related but architecturally different:

1. OpenClaw is a full-stack agent platform with a distributed control-plane design centered on a Gateway, queueing, streaming, integrations, and node execution. It is explicitly built for continuous, unattended operation and proactive interactions (follow-ups, reminders, wake-ups, schedule-driven behavior).
2. Pi is a lightweight agent engine/tooling runtime (CLI + SDK + RPC) designed for embedding and composition. It is highly programmable and can support always-on operation, but typically needs external orchestration (process supervision, queue workers, integration adapters) to become a full proactive platform.

Bottom line:

1. OpenClaw provides more out-of-the-box always-on/proactive platform primitives.
2. Pi provides better low-level composability and library-style embedding.
3. A common implementation pattern in the ecosystem is to use Pi as the execution core and wrap it with OpenClaw-like control plane, integration gateway, and persistent scheduling.

## 2. Research Method and Source Quality

This doc prioritizes primary sources:

1. Official project repositories.
2. Official project documentation.
3. First-party project website content.

No conclusions are based solely on third-party summaries.

## 3. OpenClaw: Functional Profile

## 3.1 Product and operational intent

From the official README and docs, OpenClaw positions itself as:

1. Personal AI assistant across desktop/web/mobile interfaces.
2. Local-first, but network-capable and distributed.
3. Unattended-capable with explicit mechanisms for asynchronous execution.

Feature-level signals for always-on/proactive operation include:

1. Session follow-up APIs.
2. Session steering commands.
3. Wake-up/reminder style interaction semantics.
4. Node/offload concepts for distributed tool execution.

## 3.2 Major architectural components

OpenClaw docs describe a layered architecture:

1. Gateway:
   1. Central coordination service.
   2. Authentication and authorization boundary for components.
   3. Message routing and queue management.
   4. Supports WebSocket and local Unix socket access modes.
2. Clients:
   1. Terminal, web, and mobile-facing clients.
   2. Connect to Gateway rather than owning all state.
3. Agent engine:
   1. Runs reasoning/tool loops.
   2. Maintains session context and loop state transitions.
4. Nodes:
   1. Distributed execution workers.
   2. Can run tools remotely while session control remains centralized.
5. Integration service:
   1. Handles external provider APIs (GitHub, Slack, Gmail, etc.).
   2. Decouples provider credentials and interaction logic from agent core loops.
6. Channel layer:
   1. Delivery/interaction surfaces abstracted from core logic.
   2. Enables multi-channel operation without rewriting reasoning core.

## 3.3 Agent loop and state machine model

OpenClaw documents a formal loop with explicit state transitions. Important implementation characteristics:

1. Stepwise finite-state progression (initialization, prompt build, model call, tool handling, continuation/end conditions).
2. Separation between:
   1. Loop control state.
   2. Message context queue.
   3. Persisted session artifacts.
3. Handling of model stop reasons such as:
   1. tool-use requests
   2. context window constraints
   3. turn completion boundaries

This is materially important for always-on reliability because deterministic state transitions make resumes/retries safer after crashes or network partitions.

## 3.4 Integration architecture patterns

The integration architecture docs show several patterns commonly seen in robust agent platforms:

1. Provider traffic centralization:
   1. OpenClaw Pi clients and sessions route through Integration Service.
   2. Credentials and policy are managed at service boundaries.
2. Profile-driven model/provider routing:
   1. Auth profiles and task profiles can be selected dynamically.
   2. Supports routing policies and failover behavior.
3. Queueing and backpressure:
   1. Requests can be queued with rate-limiting behavior.
   2. Reduces provider-throttle cascades during bursts.
4. Stream-resume support:
   1. Event streams can be resumed after interruption.
   2. Critical for long-running unattended sessions.
5. Observability hooks:
   1. Tracing and metrics support across integration boundaries.
   2. Enables debugging cross-service failures.

## 3.5 Gateway configuration and operational controls

Gateway docs expose operational controls that map directly to production-readiness:

1. Server/listener configuration:
   1. host/port/path.
   2. optional Unix socket mode.
2. Authentication controls:
   1. keys/roles for service-to-service access.
3. Discovery/network options:
   1. mDNS and local-discovery settings.
4. Performance profile:
   1. memory/perf presets.
   2. interval/timeouts.
5. Tracing and logging:
   1. configurable diagnostics for distributed operations.

## 4. OpenClaw Variants and Evolution

You asked specifically for all variants. Based on primary evidence, these are the variants/evolution tracks that can be confidently identified.

## 4.1 Naming lineage variants (confirmed)

Two historical repository names redirect to `openclaw/openclaw`:

1. `clawdbot/clawdbot` -> redirects to `openclaw/openclaw`.
2. `moltbot/moltbot` -> redirects to `openclaw/openclaw`.

Interpretation:

1. These appear to represent prior naming/branding stages in the same project lineage.
2. The currently active canonical project is `openclaw/openclaw`.

## 4.2 Release-channel variants (confirmed)

OpenClaw README exposes distinct install/update channels:

1. stable
2. beta
3. dev

These are effectively product variants in operational risk and feature velocity:

1. stable: lowest change volatility.
2. beta: pre-release hardening.
3. dev: newest features, highest churn.

## 4.3 Deployment topology variants (confirmed)

OpenClaw supports multiple runtime topologies:

1. local single-host mode.
2. headless/hosted server mode.
3. distributed mode with remote nodes and centralized Gateway.
4. browser and containerized sandbox-related deployment artifacts.

These topology variants materially affect reliability, cost, and security posture.

## 4.4 Interface/channel variants (confirmed)

From official materials:

1. terminal-centric client.
2. web and mobile clients.
3. integration channels (Slack/GitHub/email-class patterns via integration architecture).

## 4.5 What could not be fully confirmed from primary sources

1. Comprehensive mapping of every community fork as a "variant" is not tractable from first-party docs (the fork graph is large and continuously changing).
2. Third names sometimes discussed in community chatter were not confirmed via first-party canonical docs/repo redirects in this research pass.

## 5. Pi: Functional Profile

## 5.1 Core intent

Pi is presented as a coding-agent runtime focused on:

1. minimal core.
2. composability via SDK/RPC.
3. ergonomic CLI for direct operator use.

The design favors embedding and orchestration by host systems.

## 5.2 Operating modes

Pi README and docs show four operational modes:

1. interactive CLI mode.
2. one-shot print mode.
3. RPC server mode.
4. SDK/library mode.

This mode split is a strong implementation signal:

1. CLI for humans.
2. RPC for process boundary decoupling.
3. SDK for in-process composition.

## 5.3 Session and conversation model

Pi supports:

1. session IDs and parent session IDs (branching trees).
2. message append/query APIs.
3. queue-oriented task dispatch and steering.
4. completion waiting with configurable behavior.

This session tree + queue pattern is one of the key implementation primitives needed for proactive agents.

## 5.4 RPC architecture

Pi RPC docs show:

1. explicit RPC server startup (`--rpc start` with configurable host/port).
2. optional auth key requirement (`PI_RPC_AUTH_KEY`).
3. method-oriented APIs for session lifecycle:
   1. initializeSession
   2. appendMessage
   3. querySession
   4. interruptSession

This separation makes Pi suitable for:

1. durable worker architectures.
2. supervisor-restarted processes.
3. polyglot orchestration where host app is not TypeScript.

## 5.5 SDK and structured outputs

Pi SDK patterns include:

1. agent factory construction (`createAgent()` style).
2. typed/structured output constraints (Zod-backed schemas).
3. stream/event listeners for tool and assistant events.
4. steering API for live course-correction.

This allows implementation of:

1. deterministic machine-readable outputs.
2. human-in-the-loop interventions without restarting sessions.

## 5.6 Settings: model routing and resilience

Pi settings docs expose advanced model policy controls:

1. model profiles with multiple model groups.
2. task-profile overrides.
3. retries with timeout/backoff/fallback profile.
4. load balancing strategy options (including weighted random and failover behavior).

This is an important technical pattern for production-grade agent systems where provider/model reliability is variable.

## 6. OpenClaw vs Pi: Technical Implementation Pattern Comparison

## 6.1 Platform shape

1. OpenClaw:
   1. opinionated platform architecture.
   2. includes control-plane and integration-plane patterns.
2. Pi:
   1. runtime/harness architecture.
   2. expects orchestration to be built around it.

## 6.2 Proactive/always-on readiness

1. OpenClaw:
   1. native support for unattended operation patterns.
   2. explicit gateway/session/integration structures that fit long-lived tasks.
2. Pi:
   1. can be always-on when run in RPC/SDK worker systems.
   2. requires additional scheduling, persistence, and integration layers.

## 6.3 Control plane patterns

1. OpenClaw:
   1. central Gateway + distributed nodes pattern.
2. Pi:
   1. embed per-worker or host behind a custom Gateway/service.

## 6.4 Integration and channel pattern

1. OpenClaw:
   1. explicit integration service abstraction.
   2. good for centralized credential governance.
2. Pi:
   1. integration adapters are generally application-defined.
   2. high flexibility, more engineering lift.

## 6.5 Operational complexity

1. OpenClaw:
   1. faster path to platform-level behavior.
   2. more moving parts to understand.
2. Pi:
   1. simpler core runtime.
   2. most platform concerns are your responsibility.

## 7. Implementation Pattern Catalog (Cross-Framework)

These are the concrete patterns that repeat across robust proactive agent frameworks:

## 7.1 Control plane + execution plane split

Pattern:

1. Keep session orchestration in one service.
2. Execute tools/workloads in isolated workers/nodes.

Benefits:

1. easier scaling.
2. safer sandboxing.
3. cleaner failure boundaries.

## 7.2 Queue-first session mutation

Pattern:

1. append messages/commands to a queue.
2. process with lease/claim semantics.
3. support steering/interruption as first-class queue items.

Benefits:

1. idempotency control.
2. resilience under retries.
3. smoother multi-client concurrency.

## 7.3 State-machine agent loops

Pattern:

1. finite loop states with explicit transitions.
2. persist state checkpoints between transitions.

Benefits:

1. deterministic recovery.
2. debuggable failure modes.

## 7.4 Integration gatewaying

Pattern:

1. route external API interactions through a dedicated integration layer.
2. attach model/provider policies there.

Benefits:

1. centralized secret management.
2. consistent throttling/retry behavior.
3. cleaner observability.

## 7.5 Model routing policies

Pattern:

1. profile-based model selection.
2. fallback chain and per-attempt timeouts.
3. optional weighted balancing.

Benefits:

1. better reliability under provider outages.
2. controllable latency/cost tradeoffs.

## 7.6 Resumable event streams

Pattern:

1. stream tool/assistant events with resume tokens.
2. persist enough metadata to continue after disconnects.

Benefits:

1. robust long-running UX.
2. better support for mobile/intermittent clients.

## 7.7 Security boundary hardening

Pattern:

1. service auth keys/roles between components.
2. strict credential compartmentalization.
3. per-integration policy controls.

Benefits:

1. reduced blast radius.
2. easier compliance/audit story.

## 8. Practical Architecture Guidance for Building Similar Systems

If the goal is a proactive, always-on agent platform with Slack/GitHub-style integrations:

Necessary baseline:

1. durable queue for session mutations.
2. explicit agent-loop state machine.
3. integration service boundary for external APIs.
4. retry/backoff + model fallback policy layer.
5. resumable event streaming.
6. process supervision and health checks for always-on workers.

Optional but high value:

1. distributed node execution pool.
2. weighted model routing.
3. semantic memory/index service.
4. multi-channel federation with normalized event model.
5. integrated tracing across control and execution planes.

## 9. Risks and Caveats

1. "All variants" in open-source ecosystems changes continuously. This doc only marks variants confirmed through first-party evidence captured on March 17, 2026.
2. Star counts/fork counts and release cadence are time-variant.
3. Some deep internal behaviors require code-level audits beyond docs/README to fully validate runtime guarantees.

## 10. Source Index

Primary sources used:

1. OpenClaw repository README:
   1. https://github.com/openclaw/openclaw/blob/main/README.md
2. OpenClaw architecture docs:
   1. https://docs.openclaw.ai/concepts/architecture
3. OpenClaw agent-loop docs:
   1. https://docs.openclaw.ai/concepts/agent-loop
4. OpenClaw integration architecture docs:
   1. https://docs.openclaw.ai/concepts/integration-architecture
5. OpenClaw gateway configuration docs:
   1. https://docs.openclaw.ai/gateway/configuration
6. Historical OpenClaw lineage redirects:
   1. https://github.com/clawdbot/clawdbot
   2. https://github.com/moltbot/moltbot
7. Pi official site:
   1. https://pi.dev/
8. Pi repository README:
   1. https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/README.md
9. Pi RPC docs:
   1. https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/rpc.md
10. Pi SDK docs:
   1. https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/sdk.md
11. Pi settings docs:
   1. https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/settings.md

## 11. Appendix: Additional OpenClaw Variants (Deep Technical Review)

This appendix evaluates the additional variants you listed using primary sources only.

Verification levels used:

1. Confirmed: direct first-party code/docs evidence exists.
2. Partially confirmed: product claims exist, but core implementation is not publicly auditable.
3. Not confirmed: specific claim could not be verified in first-party sources.

## 11.1 Nanobot (HKUDS/nanobot)

Verification status: Confirmed

Primary-source profile:

1. Language/runtime:
   1. Python package (`nanobot-ai`) with CLI entrypoints.
2. Positioning:
   1. Explicitly "inspired by OpenClaw".
   2. Claims "99% fewer lines of code" than OpenClaw.
3. Always-on/proactive orientation:
   1. `gateway` mode for channel-connected long-running operation.
   2. Scheduled task and automation workflows called out in docs/README.
4. Channel surface:
   1. Broad channel support (Telegram, Discord, WhatsApp, Feishu, Slack, Email, QQ, DingTalk, WeCom, Matrix, etc.) described in README.
5. Model/provider layer:
   1. OpenAI-compatible and multi-provider support.
   2. OAuth-based provider flows (for some providers) are documented.

Technical implementation patterns:

1. Plugin architecture for channels:
   1. Discovery via Python entry points (`nanobot.channels`).
   2. Channel contract centered on a `BaseChannel` abstraction (`start`, `stop`, `send`, `_handle_message`).
   3. Config-driven channel enablement (`channels.{name}.enabled`).
2. Eventing/message dispatch:
   1. Channel adapters push inbound messages through a common handler path.
   2. Outbound response contract supports metadata for progress/stream chunks.
3. Security control pattern:
   1. Per-channel sender allowlisting (`allowFrom`) enforced at the channel base layer.
4. Extensibility model:
   1. New channels are package-level plugins, not mandatory core merges.
   2. This keeps the core runtime small while allowing ecosystem growth.

Operational and architectural takeaways:

1. Nanobot is best viewed as a compact Python control-plane/runtime fusion with broad channel adapters and plugin discovery.
2. Compared with OpenClaw, it emphasizes operational simplicity over deeply separated distributed components.

## 11.2 PicoClaw (sipeed/picoclaw)

Verification status: Confirmed

Primary-source profile:

1. Language/runtime:
   1. Go-based, binary-first assistant.
2. Positioning:
   1. Lightweight and low-cost hardware oriented.
   2. Readme claims include low RAM and fast cold start characteristics.
3. Maturity signal:
   1. README includes explicit early-development and security caution.

Technical implementation patterns:

1. Tool subsystem modularization:
   1. `tools` config namespaces (`web`, `exec`, `mcp`, `cron`, `skills`).
   2. Safety defaults in execution tooling (`exec`) via deny-pattern blocking for destructive command classes.
2. MCP scale-management pattern:
   1. Discovery mode to avoid loading hundreds of MCP tools into model context at once.
   2. Search-based on-demand unlock with TTL window for discovered tools.
3. Provider/auth plugin pattern:
   1. Provider interface implementation in Go.
   2. Optional OAuth auth handlers integrated in auth command layer.
4. OAuth deep pattern (Antigravity docs):
   1. OAuth 2.0 PKCE flow.
   2. Dual-mode auth UX (automatic localhost callback and manual/headless copy-paste).
   3. Token and profile storage in local auth file (`~/.picoclaw/auth.json`).
   4. Provider-specific schema adaptation and quota/usage handling.

Always-on/proactive implications:

1. Go static runtime + explicit gateway mode favor low-footprint persistent agents.
2. Discovery-based tool loading is a direct tactic for keeping long-running sessions cost-efficient.

## 11.3 ZeroClaw (zeroclaw-labs/zeroclaw)

Verification status: Confirmed (with important caveats)

Primary-source profile:

1. Language/runtime:
   1. Rust-first runtime framing.
2. Positioning:
   1. Trait-driven, swappable providers/channels/tools.
   2. Lean-memory and portable-deployment focus in README messaging.
3. Documentation surface:
   1. Extensive operator-oriented docs, runbooks, troubleshooting, and security sections.

Technical implementation patterns:

1. Operations-first governance:
   1. Clear runtime mode split (`daemon`, `gateway`, service install/start/status).
   2. Operator checklists and incident triage runbooks.
2. Security program framing:
   1. Dedicated security docs taxonomy.
   2. Fraud-prevention/offical-channel controls in docs.
3. Runtime hardening roadmap:
   1. Sandboxing and resource-limit docs exist, but are explicitly marked proposal/roadmap.
   2. Current-vs-proposed behavior is clearly delineated in docs.

Critical caveat:

1. Some strong security/runtime claims around OS-level sandboxing are roadmap-oriented rather than confirmed as current default behavior in public docs.
2. The specific user-supplied claim "<10ms startup, ~3.4MB binary" was not directly confirmed from primary sources used in this pass.

Always-on/proactive implications:

1. ZeroClaw shows strong operational discipline for long-running service management.
2. It appears to be transitioning from app-layer protections toward deeper OS/runtime isolation.

## 11.4 NanoClaw (qwibitai/nanoclaw)

Verification status: Confirmed

Primary-source profile:

1. Language/runtime:
   1. Node.js/TypeScript host orchestration.
   2. Claude Agent SDK for agent execution.
2. Core proposition:
   1. Container-isolated execution per agent run.
   2. Security-first framing against monolithic single-process alternatives.

Technical implementation patterns:

1. Split host and execution zones:
   1. Host orchestrator handles channels, SQLite, scheduling, routing, and IPC.
   2. Agent execution happens in isolated containerized environments.
2. Channel composition:
   1. No baked-in channels in core by default.
   2. Channels added as Claude skills and self-register via registry/barrel imports.
3. Persistent state model:
   1. SQLite-backed message/session/task metadata.
   2. Filesystem-backed memory hierarchy via `CLAUDE.md` at global and group scopes.
4. Scheduling as MCP-native control plane:
   1. `schedule_task`, `list_tasks`, `pause/resume/cancel`, and `send_message` tools exposed through built-in MCP surface.
5. Security architecture:
   1. Container isolation as primary boundary.
   2. External mount allowlist not mounted into containers.
   3. Credential proxy pattern so real API credentials do not enter container env/filesystem.
   4. Session isolation per group.
6. Long-running behavior:
   1. Polling message loop, scheduler loop, IPC watcher.
   2. Launchd service support for always-on operation.

Always-on/proactive implications:

1. NanoClaw is a clear "single orchestrator + sandboxed execution workers" design.
2. It provides strong implementation examples for safe unattended automation.

## 11.5 TrustClaw (trustclaw.app / Composio)

Verification status: Partially confirmed

Primary-source profile:

1. Public product positioning (website):
   1. 24/7 assistant messaging.
   2. 1000+ tools via OAuth.
   3. Sandboxed execution.
   4. Scheduling/autopilot framing.
2. Platform lineage:
   1. Presented as Composio-backed/Composio-built service.

Technical implementation patterns (inferred from first-party product docs/pages):

1. Managed integration surface:
   1. Centralized connector/tool catalog with delegated auth.
2. OAuth-first execution model:
   1. Emphasis on managed OAuth flows over direct API-key handling.
3. Managed sandbox model:
   1. Remote isolated execution environments are productized.
4. JIT tool invocation pattern:
   1. "Search tools by intent, execute with scoped auth" is a repeated Composio-level pattern.

Evidence limitations:

1. Publicly accessible open-source code for TrustClaw core runtime was not identified in this pass.
2. Therefore, low-level implementation details (queue semantics, state machine internals, actual sandbox boundary implementation, retention model) cannot be independently audited from source code here.

Always-on/proactive implications:

1. Functionally, TrustClaw is framed as a managed always-on proactive operator.
2. Technically, treat it as a closed managed platform unless code-level artifacts are published.

## 11.6 IronClaw (nearai/ironclaw)

Verification status: Confirmed

Primary-source profile:

1. Language/runtime:
   1. Rust-based framework in NEAR AI ecosystem.
2. Security posture:
   1. WASM sandboxing for untrusted tools.
   2. Credential boundary protections and endpoint allowlisting.
   3. Prompt-injection mitigation language in README.
3. Always-on/proactive profile:
   1. Multi-channel operation (REPL/webhooks/WASM channels/web gateway patterns).
   2. Routines (cron, event, webhook) and heartbeat-based background execution.
   3. Parallel job handling and recovery framing.

Technical implementation patterns:

1. Pluggable channel model:
   1. Channel artifacts are WASM binaries + capabilities descriptors.
   2. Telegram setup docs show pairing/allowlist gates and webhook vs polling mode.
2. Provider abstraction and routing:
   1. Default NEAR AI backend with broad provider compatibility (Anthropic/OpenAI/Gemini/Ollama/Bedrock/OpenAI-compatible endpoints).
   2. Setup wizard flow for backend bootstrapping.
3. Persistent memory/data model:
   1. Hybrid search claims and filesystem persistence patterns in README.
   2. Postgres + pgvector prerequisite indicates vector-backed memory/retrieval strategy.

Always-on/proactive implications:

1. IronClaw is architected as a security-forward always-on system, especially via WASM tool boundaries plus background routine mechanisms.

## 11.7 Clawlet (mosaxiv/clawlet)

Verification status: Confirmed

Primary-source profile:

1. Language/runtime:
   1. Go-based, static binary, no CGO runtime requirement.
2. Positioning:
   1. Lightweight personal AI assistant inspired by OpenClaw and Nanobot.
3. Memory profile:
   1. Hybrid semantic memory search with bundled SQLite + sqlite-vec.

Technical implementation patterns:

1. Modular internal package architecture:
   1. `agent`, `tools`, `llm`, `channels`, `bus`, `session`, `memory`, `skills`.
2. Always-on runtime mode:
   1. `gateway` mode designed as long-lived process with in-memory message bus routing.
3. Provider and auth pattern:
   1. Multi-provider support with OpenAI-compatible and local backends.
   2. OAuth flow for OpenAI Codex in CLI.
4. Channel pattern:
   1. Telegram long polling and WhatsApp linked-device flow documented.
5. Safety/test posture:
   1. Repository guidelines explicitly call out tool-safety boundaries and concurrency testing for critical modules.

Claim caveat:

1. The descriptor "more advanced Nanobot" is not a first-party claim verified in sources used here; treat as community phrasing rather than confirmed project positioning.

## 11.8 Cross-Variant Pattern Synthesis

Across these variants, recurring implementation motifs are:

1. Binary/small-runtime emphasis:
   1. Go and Rust variants optimize cold-start and low-memory footprints.
2. Security hardening divergence:
   1. Some projects rely on app-layer allowlists.
   2. Others push toward OS/container/WASM isolation.
3. Tool-surface scaling:
   1. Discovery or lazy-loading patterns are used to keep prompt/context pressure manageable.
4. Always-on realization model:
   1. Long-lived gateway/daemon mode.
   2. Scheduler/routine loops.
   3. Health/ops runbooks and service wrappers.
5. Integration governance split:
   1. Open-source variants often expose credential handling details directly.
   2. Managed platforms (TrustClaw/Composio-style) abstract auth/sandbox as a hosted service layer.

## 11.9 Claim Validation Matrix (for user-provided summaries)

1. Nanobot "~99% smaller than OpenClaw": Confirmed from Nanobot README claim.
2. PicoClaw "Go-based minimalist, edge-focused": Confirmed.
3. ZeroClaw "Rust/high-performance/security": Confirmed in framing; exact "<10ms startup/~3.4MB binary" not confirmed in primary docs reviewed.
4. NanoClaw "isolated Docker containers safer API connectivity": Confirmed.
5. TrustClaw "platform-oriented, OAuth, 1000+ tools": Product-level claims confirmed; implementation internals not auditable from public code in this pass.
6. IronClaw "nearai Rust + WASM sandboxing": Confirmed.
7. Clawlet "Go high-performance Nanobot-like alternative": Confirmed as Go lightweight OpenClaw/Nanobot-inspired; "more advanced Nanobot" wording not confirmed as first-party claim.

## 12. Appendix Sources

Additional primary sources used for the appendix:

1. Nanobot repository README:
   1. https://github.com/HKUDS/nanobot
   2. https://raw.githubusercontent.com/HKUDS/nanobot/main/README.md
2. Nanobot channel plugin guide:
   1. https://raw.githubusercontent.com/HKUDS/nanobot/main/docs/CHANNEL_PLUGIN_GUIDE.md
3. PicoClaw repository README:
   1. https://github.com/sipeed/picoclaw
   2. https://raw.githubusercontent.com/sipeed/picoclaw/main/README.md
4. PicoClaw tools configuration:
   1. https://raw.githubusercontent.com/sipeed/picoclaw/main/docs/tools_configuration.md
5. PicoClaw Antigravity OAuth/provider internals doc:
   1. https://github.com/sipeed/picoclaw/blob/main/docs/ANTIGRAVITY_AUTH.md
6. ZeroClaw repository README:
   1. https://github.com/zeroclaw-labs/zeroclaw
   2. https://raw.githubusercontent.com/zeroclaw-labs/zeroclaw/main/README.md
7. ZeroClaw docs hub and runbook/security proposal docs:
   1. https://github.com/zeroclaw-labs/zeroclaw/tree/main/docs
   2. https://raw.githubusercontent.com/zeroclaw-labs/zeroclaw/main/docs/README.md
   3. https://raw.githubusercontent.com/zeroclaw-labs/zeroclaw/main/docs/operations-runbook.md
   4. https://raw.githubusercontent.com/zeroclaw-labs/zeroclaw/main/docs/sandboxing.md
   5. https://raw.githubusercontent.com/zeroclaw-labs/zeroclaw/main/docs/resource-limits.md
   6. https://raw.githubusercontent.com/zeroclaw-labs/zeroclaw/main/docs/security/README.md
8. NanoClaw repository and specs:
   1. https://github.com/qwibitai/nanoclaw
   2. https://raw.githubusercontent.com/qwibitai/nanoclaw/main/README.md
   3. https://raw.githubusercontent.com/qwibitai/nanoclaw/main/docs/SPEC.md
   4. https://raw.githubusercontent.com/qwibitai/nanoclaw/main/docs/SECURITY.md
9. TrustClaw product page:
   1. https://www.trustclaw.app/
10. Composio platform page (TrustClaw platform context):
   1. https://composio.dev/
11. IronClaw repository README:
   1. https://github.com/nearai/ironclaw
   2. https://raw.githubusercontent.com/nearai/ironclaw/staging/README.md
12. IronClaw provider/channel docs:
   1. https://github.com/nearai/ironclaw/tree/staging/docs
   2. https://raw.githubusercontent.com/nearai/ironclaw/staging/docs/LLM_PROVIDERS.md
   3. https://raw.githubusercontent.com/nearai/ironclaw/staging/docs/TELEGRAM_SETUP.md
13. Clawlet repository:
   1. https://github.com/mosaxiv/clawlet
   2. https://raw.githubusercontent.com/mosaxiv/clawlet/main/README.md
   3. https://raw.githubusercontent.com/mosaxiv/clawlet/main/AGENTS.md
