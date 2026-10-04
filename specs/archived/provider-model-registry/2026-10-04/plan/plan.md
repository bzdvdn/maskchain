# Provider & Model Registry Plan

## Phase Contract

Inputs: `specs/active/provider-model-registry/spec.md` and targeted repo context.
Outputs: `plan.md` (data model noted inline).
Stop if: the spec is too vague to plan safely.

## Goal

Make the provider model catalog real: declarative providers (with `models`) in YAML that seed a brand-new store, a catalog that is fully manageable in the admin UI, wildcard routes that catch unlisted models, and a `GET /v1/models` that reflects the union of routed and catalog models. The catalog is materialized as existing global routes, so no new table is needed and the UI's current provider→models behavior becomes the single mechanism.

## MVP Slice

- `providers[].models[]` in YAML seeds global routes for a fresh store; `GET /v1/models` includes them. Must satisfy `AC-001`, `AC-005`.

## First Validation Path

1. Unit-test the seed path: a fresh store seeds provider + its global routes; an existing provider is left untouched.
2. Unit-test the selector: wildcard resolves an unlisted model; an exact route wins.
3. `curl GET /v1/models` after startup contains a YAML-declared model.

## Scope

- `routing.providers[].models[]` (YAML) → global routes at seed time.
- Per-provider idempotent seeding; YAML never mutates an existing provider.
- Wildcard route matching with precedence.
- `/v1/models` aggregation over tenant + global exact routes.
- Admin API + UI add/remove model for a provider; validation of catalog/route entries.
- Untouched: provider clients, path fidelity, shield/budget chains, DB schema.

## Performance Budget

- Selector wildcard lookup adds a bounded scan of the tenant's routes (already iterated today); no new allocations beyond the existing loop.
- Startup seeding is O(providers × models); ≤ no measurable delay for 1000 models.

## Implementation Surfaces

- `src/internal/infra/config/config.go` — add `Models []string` to `ProviderConfig`.
- `src/internal/domain/routing/config.go` — add `Models []string` to domain `ProviderConfig`.
- `src/cmd/internal/bootstrap/routing.go` — carry `Models` into the registry seed input.
- `src/internal/adapters/repository/postgres/routing_registry.go` — per-provider idempotent `SeedFromYAML` (+ its global routes) instead of all-or-nothing.
- `src/internal/domain/routing/repository.go` — seed/route contract if needed for model removal.
- `src/internal/domain/routing/service/selector.go` — wildcard matching + precedence.
- `src/internal/domain/routing/route.go` (or a helper) — wildcard predicate.
- `src/internal/api/self_handler.go` — `modelsForTenant` includes global exact routes, excludes wildcard patterns.
- `src/internal/api/handler/admin/routing_handler.go` — remove-model-for-provider endpoint; validate wildcard provider refs; exclude patterns from `ListModels`.
- `src/internal/api/admin.go` — register the remove-model route.
- `src/internal/infra/config/validator.go` — reject wildcard routes with unknown provider / malformed pattern.
- UI: `ui/src/pages/Providers.tsx`, `ui/src/pages/Models.tsx`, `ui/src/api/routing.ts` — add/remove a model for a provider.
- Tests: `routing_registry_test.go`, `service_test.go`, `self_handler_test.go`, `routing_handler_test.go`, `config_test.go`, UI tests.

## Bootstrapping Surfaces

- none — routes/registry/UI already exist.

## Architecture Impact

- Local: config gains a field; seed becomes incremental; selector gains wildcard; self-service catalog grows.
- Cross-boundary: `ProviderConfig` shape (config → domain → seed); admin API gains a model-removal operation.
- Compatibility: additive. Existing global routes and all-or-nothing boots still work (a populated store is untouched); no DB migration.

## Acceptance Approach

- AC-001 → YAML `models` materialize as global routes on a fresh store; surfaces `config.go`, `bootstrap/routing.go`, `routing_registry.go`; proof: seed test + `GET /v1/models`.
- AC-002 → UI add model = provider upsert/models (existing) persisting a global route; surfaces `routing_handler.go`; proof: admin test + post-restart read.
- AC-003 → selector wildcard fallback; surfaces `selector.go`; proof: request resolves an unlisted model to the wildcard provider.
- AC-004 → precedence puts exact before wildcard; surfaces `selector.go`; proof: exact route wins over wildcard.
- AC-005 → catalog union (tenant + global exact), dedup, key scope; surfaces `self_handler.go`; proof: scoped/unscoped `/v1/models` tests.
- AC-006 → seed only inserts absent providers; surfaces `routing_registry.go`; proof: UI entry survives a re-seed.
- AC-007 → validation at config load and admin API; surfaces `validator.go`, `routing_handler.go`; proof: bad wildcard fails with a named error.
- AC-008 → UI/API add + remove model round-trip; surfaces `routing_handler.go`, `admin.go`, UI; proof: model appears then disappears from `/v1/models`.

## Data and Contracts

- Data model: no change — no new tables or migrations. The catalog is materialized as rows in the existing `routing_model_routes` (global tenant `"*"`), reusing the table already written by the admin provider upsert.
- Contracts: `ProviderConfig` gains an additive `models` field (YAML + admin request already carries it); a new admin operation removes a provider's model (global route).
- Provenance: seeded rows stay `source: "yaml"`; UI edits stay `source: "ui"`.

