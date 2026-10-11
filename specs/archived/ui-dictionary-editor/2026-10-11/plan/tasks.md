# Tenant Dictionary Editor Tasks

## Phase Contract

Inputs: `specs/active/ui-dictionary-editor/spec.md` (no plan; express lane).
Outputs: ordered executable tasks with coverage mapping.
Stop if: tasks would be vague or acceptance coverage cannot be mapped.

## Surface Map

| Surface | Tasks |
|---------|-------|
| ui/src/components/DictionaryEditor.tsx | T1.1, T2.1 |
| ui/src/components/DictionaryEditor.test.tsx | T3.1 |
| ui/src/pages/Tenants/TenantForm.tsx | T2.1 |
| ui/src/pages/Tenants/TenantForm.test.tsx | T3.2 |
| ui/ | T3.3 |

## Implementation Context

- MVP Goal: add/edit/remove tenant dictionaries in `TenantForm` and persist via the existing tenant create/update `dictionaries` field.
- Acceptance Boundaries: AC-001, AC-002, AC-003, AC-004, AC-005.
- Key Rules: UI-only change; reuse `DictionaryItem` and the existing save path; no new endpoint or domain change; no comments in code; keep the v2 design system.
- Data/Domain Invariants: dictionary = `{ name, entries, match_mode }`; `match_mode` ∈ `exact|contains|regex|fuzzy`; `entries` may be strings or JSON-serializable values.
- Contracts/Protocols: `createTenant`/`updateTenant` already send `dictionaries`; `updateDictionaries()` stays unused unless a future task calls it.
- Proof Signals: component tests asserting payloads and validation; `npx vitest run` + `npm run typecheck` clean.
- Data model: no change.
- Out of Scope: Shield dictionaries page, API/domain changes, detector binding, import/export.

## Phase 1: Editor component

Goal: a self-contained, controlled editor with round-trip entries serialization.

- [x] T1.1 Implement `DictionaryEditor` (controlled `value`/`onChange` of `DictionaryItem[]`) with add/remove dictionary, name input, match-mode select, and a per-line entries textarea; include `entriesToLines`/`linesToEntries` helpers (JSON-parse with string fallback). Touches: ui/src/components/DictionaryEditor.tsx
  Proof: code ui/src/components/DictionaryEditor.tsx entriesToLines

## Phase 2: Wire into the tenant form

Goal: the editor drives create and edit saves with validation.

- [x] T2.1 Integrate `DictionaryEditor` into `TenantForm` bound to `form.dictionaries`, and add client validation (non-empty unique name, ≥1 entry) that blocks save with an inline error when invalid. Touches: ui/src/pages/Tenants/TenantForm.tsx, ui/src/components/DictionaryEditor.tsx
  Proof: code ui/src/pages/Tenants/TenantForm.tsx validate

## Phase 3: Validation

Goal: prove the behavior and leave the package reviewable.

- [x] T3.1 Unit-test the editor: add/remove dictionary, structured-entry round-trip, and the (de)serialization helpers. Touches: ui/src/components/DictionaryEditor.test.tsx
  Proof: test ui/src/components/DictionaryEditor.test.tsx entriesToLines
- [x] T3.2 Integration-test `TenantForm`: create includes dictionaries (AC-001), edit persists changes (AC-002), invalid input blocks save (AC-004), removal drops the dictionary (AC-005). Touches: ui/src/pages/Tenants/TenantForm.test.tsx
  Proof: test ui/src/pages/Tenants/TenantForm.test.tsx
- [x] T3.3 Run `npx vitest run` + `npm run typecheck`; fix regressions. Touches: ui/
  Proof: test ui/src/pages/Tenants/TenantForm.test.tsx

## Acceptance Coverage

- AC-001 -> T2.1, T3.2
- AC-002 -> T2.1, T3.2
- AC-003 -> T1.1, T3.1
- AC-004 -> T2.1, T3.2
- AC-005 -> T2.1, T3.2

## Notes

- Prefer a `DictionaryEditor` component over growing `TenantForm` inline, so it can be reused later by the Shield page if that is ever scheduled.
- The entries textarea keeps one entry per line; a valid JSON line becomes a structured entry, everything else a string.
- Every `[x]` task needs a `Proof:` line on the next line; no proof means not done.
