---
report_type: inspect
slug: mask-token-format
status: pass
docs_language: en
generated_at: 2026-08-30
---

# Inspect Report: mask-token-format

## Scope

- snapshot: deep quality review of the mask-token-format spec — clean (id-free) default tokens for `/mask`, `format=clean|id|redact` parameter, sequential per-doc unmask with legacy backward compatibility. Standalone mask domain + handler only; conversation/proxy shield tokens untouched.
- artifacts:
  - CONSTITUTION.md (via `.speckeep/constitution.summary.md`)
  - specs/active/mask-token-format/spec.md
- baseline: `check-ready.sh inspect` and `inspect-spec.sh` both exit 0 (errors=0, warnings=0 after wording fix); 7 RQ ↔ 7 AC, all AC carry Given/When/Then with observable outcomes.

## Verdict

- status: pass

## Errors

- none

## Warnings

- none. The default-change risk for existing `/mask` consumers (output tokens change when `format` is omitted) is an accepted, recorded product decision: spec Assumptions + Open Questions state `clean` is the new default and `format=id` is the migration path — no silent-change without choice.

## Questions

- none beyond what the spec records (redact mode and param naming were the open items — both resolved into the spec as `format=redact` and a single `format` param).

## Suggestions

- S-1: in plan, model the token style as a mask-domain enum (`FormatClean | FormatID | FormatRedact`) threaded through `MaskFromResults`, defaulting to `clean` at the handler when the param is absent — keeps the domain testable without HTTP.
- S-2: give `MaskEntry` a `Reversible` field (JSON-additive, defaulting `true` so legacy entries stay restorable); `redact` entries store `false` so `/unmask` can return 400 "not reversible" instead of 404 for known docs.
- S-3: keep `UnmaskText` per-document sequential application as the only semantic; document the single known limitation (a body mixing tokens from two different docs) in `plan` and the handler docs.

## Traceability

- No plan.md/tasks.md yet. AC coverage is 1:1 with RQ-001..RQ-007, each AC naming an observable outcome. When tasks land, every AC must be covered by ≥1 task and every completed task must carry a `Proof:` line.

## Next Step

- safe to proceed to plan; plan must record: the token-style enum, the `Reversible` field addition (data-model note), sequential unmask semantics, and the `format` param validation at the handler boundary.