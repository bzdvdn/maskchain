---
report_type: verify
slug: shield-detector-catalog
status: pass
docs_language: en
generated_at: 2026-09-21
---

# Verify Report: shield-detector-catalog

## Scope

- snapshot: a read-only admin `GET /api/v1/shield/catalog` reports the registered detector types, allowed reactions and loaded compliance packs, and the UI pack selectors are driven by it instead of a hardcoded list.
- verification_mode: default
- artifacts:
  - CONSTITUTION.md
  - specs/active/shield-detector-catalog/spec.md
  - specs/active/shield-detector-catalog/plan.md
  - specs/active/shield-detector-catalog/tasks.md
- inspected_surfaces:
  - src/internal/api/handler/admin/catalog_handler.go (+ test)
  - src/internal/api/admin.go (route), src/internal/api/admin_test.go
  - src/cmd/internal/bootstrap/compliance.go
  - src/cmd/admin/run.go, src/cmd/all/admin.go
  - ui/src/api/shield.ts, ui/src/hooks/useShieldCatalog.ts
  - ui/src/pages/Compliance.tsx (+ test), ui/src/pages/Tenants/TenantDetail.tsx (+ test)
  - ui/src/api/tenants.ts

## Verdict

- status: pass
- archive_readiness: safe
- summary: all 9 tasks carry `Proof:` lines, 5 catalog Go tests and 3 UI tests pass, every AC maps to passing evidence, and the hardcoded pack list is gone from production UI code.

## Checks

- task_state: completed=9, open=0 (verify-task-state.sh: `OK: all tasks are marked complete`; `PROOFS_MISSING=0`).
- acceptance_evidence:
  - AC-001 -> `TestCatalogDetectorsAndReactions` (sorted detectors + 4 reactions) and route wiring `admin.go:RegisterCatalogHandler`.
  - AC-002 -> `TestCatalogPacks` (key+name, sorted).
  - AC-003 -> `TestCatalogRequiresAdminSession` (401 without session).
  - AC-004 -> `TestCatalogOmitsUnregisteredType` (declared `presidio` absent).
  - AC-005 -> `TestCatalogEmptyPacks` (200, `packs: []`, detectors/reactions present).
  - AC-006 -> `Compliance.test.tsx` (options from mocked catalog, no hardcoded pack) + `TenantDetail.test.tsx`; `COMPLIANCE_PACKS` removed from `ui/src/api/tenants.ts` and absent from production sources.
- implementation_alignment:
  - `CatalogHandler.build()` sorts detectors, the four reactions and packs by key, and returns an empty (non-nil) pack list when the registry is nil.
  - `ReferenceDetectorRegistry` is shared by preset validation and the catalog, so advertised types mirror the validated set.
  - Both admin binaries register the handler with reference detector types and the loaded compliance registry.

## Errors

- none

## Warnings

- none

## Questions

- none

## Not Verified

- Live admin session end-to-end with a real preset directory: route auth is verified with a stub session middleware, and pack loading is verified at the handler level; no running admin instance was exercised.
- Envelope wrapping of the catalog response was not asserted in the handler test (the test decodes the raw handler body); the shared envelope middleware is covered by other tests.
- The catalog was not confirmed against a live gateway detector set — both sides use the same reference registry by construction.

## Next Step

- safe to archive
