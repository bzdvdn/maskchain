# Providers, Models & Routing IA Tasks

## Phase Contract

Inputs: `plan.md` + `spec.md` (decisions pinned).
Outputs: ordered executable tasks with coverage mapping.
Stop if: acceptance coverage cannot be mapped — not the case.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/domain/routing/config.go | T1.1 |
| src/internal/domain/routing/service/selector.go | T1.1 |
| src/internal/domain/routing/service/service_test.go | T4.1 |
| src/internal/api/dto/routing.go | T1.2 |
| src/internal/api/handler/admin/routing_handler.go | T2.1, T2.2 |
| src/internal/api/handler/admin/routing_handler_test.go | T4.2 |
| src/internal/api/admin.go | T2.1 |
| src/cmd/admin/run.go | T2.3 |
| src/cmd/all/admin.go | T2.3 |
| ui/src/api/routing.ts | T1.3 |
| ui/src/pages/Providers.tsx | T2.4 |
| ui/src/pages/Models.tsx | T3.1 |
| ui/src/pages/Routing.tsx | T3.2 |
| ui/src/App.tsx | T3.3 |
| ui/src/components/Layout.tsx | T3.3 |
| ui/src/components/CommandPalette.tsx | T3.3 |
| src/internal/adapters/provider/models.go | T5.1 |
| ui/src/pages/Providers.tsx | T2.4, T5.2 |
| ui/src/pages/Providers.test.tsx | T4.3, T5.3 |
| ui/src/pages/Models.test.tsx | T4.3 |
| ui/src/pages/Routing.test.tsx | T3.2, T4.3 |
| ui/src/components/Layout.test.tsx | T4.3 |

## Implementation Context

- MVP Goal: a model can have global default providers that apply to every tenant without an override, and a provider can be created/edited with a proxy and its models.
- Acceptance Boundaries: AC-001..AC-010.
- Key Rules: global marker is the reserved tenant `*` with a two-pass selector (tenant wins, global fallback) — DEC-001; aggregate models endpoint on the routing handler — DEC-002; provider+models written atomically via `RunInTx` with placeholder cost rates created only if absent — DEC-003; UI split into three pages — DEC-004; api_type-aware, masked-secret-aware provider validation plus a proxy field — DEC-005; global routes are `routing_model_routes` rows with tenant `*` — DEC-006.
- Invariants: existing routes for `""`/`default` keep selecting; tenant overrides are never mutated by global edits; `*` is never offered as a tenant and never counted as an override; no schema change.
- Contracts/Protocols: `GET /api/v1/routing/models` → `{model, input_price_per_1k, output_price_per_1k, currency, default_providers:[], override_count, source}`; `PUT /api/v1/routing/providers` accepts optional `models:[string]`; provider `api_keys` may be empty for `ollama`/`bedrock` (bedrock uses AWS creds).
- Scope Boundaries: do not add routing strategies or per-tenant pricing; do not change tenant/budget/key pages; do not migrate existing routes.
- Proof Signals: Go selector tests (fallback/precedence/no-route/isolation), handler tests (aggregate, atomic success/rollback, masked key/proxy), UI tests for the three pages and navigation, `go test`/`vitest` green.
- References: DEC-001..DEC-006 (plan.md), RQ-001..RQ-010 (spec.md).

## Phase 1: Foundation

Goal: the global marker, the selector fallback, the DTOs and the UI client exist.

- [x] T1.1 Add the global tenant marker and selector fallback — outcome: `GlobalTenant` is defined and `Select`/`GetProviderList` prefer a tenant route and fall back to the global one, preserving existing behavior for `""`/`default`. Touches: src/internal/domain/routing/config.go, src/internal/domain/routing/service/selector.go
      Proof: code src/internal/domain/routing/service/selector.go providersFor
- [x] T1.2 Extend routing DTOs — outcome: the provider request accepts a `models` list and a model-aggregate DTO exists for the Models page. Touches: src/internal/api/dto/routing.go
      Proof: code src/internal/api/dto/routing.go ModelAggregate
- [x] T1.3 Extend the UI routing client — outcome: `listModels()` and `upsertProvider` with `models` are available with types. Touches: ui/src/api/routing.ts
      Proof: code ui/src/api/routing.ts listModels

## Phase 2: MVP Slice

Goal: global defaults work and providers can be created with models.

- [x] T2.1 Add the model aggregate endpoint — outcome: `GET /api/v1/routing/models` returns cost, default providers and override count per model, excluding the global marker from the override count. Touches: src/internal/api/handler/admin/routing_handler.go, src/internal/api/admin.go
      Proof: code src/internal/api/handler/admin/routing_handler.go ListModels
- [x] T2.2 Make the provider upsert atomic and api_type-aware — outcome: provider + global routes + placeholder cost rates are written in one transaction, keys may be empty for ollama/bedrock, and a failure leaves nothing behind. Touches: src/internal/api/handler/admin/routing_handler.go
      Proof: code src/internal/api/handler/admin/routing_handler.go UpsertProvider
