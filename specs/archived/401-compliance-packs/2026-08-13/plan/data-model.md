# Compliance Packs (401) Data Model

## Scope

- Related `AC-*`: none (pack is config, not persisted data)
- Related `DEC-*`: DEC-001, DEC-002
- Status: `no-change`
- Explicit: no significant data-model change required for this feature.

## No-Change Stub

- Status: `no-change`
- Reason: this feature adds no persisted entities, value objects, state transitions, or contract-relevant payload shapes. `CompliancePack` is a static, versioned-in-repo YAML config artifact loaded at startup; tenant shield policy is already stored in PostgreSQL via the existing repository path and is only read/composed by the apply and report services. No schema migration is required.
- Revisit triggers:
  - a new persistable state appears (e.g., storing per-tenant applied pack version or audit of apply decisions),
  - new invariants or lifecycle states for packs are introduced,
  - an API/event payload shape needs to be tracked here (if apply/report responses grow into a stable contract).
