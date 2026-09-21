---
report_type: verify
slug: routing-ia
status: pass
docs_language: en
generated_at: 2026-09-21
---

# Verify Report: routing-ia

## Scope

- snapshot: routing is split into Providers / Models / Routing, backed by a reserved global default (`*`) with selector fallback, atomic provider+models writes, api_type-aware validation with egress proxy, and provider model discovery with a UI picker.
- verification_mode: default
- artifacts:
  - CONSTITUTION.md
  - specs/active/routing-ia/spec.md
  - specs/active/routing-ia/plan.md
  - specs/active/routing-ia/tasks.md
- inspected_surfaces:
  - src/internal/domain/routing/{config.go,service/selector.go} (+ service test)
  - src/internal/adapters/provider/models.go (+ provider test)
  - src/internal/api/handler/admin/routing_handler.go (+ test)
  - src/internal/api/admin.go, src/internal/api/dto/routing.go
  - src/cmd/admin/run.go, src/cmd/all/admin.go
  - ui/src/api/routing.ts, ui/src/pages/{Providers,Models,Routing}.tsx, ui/src/App.tsx, ui/src/components/{Layout,CommandPalette}.tsx
  - ui/src/pages/{Providers,Models,Routing}.test.tsx, ui/src/components/Layout.test.tsx

## Verdict

- status: pass
- archive_readiness: safe
- summary: all 16 tasks carry `Proof:` lines, selector/handler/provider/UI tests pass, every AC maps to passing evidence, and existing routing behavior for `""`/`default` is preserved.

## Checks

- task_state: completed=16, open=0 (verify-task-state.sh: `OK: all tasks are marked complete`; `PROOFS_MISSING=0`).
- acceptance_evidence:
  - AC-001 -> `TestRouteSelectorGlobalFallback` (unconfigured tenant uses the global provider).
  - AC-002 -> `TestRouteSelectorGlobalFallback` (tenant override wins; no-route preserved).
  - AC-003 -> `TestRoutingHandlerListModelsAggregate` (cost + defaults + override count, `*` excluded).
  - AC-004 -> `TestRoutingHandlerUpsertProviderWithModels` (tx used, routes + placeholder rates) and `TestRoutingHandlerUpsertProviderRollback`.
  - AC-005 -> `TestRoutingHandlerUpsertProviderKeyValidation` (openai 400, ollama/bedrock 200) + Providers test (masked-key save with proxy).
  - AC-006 -> Models test (cost/defaults/delete via `*`).
  - AC-007 -> Routing test (override row, `*` not shown as tenant, inherited indicator).
  - AC-008 -> Layout test (Providers, Models, Routing links).
  - AC-009 -> `TestRouteSelectorGlobalFallback` empty-tenant case plus the existing routing suite.
  - AC-010 -> `TestRouteSelectorOverrideIsolation` (global edit leaves the tenant override intact).
  - AC-011 -> `TestModelDiscoverer` (openai auth/parse, ollama parse, unsupported, non-2xx) + `TestRoutingHandlerListProviderModels` (200/501/404).
  - AC-012 -> Providers test (load + select models; manual fallback after a load error).
- implementation_alignment:
  - `RouteSelector.providersFor` resolves tenant first then `routing.GlobalTenant`, keeping `""`→`default` unchanged.
  - `UpsertProvider` writes provider, global routes and placeholder cost rates inside `RunInTx` (only when a rate is absent) and relaxes keys for ollama/bedrock.
  - `provider.ModelDiscoverer` reuses the chat adapters' auth rules and the egress transport (proxy-aware); unsupported types return `routing.ErrModelsUnsupported`.
  - The UI exposes three pages; the Providers form validates by api_type and loads models with a manual fallback.

## Errors

- none

## Warnings

- none

## Questions

- none

## Not Verified

- Live provider `/models` calls were not exercised (no network); discovery is verified with `httptest` servers and a fake handler discoverer.
- Gemini and Bedrock model listing are intentionally unsupported (501); not exercised beyond the unsupported path.
- Atomicity is verified with a fake transaction runner plus fake repositories; no real Postgres rollback test was run.
- Masked-secret preservation on upsert is covered by existing repository tests, not re-run here against a live database.

## Next Step

- safe to archive
