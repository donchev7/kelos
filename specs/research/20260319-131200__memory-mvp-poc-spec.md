# Moontide Memory MVP POC Spec (Initial)

Date: 2026-03-19  
Status: Draft v0 (for iteration)  
Owner: Moontide  
Purpose: Define a safe, final-state-aligned MVP for Moontide memory with explicit in-scope and out-of-scope boundaries.

## 1. Why this MVP exists

Moontide needs a memory system that is immediately useful in real agent workflows, while staying aligned with the long-term architecture (temporal org graph + strict access control + global shared org memory).

This MVP is a production-shaped POC, not a throwaway experiment.

## 2. MVP goals

1. Build a shared org memory that agents can read/write in sessions.
2. Make retrieval materially better than simple mention matching or plain vector-only RAG.
3. Preserve temporal evolution of facts and principles.
4. Enforce access constraints so agents cannot see/edit beyond allowed boundaries.
5. Keep all changes human-auditable and reversible.

## 3. MVP non-goals

1. Full autonomous governance workflows with approvals.
2. Fully automated org activation loops (invites/org chart/agent deployment autopilot).
3. All connectors in v1 (Jira, Outlook, SharePoint deferred).
4. Dedicated graph database migration in v1.
5. Enterprise-grade compliance feature completeness in first POC pass.

## 4. Explicit will do vs will not do

## 4.1 MVP will do

1. Ingest from GitHub and Slack only.
2. Construct org-scoped memory claims with provenance and temporal fields.
3. Build one canonical graph per org with logical partitions/tags.
4. Provide context-aware hybrid retrieval:
   1. symbolic filter + temporal ranking + semantic candidate scoring
5. Expose memory to all org agents as global shared memory.
6. Enforce strict org segregation and access-aware retrieval.
7. Support human view/edit/supersede/delete with audit trail.
8. Default updates to active (no approval gate), with full traceability.
9. Run agents as service identity with creator-ceiling permissions.

## 4.2 MVP will not do

1. No cross-org memory sharing.
2. No approval blocking step for high-impact edits.
3. No Jira/Outlook/SharePoint ingestion in first release.
4. No separate physical graphs inside an org.
5. No agent self-elevation beyond creator ceiling.
6. No silent destructive overwrite of prior memory states.

## 5. Architecture shape (aligned to final state)

1. Canonical source of truth: Postgres.
2. Memory data model: claim-centric + temporal + provenance.
3. Retrieval service: hybrid orchestration over symbolic + temporal + semantic scoring.
4. Graph representation: canonical org graph in Postgres model with partition tags and policy-filtered views.
5. Access model: policy checks at read and write time.
6. Agent interface: unified memory read/write API available to all org agents.
7. Operator interface: Memory Studio for inspection, edit, supersession, and rollback.

## 6. Data model requirements (MVP)

1. Claim record fields (minimum):
   1. `org_id`
   2. `scope` (`agent`, `session`, `org_domain`)
   3. `claim_type` (`fact`, `constraint`, `preference`, `task_context`)
   4. `subject_key`
   5. `value`
   6. `status` (`active`, `superseded`, `deleted`)
   7. `valid_from`, `valid_to`
   8. `recorded_at`
   9. `source_type`, `source_id`, `source_excerpt`
   10. `confidence`
   11. `partition_tags`
2. Event log fields (minimum):
   1. `event_type` (`created`, `updated`, `superseded`, `deleted`, `restored`)
   2. actor metadata (`system`, `user`, `agent_id`)
   3. before/after payload snapshot
   4. timestamp

## 7. Retrieval requirements (MVP)

1. Must support query intent classes:
   1. factual lookup
   2. policy/constraint retrieval
   3. temporal state query
   4. contextual task recall
2. Must rank by:
   1. access validity
   2. current truth validity (`valid_to` semantics)
   3. temporal recency
   4. confidence
   5. semantic relevance
3. Must provide provenance IDs with every retrieved memory item.
4. Must cap prompt injection size with deterministic truncation policy.

## 8. Access and update policy requirements (MVP)

1. Read and write checks are mandatory on every operation.
2. Agent service identity is constrained by creator’s maximum effective permissions.
3. Updates are active by default when permission checks pass.
4. Hierarchy/context constraints apply to write authority.
5. Inferred and derived claims inherit access constraints from source lineage.

