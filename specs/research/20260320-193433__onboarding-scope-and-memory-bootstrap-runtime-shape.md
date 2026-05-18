# Onboarding Scope Selection and Memory Bootstrap Runtime Shape

Date: 2026-03-20  
Status: Draft v1  
Purpose: Define the broad runtime functional shape after GitHub + Slack connection, focused on source selection, ingestion, memory construction, and storage boundaries.

## 1. Scope of this document

This document answers four product-runtime questions:

1. What data we will get.
2. How we will get it.
3. What we will do with it.
4. Where we will store both source data and memory.

This is intentionally not an implementation spec.

## 2. Runtime flow at a glance

After mandatory connector auth (GitHub + Slack), onboarding continues as:

1. User selects ingestion scope:
   1. GitHub repos
   2. Slack channels
2. Moontide creates a source selection manifest for the org.
3. Bootstrap ingestion runs:
   1. historical backfill for selected sources
   2. first-pass memory build
4. Continuous ingestion starts for the same selected sources.
5. User sees initial memory in Memory Studio and can edit/correct.

## 3. What data we will get

## 3.1 GitHub data (from selected repos)

1. Repository metadata:
   1. repo identity, default branch, visibility, language signals
2. Contribution and collaboration signals:
   1. PRs, reviews, comments, review requests, commit metadata
3. Change intent and engineering decisions:
   1. PR titles/descriptions, linked issues, merge rationale in discussions
4. Work artifacts:
   1. issues, labels, milestones (when available in selected scope)
5. Structural context:
   1. code ownership hints, paths touched, service/component hints from repo structure

## 3.2 Slack data (from selected channels)

1. Channel metadata:
   1. channel identity, type, membership context
2. Conversation streams:
   1. messages, replies/threads, reactions, timestamps
3. Operational context:
   1. incident chatter, decision conversations, status updates
4. Collaboration patterns:
   1. who interacts with whom, where, and how often

## 3.3 Derived data Moontide will create from source data

1. Entity candidates:
   1. people, teams, repos, services, channels, projects, incidents
2. Relationship candidates:
   1. collaborates_with, owns, reviews_for, depends_on, participates_in
3. Claim candidates:
   1. facts, constraints, preferences, task context
4. Temporal signals:
   1. start/end of validity, supersession chains, recency windows
5. Provenance bindings:
   1. every claim linked to specific source evidence and timestamps

## 4. How we will get it

## 4.1 Resource discovery and selection

1. Connector integrations enumerate available repos/channels for the connected app installations.
2. User selects the exact resources to ingest.
3. Moontide stores this selection as the org’s active ingestion scope.

## 4.2 Bootstrap acquisition

1. Moontide runs bounded historical backfill for selected resources.
2. Data arrives as source events/documents into ingestion pipelines.
3. Pipelines process data with idempotent semantics so retries do not duplicate memory.

## 4.3 Ongoing acquisition

1. After bootstrap, Moontide keeps selected resources in continuous sync.
2. New source events are incrementally ingested and reconciled into existing memory.
3. Scope changes (new/removed repos or channels) update ingestion behavior going forward.

## 5. What we will do with the data

## 5.1 Transform source artifacts into memory units

1. Normalize source payloads into a shared internal event shape.
2. Resolve identities across sources (same person/repo/service across systems).
3. Extract candidate entities, relationships, and claims.
4. Attach provenance and confidence to each candidate.

## 5.2 Build temporal org memory

1. Merge accepted candidates into org memory graph/claim set.
2. Detect conflicts and represent them as supersession or competing claims, not destructive overwrite.
3. Maintain current truth plus historical truth with temporal fields.
4. Keep all writes auditable with actor and source lineage.

## 5.3 Make memory usable by users and agents

1. Expose memory in Memory Studio for human inspection and edits.
2. Expose memory to agents via shared org memory retrieval/write interfaces.
3. Apply access controls consistently for read and write paths.
4. Return provenance with retrieved memory so outputs remain explainable.

## 6. Where we will store data and memory

## 6.1 Storage layers and purpose

1. Source ingestion layer:
   1. stores connector-ingested source artifacts/events needed for replay and reconciliation
2. Canonical memory layer:
   1. stores org memory claims, relationships, temporal validity, and event/audit history
3. Derived retrieval layer:
   1. stores search/ranking artifacts used to improve recall/latency (rebuildable from canonical memory)
4. Evidence linkage layer:
   1. stores source references and excerpts needed for provenance drill-down

## 6.2 Source of truth rule

1. Canonical org memory is the source of truth for agent/user memory behavior.
2. Raw source data is evidence input and replay substrate.
3. Derived retrieval artifacts are accelerators, not truth.

## 6.3 Tenancy and access boundaries

1. All canonical memory is strictly segregated by Moontide org.
2. Access is enforced at retrieval and mutation time using org + actor permissions.
3. Agent service identity cannot exceed the creator’s effective access ceiling.

## 7. Runtime outputs a user should expect

After completing source selection and bootstrap, users should have:

1. An initial org memory graph populated from selected repos/channels.
2. A timeline-aware memory bank with provenance on key claims.
3. A review surface (Memory Studio) showing what was inferred and from where.
4. Agent sessions that can use the new shared memory immediately.

## 8. In-scope vs out-of-scope for this onboarding stage

## 8.1 In scope now

1. GitHub + Slack resource selection.
2. Bootstrap memory build from selected sources.
3. Continuous ingestion for selected resources.
4. Memory visibility/editability and agent usage.

## 8.2 Out of scope for this stage

1. Jira/Outlook/SharePoint ingestion.
2. Approval-gated governance workflows.
3. Fully autonomous org activation loops.
4. Cross-org memory sharing.

## 9. Practical framing

This onboarding phase is the transition from:

1. "Connector is authorized"

to:

1. "Org has an operational memory bank built from explicitly selected work surfaces, actively updated, and usable by both humans and agents."
