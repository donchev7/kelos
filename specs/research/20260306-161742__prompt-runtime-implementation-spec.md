# Moontide AI - Prompt Runtime Implementation Spec

**Date:** 2026-03-06  
**Status:** Ready for implementation  
**Scope:** BFF runtime only (prompt behavior for `slack_thread` and `background_triggered`)

---

## 1. Objective

Replace legacy fixed prompting with a deterministic prompt runtime that is driven by each agent definition.

Primary requirement:

1. `slack_thread` agents must use their own custom prompt context from user config (`name`, `objective`, `instructions`, runtime policy context), not the predefined Codebase Q&A thread prompt.

---

## 2. Problem Statement

Current behavior has a product mismatch:

1. `background_triggered` runs already include agent-specific objective/instructions in prompt construction.
2. `slack_thread` runs still use a mostly fixed thread prompt in `buildThreadPrompt(...)` with generic codebase Q&A framing.
3. This makes Agent Factory appear configurable while thread behavior still feels hardcoded.

Code references:

1. `apps/bff/src/agents/code-search-runtime.ts` (`buildThreadPrompt`).
2. `apps/bff/src/agents/codebase-qa-runtime-core.ts` (`askQuestionFromSlack` -> `answerQuestionFromThreadSession`).
3. `apps/bff/src/agents/agent-run-core.ts` (`buildRunPrompt` for background).

---

## 3. Non-Goals

1. No FE/UI schema expansion in this spec.
2. No new model/provider abstraction in this spec.
3. No output-template/section enforcement redesign in this spec.
4. No Temporal adoption in this spec.

---

## 4. Target Product Behavior

## 4.1 Slack Thread Prompt Behavior

1. First turn for a new runtime session must bootstrap with:
   - platform baseline rules,
   - agent identity (`name`),
   - agent objective,
   - agent instructions,
   - repo scope mapping,
   - runtime constraints summary (read-only, allowed actions summary).
2. Follow-up turns in same runtime session must send only sanitized user message plus minimal turn metadata.
3. Follow-up turns must be pure user-message payload after existing filtering/dedupe/mention-stripping logic. No additional prompt wrapper/header is added.
4. Full prompt replay is allowed only for re-bootstrap conditions:
   - OpenCode session missing/stale,
   - sandbox/session recreation,
   - agent config version change.
5. When re-bootstrap occurs, post a Slack notice in-thread that context was refreshed.

## 4.2 Background Prompt Behavior

1. Background runs continue to include trigger metadata and repository targeting.
2. Background runs must use the same shared prompt assembly primitives as thread bootstrap for consistency.

## 4.3 Prompt Source of Truth

1. Prompt composition inputs come from typed persisted agent definition fields.
2. No hardcoded product persona text like "You are a read-only codebase Q&A agent..." in thread path.

---

## 5. Prompt Architecture Design

## 5.1 New Prompt Builder Module

Create a dedicated module, for example:

1. `apps/bff/src/agents/prompt-runtime.ts`

Responsibilities:

1. Build thread bootstrap prompt.
2. Build background run prompt.
3. Build follow-up user-turn payload (lightweight).
4. Expose prompt version/hash helpers for observability.

## 5.2 Canonical Prompt Layers

Each bootstrap prompt is composed in this order:

1. `Platform Layer`
   - Stable operational rules (no fabrication, evidence preference, concise answer style).
2. `Agent Layer`
   - `Agent name`
   - `Objective`
   - `Instructions`
3. `Scope Layer`
   - allowlisted repositories and local paths.
4. `Runtime Layer`
   - tool/permission/network summary in human-readable form.
5. `Context Layer`
   - trigger details (background) or recent thread summary (thread bootstrap only).
6. `Turn Layer`
   - latest user message.

## 5.3 Follow-Up Turn Strategy

1. Add a `sessionBootstrapped` decision in runtime flow.
2. If bootstrapped and session valid, call OpenCode with only the latest sanitized user turn text (no additional wrapper text).
3. Do not include full conversation transcript in normal follow-up flow.
4. If re-bootstrap condition is hit, send full layered bootstrap prompt once.
5. After re-bootstrap, emit a user-facing Slack notice in the thread.

---

## 6. Required Runtime Changes

## 6.1 `codebase-qa-runtime-core.ts`

1. Pass agent prompt inputs (`definition.name`, `definition.objective`, `definition.instructions`) into thread answer call.
2. Pass `definition.configVersion` (or equivalent) for bootstrap invalidation checks.

