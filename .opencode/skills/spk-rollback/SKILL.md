---
name: spk-rollback
description: SpecKeep phase "rollback" — Roll back completed tasks for a feature, returning them to unfinished state.
---

# /spk-rollback

You act as a **release engineer**. Change only the declared state (checkboxes and, when asked, code) with full transparency about what was reverted.

You roll back completed tasks for one feature, returning them to unfinished state.

## Phase Contract

Inputs: `<specs_dir>/<slug>/tasks.md` (required).
Outputs: updated tasks.md with requested tasks unmarked as incomplete.
Stop if: slug is missing, tasks.md does not exist, or no completed tasks exist.

## Rules

- Read `<specs_dir>/<slug>/tasks.md` and list all completed `[x]` tasks grouped by phase, with their `Touches:` surfaces.
- Ask the user which tasks to roll back (by ID like `T1.1,T1.2` or `all`). If the user specifies a phase (e.g., `phase T1`), roll back all tasks in that phase.
- For each rolled-back task:
  1. Change `[x]` to `[ ]` in tasks.md.
  2. Do NOT revert code changes automatically — the user may want to keep the code.
  3. If the user also asks to revert code, use `git checkout -- <file>` on each Touches: file for those tasks.
- After rollback, the feature phase state reverts to `implement`.
- Do not touch `[ ]` tasks — only roll back `[x]` tasks.

## Output expectations

- List affected tasks per task ID: whether checkbox was reverted and/or code was reverted.
- Show updated task state: `completed=<n>`, `open=<n>`.
- If code was reverted, list the git-checkout commands run.
- End with standard end block (see AGENTS.md), exact shape:
  ```
  Slug: <slug>
  Status: <phase label>
  Artifacts: <paths>
  Blockers: <none | reason>
  Ready for: /spk-implement <slug>
  ```
- The `Ready for:` line above is the mandatory final line — end with it.

---

Reminders:

- Write/patch only the artifacts named above; keep context to the current slug and Touches: surfaces.
- Do not expand scope, re-plan, or commit without being asked.
- End with the end block and preserve the prompt's exact final line.
- Gate: speckeep check <slug> → fix findings or report a blocker.
- Canonical source (kept in sync automatically): .speckeep/templates/prompts/rollback.md

Evidence: every completed task in `tasks.md` must carry a `Proof:` line (format `Proof: kind path anchor`, e.g. `Proof: test src/tests/export_test.go TestRunExport`). A task without `Proof` is not complete; `speckeep trace` and archive gates read exactly these records.
