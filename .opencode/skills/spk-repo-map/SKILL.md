---
name: spk-repo-map
description: SpecKeep phase "repo-map" — Update REPOSITORY_MAP.md navigation index.
---

# /spk-repo-map

You act as a **codebase cartographer**. Keep the map compact, accurate, and cheap to navigate — never let index maintenance grow into documentation.

Update `REPOSITORY_MAP.md` — a compact, code-only navigation index.

## Phase Contract

Inputs: project filesystem state (no spec required).
Outputs: updated `REPOSITORY_MAP.md` at project root.
Stop if: no structural changes detected (check trigger checklist first).

## Policy

- Keep `REPOSITORY_MAP.md` compact and code-only (paths + short role descriptions).
- Language-agnostic: detect stack from repository markers (e.g. `go.mod`, `package.json`, `pyproject.toml`, `Cargo.toml`, `pom.xml`, `*.csproj`) and adapt sections to the detected stack.
- Do not assume Go-specific layout in non-Go projects.
- Hard size cap: keep the map short (target up to 180 lines); if it grows, compress instead of expanding prose.
- Update in place (minimal diff): preserve unchanged lines/order and edit only impacted entries/sections.
- Do not rewrite the whole file if only a subset changed.
- If `REPOSITORY_MAP.md` does not exist, create it from template; otherwise patch existing content.
- Exclude from indexing: `src/internal/agents/**`, `.speckeep/**`, `specs/archived/**`, `.git/**`, `bin/**`, `demo/**`, `docs/**`, `TESTS/**`, `node_modules/**`, `vendor/**`, `dist/**`, `build/**`, `coverage/**`.
- Note: project settings are already sourced from `.speckeep/speckeep.yaml`; do not duplicate that config in the map.

## Template

```md
# Repository Map

## Entry Points
- `<path>` — `<runtime/service/cli entrypoint>`

## Top-Level Code
- `<path>` — `<module role>`

## Key Paths
- `<path>` — `<what is implemented here>`

## Where To Edit
- `<change type>` — `<likely paths>`

## Excluded
- `<glob>` — `excluded from indexing`
```

## Output expectations

- List changed/added/removed entries.
- Confirm the map is up to date and within the size cap.
- Include a short summary block: `Slug`, `Status`, `Artifacts`, `Blockers`.
- Final line: `Ready for: /spk.implement <slug>` (resume the phase that requested the map update).

---

Reminders:

- readiness: ./.speckeep/scripts/check-ready.sh repo-map [<slug>] (run it, trust the exit code).
- Write/patch only the artifacts named above; keep context to the current slug and Touches: surfaces.
- Do not expand scope, re-plan, or commit without being asked.
- End with the end block and preserve the prompt's exact final line.
- Gate: speckeep check <slug> → fix findings or report a blocker.
- Canonical source (kept in sync automatically): .speckeep/templates/prompts/repo-map.md

Evidence: every completed task in `tasks.md` must carry a `Proof:` line (format `Proof: kind path anchor`, e.g. `Proof: test src/tests/export_test.go TestRunExport`). A task without `Proof` is not complete; `speckeep trace` and archive gates read exactly these records.
