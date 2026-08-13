# Compliance Packs (401) Plan

## Phase Contract

Inputs: spec, inspect (pass), minimal shield-domain repo context.
Outputs: plan, data-model (status: no-change).
Stop if: spec too vague to plan safely — it is not; clear scope and 5 ACs.

## Objective

Group existing shield rule elements into declarative compliance presets. A `CompliancePack` defines, per pack key ("HIPAA", "PCI DSS", "GDPR", "Legal"), which detectors are enabled and which reaction/resolution/masking apply by default. Applying a pack to a tenant writes that tenant's shield config in one action; the compliance report compares the tenant's current rules to the preset, marking active vs. deviated. No new detectors/reactions — the pack references existing `DetectorType`s and `ReactionExecutor`s by identifier, so it is a config-coordination layer over the current shield domain.

## MVP Slice

- Minimal independently demonstrable increment: static `CompliancePack` YAML loading + one-action apply to a tenant + compliance report (active/deviated).
- ACs that must close before scope expands: AC-001 (one-click apply), AC-002 (report active/deviated), AC-004 (extensibility via YAML).

## First Validation Path

- Short scripted/manual path: start the service with preset YAMLs, apply `HIPAA` to a test tenant via the admin API, then call the report endpoint and confirm the tenant's rules equal the preset and report shows them active; mutate one rule and confirm it shows `deviated`. Unit tests cover the same assertions without a live DB.

## Scope

- Implementation zone 1: `CompliancePack` domain — entity, YAML preset loading/validation, store of packs by key.
- Implementation zone 2: apply-pack application service (writes tenant shield config from preset in one call) + compliance-report service (active vs. deviated).
- Implementation zone 3: admin API + UI one-click apply and report surfaces.
- Explicit boundary left untouched: detectors, reactions, masking, resolutions themselves are NOT modified — the pack only references existing elements by ID; tenant-policy storage mechanics (PostgreSQL) are reused, not replaced.

## Performance Budget

- none — pack apply/report are operator-time operations (tenants-scale, not request-path hot path); no meaningful latency/memory constraints.

## Implementation Surfaces

- `src/internal/domain/compliance/` (new) — `Pack` entity, `PackLoader` (YAML parse/validate), pack registry; why new: no existing home for declarative presets; shield domain owns singular rules, not bundles.
- `src/internal/app/compliance/` (new) — `ApplyPackService` (one-action apply to tenant), `ComplianceReportService` (active/deviated); why new: app-layer orchestration over existing tenant-policy write path.
- `src/internal/domain/shield/` (existing) — referenced surfaces: `detector/registry.go` (DetectorType set), `reaction/` (ReactionExecutor, mask/block/redact/alert), `resolver/tenant_resolver.go` (DBFirstTenantResolver tenant policy source), `entity/tenant.go` (per-tenant PII/detector config). Modified only to read/compose existing config, not to alter rule semantics.
- `src/internal/api/handler/admin/` (existing) — new admin endpoints: apply pack, compliance report; accepts existing adminSessionMw.
- `src/internal/infra/config/` (existing) — configuration for the preset directory path + enabled packs.
- `ui/src/pages/Tenants/` or `ui/src/pages/Budgets`-style admin page (existing pattern) — one-click apply + report view.

## Bootstrapping Surfaces

- `src/internal/domain/compliance/` and `src/internal/app/compliance/` must exist before behavior; preset YAML fixture set (testdata) for validation.

## Architecture Impact

- Local: new compliance packages sit alongside existing shield domain/app packages; no static interface churn to detectors/reactions.
- Integration: apply/report reuse the existing tenant repository write/read path (DBFirstTenantResolver); no new storage.
- Migration/compatibility: additive and config-driven; existing tenants unaffected until a pack is applied; no schema migration required.

## Acceptance Approach

- AC-001 one-click apply -> approach: apply-pack endpoint resolves pack key -> set of rules written to tenant config in one transaction; surfaces: `domain/compliance`, `app/compliance`, `handler/admin`, tenant repo; proof: unit test asserts tenant rules == preset + API returns confirmation.
- AC-002 report active/deviated -> approach: report service diffs tenant config vs preset across listed rule dimensions; surfaces: `app/compliance`, `handler/admin`, UI; proof: test mutates a rule and sees `deviated`; manual report read.
- AC-003 unknown element rejection -> approach: pack validation fails on unknown detector/reaction ID before any write; surfaces: `domain/compliance` loader, apply service; proof: test applies broken pack, asserts error + tenant unchanged.
- AC-004 extensibility via YAML -> approach: loader scans preset dir, registers each pack by key without code change; surfaces: `domain/compliance`; proof: test adds fixture YAML, sees pack listed and valid.
- AC-005 customization preserved -> approach: apply is idempotent and does not overwrite existing non-preset rule customizations; report flags them deviated; surfaces: `app/compliance`, report; proof: test customizes, re-applies, asserts preservation.

