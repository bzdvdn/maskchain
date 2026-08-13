---
report_type: inspect
slug: 402-zero-retention-mode
status: concerns
docs_language: en
generated_at: 2026-08-13
---

# Inspect Report: 402-zero-retention-mode

## Scope

- snapshot: Deep review of the zero-retention-mode spec (per-tenant `full|meta|none` conversation-log retention, global default, UI mode control + masked/not-masked request-log filter) against the constitution and the existing conversation/tenant surfaces.
- artifacts:
  - .speckeep/constitution.summary.md
  - specs/active/402-zero-retention-mode/spec.md

## Verdict

- status: concerns

The spec is structurally sound (10 ACs, 8 RQs, Given/When/Then present, no placeholders, English-compliant, single feature). Two findings are non-blocking but must be reflected when the spec/plan is written: an internal inconsistency in the `none`-mode semantics of blocked anomalies, and an unquantified success criterion.

## Errors

- none

## Warnings

- **W1 — AC-004 + Scope (line 39): `none`-mode blocked-anomaly recording is internally contradictory.** AC-004's Given states "a tenant with mode `meta` (or `none`)" and the Scope bullet says "Blocked anomalies in `meta`/`none` are represented only by detector + category". This directly conflicts with RQ-006 ("MUST NOT create conversation log rows") and AC-003 ("zero conversation log rows exist") for `none` — a row cannot be recorded in a mode that writes no rows, and the recorded decision "none = no rows" (metadata only in `meta`, truly nothing in `none`) makes `none` trace-free. Fix: restrict AC-004 and the Scope bullet to `meta` only, and add an explicit statement (Spec Assumptions or Edge Cases) that in `none` mode even a blocked anomaly leaves no record apart from aggregate counters.
- **W2 — SC-002: "within noise" is unmeasurable.** The p99 baseline is a good anchor but "unchanged within noise" has no tolerance. Fix: quantify, e.g. "p99 proxy latency delta < 100 ms or within ±5% of the pre-change baseline".

## Questions

- none

## Suggestions

- LiteLLM appears in Context/First Deployable Outcome/Open Questions as a UI reference for the request-log page. This was explicitly user-provided ("like request logs in litellm") and is a UX benchmark, not a dependency or a technology pin — no action needed, but keep it as a reference only in plan.
- The `ConversationLog` non-empty-request invariant that `meta` must relax is already acknowledged in Context — ensure it lands as a concrete decision (a metadata-only variant vs. relaxed validation) in plan.

## Traceability

- Acceptance criteria AC-001..AC-010 map to RQ-001..RQ-008. No tasks.md exists yet, so plan-level AC→task coverage is not verifiable at this phase.
- W1 affects AC-004 and RQ-006/AC-003 semantics; W2 affects SC-002.

## Next Step

- Apply the two trivial refinements (scope AC-004/Scope to `meta`; quantify SC-002) and proceed to planning. The `none`-mode blocked-anomaly semantics (no trace) is already decided and must be encoded as such.