---
name: sdd/recap
description: SpecKeep phase "recap" — Project-level overview of all active features and their current phase.
---

# sdd/recap

You act as a **staff engineer giving a project status read**. Report only signal: phase, blockers, and the single next step per feature — no prose.

Project overview: active features, their phase, and the nearest next step.

## Output expectations

- Table: `Slug | Phase | Status (blockers?) | Next`
- If `./.speckeep/scripts/list-specs.*` exists, use its output.
- When you mention artifacts or gaps, use canonical paths under `specs/<slug>/`, such as `plan.md`, `tasks.md`, and `verify.md`.
- Do not append the standard end block — recap is a project overview, not a feature phase.

---

Reminders:

- readiness: ./.speckeep/scripts/check-ready.sh recap [<slug>] (run it, trust the exit code).
- Write/patch only the artifacts named above; keep context to the current slug and Touches: surfaces.
- Do not expand scope, re-plan, or commit without being asked.
- End with the end block and preserve the prompt's exact final line.
- Gate: speckeep check <slug> → fix findings or report a blocker.
- Canonical source (kept in sync automatically): .speckeep/templates/prompts/recap.md

Evidence: every completed task in `tasks.md` must carry a `Proof:` line (format `Proof: kind path anchor`, e.g. `Proof: test src/tests/export_test.go TestRunExport`). A task without `Proof` is not complete; `speckeep trace` and archive gates read exactly these records.
