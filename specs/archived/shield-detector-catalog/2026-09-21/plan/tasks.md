# Shield Detector & Pack Catalog Tasks

## Phase Contract

Inputs: `plan.md` + `spec.md` (decisions pinned).
Outputs: ordered executable tasks with coverage mapping.
Stop if: acceptance coverage cannot be mapped — not the case.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/cmd/internal/bootstrap/compliance.go | T1.1 |
| src/internal/api/handler/admin/catalog_handler.go | T1.2 |
| src/internal/api/handler/admin/catalog_handler_test.go | T4.1 |
| src/internal/api/admin.go | T2.1 |
| src/cmd/admin/run.go | T2.2 |
| src/cmd/all/admin.go | T2.2 |
| src/internal/api/admin_test.go | T4.2 |
| ui/src/api/shield.ts | T3.1 |
| ui/src/hooks/useShieldCatalog.ts | T3.2 |
| ui/src/pages/Compliance.tsx | T3.2 |
| ui/src/pages/Tenants/TenantDetail.tsx | T3.2 |
| ui/src/pages/Tenants/TenantDetail.test.tsx | T3.2 |
| ui/src/api/tenants.ts | T3.2 |
| ui/src/pages/Compliance.test.tsx | T4.3 |

## Implementation Context

- MVP Goal: an admin-session `GET /api/v1/shield/catalog` returns registered detector types, allowed reactions and loaded packs; the UI pack selector is driven by it.
- Acceptance Boundaries: AC-001..AC-006.
- Key Rules: catalog is runtime truth (only registered types, DEC-002); single reference detector set shared with preset validation (DEC-001); no hardcoded UI fallback list (DEC-004).
- Invariants: deterministic order (sort detectors, reactions, packs by key); nil/empty compliance registry → `packs: []` with HTTP 200; route registered inside the DB-backed admin block (DEC-003).
- Errors/Codes: missing/invalid admin session → `401 UNAUTHORIZED`; no other error paths (read-only, in-memory).
- Contracts/Protocols: `GET /api/v1/shield/catalog` → envelope `data = {detectors:[string], reactions:[string], packs:[{key,name}]}`; reactions are the four `entity.Reaction` values.
- Scope Boundaries: do not expose the catalog on the gateway; do not report per-tenant rule status; do not implement new detector types.
- Proof Signals: handler unit tests (sets, ordering, empty packs, unregistered type), admin route auth test, UI test rendering options from a mocked catalog, `go test`/`vitest` green.
- References: DEC-001..DEC-005 (plan.md), RQ-001..RQ-006 (spec.md).

## Phase 1: Foundation

Goal: the reference detector set is shared and the catalog payload can be built.

- [x] T1.1 Export the reference detector registry — outcome: bootstrap exposes one detector registry reused by preset validation and available to admin wiring. Touches: src/cmd/internal/bootstrap/compliance.go
      Proof: code src/cmd/internal/bootstrap/compliance.go ReferenceDetectorRegistry
- [x] T1.2 Implement the catalog handler — outcome: given detector types and a compliance registry, it returns sorted detectors, the four reactions, and packs as `{key,name}`; nil packs yields an empty list. Touches: src/internal/api/handler/admin/catalog_handler.go
      Proof: code src/internal/api/handler/admin/catalog_handler.go CatalogHandler

## Phase 2: MVP Slice

Goal: the endpoint is reachable and protected.

- [x] T2.1 Register the catalog route under the admin session — outcome: `GET /api/v1/shield/catalog` is served by the admin server behind the session middleware. Touches: src/internal/api/admin.go
      Proof: code src/internal/api/admin.go RegisterCatalogHandler
- [x] T2.2 Wire the catalog handler in the admin binaries — outcome: admin and combined processes register the handler with the reference detector types and the loaded compliance registry (nil allowed). Touches: src/cmd/admin/run.go, src/cmd/all/admin.go
      Proof: code src/cmd/admin/run.go RegisterCatalogHandler

## Phase 3: Core Implementation

Goal: the UI consumes the catalog instead of a hardcoded list.

- [x] T3.1 Add the UI catalog client — outcome: a typed `getShieldCatalog()` returns detectors, reactions and packs. Touches: ui/src/api/shield.ts
      Proof: code ui/src/api/shield.ts getShieldCatalog
- [x] T3.2 Drive the pack selectors from the catalog — outcome: the Compliance page and the tenant Compliance tab populate pack options from the catalog and the hardcoded pack constant is removed; empty catalog shows an empty state. Touches: ui/src/hooks/useShieldCatalog.ts, ui/src/pages/Compliance.tsx, ui/src/pages/Tenants/TenantDetail.tsx, ui/src/pages/Tenants/TenantDetail.test.tsx, ui/src/api/tenants.ts
      Proof: code ui/src/pages/Compliance.tsx Compliance

## Phase 4: Validation

Goal: prove behavior and leave the package reviewable.

- [x] T4.1 Add handler unit coverage — outcome: tests assert exact detector/reaction sets, packs mapping, sorted order, a declared-but-unregistered type being absent, and empty packs returning 200 with detectors present (AC-001, AC-002, AC-004, AC-005). Touches: src/internal/api/handler/admin/catalog_handler_test.go
      Proof: test src/internal/api/handler/admin/catalog_handler_test.go TestCatalogDetectorsAndReactions
- [x] T4.2 Add route auth coverage — outcome: the catalog route returns 401 without a session when the session middleware is active (AC-003). Touches: src/internal/api/admin_test.go
      Proof: test src/internal/api/admin_test.go TestCatalogRequiresAdminSession
- [x] T4.3 Add UI coverage for catalog-driven options — outcome: the Compliance selector renders options from a mocked catalog response (AC-006). Touches: ui/src/pages/Compliance.test.tsx
      Proof: test ui/src/pages/Compliance.test.tsx Compliance

## Acceptance Coverage

- AC-001 -> T1.1, T1.2, T2.1, T2.2, T4.1
- AC-002 -> T1.2, T4.1
- AC-003 -> T2.1, T4.2
- AC-004 -> T1.2, T4.1
- AC-005 -> T1.2, T4.1
- AC-006 -> T3.1, T3.2, T4.3

## Notes

- Ordering is dependency-driven: shared registry → handler → route/wiring → UI → tests.
- No data model or migration work; no config change.
- T4.3 must mock the catalog client, not hit a live backend.
