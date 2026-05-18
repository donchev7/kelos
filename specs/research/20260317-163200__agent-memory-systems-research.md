# Agent Memory Systems Research: Supermemory, Mem0, Nuggets, and Related Architectures

Date: 2026-03-17
Status: Research synthesis
Scope: production and emerging memory systems for LLM agents, from Markdown/file memory to embeddings, knowledge graphs, and tensor-style memory

## 1. Executive Summary

Agent memory has now split into five practical families:

1. File memory (`MEMORY.md`, logs, profiles): simple, transparent, cheap, high manual overhead.
2. Embedding memory (vector stores): scalable semantic recall, weaker temporal/causal reasoning unless heavily engineered.
3. Graph memory (knowledge/temporal graphs): strongest for evolving truth, entities, and relationship-aware retrieval.
4. Hybrid memory (vector + graph + profile + tool-facing APIs): current production default for serious assistants.
5. Neural/tensor memory (latent or learned memory modules): promising for long-horizon reasoning, but mostly research-stage for production agents.

The strongest current production pattern is hybrid:

1. `write path`: extract facts/preferences/events from messages.
2. normalize + dedupe + conflict handling.
3. store in vector index (semantic recall) and graph layer (entity/temporal consistency).
4. retrieve with scoped filters + hybrid reranking.
5. feed selected memory back into prompt as structured sections.

If you are building today for reliability and iteration speed:

1. Start with file + vector.
2. Add graph for time-aware truth and relationship-heavy workflows.
3. Add MCP-facing memory tools for cross-client portability.
4. Treat tensor memory as an experimental enhancement, not your baseline.

## 2. Research Method and Source Quality

This report is based on official docs, official repositories, and primary research papers (arXiv), with emphasis on implementation details rather than marketing claims.

High-confidence source types used:

1. Official docs and API references (`supermemory.ai`, `docs.mem0.ai`, `help.getzep.com`, `docs.langchain.com`, `docs.letta.com`, `developers.llamaindex.ai`).
2. Official GitHub repos (`mem0ai/mem0`, `supermemoryai/memorybench`, `NeoVertex1/nuggets`, `getzep/graphiti`, `langchain-ai/langmem`, `letta-ai/letta`).
3. Research papers (`Mem0`, `Zep`, `MemGPT`, `Titans`, `A-MEM`, `MemOS`, `MemGen`).

Evidence caveat:

1. Benchmark deltas from vendor repos/docs should be treated as directional unless independently replicated.
2. Some platforms evolve quickly; API shapes and release details can shift after publication date.

## 3. Memory Architecture Taxonomy (From MD Files to Tensors)

### 3.1 File/Markdown Memory

Core idea:

1. Store durable facts in files (`MEMORY.md`, profile JSON, notes per user/project).
2. Inject files into prompt at runtime.

Strengths:

1. Human-auditable.
2. Zero infra and easy versioning via Git.
3. Works well for coding agents and personal assistants.

Limits:

1. Retrieval quality degrades with scale.
2. No native semantic recall unless combined with embedding search.
3. Conflict/temporal validity usually manual.

### 3.2 Embedding/Vector Memory

Core idea:

1. Encode memory units into embeddings.
2. Retrieve by similarity (+ filters, thresholds, rerankers).

Strengths:

1. Fast, scalable, proven.
2. Good for personalization and natural-language recall.

Limits:

1. Weak temporal truth modeling by default.
2. Contradictory facts need explicit resolution logic.

### 3.3 Knowledge Graph / Temporal Graph Memory

Core idea:

1. Extract entities/facts/relations.
2. Maintain linked graph with temporal metadata and invalidation logic.

Strengths:

1. Better for dynamic truth and relationships.
2. Better at multi-hop and temporally sensitive questions.

Limits:

1. More complex ingestion and maintenance.
2. Requires schema/governance discipline.

### 3.4 Hybrid Memory (Vector + Graph + Profile)

Core idea:

1. Use vector search for broad candidate recall.
2. Use graph for consistency, related entities, and temporal context.
3. Use profile blocks for stable, high-priority memory.

Strengths:

1. Best practical tradeoff in production.

Limits:

1. Highest system complexity.

### 3.5 Neural/Tensor Memory

Core idea:

1. Memory is represented as latent state or learned modules (not only external DB records).
2. Retrieval and usage are tightly coupled to model internals or reasoning traces.