## 9. Connector scope (MVP)

## 9.1 In scope

1. GitHub
2. Slack

## 9.2 Out of scope (deferred)

1. Jira
2. Outlook
3. SharePoint

## 9.3 Deferred connector readiness requirements

1. Connector contracts must be standardized now so deferred sources plug in without schema rewrite.
2. Ingestion layer must already support:
   1. event stream ingestion
   2. backfill jobs
   3. replay/reconciliation
   4. per-source rate limiting

## 10. Human interface requirements (MVP)

1. Memory Studio list and graph views with filtering by type, scope, source, and time.
2. Diff and timeline view for each claim.
3. Manual edit/supersede/delete with role-aware permission checks.
4. Provenance drill-down to source artifact/message/event.
5. Basic rollback capability through supersession restore.

## 11. OSS memory tools: what to use now vs evaluate

This section uses existing internal research in [20260317-163200__agent-memory-systems-research.md](/Users/shan/Documents/sandbox/starlight/moontide_ai/docs/20260317-163200__agent-memory-systems-research.md).

## 11.1 Core path for MVP (build in-house)

1. Build claim store, temporal semantics, and access policy in Moontide core.
2. Keep retrieval orchestration in-house to preserve product differentiation and permission guarantees.

Rationale:

1. Memory is core product IP for Moontide, not a peripheral plugin.
2. Access + hierarchy semantics are domain-specific and must be first-class.

## 11.2 Evaluation track (parallel, limited)

1. Mem0/OpenMemory:
   1. evaluate as optional semantic retrieval backend adapter
2. Zep/Graphiti:
   1. evaluate temporal graph retrieval patterns and APIs
3. Supermemory:
   1. evaluate API ergonomics and graph memory operations for potential interoperability ideas

Evaluation constraints:

1. No hard dependency for MVP launch.
2. Adapter boundary only; core claim/provenance/access model remains Moontide-owned.

## 11.3 Not selected for MVP core

1. Nuggets/HRR-style tensor memory as primary runtime store.
2. Full managed external memory API as sole source of truth.

## 11.4 Deeper code-level findings and reuse candidates (with links)

Note: this section captures a deeper pass done on 2026-03-19 across OSS code and official docs to identify what Moontide should directly reuse vs adapt.

## 11.4.1 Mem0 OSS (code-level patterns worth reusing)

Code references:

1. Memory orchestrator:
   1. https://github.com/mem0ai/mem0/blob/main/mem0/memory/main.py
2. Graph augmentation module:
   1. https://github.com/mem0ai/mem0/blob/main/mem0/memory/graph_memory.py
3. Storage abstraction:
   1. https://github.com/mem0ai/mem0/blob/main/mem0/memory/storage.py
4. Base config model:
   1. https://github.com/mem0ai/mem0/blob/main/mem0/configs/base.py
5. Platform quickstart/docs:
   1. https://docs.mem0.ai/platform/quickstart

Implementation patterns observed:

1. Scoped memory ops are enforced around identifiers (`user_id`, `agent_id`, `run_id`) and filters are built first before add/search logic.
2. `add(...)` behavior supports two modes:
   1. inference mode (`infer=True`) for extraction/dedupe/update semantics
   2. raw append mode (`infer=False`) for direct memory insertion
3. Write path fans out to vector + optional graph path, with graph used as augmentation, not as the single ranking path.
4. Graph path performs entity extraction, relation extraction, conflict deletion candidate generation, then relationship insertion.
5. Retrieval path includes semantic match plus additional reranking and filtering steps.

Direct reuse decisions for Moontide:

1. Reuse the concept of strict scope-first filters, but map to Moontide authz model (`org_id` + actor + capability checks), not just caller-supplied ids.
2. Reuse split write modes:
   1. inferred memory extraction for conversational/connector events
   2. explicit/manual writes for user edits and operator corrections
3. Do not reuse Mem0 as source-of-truth storage; keep Moontide claim/event model in Postgres.
4. Keep a Mem0-style adapter boundary only for optional candidate generation experiments.

## 11.4.2 OpenMemory (MCP surface patterns)

References:

1. Overview:
   1. https://docs.mem0.ai/openmemory/overview
2. Quickstart:
   1. https://docs.mem0.ai/openmemory/quickstart
