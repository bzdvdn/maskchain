# Compliance Packs (401) Tasks

## Phase Contract

Inputs: plan.md, spec.md (for AC boundaries).
Outputs: ordered executable tasks with AC coverage.
Stop if: tasks become vague or coverage cannot be mapped — it is not; all 5 ACs map to concrete work.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/domain/compliance/ (new) | T1.1, T1.2, T3.1, T4.1 |
| src/internal/app/compliance/ (new) | T2.1, T2.2, T3.1, T3.2, T4.1 |
| src/internal/api/handler/admin/ | T2.3, T2.4, T3.2 |
| src/internal/infra/config/ | T1.2, T2.1 |
| src/cmd/internal/bootstrap/ | T2.3, T2.4 |
| src/internal/domain/shield/ (detector/registry.go, reaction/) | T1.1 |
| src/internal/domain/shield/entity/tenant.go | T2.2 |
| src/internal/api/server.go | T2.3, T2.4 |
| ui/src/pages/Tenants/ | T3.3 |
| specs/active/401-compliance-packs/ (presets + testdata) | T1.1, T1.2, T4.1 |
| deployments/ | T4.2 |

## Implementation Context

- MVP goal: static CompliancePack YAML presets load at startup; apply a pack to a tenant in one action; report shows active vs. deviated rules. (DEC-001..004, RQ-001..006)
- Invariants/semantics:
  - Packs reference existing detector/reaction/masking IDs by identifier; they never redefine rule semantics (DEC-002).
  - Pack is static config, not a persisted entity; tenant shield policy stays in PostgreSQL (data-model.md no-change).
  - Apply is one atomic action; unknown pack element => clear error, tenant unchanged (RQ-005, AC-003).
  - Apply is idempotent and preserves operator customizations; report flags them deviated, never overwrites (AC-002, AC-005).
- Errors/codes:
  - Unknown detector/reaction ID in YAML => load/apply error; that pack skipped or rejected with a clear message.
  - YAML syntax/schema error => that pack not loaded with recorded errors; existing packs keep working (Edge Cases w/ Edge Cases).
  - Unknown pack key on apply => error, no tenant change.
- Contracts/protocol:
  - CompliancePack YAML shape: pack key -> {detectors: [{type, enabled, reaction, masking}], resolutions?} presets; fixtures under testdata.
  - New admin endpoints (additive): apply-pack(POST), compliance-report(GET), both behind existing adminSessionMw.
  - No schema migration, no data-model change, no new storage (data-model.md no-change).
- Scope boundaries:
  - Do NOT modify detectors/reactions/masking/resolutions — packs only reference existing ones.
  - Do NOT add multi-pack-per-tenant, reverse-generation from rules, pack import/export (spec Out of Scope).
- Proof signals: unit tests for loader/validator/apply/report + repo-wide go build/vet/test + golangci-lint clean; manual admin apply + report against a test tenant.
- References: DEC-001..DEC-004, DM (no-change), RQ-001..006, AC-001..005.

## Phase 1: Foundation

Goal: establish the CompliancePack domain + loader/validation so downstream phases depend on a valid pack source.

- [x] T1.1 Add domain/compliance pack entity + YAML loader — outcome: a `CompliancePack` entity (pack key -> enabled detectors with reaction/masking, resolutions) with a loader that parses preset YAML and validates referenced detector/reaction IDs against the existing shield registry; unknown IDs produce a clear per-pack error; packs are registered by key. Touches: src/internal/domain/compliance/ (new), src/internal/domain/shield/detector/registry.go, specs/active/401-compliance-packs/ (testdata presets)
      Proof: code src/internal/domain/compliance/pack.go Validate
      Proof: code src/internal/domain/compliance/loader.go LoadPacksFromDir
      Proof: test src/internal/domain/compliance/loader_test.go TestLoadPacksFromDir_RegistersByKey
      Proof: chore specs/active/401-compliance-packs/testdata/hipaa.yaml
- [x] T1.2 Add config for preset dir + enabled packs — outcome: `compliance` config section (preset path, enabled pack keys, defaults) with validation; loader is wired from config. Touches: src/internal/infra/config/config.go, src/internal/infra/config/defaults.go, src/internal/infra/config/validator.go, specs/active/401-compliance-packs/ (preset fixtures)
      Proof: code src/internal/infra/config/config.go ComplianceConfig
      Proof: code src/internal/infra/config/defaults.go defaultCompliancePresetDir
      Proof: code src/internal/infra/config/validator.go validateCompliance
      Proof: test src/internal/infra/config/compliance_test.go TestValidateCompliance_Valid

## Phase 2: MVP Slice

Goal: deliver the minimal independently demonstrable value — one-click apply and the compliance report.

- [x] T2.1 Add ApplyPackService writing tenant config in one action — outcome: app service resolves a pack key, builds the tenant's shield rules from the preset, and applies them atomically via the existing tenant repository path; unknown pack key or invalid preset yields an error and leaves the tenant unchanged. Touches: src/internal/app/compliance/ (new), src/internal/infra/config/, specs/active/401-compliance-packs/ (testdata)
      Proof: code src/internal/app/compliance/apply.go Apply
      Proof: test src/internal/app/compliance/service_test.go TestApplyPackService_AppliesPreset
