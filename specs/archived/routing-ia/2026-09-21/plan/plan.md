# Providers, Models & Routing IA Plan

## Phase Contract

Inputs: `spec.md` + minimal repo context.
Outputs: `plan.md` (no data-model/contracts expansion).
Stop if: spec is vague — not the case; decisions pinned below.

## Goal

Split routing into three surfaces backed by a global default. Backend: a reserved global tenant marker with selector fallback, an aggregate models endpoint, and an atomic provider+models write. Frontend: Providers / Models / Routing pages, with api_type-aware provider validation and the egress proxy exposed. Storage is reused as-is (global routes are rows with tenant `*`), so no migration.

## MVP Slice

- Global fallback + provider CRUD fix (validation + proxy) + Providers page.
- AC covered: AC-001, AC-002, AC-005.

## First Validation Path

1. Create a provider (openai/openrouter) with a proxy and two model ids.
2. Open Models: both models show the provider as default and cost 0.
3. Set a cost; send a request as a tenant without an override → default provider used.
4. Add a tenant override on Routing → that tenant uses its own provider order.

## Scope

- Backend: global marker + selector fallback; aggregate endpoint; atomic provider+models; api_type-aware upsert validation.
- Frontend: three pages, navigation, API client additions, provider form (proxy, AWS, masked-aware validation, models chips).
- No schema change; global routes are `routing_model_routes` rows with tenant `*`.

## Out of Scope

- New routing strategies, per-tenant pricing, provider registry YAML, tenant/budget/key page rework.

## Performance Budget

- Selector adds at most one extra rule scan (global) per selection; registry rules are in-memory and small. Aggregate endpoint is two list queries. No hot-path allocation change beyond a constant.

## Implementation Surfaces

- `src/internal/domain/routing/config.go` — new `GlobalTenant` constant.
- `src/internal/domain/routing/service/selector.go` — two-pass selection (tenant then global) for `Select` and `GetProviderList`.
- `src/internal/domain/routing/service/service_test.go` — fallback/precedence/no-route tests.
- `src/internal/api/dto/routing.go` — `ProviderRequest.Models` and the model aggregate DTO.
- `src/internal/api/handler/admin/routing_handler.go` — aggregate endpoint; atomic provider+models; api_type-aware key validation.
- `src/internal/api/handler/admin/routing_handler_test.go` — aggregate/atomic/masked-key tests.
- `src/internal/api/admin.go` — register `GET /routing/models`.
- `src/cmd/admin/run.go`, `src/cmd/all/admin.go` — pass the cost-rate repo and a transaction runner to the routing handler.
- `ui/src/api/routing.ts` — `listModels()`, `upsertProvider` with `models`, model aggregate type.
- `ui/src/pages/Providers.tsx` (new), `ui/src/pages/Models.tsx` (new), `ui/src/pages/Routing.tsx` (reworked to tenant overrides).
- `ui/src/App.tsx`, `ui/src/components/Layout.tsx`, `ui/src/components/CommandPalette.tsx` — routes and navigation.
- `ui/src/pages/Providers.test.tsx`, `ui/src/pages/Models.test.tsx`, `ui/src/pages/Routing.test.tsx` — UI tests.

## Bootstrapping Surfaces

- none — storage, repository, selector, handler and UI primitives exist.

## Architecture Impact

- Local: selector gains a fallback; the routing handler gains cost-rate access and a transaction runner.
- Integration: the admin API grows two operations; the UI splits one page into three.
- Compatibility: existing routes for `""`/`default` are untouched; `*` is new and reserved. No migration.

## Acceptance Approach

- AC-001 → selector two-pass global fallback; proof: selector test for an unconfigured tenant.
- AC-002 → tenant route precedence and no-route; proof: selector tests for override and no-route.
- AC-003 → aggregate endpoint over `ListRules` + `CostRateRepository.List`; proof: handler test.
- AC-004 → handler runs provider + routes + placeholder cost rates in one `RunInTx`; proof: handler test for success and rollback.
- AC-005 → upsert validation by api_type; proxy persisted; masked secrets preserved; proof: handler/repo tests + UI validation tests.
- AC-006 → Models page uses aggregate + cost-rate/route APIs; delete removes both; proof: UI tests.
- AC-007 → Routing page lists tenant overrides and shows inheritance; proof: UI test.
- AC-008 → nav entries; proof: navigation test.
- AC-009 → existing selector/registry tests unchanged plus a regression case for `default`.
- AC-010 → isolation test: editing the global route leaves tenant overrides intact.
- AC-011 → `ModelDiscoverer` per api_type via egress; unsupported returns `ErrModelsUnsupported`; proof: provider discovery tests + handler test.
- AC-012 → Providers form loads and selects models with a manual fallback; proof: UI picker tests.

## Data and Contracts