- [x] T2.3 Wire the cost-rate repo and transaction runner — outcome: both admin binaries construct the routing handler with cost rates and a transaction runner. Touches: src/cmd/admin/run.go, src/cmd/all/admin.go
      Proof: code src/cmd/admin/run.go NewRoutingHandler
- [x] T2.4 Build the Providers page — outcome: full provider CRUD with proxy, auth, AWS and models chips, with api_type-aware masked-secret-aware validation. Touches: ui/src/pages/Providers.tsx
      Proof: code ui/src/pages/Providers.tsx Providers

## Phase 3: Core Implementation

Goal: the Models and Routing pages and navigation.

- [x] T3.1 Build the Models page — outcome: models list cost, default providers and override count; editing sets cost and ordered defaults; deleting removes cost rate and global route. Touches: ui/src/pages/Models.tsx
      Proof: code ui/src/pages/Models.tsx Models
- [x] T3.2 Rework the Routing page — outcome: it manages per-tenant overrides only and shows when a model inherits the global default. Touches: ui/src/pages/Routing.tsx, ui/src/pages/Routing.test.tsx
      Proof: code ui/src/pages/Routing.tsx Routing
- [x] T3.3 Add navigation and routes — outcome: Providers, Models and Routing appear as separate destinations with routes registered. Touches: ui/src/App.tsx, ui/src/components/Layout.tsx, ui/src/components/CommandPalette.tsx
      Proof: code ui/src/components/Layout.tsx navSections

## Phase 4: Validation

Goal: prove behavior and leave the package reviewable.

- [x] T4.1 Add selector coverage — outcome: tests assert global fallback, tenant precedence, no-route, preserved `default` behavior, and isolation of overrides from global edits (AC-001, AC-002, AC-009, AC-010). Touches: src/internal/domain/routing/service/service_test.go
      Proof: test src/internal/domain/routing/service/service_test.go TestRouteSelectorGlobalFallback
- [x] T4.2 Add handler coverage — outcome: tests assert the aggregate shape, atomic success and rollback, and masked-key/proxy/ollama/bedrock validation (AC-003, AC-004, AC-005). Touches: src/internal/api/handler/admin/routing_handler_test.go
      Proof: test src/internal/api/handler/admin/routing_handler_test.go TestRoutingHandlerListModelsAggregate
- [x] T4.3 Add UI coverage — outcome: tests assert Providers validation and proxy, Models cost/defaults/delete, Routing overrides and inheritance, and the three navigation entries (AC-005, AC-006, AC-007, AC-008). Touches: ui/src/pages/Providers.test.tsx, ui/src/pages/Models.test.tsx, ui/src/pages/Routing.test.tsx, ui/src/components/Layout.test.tsx
      Proof: test ui/src/pages/Providers.test.tsx Providers

## Phase 5: Provider model discovery

Goal: operators can pull a provider's model list instead of typing ids.

- [x] T5.1 Add provider model discovery — outcome: an admin endpoint returns a provider's model ids by querying its models API with the provider's auth and egress proxy, and returns a clear unsupported error for provider types without a models API. Touches: src/internal/adapters/provider/models.go, src/internal/api/handler/admin/routing_handler.go, src/internal/api/admin.go, src/cmd/admin/run.go, src/cmd/all/admin.go
      Proof: code src/internal/adapters/provider/models.go ModelDiscoverer
- [x] T5.2 Build the model picker in the Providers form — outcome: a searchable list loaded from the provider populates the provider's models, with manual entry kept as a fallback when loading fails. Touches: ui/src/api/routing.ts, ui/src/pages/Providers.tsx
      Proof: code ui/src/pages/Providers.tsx loadModels
- [x] T5.3 Add discovery coverage — outcome: tests assert discovery parsing and the unsupported error for a provider type, and the UI loads and selects models plus the manual fallback (AC-011, AC-012). Touches: src/internal/adapters/provider/provider_test.go, src/internal/api/handler/admin/routing_handler_test.go, ui/src/pages/Providers.test.tsx
      Proof: test src/internal/adapters/provider/provider_test.go TestModelDiscoverer

## Acceptance Coverage

- AC-011 -> T5.1, T5.3
- AC-012 -> T5.2, T5.3
- AC-001 -> T1.1, T4.1
- AC-002 -> T1.1, T4.1
- AC-003 -> T1.2, T2.1, T4.2
- AC-004 -> T2.2, T2.3, T4.2
- AC-005 -> T2.2, T2.4, T4.2, T4.3
- AC-006 -> T1.3, T3.1, T4.3
- AC-007 -> T3.2, T4.3
- AC-008 -> T3.3, T4.3
- AC-009 -> T1.1, T4.1
- AC-010 -> T2.1, T4.1

## Notes

- No data model, migration, or config change; the global default is a `routing_model_routes` row with tenant `*`.
- Deliver in increments: MVP (AC-001/002/005) first, then aggregate/atomic, Models, Routing, nav, tests.
- Keep `Routing.tsx` provider/cost-rate tables removed only after the new pages exist.