Strengths:

1. Potentially tighter cognition-memory coupling.
2. May reduce retrieval orchestration overhead for some tasks.

Limits:

1. Limited production tooling and observability.
2. Harder debugging/compliance compared to explicit external memory stores.

## 4. Deep Dive: Supermemory

### 4.1 Functional Model

Supermemory explicitly distinguishes:

1. `documents`: raw inputs (PDF, URL, text, etc.).
2. `memories`: extracted semantic units, embedded and connected.

Docs describe it as a living graph where facts evolve and connect, rather than static document retrieval.

### 4.2 Graph Semantics

Supermemory docs define three key relationships:

1. `updates`: new fact supersedes old fact (`isLatest` used to resolve current truth).
2. `extends`: adds detail without invalidating base fact.
3. `derives`: inferred facts from existing patterns.

Implementation significance:

1. Temporal truth management is built into memory relationships, not only into prompt heuristics.
2. Retrieval can prefer latest-valid facts while preserving history for audits/explanations.

### 4.3 API Surface and Versioned Flows

Observed docs patterns:

1. v3 `/documents` style ingestion for content processing.
2. v4 `/memories` endpoints for direct memory CRUD-like operations.
3. Explicit `containerTag(s)` and metadata filters for partitioning.

Practical implication:

1. Teams can separate heavy ingestion pipelines from direct memory writes (e.g., preferences, user profile facts).

### 4.4 MCP Integration Pattern

Supermemory MCP docs show:

1. MCP endpoint at `https://mcp.supermemory.ai/mcp`.
2. OAuth default auth; API key (`sm_...`) alternative.
3. `x-sm-project` header for project scoping.
4. Tools like `memory`, `recall`, `whoAmI`.

Why this matters:

1. Memory can be made client-agnostic across MCP-capable IDEs/assistants.
2. Project scoping reduces cross-tenant contamination.

### 4.5 Where Supermemory Fits

Best fit:

1. SaaS memory API adoption with minimal infra ownership.
2. Teams needing graph evolution + MCP access quickly.

Watchouts:

1. Ensure strong tenancy boundaries and explicit filters in app-side logic.
2. Validate migration path if using both v3/v4 surfaces.

## 5. Deep Dive: Mem0 and OpenMemory

### 5.1 Mem0 Platform and OSS Split

Mem0 provides:

1. hosted platform (`MemoryClient` / API keys / dashboard).
2. OSS SDK with configurable stack.

As of March 3, 2026, `mem0ai/mem0` shows release `v1.0.5` on GitHub.

### 5.2 Write Path Semantics (`add`)

Docs describe a pipeline:

1. information extraction from messages.
2. conflict resolution/dedupe (when `infer=True`, default).
3. storage in vector store, optional graph store.

Important detail:

1. `infer=False` stores payload as-is; dedupe behavior changes and duplicates can accumulate.

### 5.3 Search Path Semantics (`search`)

Docs describe:

1. query processing and embedding search.
2. logical filters (`user_id`, categories, metadata).
3. optional reranking and thresholds.

Operational best practice from docs:

1. Always scope retrieval by user/session identifiers to prevent memory bleed.

### 5.4 Graph Memory in Mem0 OSS

Docs show Graph Memory design:

1. entity/relationship extraction during writes.
2. vector results plus graph relations returned in parallel.
3. graph relations augment context but do not automatically reorder vector hits.

Backends noted in docs:

1. Neo4j
2. Memgraph
3. Neptune
4. Kuzu

### 5.5 Configurability and Self-Hosting

OSS configuration supports pluggable:

1. LLM provider
2. embedder
3. vector store
4. reranker
5. graph store

This is valuable when you need infra portability or compliance constraints.

### 5.6 OpenMemory (Mem0-powered MCP Layer)

OpenMemory docs position it as local-first/private memory for MCP clients.

Key traits:

1. standardized memory tools (`add_memories`, `search_memory`, `list_memories`, `delete_all_memories`).
2. client integrations (Claude Desktop, Cursor, Windsurf, etc.).
3. hosted option (`app.openmemory.dev`) and local/self-hosted path.

Deployment caveat from docs:

1. quick container setup can be ephemeral unless persistent storage is configured.

### 5.7 Where Mem0 Fits

Best fit:

1. teams wanting full control from OSS to managed platform.
2. developers who need pluggable stores and graph augmentation.

Watchouts:

1. manage inference mode consistency (`infer=True/False`) to avoid duplicate drift.
2. benchmark claims should be validated with your own workloads.

## 6. Deep Dive: Nuggets (NeoVertex1/nuggets)

User-provided source: `https://github.com/NeoVertex1/nuggets`.

### 6.1 Core Idea

Nuggets is a local/personal assistant architecture using HRR (Holographic Reduced Representations), not a classic vector DB memory stack.

README + technical doc indicate:

1. topic-scoped “nuggets” as key-value memory spaces.
2. HRR binding/unbinding over complex-valued vectors.
3. deterministic reconstruction of vectors from seeded PRNG, with JSON persistence.
4. promotion of repeatedly recalled facts (3+) into `MEMORY.md`.

### 6.2 Implementation Profile

Repo structure highlights:

1. `src/nuggets/`: HRR math and memory engine.
2. `src/gateway/`: Telegram/WhatsApp gateway, per-user process handling.
3. `.pi/extensions/`: memory and proactive scheduling integration.

Interesting technical patterns:

1. local JSONL/process-RPC style orchestration.
2. strong local-first operation and low external infra dependencies.
3. explicit proactive loop (heartbeat + cron).

### 6.3 HRR Technical Positioning

`HRR_MEMORY_SYSTEM.md` frames memory as superposed complex-valued tensors with algebraic retrieval.

Architectural advantages claimed:

1. constant-size memory object semantics.
2. very fast local recall path.
3. no mandatory vector DB dependency.

Tradeoffs:

1. interference/capacity management is non-trivial at scale.
2. this is a more specialized paradigm than mainstream vector+graph stacks.
3. ecosystem maturity (ops tooling, observability, standards) is lower than mainstream memory APIs.

### 6.4 Where Nuggets Fits

Best fit:

1. personal/local agent experiments.
2. teams exploring non-vector memory paradigms.

Watchouts:

1. evaluate robustness for multi-tenant enterprise contexts.
2. build explicit safety/observability layers if moving toward production.

## 6.5 Deep Dive: OpenViking (volcengine/OpenViking)

Repository: `https://github.com/volcengine/OpenViking`.

### 6.5.1 Core Positioning

OpenViking is framed as a context database for agents, not just a vector index. Its core design claim is to unify memory, resources, and skills under a filesystem paradigm (`viking://...`) with hierarchical loading and observable retrieval trajectories.

### 6.5.2 Context as Filesystem + URI Addressing

Concept docs describe unified URI scopes such as:

1. `viking://resources/...` for imported knowledge/resources.
2. `viking://user/memories/...` for user memory.
3. `viking://agent/skills/...` for skills and agent-side context.
4. `viking://session/{id}/...` for session state and archives.

The URI model is important because retrieval, filtering, and filesystem operations use the same namespace. This reduces fragmentation between “memory APIs” and “content APIs”.

### 6.5.3 L0/L1/L2 Layered Context Model

OpenViking’s concept docs define progressive layers:

1. L0 (`.abstract.md`): short abstract, cheap routing signal.
2. L1 (`.overview.md`): directory-level summary/navigation context.
3. L2: full content and multimodal payloads.

Generation model:

1. asynchronous bottom-up generation (leaf to root).
2. session archiving also generates L0/L1 artifacts for history chunks.

Implementation impact:

1. explicit token budget control by defaulting to L1 and escalating to L2 only when needed.
2. more explainable retrieval path than flat chunk-only RAG.

### 6.5.4 Dual-Layer Storage Pattern

Architecture docs describe separation of concerns:

1. AGFS/VikingFS layer stores full content and directory structure.
2. vector index stores vectors + URIs/metadata references (not full file content).

This pattern maps well to production debugging:

1. retrieval can be traced back to stable filesystem paths.
2. index rebuilds are less risky because canonical content lives outside index records.

### 6.5.5 Retrieval Pipeline and Data Structures

Concept/API docs indicate a multi-stage retrieval flow:

1. intent analysis (can produce multiple typed sub-queries).
2. hierarchical recursive retrieval over directory tree.
3. rerank stage.
4. typed result grouping (`memories`, `resources`, `skills`) in `FindResult`.

