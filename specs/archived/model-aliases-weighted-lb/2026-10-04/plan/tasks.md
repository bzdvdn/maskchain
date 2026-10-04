# Model Aliases & Weighted Load Balancing Tasks

## Phase Contract

Inputs: `specs/active/model-aliases-weighted-lb/plan.md`, `spec.md`, `data-model.md`.
Outputs: ordered executable tasks with coverage mapping.
Stop if: tasks would be vague or acceptance coverage cannot be mapped.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/infra/config/config.go | T1.1 |
| src/internal/domain/routing/config.go | T1.1 |
| src/internal/domain/routing/health_status.go | T1.2 |
| src/internal/domain/routing/alias.go | T1.2 |
| src/internal/domain/routing/repository.go | T1.4 |
| src/internal/api/handler/admin/routing_handler_test.go | T1.4, T3.3 |
| src/internal/adapters/repository/postgres/migrations/022_provider_weight_and_aliases.up.sql | T1.3 |
| src/internal/adapters/repository/postgres/migrations/022_provider_weight_and_aliases.down.sql | T1.3 |
| src/internal/adapters/repository/postgres/routing_registry.go | T1.3 |
| src/cmd/internal/bootstrap/routing.go | T1.5 |
| src/cmd/gateway/providers.go | T1.5 |
| src/cmd/all/run.go | T1.5 |
| src/cmd/admin/run.go | T1.5 |
| src/cmd/all/admin.go | T1.5 |
| src/internal/adapters/repository/postgres/routing_registry_test.go | T1.6 |
| src/internal/infra/config/config_test.go | T1.6, T3.3 |
| src/internal/domain/routing/service/registry.go | T2.1 |
| src/internal/domain/routing/service/selector.go | T2.1 |
| src/internal/domain/routing/service/service_test.go | T2.2 |
| src/internal/api/dto/routing.go | T3.1 |
| src/internal/api/handler/admin/routing_handler.go | T3.1 |
| src/internal/api/admin.go | T3.1 |
| src/internal/infra/config/validator.go | T3.2 |
| src/internal/api/handler/admin/routing_handler_test.go | T3.3 |
| ui/src/api/routing.ts | T3.4 |
| ui/src/pages/Providers.tsx | T3.4 |
| ui/src/pages/Routing.tsx | T3.4 |
| ui/src/pages/Providers.test.tsx | T3.5 |
| ui/src/pages/Routing.test.tsx | T3.5 |
| Makefile | T4.1 |

## Implementation Context

- MVP goal: single-hop tenant alias resolution before selection, and priority-tier + weighted selection among healthy providers (AC-001, AC-003, AC-005, AC-006, AC-007, AC-008).
- Invariants/semantics: alias map is tenant-scoped and single-hop, applied before exact/wildcard route lookup; candidate set = healthy providers of the minimum `priority` tier; effective weight = `weight` when `> 0` else `1`; equal effective weights → first declared; fallback chain = weighted primary first, then the rest in declared order.
- Errors/codes: alias target with no route → `NO_ROUTE`; negative weight / empty alias target → rejected (config error or HTTP 400).
- Contracts/protocols: `routing.aliases[].{tenant,alias,target}` in YAML; provider `weight` (int, default 0); endpoints `GET/PUT/DELETE /api/v1/routing/aliases`; provider DTO gains `weight`.
- Scope boundaries: do not add key-level/request-level aliases, multi-hop chains, latency/cost strategies, or fallback weighting; key model-scope checks evaluate the requested (pre-alias) name (DEC-001).
- Proof signals: selector tests (alias, no-route, wildcard target, tier, weighted via stubbed RNG, equal-weight, unhealthy); repo alias/weight persistence; handler/validator tests; UI alias/weight tests.
- References: DEC-001..DEC-007, DM-001, DM-002, RQ-001..RQ-009 (no mandatory re-read).

## Phase 1: Foundation (config, schema, persistence)

Goal: model weight and aliases exist in config, domain, and the routing store.

- [x] T1.1 Add `Weight` to `ProviderConfig` and `Aliases []AliasConfig{Tenant,Alias,Target}` to `RoutingConfig` in config and domain packages (AC-006, AC-009). Touches: src/internal/infra/config/config.go, src/internal/domain/routing/config.go
      Proof: code src/internal/infra/config/config.go AliasConfig
- [x] T1.2 Add `Weight` to the domain `Provider` (constructor) and a new `Alias` entity with single-hop helpers (DEC-005, AC-001). Touches: src/internal/domain/routing/health_status.go, src/internal/domain/routing/alias.go
      Proof: code src/internal/domain/routing/alias.go ResolveAlias
- [x] T1.3 Add migration `022` (`routing_providers.weight`, `routing_aliases` table) and implement weight read/write plus alias CRUD + idempotent alias seeding (DIM-002, DEC-002, DEC-006, AC-009). Touches: src/internal/adapters/repository/postgres/migrations/022_provider_weight_and_aliases.up.sql, src/internal/adapters/repository/postgres/migrations/022_provider_weight_and_aliases.down.sql, src/internal/adapters/repository/postgres/routing_registry.go
      Proof: code src/internal/adapters/repository/postgres/routing_registry.go SeedAliasesFromYAML
