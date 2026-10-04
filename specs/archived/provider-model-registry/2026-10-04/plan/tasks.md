# Provider & Model Registry Tasks

## Phase Contract

Inputs: `specs/active/provider-model-registry/plan.md`, `spec.md`.
Outputs: ordered executable tasks with coverage mapping.
Stop if: tasks would be vague or acceptance coverage cannot be mapped.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/infra/config/config.go | T1.1 |
| src/internal/domain/routing/config.go | T1.1 |
| src/internal/adapters/repository/postgres/routing_registry.go | T1.2 |
| src/cmd/internal/bootstrap/routing.go | T1.2 |
| src/internal/adapters/repository/postgres/routing_registry_test.go | T1.4 |
| src/internal/api/self_handler.go | T1.3 |
| src/internal/api/self_handler_test.go | T1.4 |
| src/internal/domain/routing/route.go | T2.1 |
| src/internal/domain/routing/service/selector.go | T2.1 |
| src/internal/domain/routing/service/service_test.go | T2.2 |
| src/internal/api/handler/admin/routing_handler.go | T3.1, T3.2 |
| src/internal/api/admin.go | T3.1 |
| src/internal/domain/routing/repository.go | T3.1 |
| src/internal/infra/config/validator.go | T3.2 |
| src/internal/api/handler/admin/routing_handler_test.go | T3.3 |
| src/internal/infra/config/config_test.go | T3.3 |
| ui/src/pages/Providers.tsx | T3.4 |
| ui/src/pages/Models.tsx | T3.4 |
| ui/src/api/routing.ts | T3.4 |
| ui/src/pages/Providers.test.tsx | T3.5 |
| ui/src/pages/Models.test.tsx | T3.5 |
| Makefile | T4.1 |

## Implementation Context

- MVP goal: `routing.providers[].models[]` from YAML seeds global routes on a fresh store, and `GET /v1/models` lists them (AC-001, AC-005).
- Invariants/semantics: the catalog IS the existing `routing_model_routes` table (global tenant `"*"`), no new table; seed inserts a provider + its models only when the provider is absent (never mutates an existing one); a wildcard is a route whose model contains `*` (canonical `/*`), acting as a catch-all; precedence exact-tenant → wildcard-tenant → exact-global → wildcard-global; the requested model is forwarded unchanged (no prefix stripping); patterns are excluded from every model list.
- Errors/codes: invalid catalog/route entries rejected at config load and admin API with a message naming the route/provider (no partial write).
- Contracts/protocols: `ProviderConfig` gains additive `models` (YAML + admin request already sends it); new admin op `DELETE /api/v1/routing/providers/:name/models/:model`; existing routes endpoints unchanged.
- Scope boundaries: do not add a DB table/migration; do not strip `<provider>/` from forwarded model names; no media/translation changes.
- Proof signals: seed-idempotency test; selector wildcard/precedence tests; `/v1/models` union+scope test; admin remove-model test; config validation test; UI add/remove tests.
- References: DEC-001..DEC-007, RQ-001..RQ-008 (no mandatory re-read).

## Phase 1: Foundation (catalog as global routes)

Goal: carry provider models into the registry and materialize them on seed; surface them in the self-service catalog.

- [x] T1.1 Add `Models []string` to `ProviderConfig` in config and domain packages (DEC-001, AC-001). Touches: src/internal/infra/config/config.go, src/internal/domain/routing/config.go
      Proof: code src/internal/infra/config/config.go Models
- [x] T1.2 Make `SeedFromYAML` per-provider insert-if-absent and materialize each seeded provider's `models` as global routes; carry `Models` through bootstrap (DEC-001, DEC-002, AC-001, AC-006). Touches: src/internal/adapters/repository/postgres/routing_registry.go, src/cmd/internal/bootstrap/routing.go
      Proof: code src/internal/adapters/repository/postgres/routing_registry.go SeedFromYAML
- [x] T1.3 Include a tenant's global (`"*"`) exact routes in `modelsForTenant`, exclude `*` patterns, de-duplicate (DEC-005, AC-005). Touches: src/internal/api/self_handler.go
      Proof: code src/internal/api/self_handler.go modelsForTenant
