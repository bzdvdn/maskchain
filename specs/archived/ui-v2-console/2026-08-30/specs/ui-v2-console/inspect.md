---
report_type: inspect
slug: ui-v2-console
status: pass
docs_language: en
generated_at: 2026-08-30
---

# Inspect Report: ui-v2-console

## Scope

- snapshot: deep quality review of the UI v2 Console spec — design-system foundation, Operations HQ dashboard, Keys product page, Analytics/Routing/Tenant/Settings redesigns, mostly frontend-only over existing APIs (single read-only status endpoint exception).
- artifacts:
  - CONSTITUTION.md (via `.speckeep/constitution.summary.md`)
  - specs/active/ui-v2-console/spec.md
  - specs/active/ui-v2-console/inspect.md (this report)
- baseline: `check-ready.sh inspect` and `inspect-spec.sh` both exit 0 (errors=0, warnings=0); 11 RQ ↔ 11 AC, all AC carry Given/When/Then with observable outcomes.

## Verdict

- status: pass

## Errors

- none

## Warnings

- none — all prior warnings (W-1..W-3) resolved by product decisions recorded in spec Open Questions:
  - W-1 constitution scope → **decision: control-plane console**; aligned via `/spk.constitution` (CONSTITUTION.md v1.4.0) — completed 2026-08-30.
  - W-2 workspace switcher → **decision: real scoping**; RQ-003 updated to require actual-data scoping, mechanism deferred to plan.
  - W-3 Settings data source → **decision: yes**; a minimal read-only server/status endpoint is allowed (RQ-009, Non-Goals carve-out, Scope bullet).
  - W-4 wording → success is measured through SC-001..003 only.

## Resolved Decisions

- OQ-1: UI is the operator control-plane console; constitution line aligned via `/spk.constitution` (CONSTITUTION.md v1.4.0) — done 2026-08-30.
- OQ-2: minimal read-only server/status endpoint (uptime, version, key-at-rest) allowed for Settings; sole backend addition.
- OQ-3: workspace/tenant switcher performs real data scoping of Dashboard/Analytics/Budgets; mechanism (tenant param vs client-side partition) chosen in plan.
- OQ-4: dark-first for the v2 pass; existing light mappings preserved, not redesigned.

## Questions

- none.

## Suggestions

- S-1: keep the MVP slice exactly as scoped (foundation + Dashboard + Keys, AC-001..006); land the design-system migration pass before page work so each subsequent page reuses stable primitives.
- S-2: give the Keys tag-input a place in plan for an accessibility check (keyboard entry of chips) since AC-005 depends on it.
- S-3: relative-time utility should take an absolute ISO timestamp and produce both the relative label and a locale-neutral absolute tooltip in one call, so AC-011 is testable as a single unit.
- S-4: before plan, confirm mechanism owner for RQ-003 real scoping (backend tenant param scope vs client-side partition) and record the constitutional follow-up in the feature's archive checklist.

## Traceability

- No plan.md/tasks.md exist yet. AC coverage mapping is 1:1 with RQ-001..RQ-011; each AC names an observable outcome. When tasks land, every AC must be covered by ≥1 task and every completed task must carry a `Proof:` line.

## Next Step

- safe to proceed to plan; plan must record: real-scoping mechanism for RQ-003, the read-only status endpoint surface (RQ-009), and the constitutional follow-up as an archive-checklist item (non-blocking).