- [x] T2.2 Add ComplianceReportService (active vs deviated) — outcome: service diffs a tenant's current shield config against the pack preset across detector set, per-detector reaction, and masking; returns active + deviated rules with the discrepancy; customizations are flagged deviated, not overwritten. Touches: src/internal/app/compliance/, src/internal/domain/shield/entity/tenant.go
      Proof: code src/internal/app/compliance/report.go Report
      Proof: test src/internal/app/compliance/service_test.go TestComplianceReportService_ActiveAndDeviated
- [x] T2.3 Add admin endpoint: apply-pack — outcome: POST admin endpoint behind existing adminSessionMw applies a pack to a tenant in one call and returns confirmation; wired into the router and bootstrap. Touches: src/internal/api/handler/admin/, src/internal/api/server.go, src/cmd/internal/bootstrap/
      Proof: code src/internal/api/handler/admin/compliance_handler.go HandleApplyPack
      Proof: code src/internal/api/admin.go RegisterComplianceHandler
      Proof: code src/cmd/internal/bootstrap/compliance.go LoadComplianceRegistry
      Proof: code src/cmd/all/admin.go
- [x] T2.4 Add admin endpoint: compliance-report — outcome: GET admin endpoint returns active/deviated rules per pack for a tenant; wired via server and bootstrap. Touches: src/internal/api/handler/admin/, src/internal/api/server.go, src/cmd/internal/bootstrap/
      Proof: code src/internal/api/handler/admin/compliance_handler.go HandleComplianceReport
      Proof: code src/cmd/admin/run.go
      Proof: test src/internal/api/handler/admin/compliance_handler_test.go TestComplianceHandler_Report

## Phase 3: Expansion

Goal: cover remaining behavior — extensibility and the operator-facing UI.

- [x] T3.1 Prove YAML extensibility — outcome: adding a new preset YAML file makes that pack available for apply/report without code change; loader test adds a fixture and sees the pack listed with valid rules. Touches: src/internal/domain/compliance/, src/internal/app/compliance/, specs/active/401-compliance-packs/ (testdata)
      Proof: chore specs/active/401-compliance-packs/testdata/soc2.yaml
      Proof: test src/internal/domain/compliance/loader_test.go TestLoadPacksFromDir_NewYAMLBecomesPack
- [x] T3.2 Add edge/failure coverage for apply and report — outcome: unknown pack element, unknown pack key, and YAML load errors are rejected cleanly with the tenant unchanged; re-apply is idempotent and preserves customizations. Touches: src/internal/app/compliance/, src/internal/api/handler/admin/
      Proof: test src/internal/app/compliance/service_test.go TestApplyPackService_ReapplyIsIdempotent
      Proof: test src/internal/app/compliance/service_test.go TestApplyPackService_NilRegistryRejected
      Proof: test src/internal/app/compliance/service_test.go TestComplianceReportService_UnknownPack
      Proof: test src/internal/api/handler/admin/compliance_handler_test.go TestComplianceHandler_ApplyUnknownPack
- [x] T3.3 Add UI: one-click apply + report view — outcome: admin Tenant page lets an operator apply a pack in one click and view the compliance report (active/deviated), reusing the new endpoints. Touches: ui/src/pages/Tenants/
      Proof: code ui/src/api/tenants.ts applyCompliancePack
      Proof: code ui/src/api/tenants.ts getComplianceReport
      Proof: code ui/src/pages/Tenants/TenantDetail.tsx handleApplyPack

## Phase 4: Verification

Goal: prove the feature works and leave the package reviewable.

- [x] T4.1 Add automated unit tests across loader/apply/report — outcome: repo-wide `go build/vet/test ./...` and golangci-lint clean; the domain/app/service tests pass with observable proof signals for each covered AC. Touches: src/internal/domain/compliance/, src/internal/app/compliance/, specs/active/401-compliance-packs/ (testdata)
      Proof: test src/internal/domain/compliance/loader_test.go TestLoadPacksFromDir_NewYAMLBecomesPack
      Proof: test src/internal/app/compliance/service_test.go TestApplyPackService_ReapplyIsIdempotent
      Proof: test src/internal/app/compliance/service_test.go TestComplianceReportService_UnknownPack
      Proof: test src/internal/api/handler/admin/compliance_handler_test.go TestComplianceHandler_ApplyUnknownPack
      Proof: test src/internal/infra/config/compliance_test.go
- [x] T4.2 Update deployment config examples — outcome: example config/docs mention the compliance preset dir and enabled packs (disabled/empty by default) for operators; no schema change. Touches: deployments/ (config examples), README or ROADMAP notes (docs only)
      Proof: docs deployments/docker-compose/config-runtime.yaml preset_dir
      Proof: docs deployments/helm/maskchain/values.yaml preset_dir

## Acceptance Coverage

- AC-001 -> T1.1, T2.1, T2.3, T4.1
- AC-002 -> T2.2, T2.4, T3.2, T4.1
- AC-003 -> T1.1, T1.2, T2.1, T3.2, T4.1
- AC-004 -> T1.1, T1.2, T3.1, T4.1
- AC-005 -> T2.2, T3.2, T4.1