The retrieval model emphasizes directory traversal and structural context, not only nearest-neighbor vector hits.

### 6.5.6 Session Commit and Memory Self-Iteration

Session docs describe lifecycle `Create -> Interact -> Commit` and a commit pipeline:

1. compress history (keep recent, archive older segments).
2. extract candidate long-term memories from messages.
3. pre-filter similar memories using vector retrieval.
4. LLM dedup decisions (`skip/create/none`) and per-item actions (`merge/delete`).
5. persist to AGFS + vectorize.

This is notable because it treats memory evolution as an explicit transactional process, not just appending conversation logs.

### 6.5.7 Operational Surface (API/CLI/Observer)

OpenViking exposes:

1. filesystem operations (`ls`, `read`, `tree`, `abstract`, `overview`, etc.).
2. retrieval APIs (`find`, `search`).
3. session APIs (`add_message`, `used`, `commit`).
4. observer APIs for queue/health visibility (`/api/v1/observer/queue`).

Deployment modes from docs:

1. embedded mode (local SDK with auto AGFS subprocess).
2. HTTP server mode for team/shared and cross-language use.

### 6.5.8 Fit and Tradeoffs

Best fit:

1. agents needing unified management of memory/resources/skills in one namespace.
2. teams that need retrieval observability and deterministic URI-based debugging.
3. workflows that benefit from progressive L0/L1/L2 loading.

Watchouts:

1. architecture is broader than a simple memory SDK, so integration complexity is higher.
2. async semantic queues and session commit policies should be monitored (backlogs affect freshness).
3. token/latency wins depend on disciplined layer usage (L1-first, targeted L2 expansion).

## 7. Other Strong Memory-Oriented Systems

### 7.1 Zep + Graphiti

Zep docs show a high-level memory API where `memory.add` is session-scoped for message ingestion while also building a user-level knowledge graph across sessions.

Important semantics from docs:

1. user graph integrates data across all sessions.
2. deleting a session deletes session messages but does not automatically remove graph data.

Graphiti repo + paper (arXiv 2501.13956) position:

1. temporal context graphs with validity windows and contradiction handling.
2. hybrid semantic/keyword/graph retrieval.
3. optional MCP server and REST service.

Strong fit:

1. dynamic environments where relationships and temporal truth are primary.

### 7.2 Letta (formerly MemGPT)

Letta repo and docs center on stateful agents with memory blocks.

Memory blocks docs indicate:

1. structured sections persisted across interactions.
2. inserted directly into context (XML-like memory block representation).

MemGPT paper (arXiv 2310.08560) provides conceptual basis:

1. virtual context management inspired by OS memory hierarchies.
2. multi-tier memory and control-flow/interrupt concepts.

Strong fit:

1. agents needing explicit, inspectable prompt-resident core memory plus additional memory tiers.

### 7.3 LangGraph + LangMem

LangGraph memory docs define:

1. short-term thread-scoped memory as graph state persisted via checkpointers.
2. long-term memory in stores scoped by namespaces.

LangMem adds:

1. memory tools agents can call in hot path (`manage/search`).
2. background memory manager patterns.
3. direct use with LangGraph store backends (in-memory or persistent DB-backed stores).

Strong fit:

1. teams already standardized on LangGraph workflows and durable execution.

### 7.4 LlamaIndex Memory

LlamaIndex docs provide a clear block-based memory architecture:

1. short-term token-limited chat memory.
2. flush to long-term memory blocks when thresholds exceeded.
3. predefined long-term blocks:
   - `StaticMemoryBlock`
   - `FactExtractionMemoryBlock`
   - `VectorMemoryBlock`
4. priority-based truncation when over token budget.

Strong fit:

1. developers wanting explicit token-budget control and composable memory block behavior.

## 8. Comparative Matrix (Implementation-Focused)