- [x] T1.4 Prove seed idempotency and catalog union/scope (AC-001, AC-005, AC-006). Touches: src/internal/adapters/repository/postgres/routing_registry_test.go, src/internal/api/self_handler_test.go
      Proof: test src/internal/adapters/repository/postgres/routing_registry_test.go TestSeedPlanSkipsExistingProviders

## Phase 2: Wildcard routing

Goal: resolve unlisted models through wildcard routes with correct precedence.

- [x] T2.1 Add a wildcard predicate and extend the selector with wildcard fallback and exact-over-wildcard precedence (DEC-003, DEC-004, AC-003, AC-004). Touches: src/internal/domain/routing/route.go, src/internal/domain/routing/service/selector.go
      Proof: code src/internal/domain/routing/service/selector.go providersFor
- [x] T2.2 Add precedence/wildcard tests (unlisted model resolves; exact route wins; tenant before global) (AC-003, AC-004). Touches: src/internal/domain/routing/service/service_test.go
      Proof: test src/internal/domain/routing/service/service_test.go TestRouteSelectorWildcardFallback

## Phase 3: Admin API, validation, and UI

Goal: fully manage the catalog and reject invalid entries.

- [x] T3.1 Add the remove-model admin operation (remove the provider from the route's `providers[]`, delete the route only when empty) and register it; exclude `*` patterns from `ListModels` (DEC-006, AC-008). Touches: src/internal/api/handler/admin/routing_handler.go, src/internal/api/admin.go, src/internal/domain/routing/repository.go
      Proof: code src/internal/api/handler/admin/routing_handler.go DeleteProviderModel
- [x] T3.2 Validate catalog/route entries at config load and admin API (wildcard referencing an unknown provider, malformed pattern) with a named error (DEC-007, AC-007). Touches: src/internal/infra/config/validator.go, src/internal/api/handler/admin/routing_handler.go
      Proof: code src/internal/infra/config/validator.go validateRoutingRoutes
- [x] T3.3 Add tests for model removal (incl. a shared multi-provider route) and validation failures (AC-007, AC-008). Touches: src/internal/api/handler/admin/routing_handler_test.go, src/internal/infra/config/config_test.go
      Proof: test src/internal/api/handler/admin/routing_handler_test.go TestRoutingHandlerDeleteProviderModel
- [x] T3.4 Add provider model add/remove in the UI (Providers and Models pages, API client) persisting the change (DEC-006, AC-002, AC-008). Touches: ui/src/pages/Providers.tsx, ui/src/pages/Models.tsx, ui/src/api/routing.ts
      Proof: code ui/src/api/routing.ts deleteProviderModel
- [x] T3.5 Add UI tests for add/remove model round-trip (AC-002, AC-008). Touches: ui/src/pages/Providers.test.tsx, ui/src/pages/Models.test.tsx
      Proof: test ui/src/pages/Providers.test.tsx

## Phase 4: Validation

Goal: prove the feature end to end and leave the package reviewable.

- [x] T4.1 Run the affected Go suites, the UI test suite, and `make test`; fix regressions (SC-001, SC-002). Touches: Makefile
      Proof: chore Makefile test

## Acceptance Coverage

- AC-001 -> T1.1, T1.2, T1.3, T1.4
- AC-002 -> T3.4, T3.5
- AC-003 -> T2.1, T2.2
- AC-004 -> T2.1, T2.2
- AC-005 -> T1.3, T1.4
- AC-006 -> T1.2, T1.4
- AC-007 -> T3.2, T3.3
- AC-008 -> T3.1, T3.3, T3.4, T3.5

## Notes

- MVP is Phase 1; Phases 2–3 add wildcard and management without reworking the seed.
- Phase 1 must land before Phase 3 (admin reuses the catalog materialization path).
- Keep pattern filtering (`*`) consistent between `self_handler` and `ListModels`.
- Every `[x]` task needs a `Proof:` line on the next line; no proof means not done.
