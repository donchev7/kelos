# Product Vision Draft

Moontide should be an **Org Intelligence and Agent Activation Platform**: it builds a living, temporal model of how an org actually works, then uses that model to recommend and run the right agents.

## 1. Core Product Promise

1. Connect real work tools once.
2. Build a trustworthy org memory + knowledge graph automatically.
3. Let users review, correct, and evolve that model.
4. Use the model to activate people and agents with high-confidence recommendations.

## 2. Target User Journey

1. User signs up and creates org.
2. User connects tools (GitHub, Slack, etc.).
3. Moontide ingests historical + recent activity.
4. Moontide builds initial org memory and temporal knowledge graph.
5. User enters a “Memory Studio” to review/edit/approve.
6. Moontide proposes activation actions:
   1. Invite collaborators ranked by real collaboration frequency.
   2. Suggested org structure hypotheses.
   3. Suggested initial agent deployments.
7. User accepts/rejects; accepted items become graph truth and drive automation.

## 3. Product Surfaces

1. Setup Wizard: org creation, tool auth, ingestion progress, trust settings.
2. Memory Studio: facts, relationships, timelines, confidence, edit/approve flows.
3. Graph View: entities and temporal relationships with provenance.
4. Activation Center: people invites, org structure suggestions, agent suggestions.
5. Agent Launchpad: deploy suggested agents with one-click templates.
6. Governance Console: permissions, retention, redaction, delete/export controls.

## 4. Core Domain Model

1. Entity types: Person, Team, Repo, Service, Channel, Incident, Agent, Workflow.
2. Relationship types: collaborates_with, reviews_for, owns, depends_on, reports_to, participates_in.
3. Claim model: every fact is a claim with source, confidence, valid_from, valid_to.
4. Temporal model: relationships can change over time; current truth and historical truth are both queryable.
5. Provenance model: every graph item must show where it came from and when.

## 5. Out-of-the-Box Moontide Agents (Created at Org Bootstrap)

1. Memory Curator: dedupe, merge, supersede, summarize stale context.
2. Identity Resolver: map users across tools and resolve duplicates.
3. Org Mapper: infer team structure and ownership graph.
4. Collaboration Analyst: compute invite and influence recommendations.
5. Agent Planner: suggest initial agent set based on graph patterns.
6. Policy Guard: enforce access, redaction, and approval policies.

## 6. Activation Logic (Product Behavior)

1. Invite suggestions:
   1. Rank by weighted collaboration score (frequency, recency, cross-tool interactions, repo overlap).
   2. Show explanation and confidence before invite action.
2. Org structure suggestions:
   1. Infer likely team clusters and lead relationships from work patterns.
   2. Mark all inferred hierarchy as “proposed” until user accepts.
3. Agent suggestions:
   1. Detect pain/opportunity clusters (PR bottlenecks, incident recurrence, review latency).
   2. Recommend agent blueprints tied to those clusters.

## 7. Trust, Permissions, and Safety

1. Important adjustment: “same scope as user” should be an optional mode, not default.
2. Default should be least privilege with progressive scope upgrades.
3. Every recommendation and memory write must be explainable and reversible.
4. Sensitive and high-impact claims remain active by default in v1, with strong audit trails and notification-driven oversight.
5. Provide hard delete and data export at org level.

## 8. MVP vs Later

1. MVP Must-Have:
   1. GitHub + Slack ingestion.
   2. Initial temporal org graph with confidence/provenance.
   3. Memory Studio review and edit.
   4. Invite recommendations.
   5. Initial agent recommendations and one-click deploy.
2. Later Nice-to-Have:
   1. Auto-org chart drafts from more sources (calendar, docs, tickets).
   2. Continuous graph drift detection.
   3. Multi-org benchmarking and recommendation tuning.

## 9. Success Metrics

1. Time to first trusted graph.
2. Percentage of recommended invites accepted.
3. Percentage of suggested org relationships accepted.
4. Percentage of suggested agents deployed.
5. Memory correction rate (should decline over time).
6. Agent outcome lift after activation.

## 10. Memory Deep Dive (High-Level Shape)

This section captures the current high-level shape of the Moontide memory solution.

## 10.1 Core memory must-haves

1. Retrieval quality must be semantic and context-aware, not simple mention matching or plain RAG vector similarity.
2. Memory must be temporal:
   1. represent how facts/principles evolve over time
   2. support transitions like architecture shifts while preserving history
3. Retrieval must respect user access boundaries:
   1. an agent must not retrieve data the human does not have access to
4. Updates must respect hierarchy and context:
   1. authority and role should affect which updates are allowed
5. Memory construction must support multi-source ingestion:
   1. GitHub
   2. Slack
   3. Jira
   4. Outlook
   5. SharePoint
6. Memory must be human-viewable and human-editable with the same access controls.
7. Memory must be globally shared across agents in the same org (for read, write, and runtime usage).
8. Memory must be strictly segregated by Moontide org.
9. Memory must have auditability and traceability sufficient for human review without overengineering.

## 10.2 Current decisions from this iteration

1. Update policy:
   1. memory updates default to active across domains (no approval gate in v1)
2. Runtime identity policy:
   1. agents run as service identity
   2. service identity access can be elevated only up to the creator user’s effective access ceiling
3. Org structure policy:
   1. initial structure is inferred
   2. users can edit inferred structure
4. Governance mode:
   1. no approval workflow for changes in v1
   2. prioritize visibility, auditability, and notifications over blocking gates
