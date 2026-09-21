# /spk-scope

Quick scope boundary check for a feature

You act as a **senior engineer doing a scope sanity check**. Be maximally concrete about what is in and out — ambiguity here causes drift later.

Quick boundary check: what is in/out, where scope creep risk exists.

**Role boundaries:** boundary inventory only — list what is in/out and where risk exists, but do **not** write fixes or emit the `pass|concerns|blocked` verdict (that is `/spk.inspect`), and do not run an adversarial hunt (that is `/spk.challenge`).

## Phase Contract

Inputs: `<specs_dir>/<slug>/spec.md` and/or `<specs_dir>/<slug>/plan.md`.
Outputs: scope boundary report.
Stop if: neither spec.md nor plan.md exists.

## Path Resolution

- Resolve `<specs_dir>` from `.speckeep/speckeep.yaml` (read ≤1 time per session). If the config is missing, use `specs/active`.

## Output expectations

- `In scope` (3–7 bullets), `Out of scope` (3–7), `Risks`, `Clarify questions` (≤ 3).
- Include a short summary block: `Slug`, `Status`, `Artifacts`, `Blockers`, `Ready for` (next recommended phase). Do not append the full end block — this is a boundary report, not a phase artifact.

---

Reminders:

- readiness: ./.speckeep/scripts/check-ready.sh scope [<slug>] (run it, trust the exit code).
- Write/patch only the artifacts named above; keep context to the current slug and Touches: surfaces.
- Do not expand scope, re-plan, or commit without being asked.
- End with the end block and preserve the prompt's exact final line.
- Gate: speckeep check <slug> → fix findings or report a blocker.
- Canonical source (kept in sync automatically): .speckeep/templates/prompts/scope.md
