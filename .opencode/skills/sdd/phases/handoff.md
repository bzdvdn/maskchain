---
name: sdd/handoff
description: SpecKeep phase "handoff" — Generate a session handoff document for one feature.
---

# sdd/handoff

You act as a **senior engineer handing off to a fresh session**. Write so precisely that the next session can resume with zero guesswork.

Create a short handoff for one feature.

## Phase Contract

Inputs: current phase (state), `<specs_dir>/<slug>/tasks.md`, recent changes (files/commands if known).
Outputs: handoff summary.
Stop if: tasks.md is missing.

## Path Resolution

- Resolve `<specs_dir>` from `.speckeep/speckeep.yaml` (read ≤1 time per session). If the config is missing, use `specs/active`.

## Output expectations

- `Slug`, `Phase`, `What changed`, `Open tasks`, `Blockers`, `Next command`.
- Final line (detect phase from state; resolve `workflow.verify` per the **Verify gate policy** in AGENTS.md):
  - If blocked: `Return to: /spk.<phase> <slug>`
  - If ready for next phase: `Ready for: /spk.<next> <slug>`
  - If all done and `workflow.verify: required`: `Ready for: /spk.verify <slug>`
  - If all done and `workflow.verify` is `optional`/absent: `Ready for: speckeep archive <slug> .`

---

Reminders:

- readiness: ./.speckeep/scripts/check-ready.sh handoff [<slug>] (run it, trust the exit code).
- Write/patch only the artifacts named above; keep context to the current slug and Touches: surfaces.
- Do not expand scope, re-plan, or commit without being asked.
- End with the end block and preserve the prompt's exact final line.
- Gate: speckeep check <slug> → fix findings or report a blocker.
- Canonical source (kept in sync automatically): .speckeep/templates/prompts/handoff.md

Evidence: every completed task in `tasks.md` must carry a `Proof:` line (format `Proof: kind path anchor`, e.g. `Proof: test src/tests/export_test.go TestRunExport`). A task without `Proof` is not complete; `speckeep trace` and archive gates read exactly these records.