- Data model: no change; `routing_model_routes.tenant = '*'` is the global default (documented, reserved).
- API contract: new `GET /api/v1/routing/models`; `PUT /api/v1/routing/providers` accepts an optional `models` array. Existing provider/route/cost-rate contracts unchanged.
- Config contract: none.
- No `data-model.md` needed; the reserved marker is documented in the plan and spec.

## Implementation Strategy

- DEC-001 Reserved global tenant marker `*` with selector fallback
  Why: additive and migration-free; tenant overrides stay authoritative; the existing `""`→`default` coercion is preserved.
  Tradeoff: a new reserved value; UI must forbid it as a real tenant.
  Affects: routing/config.go, selector.go.
  Validation: selector tests.
- DEC-002 Aggregate models endpoint on the routing handler
  Why: the Models page needs cost + defaults + override count in one call; composing on the client is more requests and duplicated logic.
  Tradeoff: the handler gains a cost-rate dependency (constructor + wiring change).
  Affects: routing_handler.go, admin.go, cmd wiring.
  Validation: handler test.
- DEC-003 Atomic provider+models orchestrated by the handler inside `RunInTx`
  Why: reuses the existing tx-aware context pattern and keeps repositories decoupled; a partial save is impossible.
  Tradeoff: the handler takes a transaction runner; placeholder cost rates are created only when absent.
  Affects: routing_handler.go, cmd wiring.
  Validation: success + rollback tests.
- DEC-004 Split the UI into Providers / Models / Routing
  Why: matches the operator mental model and the spec's IA.
  Tradeoff: more files; routing logic moves out of one page.
  Affects: Providers.tsx, Models.tsx, Routing.tsx, App.tsx, Layout.tsx, CommandPalette.tsx.
  Validation: page tests.
- DEC-005 api_type-aware provider validation and proxy field
  Why: fixes the reported save failure for providers with masked secrets and exposes an already-supported capability.
  Tradeoff: validation rules differ per type and must be tested.
  Affects: Providers.tsx (validation), routing_handler.go (relax keys for ollama/bedrock).
  Validation: UI + handler tests.
- DEC-007 Provider model discovery behind an adapter-injected `ModelDiscoverer`
  Why: the admin layer must not import the egress adapter; a small interface keeps layering clean and lets the handler be tested with a fake, while the real implementation lives in `adapters/provider` and reuses the existing auth-header and egress-transport code.
  Tradeoff: another constructor dependency and a per-api_type mapping; Bedrock and Gemini listing are deferred (clear unsupported error).
  Affects: adapters/provider/models.go (new), routing_handler.go, cmd wiring, UI client + Providers page.
  Validation: discovery tests with a fake provider endpoint; UI picker tests.
- DEC-006 Global routes reuse `routing_model_routes` with tenant `*`
  Why: no schema change, and the repository/selector already handle arbitrary tenant values.
  Tradeoff: the reserved marker must be excluded from tenant-facing listings.
  Affects: repository (no change), handler (filter/accept), UI (label as "Global default").
  Validation: handler + UI tests.

## Incremental Delivery

### MVP (First Value)

- Global fallback + provider CRUD fix + Providers page.
- Ready when AC-001/002/005 pass.

### Iterative Expansion

- Aggregate + atomic write (AC-003/004) → Models page (AC-006) → Routing page (AC-007) → nav (AC-008) → regression/isolation (AC-009/010).

## Sequencing Notes

- Backend first (marker/selector, then handler endpoints), then API client, then pages, then nav, then tests.
- The `*` marker must be excluded from tenant pickers and from the "override count" (only non-global tenants count).

## Risks

- Existing `default` routes could be confused with the global marker → mitigated by using `*` (distinct) and a regression test.
- Atomic write failure leaving partial data → mitigated by DEC-003 and a rollback test.
- Cost-rate placeholder overwriting a real price → mitigated by "create only if absent".
- UI validation divergence from backend → mitigated by api_type-aware rules tested on both sides.
- Scope size → delivered in increments (MVP first).

## Rollout and Compatibility

- No migration; existing routes and selection for `""`/`default` are unchanged.
- Operators adopt the global default by adding `*` routes through the new pages.
- Watch no-route metrics after rollout to confirm defaults resolve.

## Validation

- Go: selector fallback/precedence/no-route; handler aggregate/atomic/masked-key.
- TS: Providers validation (masked/bedrock/ollama, proxy), Models cost+defaults+delete, Routing overrides+inheritance, nav.
- Regression: existing routing tests green.
- Acceptance IDs covered: AC-001..AC-010. Decisions: DEC-001..DEC-006.

## Constitution Compliance

- no conflicts — routing/control-plane only; no DLP or data-plane weakening; no new dependency; tenant isolation unchanged (global default is opt-in per model).
