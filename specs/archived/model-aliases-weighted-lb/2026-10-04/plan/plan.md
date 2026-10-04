# Model Aliases & Weighted Load Balancing Plan

## Phase Contract

Inputs: `specs/active/model-aliases-weighted-lb/spec.md` and targeted repo context.
Outputs: `plan.md`, `data-model.md` (schema changes), migration.
Stop if: the spec is too vague to plan safely.

## Goal

Add two routing capabilities on top of the existing registry: a tenant-scoped alias map that rewrites a requested model to a routed model before selection, and provider weights so the selector spreads load across healthy providers of the highest-priority tier instead of always taking the first. Ordered behavior is preserved when no weights are configured.

## MVP Slice

- Single-hop tenant alias resolution before selection, and priority-tier + weighted selection with an injectable RNG. Must satisfy `AC-001`, `AC-003`, `AC-005`, `AC-006`.

## First Validation Path

1. Unit-test alias resolution (tenant-scoped, no-route target) in the selector.
2. Unit-test weighted selection with a deterministic `intN` stub (3:1 split, equal weights → first).
3. Unit-test priority tier (lower priority wins; unhealthy excluded).
4. `curl` an aliased model against a recording mock.

## Scope

- Tenant alias map (`requested → routed`), YAML-seeded + admin UI/API managed, persisted.
- Provider `weight`, persisted and UI-editable.
- Selector: alias resolution, priority-tier filtering, weighted-random pick, fallback chain ordering.
- Validation and UI surfaces for both.
- Untouched: fallback retry semantics, wildcard matching, path fidelity, shield/budget.

## Performance Budget

- Selection: O(providers in route) with one RNG draw; <1 ms p95, no allocations beyond the existing list handling.
- Alias lookup: O(1) map read.

## Implementation Surfaces

- `src/internal/infra/config/config.go` — `ProviderConfig.Weight`, `RoutingConfig.Aliases`, `AliasConfig`.
- `src/internal/domain/routing/config.go` — mirror `Weight`, `AliasConfig`, `RoutingConfig.Aliases`.
- `src/internal/domain/routing/health_status.go` — `Provider.Weight` + constructor.
- `src/internal/domain/routing/alias.go` (new) — `Alias` entity + helpers.
- `src/internal/domain/routing/repository.go` — `ListAliases`/`UpsertAlias`/`DeleteAlias`.
- `src/internal/domain/routing/service/registry.go` — store aliases + provider weight; alias lookup.
- `src/internal/domain/routing/service/selector.go` — alias resolution + tier/weighted selection (injectable `intN`).
- `src/internal/adapters/repository/postgres/routing_registry.go` — `weight` column, alias CRUD, idempotent alias seeding.
- `src/internal/adapters/repository/postgres/migrations/022_*.up.sql|down.sql` (new) — `routing_providers.weight`, `routing_aliases`.
- `src/cmd/internal/bootstrap/routing.go` — convert weights + aliases.
- `src/internal/api/dto/routing.go` — weight on provider DTOs; alias request/response.
- `src/internal/api/handler/admin/routing_handler.go` — weight passthrough; alias CRUD + validation.
- `src/internal/api/admin.go` — register alias routes.
- `src/internal/infra/config/validator.go` — negative weight, empty alias target.
- UI: `ui/src/api/routing.ts`, `ui/src/pages/Providers.tsx` (weight), `ui/src/pages/Routing.tsx` (aliases).
- Tests: selector, registry repo, routing handler, config, UI.

## Bootstrapping Surfaces

- A migration + the `routing_aliases` table must exist before alias persistence works.

## Architecture Impact

- Adds a routing-domain alias concept and a provider weight; the selector becomes the single place aliasing and balancing happen.
- `RoutingConfig`/`ProviderConfig` gain additive fields; repository interface gains alias methods.
- No change to fallback/HMAC retry logic; chain ordering only.

## Acceptance Approach

- AC-001 → alias resolved in `Select`; surfaces `selector.go`, `registry.go`; proof: selector test + mock request.
- AC-002 → alias map keyed by tenant; surfaces `registry.go`; proof: per-tenant unit test.
- AC-003 → alias target unresolved → `ErrNoRoute`; surfaces `selector.go`; proof: test asserts `ErrNoRoute` and no pick.
- AC-004 → alias target goes through wildcard resolution; surfaces `selector.go`; proof: test with `groq/*`.
- AC-005 → min-priority tier filtering; surfaces `selector.go`; proof: tier test.
- AC-006 → `chooseWeighted` with injected `intN`; surfaces `selector.go`; proof: deterministic draw test.
- AC-007 → equal/default weights return the first candidate; surfaces `selector.go`; proof: repeated-selection test.
- AC-008 → unhealthy filtered before tiering; surfaces `selector.go`; proof: health test.
- AC-009 → alias + weight persisted and round-trip; surfaces `routing_registry.go`, `routing_handler.go`, UI; proof: repo/handler tests + post-restart read.
- AC-010 → validator + handler reject bad values; surfaces `validator.go`, `routing_handler.go`; proof: validation tests.

