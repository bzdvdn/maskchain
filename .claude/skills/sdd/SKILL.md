---
name: sdd
description: SpecKeep — spec-driven development. Use when the user asks to propose, spec, inspect, plan, decompose, implement, converge, or verify a feature, or starts spec-driven development.
---

# SpecKeep / SDD

Workflow: constitution → spec → [inspect, optional] → plan → tasks → implement → archive. Verify is an optional on-demand audit; propose is the one-shot fast lane; converge is the fast closing loop.

Phase files live under .claude/skills

## How to run a phase

1. Open the phase file under phases/<phase>.md — it is self-contained with the full instructions inline (canonical source mirrored from .speckeep/templates/prompts).
2. Read .speckeep/constitution.summary.md first (fallback: CONSTITUTION.md).
3. Branch-first: work on feature/<slug> (only spec/propose may create/switch the branch).
4. Keep context narrow: current slug + Touches: surfaces only.
5. Run the readiness script: ./.speckeep/scripts/check-ready.sh <phase> <slug> and trust its exit code.
6. End every phase with the end block (Slug / Status / Artifacts / Blockers / Ready for) and preserve the prompt's exact final line.

## Gates (never skip)

- speckeep check <slug> before finishing a phase.
- speckeep converge <slug> (fast loop) or speckeep guard . (CI) before closing.
- A task is done only with a Proof: line under its [x] in tasks.md.

## Phases

- `constitution` — Create or update the project constitution
- `spec` — Create or update one feature spec
- `propose` — One-shot: turn an idea into spec + tasks (plan optional) and go straight to implement
- `inspect` — Inspect one feature for consistency and quality
- `plan` — Create or update plan artifacts for one feature
- `tasks` — Create or update tasks for one feature
- `implement` — Implement one feature from tasks
- `verify` — Verify one implemented feature package
- `converge` — Close a feature fast: re-check tasks/proofs, append follow-up tasks, repeat until converged
- `handoff` — Generate a session handoff document for one feature
- `challenge` — Adversarial review of a feature spec or plan
- `scope` — Quick scope boundary check for a feature
- `glossary` — Create or update the shared domain-language glossary
- `recap` — Project-level overview of all active features and their current phase
- `hotfix` — Create emergency fix outside the standard phase chain
- `repo-map` — Update REPOSITORY_MAP.md navigation index
- `rollback` — Roll back completed tasks for a feature, returning them to unfinished state

## Constraints

Do not:
- skip readiness scripts
- expand scope / re-plan during implement
- mark done without observable proof
- run git commit/push/tag or open a PR unless explicitly asked
- read the full repo instead of the minimum slice
