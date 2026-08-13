# Compliance Packs (401)

## Scope Snapshot

- In scope: ready-made Content Shield rule sets («HIPAA», «PCI DSS», «GDPR», «Legal») as YAML presets, applied to a tenant in one action with the ability to customize on top.
- Out of scope: changes to the detectors/reactions/resolutions/masking themselves (this feature only groups them into presets), and tenant management itself.

## Goal

Regulated customers (HIPAA/PCI/GDPR) expect ready-made rule sets «out of the box» rather than manually configuring each detector/reaction/masking one by one. This feature lets an operator apply a ready compliance pack to a tenant in one click, customize on top of it, and see which pack rules are active or deviated from the preset. Success is visible when an operator can apply a pack to a tenant without manually listing rules and can reconcile the current tenant state against the pack baseline.

## Main Scenario

1. Starting point: operator selects a tenant to which regulatory requirements must be applied.
2. Main action: operator selects a pack («HIPAA», «PCI DSS», «GDPR», «Legal»); the system applies the preset-defined rule set (detectors + reactions + resolutions + masking) to the tenant in one action; individual rules can be added/changed on top.
3. Result: tenant rules match the pack preset (accounting for customizations), and this is visible in the compliance report — which pack rules are active and which are deviated.
4. Error/fallback path: if a pack references an unknown detector/reaction, application is rejected with a clear error; overrides are preserved and surfaced as «deviations» in the report.

## User Stories

- P1 Story (HIPAA-compliant tenant in one click): operator applies the «HIPAA» pack to a tenant and immediately gets an active protective rule set, without manual assembly.
- P2 Story (compliance report): operator sees which pack rules are active and which are deviated by customization on a given tenant.

## MVP Slice

The smallest slice — statically defined compliance packs (YAML, loaded at startup), applied to a tenant in one step; a compliance report (active/deviated rules). This slice must close AC-001, AC-002, AC-004 first.

## First Deployable Outcome

Manually verifiable result: operator applies the «HIPAA» pack to a tenant and sees the pack's active rules in the compliance report; customization shows as a deviation. The feature ships independently: the pack is a config artifact applied through existing tenant-policy mechanisms.

## Scope

- In: a `CompliancePack` layer (entity + YAML preset loading), applying a pack to a tenant in one action, compliance report (active/deviated rules), customization on top of the preset.
- In: the `src/internal/domain/shield/` surface (detectors/reactions/resolutions/masking already exist — the pack references them), config, API handlers, and the one-click UI application.
- Deliberately included: reading/validating YAML presets; conformity expressed as «which pack rule is active on the tenant».

## Context

- Existing flow: Content Shield already provides detectors (PHI, PII, financial, secrets, dictionary, prompt-injection), reactions (mask/block/redact/alert), resolutions, and masking at the tenant level — the pack groups these ready elements without introducing new ones.
- Tenant policy is stored in PostgreSQL (dictionaries, PII rules, preprocessors); shield rules have scope tenant|key|model.
- Assumption: applying a pack must preserve existing policy mechanisms and not duplicate them; the pack is a declarative «how by default» source, over which a tenant may deviate.

## Dependencies

- Cross-spec: uses existing domains for reactions (`src/internal/domain/shield/reaction/`), resolutions (`src/internal/domain/shield/resolver/`), masking, and detectors — the pack references them by identifier.
- External: none — packs are static in the repository, no external service required.
- Constraint: the UI applies a pack through the existing admin/tenant API; requires that API to be present in a given environment.

## Requirements

- RQ-001 The system MUST provide statically defined compliance packs («HIPAA», «PCI DSS», «GDPR», «Legal»), each as YAML describing a preset: which detectors are enabled and which reactions/resolutions/masking apply by default.
- RQ-002 The system MUST support applying a compliance pack to a tenant in one action, setting the tenant's rules from the preset without manual enumeration.
- RQ-003 The system MUST allow customization on top of the preset: the operator can modify/add individual tenant rules after application.
- RQ-004 The system MUST provide a compliance report per pack: which pack rules are active on the tenant and which are deviated by customization.
- RQ-005 The system MUST reject pack application with a clear error if the preset references an unknown/nonexistent element (detector/reaction/resolution).
- RQ-006 The system MUST be extensible: adding a new pack is adding a YAML file, without changing execution code.

## Out of Scope

- Changing the detectors/reactions/resolutions/masking themselves — packs only reference existing ones.
- Generating packs from user rules (reverse) — out of the slice.
- Auto-applying packs on schedule/data discovery — out of scope.
- Import/export of packs between instances — out of scope.
- Deferred: multiple packs on one tenant (simultaneously applying more than one pack) — not silently carried into the implementation.

## Acceptance Criteria

### AC-001 One-click pack application

- Why it matters: operator satisfies regulatory requirements without manual rule assembly.
- **Given** an existing tenant without an applied pack, and a ready «HIPAA» preset
- **When** the operator applies the «HIPAA» pack to the tenant
- **Then** the tenant's rules match the pack preset (enabled detectors, reactions, resolutions, masking), and the API/UI returns confirmation of application
- Evidence: a test applies a pack to a tenant and verifies the recorded rules are identical to the preset; manual scenario via API/UI.

### AC-002 Compliance report: active and deviated

- Why it matters: the operator needs to see deviation from the standard, not just application.
- **Given** a tenant with an applied pack and a customization of one rule (modified or disabled)
- **When** the operator requests the compliance report for the pack
- **Then** the report lists pack rules as active and shows modified/disabled ones as deviated with the discrepancy indicated
- Evidence: a test customizes a rule and verifies deviated status in the report; manual report inspection.

### AC-003 Validation of unknown pack elements

- Why it matters: a broken preset must not silently apply partially.
- **Given** a pack referencing a nonexistent detector or reaction
- **When** the operator attempts to apply such a pack
- **Then** the application is rejected with a clear error, and the tenant's rules remain unchanged
- Evidence: a test applies a broken pack and verifies the error plus unchanged tenant rules.

### AC-004 Extensibility through YAML

- Why it matters: adding a framework must not require rewriting code.
- **Given** a new pack YAML file in the presets directory
- **When** the system starts/reloads presets
- **Then** the new pack is available for application without changing execution code, and its declared rules are valid
- Evidence: a test adds a test YAML, loads presets, and sees the pack in the list.

### AC-005 Customization on top of the preset is preserved

- Why it matters: «standard plus local amendments» is the real adoption path.
- **Given** a tenant with an applied pack and a rule added/modified by the operator
- **When** the operator re-fetches the tenant's rules
- **Then** the customization is preserved and shown in the report as a deviation, not overwritten
- Evidence: a test customizes, re-applies/re-reads, and verifies the change persists.

## Assumptions

- Packs are static and versioned in the repository (not managed at runtime).
- Pack application relies on existing tenant-policy write mechanisms and does not introduce a new storage for rules.
- Within this slice a tenant can have one pack applied at a time (multi-pack is out of scope).

## Success Criteria

- SC-001 Applying a pack to a tenant is a single step/call and does not require enumerating individual rules.
- SC-002 The compliance report is available without manually opening the source YAML.

## Edge Cases

- Tenant without an applied pack: the compliance report shows an empty set of active pack rules (nothing applied).
- A pack referencing an element globally disabled by config: behavior is fixed as valid application with a note in the report.
- Re-applying the same pack: idempotent — must not duplicate rules or overwrite customizations.
- YAML loading error (syntax/schema): the pack is not loaded with a list of errors; existing packs keep working.

## Open Questions

- none