## Data and Contracts

- Data model changes: `routing_providers` gains `weight INTEGER NOT NULL DEFAULT 0`; new `routing_aliases(tenant, alias, target, source, PRIMARY KEY(tenant, alias))`. See `data-model.md`.
- Contracts: `ProviderConfig` gains `weight`; new alias endpoints `GET/PUT/DELETE /api/v1/routing/aliases`; `RoutingConfig.Aliases` in YAML.
- Compatibility: weight defaults to 0 (unweighted); existing routes behave identically; aliases are additive.

## Implementation Strategy

- DEC-001 Alias resolution lives in the selector, not a middleware
  Why: one place already owns model→provider resolution, and non-chat endpoints all funnel through `Select`. Not: rewriting the request body in middleware.
  Tradeoff: key model-scope checks run before aliasing, so they evaluate the requested name (recorded limitation).
  Affects: `selector.go`, `registry.go`.
  Validation: alias tests; scope behavior documented.
- DEC-002 Persist aliases in a dedicated `routing_aliases` table
  Why: AC-009 requires UI-durable, restart-safe aliases; the existing tenant→model field is config-only and unused. Not: overloading `routing_model_routes`.
  Tradeoff: a migration and repository methods.
  Affects: migration, `routing_registry.go`, `repository.go`.
  Validation: repo round-trip test.
- DEC-003 Weighted pick with an injectable RNG
  Why: deterministic tests for AC-006 without flaky statistics; production uses `math/rand`.
  Not: asserting statistical ratios in tests.
  Tradeoff: an extra constructor seam.
  Affects: `selector.go`.
  Validation: `chooseWeighted` unit test.
- DEC-004 Effective weight = `weight` when >0 else 1; equal effective weights → first declared
  Why: preserves today's ordered behavior (AC-007) and keeps unweighted providers participating rather than dropping them. Not: treating 0 as exclusion.
  Tradeoff: to exclude a provider, mark it unhealthy.
  Affects: `selector.go`.
  Validation: equal-weight and mixed-weight tests.
- DEC-005 Alias is single-hop and resolved before wildcard/exact lookup
  Why: RQ-001/003 ordering and no cycle risk. Not: recursive alias chains.
  Tradeoff: chained aliases require operator flattening.
  Affects: `selector.go`.
  Validation: identity/no-loop test.
- DEC-006 YAML aliases seed insert-if-absent like providers
  Why: reuse the per-entry idempotent seed semantics from the registry feature; UI edits survive restart. Not: overwrite on restart.
  Tradeoff: changing an existing alias via YAML requires the UI or (tenant,alias) delete.
  Affects: `routing_registry.go`, `bootstrap/routing.go`.
  Validation: seed test.
- DEC-007 Fallback chain = weighted primary first, then the rest in declared order
  Why: the handler already treats the returned list as the fallback chain; a reorder keeps retry semantics. Not: rebuilding fallback logic.
  Tradeoff: fallback order is not itself weighted.
  Affects: `selector.go`.
  Validation: chain-order test.

## Incremental Delivery

### MVP (First Value)

- Config/domain fields + migration/registry persistence + selector alias + tier/weighted pick; validator.
- MVP readiness: `AC-001`, `AC-003`, `AC-005`, `AC-006`, `AC-007`, `AC-008` pass.

### Iterative Expansion

- Admin API + UI for aliases and weight → `AC-002`, `AC-004`, `AC-009`, `AC-010`.

## Sequencing Notes

- Migration + repository + config first (alias/weight persistence).
- Registry alias storage + bootstrapping before selector alias resolution.
- Selector changes are the core; admin/UI last.
- Keep `GetProviderList` consistent with alias resolution if it is used outside the handler.

## Risks

- Key model scopes evaluate the requested name, not the routed name (DEC-001).
  Mitigation: documented; revisit by resolving aliases before `model_access` if required.
- Weighted randomness could make routing hard to reproduce.
  Mitigation: deterministic tests via injected RNG; equal-weight deterministic default.
- Migration on existing deployments.
  Mitigation: additive column with default + new table; no data backfill needed.
- Seed semantics for aliases mirror providers (existing entries untouched).
  Mitigation: covered by a seed test.

## Rollout and Compatibility

- Additive migration; no flag. Unweighted/alias-free configs behave exactly as before.
- Operational follow-up: audit entries for alias/weight changes via existing routing audit.

## Validation

- Automated: `go test ./src/internal/domain/routing/... ./src/internal/api/handler/admin/... ./src/internal/infra/config/... ./src/internal/adapters/repository/postgres/...` plus `make test`; UI vitest.
- New tests: alias resolution, weighted split (stubbed RNG), priority tier, unhealthy exclusion, alias/weight persistence, validation, UI alias/weight edits.
- Proves: `AC-001`–`AC-010`; `DEC-001`–`DEC-007`.

## Constitution Compliance

- no conflicts. Tenant-scoped policy preserved; Content Shield unchanged; native-only data plane; no new runtime dependencies.
