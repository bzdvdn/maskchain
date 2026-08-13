---
description: Roll back completed tasks for a feature, returning them to unfinished state
argument-hint: [request]
---

Follow ".speckeep/templates/prompts/rollback.md".

Command: `/spk.rollback [request]`

User arguments:
{{arguments}}

Requirements:
- read project.constitution_file (default: CONSTITUTION.md) first when the prompt requires it
- If the phase needs constitution context, load `.speckeep/constitution.summary.md` first when it exists; fall back to `project.constitution_file` only when the summary is absent.
- Evidence: every completed task in `tasks.md` must carry a `Proof:` line (format `Proof: kind path anchor`, e.g. `Proof: test src/tests/export_test.go TestRunExport`). A task without `Proof` is not complete; `speckeep trace` and archive gates read exactly these records.
- use only the minimum repository context needed
- Preserve the exact final line from the prompt file: `Ready for: ...` or `Return to: ...` with no paraphrase and no omission.

- Scripts to execute:
  - `./.speckeep/scripts/list-open-tasks.sh <slug>`