- [x] T1.4 Extend the `RegistryRepository` interface with `ListAliases`/`UpsertAlias`/`DeleteAlias`/`SeedAliasesFromYAML` (DEC-002, AC-009). Touches: src/internal/domain/routing/repository.go, src/internal/api/handler/admin/routing_handler_test.go
      Proof: code src/internal/domain/routing/repository.go RegistryRepository
- [x] T1.5 Carry provider weight and YAML aliases through bootstrap into the registry seed and the in-memory registry wiring (DEC-006, AC-009). Touches: src/cmd/internal/bootstrap/routing.go, src/cmd/gateway/providers.go, src/cmd/all/run.go, src/cmd/admin/run.go, src/cmd/all/admin.go
      Proof: code src/cmd/internal/bootstrap/routing.go LoadRoutingFromDB
- [x] T1.6 Prove weight + alias persistence and YAML parsing (AC-009). Touches: src/internal/adapters/repository/postgres/routing_registry_test.go, src/internal/infra/config/config_test.go
      Proof: test src/internal/infra/config/config_test.go TestRoutingConfigWeightAndAliases

## Phase 2: Selector (aliases + weighted balancing)

Goal: the selector resolves aliases and balances across the priority tier.

- [x] T2.1 Store aliases and weights in the registry; resolve the alias in `Select`; filter to the healthy minimum-priority tier; pick weighted-random via an injectable `intN`; return the primary plus the declared-order fallback chain (DEC-001, DEC-003, DEC-004, DEC-007, AC-001, AC-002, AC-003, AC-004, AC-005, AC-006, AC-007, AC-008). Touches: src/internal/domain/routing/service/registry.go, src/internal/domain/routing/service/selector.go
      Proof: code src/internal/domain/routing/service/selector.go Select
- [x] T2.2 Add selector tests: alias resolves, alias tenant-scoped, alias no-route, alias→wildcard, priority tier, weighted 3:1 via stubbed RNG, equal-weight first, unhealthy excluded (AC-001..AC-008). Touches: src/internal/domain/routing/service/service_test.go
      Proof: test src/internal/domain/routing/service/service_test.go TestRouteSelectorResolvesAlias

## Phase 3: Admin API, validation, UI

Goal: operators manage aliases and weights; invalid values are rejected.

- [x] T3.1 Add `weight` to provider DTOs/handler passthrough, alias CRUD handlers, and register `GET/PUT/DELETE /api/v1/routing/aliases` (AC-009, AC-010). Touches: src/internal/api/dto/routing.go, src/internal/api/handler/admin/routing_handler.go, src/internal/api/admin.go
      Proof: code src/internal/api/handler/admin/routing_handler.go UpsertAlias
- [x] T3.2 Reject negative weight and empty alias target at config load and admin API with a named error (AC-010). Touches: src/internal/infra/config/validator.go
      Proof: code src/internal/infra/config/validator.go validateRoutingWeightAndAliases
- [x] T3.3 Add tests for the alias/weight admin round-trip and validation failures (AC-009, AC-010). Touches: src/internal/api/handler/admin/routing_handler_test.go, src/internal/infra/config/config_test.go
      Proof: test src/internal/api/handler/admin/routing_handler_test.go TestRoutingHandlerAliasCRUD
- [x] T3.4 Add UI: provider weight input, an aliases section (tenant/alias/target) on the Routing page, and the alias API client (AC-002, AC-009). Touches: ui/src/api/routing.ts, ui/src/pages/Providers.tsx, ui/src/pages/Routing.tsx
      Proof: code ui/src/api/routing.ts upsertAlias
- [x] T3.5 Add UI tests for weight editing and alias add/remove (AC-009, AC-010). Touches: ui/src/pages/Providers.test.tsx, ui/src/pages/Routing.test.tsx
      Proof: test ui/src/pages/Routing.test.tsx

## Phase 4: Validation

Goal: prove the feature end to end and leave the package reviewable.

- [x] T4.1 Run the affected Go suites, the UI suite, and `make test`; fix regressions (SC-001, SC-002). Touches: Makefile
      Proof: chore Makefile test

## Acceptance Coverage

- AC-001 -> T1.2, T2.1, T2.2
- AC-002 -> T2.1, T2.2, T3.4
- AC-003 -> T2.1, T2.2
- AC-004 -> T2.1, T2.2
- AC-005 -> T2.1, T2.2
- AC-006 -> T1.1, T1.2, T2.1, T2.2
- AC-007 -> T2.1, T2.2
- AC-008 -> T2.1, T2.2
- AC-009 -> T1.3, T1.4, T1.5, T1.6, T3.1, T3.3, T3.4, T3.5
- AC-010 -> T3.1, T3.2, T3.3, T3.5

## Notes

- MVP is Phase 2; Phase 1 is its hard prerequisite, Phase 3 adds management.
- Keep `GetProviderList` consistent with alias resolution if it is used outside the request handler.
- Every `[x]` task needs a `Proof:` line on the next line; no proof means not done.