| System | Primary Memory Substrate | Write Pattern | Retrieval Pattern | Temporal/Conflict Model | Deployment Style | MCP Readiness |
|---|---|---|---|---|---|---|
| Supermemory | Memory graph + embeddings | content ingestion + direct memory writes | semantic + graph-aware recall | explicit update/extend/derive + `isLatest` | managed API | first-class MCP |
| Mem0 | embeddings + optional graph | `add` with infer/extract pipeline | semantic + filters + optional rerank + graph relations | conflict resolution in pipeline; graph optional | platform + OSS | OpenMemory MCP |
| Nuggets | HRR tensor-like superposition + files | key-value HRR bindings, promotion to file memory | algebraic recall + decoding | managed by recall frequency/promotion, custom logic | local-first OSS | not MCP-native by default |
| OpenViking | filesystem-native context DB + vector index | parser/tree build + async semantic queue + session commit extraction | intent analysis + hierarchical recursive retrieval + rerank | session commit dedup decisions (`skip/create/none`, `merge/delete`) + archives | OSS (embedded + HTTP server) | no first-class MCP surface in core docs |
| Zep/Graphiti | temporal knowledge graph (+ hybrid search) | session message ingestion to user graph | hybrid semantic/keyword/graph search | temporal validity + historical graph | cloud + OSS components | Graphiti MCP server |
| Letta | prompt-resident memory blocks + additional memory layers | API/agent-managed memory blocks | direct always-visible blocks + retrieval tiers | virtual context hierarchy, agent-managed updates | cloud + local tooling | tool/MCP ecosystem via Letta tools |
| LangGraph/LangMem | checkpointed state + memory store | explicit tool or background manager writes | store retrieval + agent tools | app-defined policies over persisted state | OSS + platform | MCP possible via wrappers |
| LlamaIndex | token-aware short-term + memory blocks | flush + block-specific processing | merged block content + vector retrieval | priority truncation + block logic | OSS | MCP via external adapters |

## 9. Reusable Technical Patterns Across Systems

### 9.1 Memory Write Pipeline

Canonical pipeline:

1. event capture (`messages`, tool results, docs, telemetry).
2. extraction (facts/preferences/goals/events/constraints).
3. normalization (entity canonicalization, timestamp normalization).
4. conflict handling (supersede, merge, or keep alternatives).
5. storage fan-out:
   - profile/static memory
   - vector index
   - graph edges
6. write audit log and quality scores.

### 9.2 Memory Read Pipeline

Canonical retrieval pipeline:

1. infer query intent (fact lookup, preference recall, temporal query, relational query).
2. generate candidates from vector search and optionally graph traversal.
3. rerank with recency, confidence, role, and tenant/session filters.
4. compress into structured memory payload for prompt insertion.
5. optionally self-check answer against source memory ids.

### 9.3 Temporal Truth Maintenance

Recommended rule set:

1. every fact gets `valid_from`, `valid_to?`, `source`, `confidence`, `scope`.
2. contradiction creates supersession edges instead of hard delete.
3. default retrieval returns currently valid facts but can request history mode.

### 9.4 Multi-Tenancy and Isolation

Minimum guardrails:

1. scope all writes and reads with tenant + user + workspace/project tags.
2. enforce filters server-side, not only prompt-side.
3. test for cross-tenant retrieval bleed as a blocking release gate.

### 9.5 Memory Compaction and Cost Control

Common methods:

1. score-based retention (importance + recency + reuse count).
2. rolling summarization for low-value episodic history.
3. hot/cold tiering: prompt-resident core memory vs archival stores.

## 10. Practical Blueprints

### 10.1 Blueprint A: Lightweight Markdown + Vector (Fastest to Ship)

Use when:

1. early-stage agent with low concurrency.

Design:

1. Keep canonical profile in `MEMORY.md` per user/workspace.
2. Store chat/event snippets in vector DB.
3. On each turn: read profile + top-k vector recalls.
4. Periodically rewrite profile from high-confidence facts.

Pros:

1. very transparent.
2. low ops overhead.

Cons:

1. manual conflict/temporal logic unless you add it.

### 10.2 Blueprint B: Hybrid Vector + Graph (Production Default)

Use when:

1. user facts change over time.
2. multi-hop/relationship queries matter.

Design:

1. extraction service writes to vector + graph.
2. retrieval service does hybrid candidate generation.
3. truth resolver enforces latest-valid preference while retaining history.
4. policy layer controls what enters prompt.

### 10.3 Blueprint C: MCP Memory Hub for Cross-Client Agents

Use when:

1. users switch between IDEs/assistants and want shared memory.

Design:

1. expose memory tools via MCP.
2. keep access tokens project-scoped.
3. centralize audit trail of tool-triggered memory writes.

### 10.4 Blueprint D: Local-First Privacy Stack

Use when:

1. sensitive data cannot leave local boundary.

Design:

1. self-host memory service (OpenMemory-like pattern).
2. local encrypted store + optional local embedding model.
3. no cloud sync by default; explicit export controls.

### 10.5 Blueprint E: Experimental Tensor Memory Augmentation

Use when:

1. research team evaluating long-context cognition improvements.

Design:

1. keep external symbolic memory as source-of-truth.
2. add tensor/latent memory module for reasoning enhancement.
3. compare against symbolic-only baseline on same tasks.

Do not:

1. replace auditable memory store with latent memory only for production-critical systems.

## 11. Evaluation and Benchmarking Strategy

### 11.1 Benchmark Families to Use

1. Conversational memory benchmarks (e.g., LOCOMO, LongMemEval, ConvoMem-style suites).
2. Domain-specific replay logs from your own product.

MemoryBench (Supermemory open-source benchmark framework) supports multi-provider runs and explicit provider/judge configuration; useful for reproducible harness setup.

### 11.2 Metrics That Matter in Production

Accuracy metrics:

1. recall@k on gold facts.
2. temporal correctness (current truth vs stale truth).
3. relation correctness for multi-hop questions.

Efficiency metrics:

1. p95 retrieval latency.
2. token overhead added to prompt.
3. memory write cost per conversation hour.

Safety metrics:

1. cross-tenant contamination rate.
2. contradiction resolution error rate.
3. stale-memory hallucination incidence.

### 11.3 Benchmark Pitfalls

1. Over-reliance on single judge model.
2. Missing adversarial temporal updates in test data.
3. Ignoring write-path quality (many systems only compare read quality).

## 12. Security, Compliance, and Governance Patterns

Baseline controls:

1. encrypt at rest and in transit.
2. append-only memory event log.
3. data retention and right-to-delete flows.
4. PII-aware extraction policies (redact before storage where required).
5. scoped API keys and least-privilege tool permissions.

Agent-specific controls:

1. memory write approval rules for high-risk domains.
2. source-attributed memory records (who/what wrote it).
3. immutable provenance IDs for forensics.

## 13. Design Recommendations by Maturity Stage

### Stage 0-1 (Prototype)

1. file memory + vector retrieval.
2. strict user scoping.
3. nightly memory compaction.

### Stage 2 (Early Production)

1. hybrid vector + graph for temporal/relational correctness.
2. formal memory schema and contradiction policy.
3. benchmark harness in CI for regression detection.

### Stage 3+ (Scale and Multi-Agent)

1. MCP memory plane for cross-client consistency.
2. background memory managers for consolidation.
3. research branch for tensor/latent memory augmentation while keeping symbolic ground truth.

## 14. Closing Synthesis

The industry trend is clear:

1. pure-RAG memory is insufficient for long-lived agents.
2. production systems are converging on hybrid memory (profile + vector + graph).
3. MCP is becoming a practical interoperability layer for memory tools.
4. tensor/neural memory is advancing quickly but is still complementary to explicit stores for production reliability.

For immediate practical success, combine:

1. auditable explicit memory (files/profile records),
2. scalable semantic recall (vectors),
3. temporal relational correctness (graphs),
4. disciplined evaluation and tenancy controls.

## 15. Reference Implementation Patterns (Concrete)

### 15.1 File-First Memory Layout (`MEMORY.md` + JSON)

Minimal structure that remains human-auditable:

```text
memory/
  users/
    {user_id}/
      MEMORY.md
      profile.json
      episodic.jsonl
      write_audit.jsonl
```

Recommended `profile.json` shape:

```json
{
  "user_id": "u_123",
  "preferences": [
    {
      "key": "meeting_style",
      "value": "agenda-first",
      "source": "chat:msg_984",
      "confidence": 0.92,
      "valid_from": "2026-03-17T09:30:00Z",
      "valid_to": null
    }
  ],
  "constraints": [],
  "goals": [],
  "updated_at": "2026-03-17T09:31:12Z"
}
```

When to use:

1. coding assistants and internal copilots where explainability is mandatory.
2. low to medium memory volume.

### 15.2 Vector Memory Schema (Postgres + pgvector Pattern)

Representative SQL model:

