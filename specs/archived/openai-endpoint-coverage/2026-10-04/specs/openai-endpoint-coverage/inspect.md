---
report_type: inspect
slug: openai-endpoint-coverage
status: concerns
docs_language: en
generated_at: 2026-10-04
---

# Inspect Report: openai-endpoint-coverage

## Scope

- snapshot: deep quality review of the JSON-text endpoint coverage spec before planning.
- artifacts:
  - CONSTITUTION.md
  - specs/active/openai-endpoint-coverage/spec.md
- code spot-checks (to verify concrete claims): `src/internal/api/middleware/model_access.go`, `src/internal/api/middleware/budget.go`, `src/internal/api/self_handler.go`, `src/internal/adapters/provider/upstream.go`.

## Verdict

- status: concerns
- No blockers. The spec is coherent, single-feature, and planable. The findings below are refinements that should be folded into the spec or resolved at plan time.

## Errors

- none

## Warnings

- W-001 (AC-001) `created` field has no defined value. The route registry has no timestamps, so a test cannot assert a specific value. Fix: pin `created: 0` in Assumptions and the AC evidence, or drop the field from the AC.
- W-002 (RQ-006, AC-007) "every new endpoint" includes `GET /v1/models`, which makes no provider call and incurs no spend. A pre-request hard-budget 429 on a discovery endpoint is likely unintended. Fix: scope RQ-006/AC-007 to the provider-calling endpoints (moderations/rerank/count_tokens); state explicitly that `GET /v1/models` is auth-only.
- W-003 (RQ-009, AC-004) Assumes rerank responses never echo the documents they were given. Some providers return the document text; a passthrough would expose placeholders (not the original) to the client, which is safe but surprising. Fix: state the assumption explicitly or add "unmask echoed documents" to scope/AC-004.
- W-004 (AC-003) The outcome depends on a still-open question (mask vs opt-out for moderations input). AC-003 is only valid if masking is the default; if operators need to moderate the original text this AC changes. Fix: confirm the default before implement, or make masking configurable and have AC-003 assert the configured behavior.
- W-005 (SC-001) "<15% p95 over direct provider call" lacks a defined baseline (which provider, which payload). Not measurable as written. Fix: name the baseline (e.g., mock provider) or move the number to the plan's performance budget.
- W-006 (Assumptions) "the detector pipeline can mask a plain string field without chat context" is plausible but unverified from the spec: the current shield middleware is chat-shaped (`messages`). Fix: confirm at plan time that a non-chat input-shield is feasible; if not, this becomes a scope/design change rather than an assumption.

## Questions

- W-001/W-004/W-003 mirror the spec's Open Questions; resolve `owned_by`/`created`, the `/api/v1/models` shape, and the moderations masking default.

## Suggestions

- Verified in code, no new work needed: model-access already returns 403 `MODEL_ACCESS_DENIED` and budget returns 429 `BUDGET_EXCEEDED`, matching AC-006/AC-007; the `model` field is present in moderations/rerank/count_tokens bodies so the existing middleware applies.
- `upstreamPathFor` in `src/internal/adapters/provider/upstream.go` must gain `/v1/moderations`, `/v1/rerank`, and `/v1/messages/count_tokens`; call this out in the plan's surfaces.
- Reuse the embeddings-style dedicated input-shield chain (skips session/conversation/cache/SSE) rather than the chat shield.

## Traceability

- AC-001..AC-009 each have Given/When/Then with observable evidence; no AC depends on an undefined artifact except for the noted `created` value (W-001).
- RQ-001..RQ-009 map 1:1 to ACs; no orphan requirements observed.
- tasks.md not present yet; AC→task coverage to be checked after `/spk-plan` + `/spk-tasks`.

## Next Step

- Safe to continue to plan; fold W-001..W-006 into the plan (or patch the spec for W-001/W-002/W-005 first).
