---
name: spk-module-map
description: SpecKeep phase "module-map" — Build/refresh the module dependency graph and per-module context cards (monorepos).
---

# /spk-module-map

You act as a **codebase cartographer** for large multi-project repositories (monorepos). Keep the module graph accurate and the context cards compact — never let index maintenance grow into documentation.

This command generates a machine-readable module graph and per-module context cards. It **complements `REPOSITORY_MAP.md`** ("what is where") with "what depends on what, layers/boundaries, and impact".

## Phase Contract

Inputs: project filesystem state (no spec required).
Outputs: `.speckeep/module-map.json` + per-module `<module>/CONTEXT.md` cards (managed block).
Stop if: the run would require semantic design changes — this command reports, it does not refactor.

## How to run

- Build/refresh: `./.speckeep/scripts/module-map.sh build` (add `--no-cards` to skip cards).
- Inspect: `module-map.sh show`; queries: `deps <module>`, `rev-deps <module>`, `card <module>`, `violations`, `cycles`, `find <path>`.
- On entering a module, read its `CONTEXT.md` (or `module-map.sh card <module>`) before editing.
- During implementation, `speckeep affected <slug>` lists the modules and tests impacted by the feature's `Touches:`.

## Policy

- Language-agnostic: Go, JS/TS, Python, Rust, Java (Maven/Gradle), .NET, PHP, Dart, Ruby, plus a generic marker fallback. If the default layers/excludes don't match the repo, record overrides in `.speckeep/module-map.yaml`.
- Treat `violations` (a lower layer depending on a higher one) and `cycles` as **advisory findings**: report them with the suggested direction (route through Shared/Contracts, introduce an interface) — do not refactor unprompted.
- Cards carry only mechanical facts (layer, dependencies, dependents, owners, where-to-edit, tests). Put reasoning in your reply, not as new prose in the graph.
- Keep the map generated: do not hand-edit `module-map.json` or the managed card block. The `REPOSITORY_MAP.md` `Where To Edit`/`Key Paths` may be derived from this graph.

## Output expectations

- Report detected languages, module count, violation/cycle counts.
- Summarize the most coupled modules (high reverse-dependencies) and any layer violations/cycles.
- If you entered a specific module, summarize its card (layer, deps, dependents, where-to-edit) briefly.
- End with the standard end block (see AGENTS.md); resume the phase that invoked you:
  ```
  Slug: <slug>
  Status: module-map
  Artifacts: .speckeep/module-map.json [, <module>/CONTEXT.md ...]
  Blockers: <none | reason>
  Ready for: /spk-<phase> <slug>   (or "speckeep archive <slug> ." when idle)
  ```
- Final line: `Ready for: /spk-<phase> <slug>` (resume the calling phase).

---

Reminders:

- Write/patch only the artifacts named above; keep context to the current slug and Touches: surfaces.
- Do not expand scope, re-plan, or commit without being asked.
- End with the end block and preserve the prompt's exact final line.
- Gate: speckeep check <slug> → fix findings or report a blocker.
- Canonical source (kept in sync automatically): .speckeep/templates/prompts/module-map.md

Evidence: every completed task in `tasks.md` must carry a `Proof:` line (format `Proof: kind path anchor`, e.g. `Proof: test src/tests/export_test.go TestRunExport`). A task without `Proof` is not complete; `speckeep trace` and archive gates read exactly these records.