## Data and Contracts

- Referenced AC: all (AC-001..AC-005). `CompliancePack` YAML is the primary contract for preset shape (detector type -> reaction/resolution/masking + enabled flags).
- Data model: no change (pack is config, not persisted entity; tenant shield config already stored in PostgreSQL). See `data-model.md` (status: no-change).
- API/event contracts: new admin endpoints are additive; no existing contract changes. Explicit: no schema migration, no rollout/backfill needed.

## Implementation Strategy

- DEC-001 Presets as static YAML scan-loaded at startup, versioned in repo
  Why: matches spec RQ-006 (extensibility = add YAML file); simplest contract, no runtime management; Not: pack DB/table — config-not-data, avoids a migration and a new subsystem. Tradeoff: adding/changing a pack requires a redeploy, acceptable for compliance presets. Affects: `domain/compliance` loader, `infra/config` security. Validation: test that a fixture YAML becomes a listed pack (AC-004).
- DEC-002 Apply references existing detector/reaction IDs, never redefines them
  Why: keeps pack a pure coordination layer — zero risk to established shield semantics; Not: pack-embedded rule logic which would fork behavior. Tradeoff: pack cannot introduce capabilities that don't exist yet. Affects: `domain/compliance`, references `detector/registry`, `reaction/`. Validation: validation rejects unknown IDs (AC-003).
- DEC-003 Apply-then-report diffed against preset, customizations flagged deviated
  Why: "standard plus local amendments" is the real adoption path; report must distinguish active from deviated without destroying operator overrides; Not: force-resync overwriting customizations. Tradeoff: report needs a stable diff definition per rule dimension. Affects: `app/compliance` apply + report. Validation: customization persists and surfaces as deviated (AC-002, AC-005).
- DEC-004 New admin endpoints behind existing adminSessionMw, UI one-click
  Why: reuses established admin auth/audit surface rather than a bespoke route; Not: new auth path. Tradeoff: endpoints inherit admin API versioning/audit. Affects: `handler/admin`, UI page. Validation: API/UI apply + report against a test tenant (AC-001, AC-002).

## Incremental Delivery

### MVP (First value)

- Bootstrapping: `domain/compliance` (entity + YAML loader + registry) with fixture presets; validation tests (AC-004).
- Apply service + admin endpoint writing tenant config in one call (AC-001, AC-003).
- Compliance report endpoint (AC-002, AC-005).
- MVP readiness: AC-001..AC-005 covered and verifiable via unit tests + one manual admin call.

### Iterative Expansion

- After MVP: UI one-click apply + report view reusing the endpoints (AC-001/AC-002 UX); richer report dimensions (per-rule severity/masking detail). Not in this plan: reverse-generation from user rules, multi-pack per tenant (both out of scope in spec).

## Implementation Order

- First: `domain/compliance` loader + validation (everything depends on a valid pack).
- Second: apply service + endpoint, then report service + endpoint (apply before differ).
- Safe to parallelize: UI scaffolding with endpoint mock; admin endpoint test fixtures.
- Behind flag: none required this slice — feature is additive and opt-in; packs are static config.

## Risks

- Pack diff/report semantics ambiguity (what counts as "deviated")
  Mitigation: define diff per rule dimension (enabled detector set, per-detector reaction, masking) in DEC-003; tests pin each.
- Unknown pack element silently degrading apply
  Mitigation: strict YAML validation before any write (DEC-002, AC-003); atomic apply (one transaction) so failure leaves tenant unchanged.
- Overwriting operator customizations on re-apply
  Mitigation: idempotent apply that preserves non-preset rules and flags them deviated (DEC-003, AC-005).
- Preset directory drift / load errors breaking startup
  Mitigation: on YAML load error, skip that pack with recorded errors and keep existing packs working (spec Edge Cases); loader returns per-file errors.

## Rollout and Compatibility

- Additive and config-driven: no backfill, no schema migration, no feature flag required this slice; existing tenants unchanged until a pack is applied.
- Operational: pack apply is an audited admin action (inherits admin audit logging); report endpoint readable by operators. Explicit: no special rollout steps beyond redeploy with preset YAML files.

## Verification

- Automated: loader/validator tests (AC-004, AC-003), apply service tests (AC-001, AC-005), report diff tests (AC-002); repo-wide `go build/vet/test ./...` + golangci-lint.
- Targeted manual: apply `HIPAA` to a test tenant via admin API, mutate one rule, confirm report shows it deviated; confirm unknown pack key errors cleanly.
- Each step confirms AC-001..AC-005 and DEC-001..DEC-004 via the Evidence lines in spec + tests above.

## Constitution Alignment

- No conflicts: groups existing shield rules, keeping Content Shield a core domain; tenants/DB-first policy storage reused rather than replaced; additive admin API + UI within existing React tenant-management scope; languages en.
