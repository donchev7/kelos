# Agent-First Product Ideas (Draft v2)

## Product Thesis

This product should be an **agent operating system**, not a catalog of prebuilt task agents.

1. Users define the agents they need for their domain.
2. We provide the opinionated runtime, memory, governance, and orchestration infrastructure.
3. Admin surfaces are powerful and deeply configurable.
4. End-user surfaces are minimal, fast, and magical.

## Core Principles

1. No fixed "core functional agent" product identity.
- We ship capabilities and blueprints.
- Customers configure agents for software, real estate, finance, support, and other verticals.

2. Opinionated infrastructure is the product.
- Durable event logs.
- Multi-layer memory system.
- Sandboxed execution runtime.
- Policy and permission engine.
- Reliability and observability control plane.
- Internal system agents that keep the platform healthy.

3. Build for model capabilities 6 months ahead.
- Asynchronous long-running jobs are default.
- Tool orchestration and multi-step plans are first-class.
- Structured outputs and contracts over unstructured chat-only flows.
- Capability-gated features so better models unlock higher autonomy without architecture rewrites.

4. Maximum admin flexibility, minimum end-user friction.
- Progressive disclosure in setup.
- Strong defaults for most teams.
- Deep controls for advanced teams.

## Product Surfaces

## 1) Control Plane (Admins/Operators)

1. Agent Factory
- Define role, goals, memory scope, tools, policies, budgets, handoff behavior.

2. Memory Studio
- Configure what gets remembered, retention windows, summarization cadence, and source trust levels.

3. Mission Control
- Full execution history, stage timeline, failures, retries, handoffs, and correlation IDs across all agents.

4. Policy and Permission Manager
- Risk tiers, approval rules, sandbox/tool restrictions, escalation paths.

5. Reliability Center
- Stuck run detection, replay/retry, terminate, dead-letter inspection.

6. Integration Fabric
- Connectors, scopes, webhooks, secrets, per-agent access boundaries.

## 2) Interaction Plane (End Users)

1. Thread Agent Roster
- A single thread can have multiple agents with explicit roles in one shared mission.

2. Background Run Mode
- "Run this and report back" with milestone updates and final artifact.

3. Agent Delegation and Handoffs
- The orchestrator delegates subtasks to specialist agents and reports progress in-thread.

4. Artifact-First Replies
- PRs, diffs, checklists, runbooks, reports, not just text.

5. Human-in-the-loop Commands
- `approve`, `deny`, `terminate`, `recap`, `why`.

6. Multiplayer Collaboration
- Multiple humans interact with the same agent/session without handoff friction.

## Platform System Agents (Infrastructure Agents We Do Ship)

These are not domain agents for customers; they are platform-maintenance agents.

1. Observer Agent
- Watches execution events, integration events, and anomalies.

2. Memory Curator Agent
- Summarizes/compacts memory, resolves duplicates, expires stale context.

3. Policy Governor Agent
- Enforces policy checks and routes approvals/escalations.

4. Reliability Repair Agent
- Detects drift/stuck states and executes safe recovery actions.

5. Orchestrator Agent
- Routes work across thread roster agents, manages handoffs, and maintains mission-level context.

6. Evaluator Agent
- Scores run quality, detects regressions, and flags low-confidence outputs for review.

## User-Defined Functional Agents

Customers define these from templates/blueprints and customize for their org.

1. Review Agent
2. Incident Agent
3. Triage Agent
4. Research Agent
5. Operations Agent
6. Domain-specific custom agents

## What Feels Magical

1. Agents own end-to-end execution, not just chat responses.
2. Every response can produce actionable artifacts.
3. Work continues in background and returns when complete.
4. Users can interrupt, redirect, or terminate deterministically.
5. Agent memory feels coherent across time without leaking irrelevant context.
6. Collaboration feels native in shared threads/channels.
7. Multiple specialized agents can collaborate inside one thread without user orchestration overhead.

## Strategic Direction Update (Bigger Bet)

This is no longer "extend thread Q&A."  
This is "build the first version of an agent operating system."

1. Agent Factory is the front door.
- Users define agents, not just configure one built-in assistant.

2. Agent Teams are native.
- Threads can host a roster of agents with defined responsibilities.

3. Orchestrator runtime is first-class.
- A system orchestrator coordinates delegation, sequencing, and handoffs.

4. Memory is a product surface.
- Memory policies and shared context are configurable and visible.

5. Event-driven autonomy is core.
- Agents can start from external signals (CI failures, PR events, incidents), not only mentions.

6. Mission Control is mandatory.
- Operators need a single pane for agent runs, handoffs, approvals, and failures.

## Execution Plan (Three-Phase)

## Phase 1 - "Wow" Platform Slice

1. Agent Factory v1
- Create and manage user-defined agents with objective, tools, policy profile, and runtime mode.

2. Mission Control v1
- Live run cards: status, approvals, failures, and trigger history.

3. Event Trigger v1
- Support both trigger types as MVP validation criteria:
  - PR opened
  - scheduled (cron-like) execution

4. MVP Objective A
- With Agent Factory, user can recreate today's Slack thread-scoped Q&A agent behavior.

5. MVP Objective B
- With Agent Factory + event triggers, user can create:
  - a Functional Test Coverage Analyzer agent
  - a Technical Architecture Adherence Analyzer agent

## Phase 2 - Teaming + Memory

1. Multi-agent teams
- Allow a single task to be executed by coordinated agent teams.
- Support a constrained team topology first (coordinator + workers).

2. Team execution model
- Partition work (for example by repo) and aggregate into one final report.

3. Memory system
- Add explicit memory layers and controls across thread/agent/org scopes.
- Add memory write/promotion controls for reliability and trust.

4. Persistent background agent identity
- Agents act as long-lived org members via recurring/event-driven execution, with persistent context between runs.

## Phase 3 - Production Hardening

1. Reliability guarantees
- Strong retry/reclaim/lease semantics for single-agent and multi-agent workloads.

2. Governance depth
- Finer policy controls, budget controls, and safer escalation defaults.

3. Observability and supportability
- Better diagnostics, alerting, and operator runbooks for incident response.

4. UX quality
- Cleaner thread rendering, deterministic command behavior, minimal noisy updates.

## Open Questions for Iteration

1. What is the minimum blueprint model we ship in v1?
2. How do we represent agent roster composition and role assignment in a thread?
3. How much autonomy do we allow by default vs opt-in?
4. How should memory be segmented across user, thread, team, and org levels?
5. What policy DSL (or policy UI) is expressive enough without being overwhelming?
6. Which interaction commands are universal across all agents?