```sql
create table memories (
  id bigserial primary key,
  tenant_id text not null,
  user_id text not null,
  session_id text,
  memory_type text not null, -- preference|fact|event|constraint
  content text not null,
  embedding vector(1536) not null,
  confidence real not null default 0.5,
  source_id text,
  valid_from timestamptz not null default now(),
  valid_to timestamptz,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index memories_tenant_user_idx on memories (tenant_id, user_id, created_at desc);
create index memories_validity_idx on memories (tenant_id, user_id, valid_to);
-- Use ivfflat or hnsw depending on your pgvector/runtime version.
```

Read pattern:

1. filter first by `tenant_id` + `user_id`.
2. include only active facts (`valid_to is null`) unless historical mode requested.
3. run ANN similarity and optional rerank.

### 15.3 Temporal Graph Model (Entity-Fact-Claim Pattern)

A robust pattern for contradiction handling:

1. `Entity` nodes (`User`, `Org`, `Project`, `Topic`).
2. `Claim` nodes containing a fact statement + provenance.
3. temporal edges (`ASSERTS`, `SUPERSEDES`, `RELATES_TO`) with timestamps.

Example Cypher-style operations:

```cypher
// New claim
CREATE (c:Claim {
  claim_id: $claim_id,
  text: $text,
  confidence: $confidence,
  valid_from: datetime($valid_from),
  valid_to: null,
  source_id: $source_id
});

// Supersede old claim
MATCH (old:Claim {claim_id: $old_id}), (new:Claim {claim_id: $new_id})
CREATE (new)-[:SUPERSEDES {at: datetime()}]->(old)
SET old.valid_to = datetime();
```

Why this pattern works:

1. preserves history without hard delete.
2. supports “current truth” and “what used to be true” queries.

### 15.4 Hybrid Retrieval Orchestrator (Pseudocode)

```python
def retrieve_memory(query, scope):
    q = normalize(query)
    intent = classify_intent(q)  # fact_lookup, preference, temporal, relational

    vec_candidates = vector_search(
        query=q,
        tenant_id=scope.tenant_id,
        user_id=scope.user_id,
        top_k=40,
    )

    graph_candidates = []
    if intent in {"temporal", "relational", "fact_lookup"}:
        graph_candidates = graph_search(
            query=q,
            tenant_id=scope.tenant_id,
            user_id=scope.user_id,
            max_hops=2,
        )

    merged = merge_candidates(vec_candidates, graph_candidates)
    filtered = apply_policy_filters(merged, scope=scope)
    reranked = rerank(filtered, features=["semantic", "recency", "confidence", "source_quality"])
    packed = pack_for_prompt(reranked[:12])
    return packed
```

### 15.5 Memory Write Pipeline with Conflict Resolution (Pseudocode)

```python
def write_memory(messages, scope):
    extracted = extract_candidates(messages)  # facts/preferences/events
    normalized = canonicalize(extracted)

    for m in normalized:
        existing = find_potential_conflicts(m, scope)
        decision = resolve_conflict(m, existing)  # keep_new | merge | keep_old | ambiguous

        if decision == "keep_new":
            supersede(existing)
            persist(m, scope)
        elif decision == "merge":
            merged = merge_memory(m, existing)
            supersede(existing)
            persist(merged, scope)
        elif decision == "keep_old":
            append_audit(m, reason="lower_confidence")
        else:
            persist_as_alternative(m, scope)
```

Key policy recommendations:

1. never silently drop contradictions; mark and resolve.
2. store source provenance for every durable memory write.

### 15.6 Tensor/HRR Adapter Pattern (Safe Production Integration)

When experimenting with HRR/tensor memory (e.g., Nuggets-like approach), use an adapter with symbolic fallback:

```python
class MemoryAdapter:
    def __init__(self, symbolic_store, tensor_store):
        self.symbolic = symbolic_store
        self.tensor = tensor_store

    def write(self, record):
        self.symbolic.write(record)         # source-of-truth
        self.tensor.encode(record)          # optional accelerator/reasoning aid

    def query(self, q, scope):
        symbolic = self.symbolic.query(q, scope)
        tensor = self.tensor.query(q, scope)
        return reconcile(symbolic, tensor)  # symbolic wins on conflict by default
```

Production safety rule:

1. keep symbolic memory as authority for compliance, audit, and deletion requirements.

## 16. Sources

### Primary docs and repos

