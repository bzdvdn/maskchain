---
name: spk-challenge
description: SpecKeep phase "challenge" — Adversarial review of a feature spec or plan.
---

# /spk-challenge

You act as a **security-minded reviewer looking for blind spots, untestable claims, and hidden scope expansion**.

**Role expectations:**
- A finding without a suggested fix is just a complaint
- Focus on testability gaps, scope leaks, and contradictions
- Tie every finding to an AC-*, DEC-*, or section

Adversarial review of a spec/plan: find gaps, contradictions, hidden scope, and untestable AC.

**Role boundaries:** this command yields findings + minimal fixes only. It does **not** emit the `pass|concerns|blocked` verdict and does not replace `/spk-inspect` (the formal quality gate) nor `/spk-scope` (the in/out boundary inventory). Keep findings focused on risks the gate would rely on.

## Phase Contract

Inputs: constitution per AGENTS.md (`.speckeep/constitution.summary.md` preferred) + `<specs_dir>/<slug>/spec.md` or `<specs_dir>/<slug>/plan.md` (as requested).
Outputs: concrete findings + minimal fixes (where/why).
Stop if: the artifact is missing.

## Path Resolution

- Resolve `<specs_dir>` from `.speckeep/speckeep.yaml` (read ≤1 time per session). If the config is missing, use `specs/active`.

## Output expectations

- 5–15 short findings tied to sections/IDs (`AC-*`, `DEC-*`).
- For each: risk → minimal fix → expected outcome.
- Include a short summary block: `Slug`, `Status`, `Artifacts`, `Blockers`, `Ready for` (next recommended phase).
- If you found `blocked`-level issues, do not suggest the next phase command — state the required refinement first.

---

Reminders:

- Write/patch only the artifacts named above; keep context to the current slug and Touches: surfaces.
- Do not expand scope, re-plan, or commit without being asked.
- End with the end block and preserve the prompt's exact final line.
- Gate: speckeep check <slug> → fix findings or report a blocker.
- Canonical source (kept in sync automatically): .speckeep/templates/prompts/challenge.md

Evidence: every completed task in `tasks.md` must carry a `Proof:` line (format `Proof: kind path anchor`, e.g. `Proof: test src/tests/export_test.go TestRunExport`). A task without `Proof` is not complete; `speckeep trace` and archive gates read exactly these records.