3. OpenMemory code path:
   1. https://github.com/mem0ai/mem0/tree/main/openmemory

Patterns observed:

1. Standardized MCP memory operations (`add_memories`, `search_memory`, `list_memories`, `delete_all_memories`) are intentionally simple and interoperable.
2. Local/self-host mode is easy to stand up but can be ephemeral without persistent storage setup.
3. Hosted mode exists for zero-setup use.

Direct reuse decisions for Moontide:

1. Reuse the tool-shape idea (simple stable memory primitives exposed over agent/tool interfaces).
2. Keep Moontide-specific policy and claim semantics behind these tool primitives.
3. Add explicit persistence and audit constraints by default (avoid ephemeral defaults for production org memory).

## 11.4.3 Graphiti/Zep (temporal graph and retrieval composition patterns)

References:

1. Graphiti nodes:
   1. https://github.com/getzep/graphiti/blob/main/graphiti_core/nodes.py
2. Graphiti edges:
   1. https://github.com/getzep/graphiti/blob/main/graphiti_core/edges.py
3. Search config:
   1. https://github.com/getzep/graphiti/blob/main/graphiti_core/search/search_config.py
4. Search recipes:
   1. https://github.com/getzep/graphiti/blob/main/graphiti_core/search/search_config_recipes.py
5. Search filters:
   1. https://github.com/getzep/graphiti/blob/main/graphiti_core/search/search_filters.py
6. Zep Graphiti paper:
   1. https://arxiv.org/abs/2501.13956

Patterns observed:

1. Strong emphasis on partitioned graph groups and typed nodes/episodes.
2. Retrieval is configurable and compositional instead of one fixed ranking path.
3. Temporal semantics are represented in graph primitives and read-time filtering/ranking.

Direct reuse decisions for Moontide:

1. Reuse the idea of query-recipe composition (different retrieval recipes by intent class).
2. Reuse graph partition concepts logically inside one org graph (domain, source, sensitivity tags).
3. Keep Moontide access policy as the first gate before retrieval/rerank stages.

## 11.4.4 LangMem (runtime memory tooling + background reflection pattern)

References:

1. Knowledge tools:
   1. https://github.com/langchain-ai/langmem/blob/main/src/langmem/knowledge/tools.py
2. Knowledge extraction/manager:
   1. https://github.com/langchain-ai/langmem/blob/main/src/langmem/knowledge/extraction.py

Patterns observed:

1. Memory tools are exposed explicitly (`manage` and `search` style behavior) over a namespace-scoped store.
2. Background reflection/extraction pattern is first-class, not only inline with user requests.
3. Structured extraction via schema-based summarization/extraction functions is built into write pipelines.

Direct reuse decisions for Moontide:

1. Reuse namespace-scoped memory tool contracts for agent runtime integration.
2. Reuse background manager pattern for post-run consolidation and temporal conflict checks.
3. Keep extraction schema tied to Moontide claim model so outputs remain directly auditable.

## 11.4.5 Supermemory (graph memory semantics that map well to Moontide claims)

References:

1. Graph memory concept:
   1. https://supermemory.ai/docs/concepts/graph-memory
2. Architecture concept:
   1. https://supermemory.ai/docs/concepts/how-it-works

Patterns observed:

1. Relationship semantics (`updates`, `extends`, `derives`) make temporal evolution explicit.
2. `isLatest` semantics are used to separate current truth from historical context.
3. Distinction between documents and memories is clear and operationally useful.

Direct reuse decisions for Moontide:

1. Reuse relationship semantics directly in Moontide claim lineage:
   1. `updates` -> supersession edge
   2. `extends` -> enrichment edge
   3. `derives` -> inferred edge with explicit confidence
2. Keep source provenance attached to every relationship operation.

## 11.5 Why Moontide should use graph-first hybrid retrieval (not plain graph-only retrieval)

The key clarification:

1. We are not proposing two competing sources of truth.
2. We are proposing one source of truth (Moontide claim graph), with hybrid retrieval signals used to find the right memories reliably.

## 11.5.1 What “plain graph only” gets right

1. Strong for temporal truth and contradiction handling.
2. Strong for entity relationships and multi-hop org reasoning.
3. Naturally auditable when modeled as claims + lineage.

## 11.5.2 Where “plain graph only” fails in practice