## Implementation Strategy

- DEC-001 Materialize the catalog as global routes, not a new table
  Why: `UpsertProvider` already turns submitted models into global routes and cost rates, so the catalog already exists as routes; a new table would duplicate the source of truth and need a migration. Not: a `routing_provider_models` table.
  Tradeoff: the per-provider model list is derived from route `providers[]`; a model routed to several providers is one route.
  Affects: `routing_registry.go`, `bootstrap/routing.go`, `routing_handler.go`, `self_handler.go`.
  Validation: AC-001/002/005/008 tests.
- DEC-002 Seed is per-provider insert-if-absent; YAML never mutates an existing provider
  Why: satisfies AC-006 (UI durability) and AC-001 (fresh bootstrap) without tombstones. Not: all-or-nothing seeding (today) or per-route tombstones.
  Tradeoff: adding a model to an existing provider via YAML requires the UI or deleting the provider first.
  Affects: `routing_registry.go`, `bootstrap/routing.go`.
  Validation: AC-006 test (existing/UI provider unchanged across re-seed).
- DEC-003 Wildcard = route model containing `*` (canonical `/*`), catch-all fallback
  Why: simplest semantics meeting AC-003/004; no new request field. A namespace prefix (`groq/*`) is preferred when the request model matches it, otherwise the wildcard is a catch-all. Not: provider-namespace prefix stripping from the forwarded model.
  Tradeoff: no model-name rewriting; the literal prefix is advisory for matching order.
  Affects: `selector.go`, `route.go`.
  Validation: AC-003/004 tests.
- DEC-004 Resolution precedence exact-tenant → wildcard-tenant → exact-global → wildcard-global
  Why: matches spec RQ-004 and keeps specific overrides authoritative. Not: wildcard-before-exact.
  Tradeoff: a wildcard can mask `NO_ROUTE` for genuinely unknown models (intended).
  Affects: `selector.go`.
  Validation: precedence table tests.
- DEC-005 `/v1/models` unions tenant + global exact routes and excludes patterns
  Why: fixes the current omission of global fallback models and satisfies AC-005. Not: expanding wildcards (unbounded).
  Tradeoff: catalog is route-driven; a declared-but-unrouted model is not listed.
  Affects: `self_handler.go`.
  Validation: scoped/unscoped tests.
- DEC-006 Removing a provider's model removes it from the route's `providers[]`, deleting the route only when empty
  Why: a route may serve several providers; unconditional deletion would drop another provider's routing. Not: delete the whole route.
  Tradeoff: removal needs a read-modify-write; use the existing transaction runner.
  Affects: `routing_handler.go`, `repository.go`, `admin.go`.
  Validation: AC-008 test with a shared route.
- DEC-007 Validation rejects wildcard routes with unknown providers / malformed patterns at load and admin
  Why: prevents silent routing breakage (AC-007). Not: lazy runtime failure.
  Tradeoff: config must be internally consistent.
  Affects: `validator.go`, `routing_handler.go`.
  Validation: AC-007 tests.

## Incremental Delivery

### MVP (First Value)

- Add `Models` to config/domain; make seeding per-provider and materialize models as global routes; include global routes in `GET /v1/models`.
- MVP readiness: `AC-001`, `AC-005` pass.

### Iterative Expansion

- Wildcard selector + precedence → `AC-003`, `AC-004`.
- Admin remove-model + UI → `AC-008`; validation → `AC-007`.
- UI persistence/restart tests → `AC-002`, `AC-006`.

## Sequencing Notes

- Config field + seed change first (compile prerequisite for bootstrap).
- Selector wildcard after the registry can carry wildcard routes (no dependency on seed).
- Admin/UI removal after `repository.go` exposes the read-modify-write helper.
- Keep `ListModels` pattern filtering with the `/v1/models` filtering to avoid showing `groq/*` as a model.

## Risks

- Incremental seeding changes current all-or-nothing behavior.
  Mitigation: insert-if-absent is a superset; a populated store is untouched (AC-006 test).
- Wildcard can shadow `NO_ROUTE`.
  Mitigation: only active when a wildcard route exists; documented and covered by precedence tests.
- Removing a model from a multi-provider route.
  Mitigation: DEC-006 read-modify-write with the transaction runner; test with a shared route.
- Patterns leaking into model lists.
  Mitigation: filter `*` in both `self_handler` and `ListModels`; test.

## Rollout and Compatibility

- No migration, no flag. Existing configs without `models` behave as before; a populated store is never mutated by seed.
- Operational follow-up: audit entries for provider/model changes already exist; extend to model removal.

## Validation

- Automated: `go test ./src/internal/domain/routing/... ./src/internal/api/... ./src/internal/infra/config/...` plus `make test`; UI tests for add/remove.
- New tests: seed idempotency, wildcard precedence, `/v1/models` union/scope, config validation, admin remove-model.
- Manual: YAML with `models`, then UI add/remove, then `curl GET /v1/models`.
- Proves: `AC-001`–`AC-008`; `DEC-001`–`DEC-007`.

## Constitution Compliance

- no conflicts. Tenant-scoped routing preserved; Content Shield unchanged; native-only data plane; no new runtime dependencies.
