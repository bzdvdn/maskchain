# SpecKeep Module-Doc Prompt (compact)

You act as a **module documenter**: write or refresh a rich, human-and-agent-friendly `CONTEXT.md` for one module — the "what / how / why" that a developer (or you, later) needs to work inside it without re-deriving everything.

This complements the generated module map: `module-map` produces mechanical facts (dependencies, dependents, layers) as a managed block; you write the prose around it.

## Phase Contract

Inputs: a module name or path (`<module>`), and `.speckeep/module-map.json` (run `./.speckeep/scripts/module-map.sh build` first if missing).
Outputs: `<module>/CONTEXT.md` (prose + preserved managed block).
Stop if: the module is not in the map, or describing it would require speculative design changes.

## How to run

- Resolve the module: from the user's argument, or from the active feature's `Touches:` via `speckeep affected <slug>`.
- Get the mechanical scaffold: `./.speckeep/scripts/module-map.sh card <module>` (layer, dependencies, dependents, owners, where-to-edit, tests).
- If `<module>/CONTEXT.md` exists, read it first; keep the existing prose and the managed block — update only what changed.

## CONTEXT.md structure (keep compact, ≤ ~60 lines)

- `## Purpose` — why this module exists; what problem it solves (1–3 lines).
- `## Responsibilities` — what it owns and explicitly does not own.
- `## How it works` — the main flow/architecture in ~5–10 bullets (key types, services, entrypoints).
- `## Key files` — 3–6 paths with one-line roles (paths relative to repo root).
- `## Public contracts` — interfaces/events/DTOs other modules rely on (stability matters most here).
- `## Dependencies` / `## Consumed by` — from the generated card block; do not duplicate detail.
- `## Gotchas` — invariants, hidden coupling, config keys, migrations, common mistakes.
- Keep the `<!-- speckeep:module-map:start/end -->` block exactly as generated.

## Policy

- Facts from the module map are authoritative; do not contradict them. Prose is your judgment — mark uncertainty explicitly.
- Do not redesign or refactor: document the current state.
- One module per run; if the request touches several modules, cover the primary one and list the rest.
- If you changed the module meanwhile, refresh the managed block: `./.speckeep/scripts/module-map.sh build` (or note that it is stale).

## Output expectations

- Write/patch `<module>/CONTEXT.md`; keep the managed block intact.
- Summarize what you documented (purpose, key flows, contracts) in a few bullets.
- End with the standard end block (see AGENTS.md); resume the calling phase:
  ```
  Slug: <slug>
  Status: module-doc
  Artifacts: <module>/CONTEXT.md
  Blockers: <none | reason>
  Ready for: /spk-<phase> <slug>   (or "speckeep archive <slug> ." when idle)
  ```
- Final line: `Ready for: /spk-<phase> <slug>` (resume the calling phase).