1. Query anchoring problem:
   1. users ask in natural language with incomplete/ambiguous entities
   2. exact graph traversal alone can miss relevant memory candidates
2. Lexical artifact problem:
   1. identifiers, ticket keys, stack traces, commit hashes, or exact policy strings are often better found via lexical/semantic recall first
3. Recall robustness problem:
   1. when extraction/entity-linking is imperfect, graph-only retrieval under-recalls

## 11.5.3 Graph-first hybrid model (recommended for Moontide MVP)

Read flow:

1. Policy gate first:
   1. constrain by `org_id`, actor identity, capability scope, and sensitivity rules
2. Candidate generation (parallel):
   1. semantic/lexical candidates over claim text + source excerpts
   2. graph-neighborhood candidates via entity/relationship traversal
3. Temporal + lineage resolution:
   1. prefer active latest-valid claims
   2. include superseded history only when query asks for evolution/history
4. Final rank:
   1. combine policy validity + temporal validity + confidence + relevance
5. Prompt pack:
   1. include compact claim payload + provenance ids + timestamps

Write flow:

1. all accepted writes update claim graph/events first
2. optional semantic indices are derived artifacts and can be rebuilt
3. therefore source of truth remains graph/claims, not vector index

## 11.5.4 Decision rule

1. If a capability affects correctness, permissions, or auditability:
   1. implement in claim graph + policy layer
2. If a capability improves candidate recall speed/coverage:
   1. implement in derived retrieval index/rerank layer

This keeps architecture stable while preserving retrieval quality.

## 11.6 Concrete reuse map for Moontide implementation

Must-have now (MVP):

1. Mem0-inspired scope-first query/write filters, but enforced by Moontide authz.
2. Supermemory-style claim relationship semantics (`updates`, `extends`, `derives`) in lineage model.
3. Graphiti-style retrieval recipes by query intent class.
4. LangMem-style background consolidation worker for post-run extraction and memory maintenance.

Optional later:

1. OpenMemory-compatible MCP bridge for external client interoperability.
2. Adapter experiments with external retrieval services (Mem0/Supermemory) behind feature flags.
3. Advanced graph backends beyond Postgres once scale and query profile justify migration.

## 12. Safety and reliability requirements (MVP)

1. Ingestion reliability:
   1. at-least-once ingestion handling
   2. idempotency keys
   3. replay window
2. Poisoning controls:
   1. source trust score
   2. suspicious update flagging
3. Auditability:
   1. full write history
   2. actor attribution
   3. reversible state transitions
4. Observability:
   1. ingestion lag
   2. retrieval latency
   3. memory hit quality indicators
   4. access-denied and policy-failure counters

## 13. Retention policy (MVP shape)

1. Hybrid retention model:
   1. baseline TTL per claim class
   2. heuristic compaction for low-value historical context
2. Preserve lineage for superseded claims unless explicit deletion policy requires purge.
3. Maintain auditable deletion events when right-to-delete is invoked.

## 14. MVP acceptance criteria

1. Agents successfully use shared org memory in live sessions.
2. Retrieval quality is measurably better than baseline keyword/mention recall on internal eval set.
3. Temporal updates behave correctly (supersession and current truth resolution).
4. No cross-org memory retrieval leakage in security tests.
5. No permission-elevation path beyond creator ceiling.
6. Human can inspect, edit, and rollback memory items in UI.
7. Full claim-level provenance and event log available for review.

## 15. Delivery phases for this MVP POC

## Phase 1: Core memory substrate

1. Claim/event schema and migrations.
2. GitHub/Slack ingestion mapping into claim candidates.
3. Base retrieval orchestration API.

## Phase 2: Agent runtime integration

1. Prompt-time memory retrieval and injection.
2. Post-run memory write/supersession pipeline.
3. Access checks in read/write paths.

## Phase 3: Human operations surface

1. Memory Studio v1 list/detail/timeline/diff.
2. Edit/supersede/delete operations with audit views.
3. Basic metrics dashboard for memory reliability and quality.

## 16. Open questions to resolve next

1. Which policy engine implementation do we adopt first for access checks?
2. How do we represent hierarchy-driven write authority in policy primitives?
3. What is the minimum internal eval dataset for retrieval quality gating?
4. Which semantic model/embedding stack do we standardize for MVP?
5. Should memory partition tags be user-editable in v1 or system-managed only?