1. Supermemory docs: https://supermemory.ai/docs
2. Supermemory architecture: https://supermemory.ai/docs/concepts/how-it-works
3. Supermemory graph memory: https://supermemory.ai/docs/concepts/graph-memory
4. Supermemory memory vs RAG: https://supermemory.ai/docs/concepts/memory-vs-rag
5. Supermemory memory operations: https://supermemory.ai/docs/memory-operations
6. Supermemory memory API add: https://supermemory.ai/docs/memory-api/creation/adding-memories
7. Supermemory MCP docs: https://supermemory.ai/docs/supermemory-mcp/mcp
8. MemoryBench docs: https://supermemory.ai/docs/memorybench/overview
9. MemoryBench repo: https://github.com/supermemoryai/memorybench
10. Mem0 docs root: https://docs.mem0.ai
11. Mem0 quickstart: https://docs.mem0.ai/platform/quickstart
12. Mem0 add memory: https://docs.mem0.ai/core-concepts/memory-operations/add
13. Mem0 search memory: https://docs.mem0.ai/core-concepts/memory-operations/search
14. Mem0 graph memory (OSS): https://docs.mem0.ai/open-source/features/graph-memory
15. Mem0 OSS config: https://docs.mem0.ai/open-source/configuration
16. OpenMemory overview: https://docs.mem0.ai/openmemory/overview
17. OpenMemory quickstart: https://docs.mem0.ai/openmemory/quickstart
18. Mem0 repo: https://github.com/mem0ai/mem0
19. Nuggets repo (user-provided): https://github.com/NeoVertex1/nuggets
20. Nuggets HRR technical document: https://github.com/NeoVertex1/nuggets/blob/main/HRR_MEMORY_SYSTEM.md
21. Zep memory docs: https://help.getzep.com/v2/memory
22. Zep sessions docs: https://help.getzep.com/v2/sessions
23. Graphiti repo: https://github.com/getzep/graphiti
24. LangGraph memory docs: https://docs.langchain.com/oss/python/langgraph/memory
25. LangGraph persistence docs: https://docs.langchain.com/oss/python/langgraph/persistence
26. LangMem docs: https://langchain-ai.github.io/langmem/
27. LangMem repo: https://github.com/langchain-ai/langmem
28. OpenViking repo: https://github.com/volcengine/OpenViking
29. OpenViking README: https://raw.githubusercontent.com/volcengine/OpenViking/main/README.md
30. OpenViking architecture concept: https://github.com/volcengine/OpenViking/blob/main/docs/en/concepts/01-architecture.md
31. OpenViking context layers: https://github.com/volcengine/OpenViking/blob/main/docs/en/concepts/03-context-layers.md
32. OpenViking Viking URI model: https://github.com/volcengine/OpenViking/blob/main/docs/en/concepts/04-viking-uri.md
33. OpenViking retrieval concept: https://github.com/volcengine/OpenViking/blob/main/docs/en/concepts/07-retrieval.md
34. OpenViking session concept: https://github.com/volcengine/OpenViking/blob/main/docs/en/concepts/08-session.md
35. OpenViking retrieval API: https://github.com/volcengine/OpenViking/blob/main/docs/en/api/06-retrieval.md
36. OpenViking system API: https://github.com/volcengine/OpenViking/blob/main/docs/en/api/07-system.md
37. Letta memory blocks docs: https://docs.letta.com/guides/core-concepts/memory/memory-blocks
38. Letta repo: https://github.com/letta-ai/letta
39. LlamaIndex memory guide: https://developers.llamaindex.ai/python/framework/module_guides/deploying/agents/memory/
40. LlamaIndex memory example: https://developers.llamaindex.ai/python/examples/memory/memory/

### Research papers

41. Mem0 paper (arXiv 2504.19413): https://arxiv.org/abs/2504.19413
42. Zep paper (arXiv 2501.13956): https://arxiv.org/abs/2501.13956
43. MemGPT paper (arXiv 2310.08560): https://arxiv.org/abs/2310.08560
44. Titans (arXiv 2501.00663): https://arxiv.org/abs/2501.00663
45. A-MEM (arXiv 2502.12110): https://arxiv.org/abs/2502.12110
46. MemOS (arXiv 2507.03724): https://arxiv.org/abs/2507.03724
47. MemGen (arXiv 2509.24704): https://arxiv.org/abs/2509.24704
