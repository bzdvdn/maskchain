# Shield Detector & Pack Catalog Plan

## Phase Contract

Inputs: `spec.md` + minimal repo context.
Outputs: `plan.md` (no data-model/contracts expansion).
Stop if: spec is vague — not the case; decisions pinned below.

## Goal

Add an admin-session read-only `GET /api/v1/shield/catalog` that reports the reference detector types, the allowed reactions and the loaded compliance packs, then switch the UI Compliance page to build its pack selector from that response. The catalog is assembled from in-memory registries, so it is cheap and deterministic.

## MVP Slice

- Handler + route + admin wiring + response shape.
- AC covered: AC-001, AC-002, AC-003.

## First Validation Path

1. Start the admin binary with a preset dir; log in.
2. `curl -H "Authorization: Bearer <token>" /api/v1/shield/catalog` → detectors, reactions, packs.
3. Same call without the header → 401.
4. Open the Compliance page → the pack selector lists the loaded packs.

## Scope

- New admin handler and route under the admin session middleware.
- Export the reference detector registry from bootstrap so the admin process can report the same set it validates presets against.
- New UI API client and Compliance page consumption; remove the hardcoded pack list.
- Untouched: gateway shield endpoints, compliance apply/report, preset loading semantics.

## Performance Budget

- In-memory only; response < 50ms (SC-001). No I/O per request, no caching layer.

## Implementation Surfaces

- `src/internal/api/handler/admin/catalog_handler.go` — new: builds the catalog response from detector types + reactions + packs.
- `src/internal/api/handler/admin/catalog_handler_test.go` — new: handler unit tests.
- `src/internal/api/admin.go` — new `RegisterCatalogHandler` under admin session.
- `src/cmd/internal/bootstrap/compliance.go` — export the reference detector registry (currently unexported) and reuse it in `LoadComplianceRegistry`.
- `src/cmd/admin/run.go`, `src/cmd/all/admin.go` — wire the catalog handler.
- `ui/src/api/shield.ts` — new: `getShieldCatalog()`.
- `ui/src/pages/Compliance.tsx` — populate packs from the catalog.
- `ui/src/api/tenants.ts` — remove the hardcoded `COMPLIANCE_PACKS` constant.
- `ui/src/pages/Compliance.test.tsx` — new: selector renders from a mocked catalog.

## Bootstrapping Surfaces

- none — all packages exist; only new files are the handler, its test, and the UI client.

## Architecture Impact

- Local: one admin handler + one exported bootstrap helper + one UI client.
- Integration: new admin API surface; the UI gains a startup fetch on the Compliance page.
- No DB migration, no config change, no gateway impact.

## Acceptance Approach

- AC-001 → handler builds detectors from the reference registry types and reactions from entity constants, sorted; proof: `catalog_handler_test.go` asserting the exact type/reaction sets.
- AC-002 → handler maps `compliance.Registry.Packs()` to `{key,name}` sorted by key; proof: handler test with two known packs.
- AC-003 → route registered with the admin session middleware; proof: admin route test without credentials returns 401.
- AC-004 → catalog uses only registered types, so a declared-but-unregistered constant never appears; proof: handler test with a registry omitting a declared type.
- AC-005 → nil/empty compliance registry yields `packs: []` and HTTP 200; proof: handler test with nil packs.
- AC-006 → Compliance page fetches `getShieldCatalog()`; `COMPLIANCE_PACKS` removed; proof: UI test with mocked catalog.

## Data and Contracts

- Data model: no change.
- API contract: new admin `GET /api/v1/shield/catalog` returning `{detectors:[string], reactions:[string], packs:[{key,name}]}` inside the standard envelope. No existing contract changes.
- Config contract: none.
- No `contracts/*` or `data-model.md` needed.

## Implementation Strategy

- DEC-001 Export the bootstrap reference detector registry
  Why: the admin process must report the same detector set it validates presets against; one source prevents drift.
  Tradeoff: one more exported symbol from bootstrap.
  Affects: src/cmd/internal/bootstrap/compliance.go.
  Validation: existing preset validation still passes; handler test uses the same set.
- DEC-002 Handler depends on plain detector types + compliance registry, not the live engine
  Why: admin does not run the shield engine; the reference registry mirrors the gateway set.
  Tradeoff: the catalog describes the supported set, not a specific gateway process's live state.
  Affects: catalog_handler.go, admin wiring.
  Validation: handler tests.
- DEC-003 Register the route inside the DB-backed admin block
  Why: admin session auth requires the session store; the catalog itself needs no data.
  Tradeoff: no DB → no catalog (same as every other admin API).
  Affects: cmd/admin/run.go, cmd/all/admin.go.
  Validation: route test via the admin server with session middleware.
- DEC-004 No hardcoded fallback pack list in the UI
  Why: a fallback list would reintroduce the drift this feature removes.
  Tradeoff: if the endpoint is unavailable the selector is empty; the page shows an error/empty state.
  Affects: Compliance.tsx, tenants.ts.
  Validation: UI test asserting options come only from the mocked catalog.
- DEC-005 Deterministic ordering
  Why: stable diffs and stable UI rendering.
  Tradeoff: none.
  Affects: catalog_handler.go.
  Validation: handler test asserts sorted output.

## Incremental Delivery

### MVP (First Value)

- Handler, route, admin wiring, response shape.
- Ready when AC-001/002/003 pass via handler + route tests.

### Iterative Expansion

- AC-004/005 edge behavior (unregistered types, empty packs).
- AC-006 UI switch-over and removal of the hardcoded list.
- Each validated independently.

## Sequencing Notes

- Export the reference registry first (handler depends on it).
- Handler + route + wiring next; UI last.
- No feature flag required.

## Risks

- Admin/gateway detector drift → mitigated by reusing the single reference registry for both preset validation and the catalog (DEC-001).
- UI has no existing Compliance test → add one with a mocked catalog (AC-006).
- `/api/v1/shield/*` exists on the gateway, which could confuse operators → the catalog lives only on admin and is documented as control-plane.

## Rollout and Compatibility

- Additive admin endpoint; no migration or compatibility impact.
- Existing compliance apply/report endpoints unchanged.
- UI degrades to an empty selector if the endpoint is missing on an older backend.

## Validation

- Unit: `catalog_handler_test.go` (detectors/reactions, packs, unregistered type, empty packs, ordering).
- Route: admin catalog endpoint requires a session (401 without credentials).
- UI: Compliance selector renders from a mocked catalog; hardcoded constant removed.
- Acceptance IDs covered: AC-001..AC-006. Decisions: DEC-001..DEC-005.

## Constitution Compliance

- no conflicts — read-only control-plane endpoint, no new dependency, no change to DLP behavior, UI stays an operator console.
