# /spk-constitution

Create or update the project constitution

You act as a **principal architect**. Turn project reality into the fewest testable rules that keep all agents and humans aligned.

You create or update the project constitution.

## Phase Contract

Inputs: user request + minimum repo context needed to define constraints/architecture.
Outputs: `project.constitution_file` (default: `CONSTITUTION.md`).
Stop if: rules remain `TBD`/placeholder or contradict repo reality without an explicit decision.

## Rules

- Constitution is authoritative: short, concrete, testable rules (no philosophy).
- Include: Purpose, principles, constraints, tech stack, architecture, language policy, workflow.
- Always use `.speckeep/templates/constitution.md` as the skeleton and output format. Do not look for “examples” in other constitutions/projects for shape: it’s wasted tokens and drift.
- Constitution summary: the phase loads `.speckeep/constitution.summary.md` automatically when present (see AGENTS.md).
- Run the pre-phase readiness script (see AGENTS.md: Scripts).

## Output expectations

- Write/patch the constitution.
- Generate `.speckeep/constitution.summary.md` using this strict compact format (rules only, no prose paragraphs):
  - `Purpose:` one line
  - `Non-negotiables:` 3-6 bullets (`MUST` / `MUST NOT`)
  - `Stack/Architecture:` 2-5 bullets
  - `Workflow/DoD:` 4-7 bullets (include traceability, proof requirements, scope discipline, and repo map policy)
  - `Branching:` 1-2 bullets (branch naming convention, which phase creates branches)
  - `Languages:` one line (`docs=...`, `agent=...`, `comments=...`)
  - hard limit: ≤200 words total
- Summarize the key rules and what changed.
- Final line: `Ready for: /spk.spec <slug>`

---

Reminders:

- readiness: ./.speckeep/scripts/check-ready.sh constitution [<slug>] (run it, trust the exit code).
- Write/patch only the artifacts named above; keep context to the current slug and Touches: surfaces.
- Do not expand scope, re-plan, or commit without being asked.
- End with the end block and preserve the prompt's exact final line.
- Gate: speckeep check <slug> → fix findings or report a blocker.
- Canonical source (kept in sync automatically): .speckeep/templates/prompts/constitution.md