5. Strategic decisions:
   1. strategic memory can be edited by users/agents that have permissions

## 10.3 Additional must-haves to complete the memory shape

1. Canonical claim model:
   1. each memory item is a claim with type, scope, source, confidence, valid_from, valid_to, recorded_at, status
2. Bitemporal model:
   1. track both business-valid time and system-recorded time
3. Provenance-first memory:
   1. every claim must link back to concrete evidence from source systems
4. Identity resolution:
   1. unify people/entities across all connected tools before graph reasoning
5. Conflict and supersession semantics:
   1. preserve history chains instead of destructive overwrite
6. Access policy engine:
   1. enforce read/write checks at query and mutation time
7. Ingestion reliability contract:
   1. webhook ingestion + backfill + replay + reconciliation
8. Human legibility:
   1. diff view, timeline view, and “why this changed” explanations
9. Portability:
   1. exportable org memory and graph state to reduce lock-in risk
10. Quality instrumentation:
    1. track retrieval relevance, temporal correctness, and access leakage incidents

## 10.4 Graph shape inside a single org

1. Start with one canonical org graph, not separate physical graphs per domain/team.
2. Implement logical partitions/views inside the graph:
   1. partition tags on nodes/edges/claims
   2. policy-filtered views by team/domain/confidentiality
3. Enforce access at claim/edge level to keep cross-domain reasoning possible without data leakage.
4. Keep a graph-routing abstraction so specific partitions can be moved to dedicated physical stores later if needed.

Rationale:

1. simpler initial implementation
2. avoids entity duplication and graph fragmentation
3. preserves flexibility to evolve without lock-in

## 10.5 Retention strategy shape

1. Use hybrid retention:
   1. baseline time windows by memory class
   2. heuristic compaction based on importance, recency, access frequency, and supersession state
2. Preserve audit trails and supersession lineage; avoid hard deletion as default.
3. Apply stricter purge and delete behavior for privacy/compliance requests where required.

## 10.6 Missing requirements to explicitly add

1. Memory poisoning defense:
   1. source trust scores
   2. suspicious-update detection and quarantine path
2. Data classification and privacy controls:
   1. claim-level sensitivity classes
   2. secret/PII redaction before persistence
3. Access control for inferred knowledge:
   1. permission checks apply to derived edges/claims, not only raw source claims
4. Permission time-awareness:
   1. store and audit effective permissions at write-time
   2. enforce current permission checks at read-time
5. Quality and regression requirements:
   1. retrieval relevance, temporal correctness, access leakage, stale-memory rates
   2. release gates for memory quality regressions
6. Freshness and reliability SLOs:
   1. ingestion lag targets by source
   2. replay/reconciliation guarantees for missed events
7. Ontology and schema evolution:
   1. versioning and migration rules for entities, relationships, and claim types
8. Identity resolution governance:
   1. merge/split confidence thresholds
   2. reversible operations and audit history
9. Backup, restore, and portability:
   1. tested backup/restore targets
   2. export/import of memory and graph with provenance
10. Cost guardrails:
    1. storage/query budget limits
    2. compaction and tiering triggers
11. Human-control UX requirements:
    1. clear diff/rollback
    2. role-based change notifications
12. Compliance lifecycle controls:
    1. right-to-delete propagation
    2. legal hold and retention exception handling

## Appendix A. Post-Setup Persona Journeys (Working Ideas)

1. Solo Founder / First Admin
Journey: sees initial graph, accepts a few high-confidence facts, deploys 2-3 starter agents (PR triage, incident watcher, release checker), gets weekly activation suggestions, keeps memory lean with quick approve/reject.
Outcome: immediate leverage with low config overhead.

2. Engineering Manager Growing Team
Journey: reviews collaborator invite suggestions ranked by real interaction frequency, sends invites, accepts proposed team/repo ownership structure, deploys team-level agents per squad.
Outcome: faster team activation and clearer ownership map.

3. Staff Engineer / Tech Lead
Journey: opens graph view when planning architecture changes, inspects dependency and ownership edges with temporal history, asks Moontide agents to clean stale facts, deploys architecture drift and test-gap agents.
Outcome: better technical decisions with less manual repo archaeology.

4. SRE / Incident Commander
Journey: incident starts, Moontide surfaces similar historical incident patterns and likely owners, auto-activates incident support agents, posts ongoing updates, writes post-incident learnings back into org memory.
Outcome: lower time-to-diagnosis and reusable incident intelligence.

5. Product / Program Lead
Journey: before a release, checks cross-team collaboration graph, gets suggested stakeholders and risk hotspots, runs release-readiness agent bundle, tracks blockers in one view.
Outcome: fewer release surprises and better coordination.

6. Security / Compliance Owner
Journey: reviews memory writes and provenance for sensitive domains, enforces stricter approval rules for high-risk claims, deploys policy and secret-exposure monitoring agents.
Outcome: safer automation with auditable controls.

7. Ops / Admin Persona
Journey: manages scopes and integration permissions over time, audits which recommendations were accepted, tunes trust thresholds, configures retention/deletion policies.
Outcome: healthy long-term governance and lower model drift.

8. New Team Member (Invited User)
Journey: joins org, sees curated context relevant to their repos/channels/team, receives suggested agents and workflows, confirms or edits inferred relationships.
Outcome: faster onboarding and less tribal knowledge loss.