## 6.2 `code-search-runtime.ts`

1. Remove direct use of legacy `buildThreadPrompt(...)` fixed persona.
2. Integrate new prompt builder module.
3. Add bootstrap/follow-up branching:
   - bootstrap prompt on new/invalid session,
   - lightweight turn message on existing valid session.
4. Keep session validation logic (`resolveOpenCodeSessionId`) as bootstrap gate.

## 6.3 `agent-run-core.ts`

1. Replace local `buildRunPrompt(...)` internals with shared prompt builder primitives.
2. Preserve existing background trigger payload coverage.

## 6.4 Runtime Message Hygiene

1. Ensure sanitized Slack text is used for turn payload.
2. Ensure bot mention stripping happens before prompt/turn assembly.
3. Ensure no raw internal scaffolding is posted back to Slack.

---

## 7. Observability Changes

## 7.1 Run Metadata

For each run, record:

1. prompt mode: `bootstrap` or `followup`.
2. prompt version string.
3. prompt hash (hash of compiled prompt text for bootstrap, hash of turn payload for follow-up).

## 7.2 Logging

Add structured logs:

1. `prompt-bootstrap-built`
2. `prompt-followup-built`
3. `prompt-rebootstrap-reason` (`session_missing`, `session_stale`, `config_version_changed`)

Do not log full prompt text.

---

## 8. Data and State Contract

## 8.1 Runtime Session State

Extend runtime session state payload to include:

1. `promptVersion`
2. `agentConfigVersionAtBootstrap`
3. `bootstrappedAt`

This allows deterministic re-bootstrap decisions.

## 8.2 Backward Compatibility

1. Existing sessions without these fields are treated as unbootstrapped and re-bootstrap once.

---

## 9. TDD Plan (Strict Order)

## 9.1 Prompt Builder Unit Tests (new)

1. Builds thread bootstrap prompt with agent name/objective/instructions.
2. Includes repository scope lines.
3. Includes runtime policy summary lines.
4. Builds follow-up turn payload without full transcript.
5. Produces stable prompt hash for same input.

## 9.2 Thread Runtime Tests

1. New session -> bootstrap mode used.
2. Existing valid session -> follow-up mode used.
3. Config version change -> re-bootstrap mode used.
4. Session missing/stale -> re-bootstrap mode used.
5. Legacy fixed "read-only codebase Q&A agent" text is absent from compiled thread prompt.
6. Follow-up payload equals sanitized user text only.
7. Re-bootstrap posts Slack notice once per re-bootstrap event.

## 9.3 Background Runtime Tests

1. Background prompt includes agent objective/instructions via shared module.
2. Trigger payload still present.
3. Prompt builder parity with thread builder layer structure.

## 9.4 Regression Tests

1. Existing Slack thread E2E tests still pass.
2. Permission approval flow tests still pass.
3. Delivery target behavior tests still pass.

---

## 10. Manual E2E Validation

1. Create two active `slack_thread` agents with very different objectives.
2. Start thread with agent A and ask a question.
3. Verify answer style/content reflects agent A instructions.
4. Continue thread and verify follow-up replies remain consistent without prompt leakage.
5. Start new thread with agent B and verify behavior shifts to B-specific objective/instructions.
6. Update an agent instruction, re-run in same thread, verify one-time re-bootstrap behavior.

---

## 11. Acceptance Criteria

1. Thread runtime no longer relies on predefined Codebase Q&A prompt text.
2. Thread runtime behavior is visibly driven by per-agent config.
3. Follow-up turns in valid sessions send only sanitized user text and do not replay full prompt.
4. Re-bootstrap is deterministic, auditable, and visible via Slack notice.
5. Background and thread prompt assembly share one canonical builder path.

---

## 12. Risks and Mitigations

1. Risk: weaker behavior consistency if follow-up prompts are too minimal.
   Mitigation: retain platform baseline in bootstrap and trigger re-bootstrap on drift signals.
2. Risk: regressions in legacy tests expecting old prompt text.
   Mitigation: rewrite tests to assert behavior contract, not literal legacy phrasing.
3. Risk: prompt drift across code paths.
   Mitigation: single prompt module with strict unit coverage.

---

## 13. Implementation Checklist

1. Add new prompt runtime module.
2. Replace thread fixed prompt path with agent-driven bootstrap/follow-up path.
3. Refactor background prompt path to shared builder.
4. Add runtime session bootstrap metadata.
5. Add prompt observability fields/logs.
6. Update tests per TDD plan and run full quality gates.
