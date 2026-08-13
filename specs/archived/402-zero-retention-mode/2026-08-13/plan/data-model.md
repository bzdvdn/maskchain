# Zero-Retention Mode Data Model

## Scope

- Related `AC-*`: `AC-001`, `AC-002`, `AC-003`, `AC-004`, `AC-006`, `AC-007`, `AC-008`
- Related `DEC-*`: `DEC-001`, `DEC-002`, `DEC-004`, `DEC-006`
- Status: `changed`

## Entities

### DM-001 Tenant

- Purpose: carries the per-tenant retention mode consumed by the capture path and admin API.
- Source of truth: `shield/entity.Tenant` populated by `PostgresTenantRepo`; config default applied in the tenant resolver.
- Invariants: `retention_mode` ∈ {`full`, `meta`, `none`}; `full` when unset (via resolver default).
- Related `AC-*`: `AC-001`, `AC-003`, `AC-005`, `AC-006`, `AC-007`
- Related `DEC-*`: `DEC-001`, `DEC-002`
- Fields:
  - `retention_mode` - VARCHAR(16), NOT NULL DEFAULT `full`, meaning mode; validated enum.
- Lifecycle:
  - created with `full` or explicit mode at tenant create; updated on tenant update; not deleted independent of tenant.
- Consistency notes: resolver must not leave an empty mode; validation rejects non-enum values before persist (AC-007).

### DM-002 ConversationLog

- Purpose: the persisted conversation record; in `meta` holds metadata + block facts without content; in `none` no row exists.
- Source of truth: conversation capture path (middleware → worker) → `PgConversationStore`.
- Invariants: in `full` request content is non-empty and encrypted (existing); in `meta` request/response/masking are empty and `detector`/`category` carry block facts (or null); never a row in `none`.
- Related `AC-*`: `AC-002`, `AC-003`, `AC-004`, `AC-008`
- Related `DEC-*`: `DEC-003`, `DEC-004`
- Fields:
  - `detector` - VARCHAR(64), nullable, block anomaly detector type (anonymized fact).
  - `category` - VARCHAR(64), nullable, block anomaly category (e.g. label); no fragment/text.
  - existing fields (id, tenant_id, model, status, masked, streamed, mask_id, request, response, masking, len, created_at) unchanged.
- Lifecycle: created by worker batch insert; deleted by retention cleanup (`DeleteOlderThan`); row only exists for `full`/`meta`.
- Consistency notes: `meta` rows must never carry content bytes; `masked` semantics unchanged so the masked filter stays correct.

## Relationships

- `DM-001 Tenant -> DM-002 ConversationLog`: 1:N by `tenant_id`; each log inherits the tenant's retention mode at capture time.

## Derived Rules

- Effective tenant mode = explicit `tenants.retention_mode` if non-empty, else global `data.retention.mode` default (DEC-002). Row existence/content follows the effective mode.

## State Transitions

- Tenant: `full` -> `meta` | `none` (admin update); validation guard rejects non-enum values (AC-007). Rows follow the mode in effect at capture; no retroactive purge.
- ConversationLog: none-existing -> `full` content row | `meta` metadata row; mode changes affect only new captures.

## Out of Scope

- New anomaly/incident table (meta facts normalization deferred) — keep two nullable columns.
- Removal/backfill of already-stored rows on downgrade.

## No-Change Stub

- Not used: this feature changes the schema (two new columns + default column).

### Migration 018_retention_mode

- `ALTER TABLE tenants ADD COLUMN IF NOT EXISTS retention_mode VARCHAR(16) NOT NULL DEFAULT 'full';`
- `ALTER TABLE conversation_logs ADD COLUMN IF NOT EXISTS detector VARCHAR(64);`
- `ALTER TABLE conversation_logs ADD COLUMN IF NOT EXISTS category VARCHAR(64);`
- Reversible via `.down.sql` (DROP COLUMN). Additive; no backfill; `full` default keeps existing behavior.