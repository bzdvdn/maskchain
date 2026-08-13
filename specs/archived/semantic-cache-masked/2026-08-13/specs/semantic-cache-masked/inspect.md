---
report_type: inspect
slug: semantic-cache-masked
status: pass
docs_language: en
generated_at: 2026-08-12
---

# Inspect Report: semantic-cache-masked

## Scope

- snapshot: inspection of the semantic cache over masked data spec — constitution compliance, AC completeness, unambiguity, single feature.
- artifacts:
  - .speckeep/constitution.summary.md
  - specs/active/semantic-cache-masked/spec.md
  - .speckeep/templates/spec.md (structural reference)

## Verdict

- status: pass

## Errors

- none

## Warnings

- Original AC-007 combined two scenarios (disable + budget-safe) with an "And" marker — violates "one AC = one feature". Split into AC-007 (disable) and AC-008 (budget-safe write). Verified after the fix: readiness 8 AC, errors=0.
- Typos in Evidence/Edge Cases ("сквозняка" → "прогон", "манальное" → "ручное", "Emb ТЕД" → "Embedding") — fixed in the Russian draft; artifact translated to English per docs=en (speckeep.yaml).
- Similarity threshold left as an open question — now fixed as 0.90 (cosine) default in Assumptions and closed. Empirical calibration moved to success criteria.

## Questions

- Streaming cache: variant A (non-streaming only) is the product default, confirmed in spec (RQ-010). Remains the only open question; does not affect planning.

## Suggestions

- Config keys to finalize in plan: `data.cache.similarity_threshold` (default 0.90), `data.cache.budget_guard_percent` (default 5) — already reflected in plan.md and data-model.md.

## Traceability

- AC-001..AC-008 — all with Given/When/Then/Evidence; each leads to an observable outcome.
- RQ-001..RQ-010 — 10 requirements closing 8 AC; RQ-010 (streaming passthrough) reflected in Assumptions and Open Questions.
- plan/tasks absent — plan↔spec↔tasks cross-check not required.

## Next Step

- safe to continue to plan
