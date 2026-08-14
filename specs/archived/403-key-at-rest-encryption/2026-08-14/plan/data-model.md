# At-rest Key Encryption, Virtual-Key Auth Cache, and Legacy Tenant Key Removal — Data Model

## Scope

- Related `AC-*`: `AC-001`, `AC-002`, `AC-003`, `AC-007`
- Related `DEC-*`: `DEC-001`, `DEC-003`, `DEC-006`
- Status: `changed`

## Entities

### DM-001 routing_providers (secret columns carry ciphertext)

- Purpose: provider registry rows; secret-bearing columns store AES-256-GCM ciphertext under `MASKCHAIN_KEYS_KEY` instead of plaintext.
- Source of truth: PostgreSQL; YAML only as initial seed (source='yaml').
- Invariants: a secret column value is either (a) legacy plaintext before migration, or (b) base64(AES-GCM(nonce||ct)) after; never re-encrypted plaintext of a masked literal (DEC-003).
- Related `AC-*`: `AC-001`, `AC-002`, `AC-003`, `AC-004`
- Related `DEC-*`: `DEC-001`, `DEC-003`, `DEC-006`
- Fields:
  - `name` - TEXT, PK, unchanged
  - `api_keys` - JSONB, encrypted at rest (array of secrets serialized then sealed)
  - `additional_headers` - JSONB, encrypted at rest (may carry secret header values)
  - `aws_access_key_id` - TEXT, encrypted at rest
  - `aws_secret_access_key` - TEXT, encrypted at rest
  - other columns (`base_url`, `auth_scheme`, `proxy_url`, `source`, ...) - unchanged, not secrets
- Lifecycle:
  - created: `UpsertProvider` (source 'ui') or `SeedFromYAML` (source 'yaml') — secrets sealed before INSERT/UPDATE
  - updated: `UpsertProvider` (ON CONFLICT) — sealed on write; masked/empty submitted values preserve stored ciphertext
  - deleted: `DeleteProvider` (also strips references from `routing_model_routes`)
  - migrated: 019 bootstrap re-encrypts legacy plaintext rows
- Consistency notes: no plaintext write path after this feature; wrong/missing key → read fails (auth error) or startup fails closed; `down` migration returns columns to original shape (values remain ciphertext — documented limitation).

### DM-002 tenants (api_keys column removed)

- Purpose: tenant identity/configuration; raw API keys are no longer a tenant attribute.
- Source of truth: PostgreSQL for tenant attributes; virtual_keys for authentication secrets.
- Invariants: tenant row has no API-key-bearing field; auth resolution exclusively via virtual_keys.
- Related `AC-*`: `AC-007`
- Fields:
  - `api_keys` - JSONB, REMOVED via migration 020
  - remaining columns (`slug`, `name`, `auth_header`, `dictionaries`, `pii_config`, `retention_mode`, `created_at`, `updated_at`) unchanged
- Lifecycle:
  - created/updated via admin handler / YAML seed — no api_keys input anymore
  - migrated: 020 drops the column after all code paths stop reading it
- Consistency notes: `compliance/apply.go` `NewTenant` call drops the apiKeys argument (no other writer).

### DM-003 virtual_keys (unchanged)

- Purpose: tenant authentication secrets (SHA-256 hashes only).
- Source of truth: PostgreSQL.
- Invariants: stores `key_hash` (SHA-256), never plaintext; plaintext returned once at creation.
- Related `AC-*`: `AC-005`, `AC-006`
- Fields: unchanged (`id`, `tenant_id`, `key_hash`, `label`, `allowed_models`, `blocked_models`, `budget_cap`, `spent`, `expires_at`, `metadata`, `enabled`, `created_at`, `updated_at`).
- Lifecycle: unchanged; `BackfillVirtualKeys` seeds from YAML (label 'legacy', idempotent); admin create/update/delete additionally invalidate the in-process cache.
- Consistency notes: cache is a read-through snapshot; authoritative state remains the DB.

## Relationships

- `DM-003.virtual_keys.tenant_id -> DM-002.tenants.slug`: one-to-many; the only tenant authentication channel after legacy removal.
- `DM-001.routing_providers`: no FK to tenants; providers are shared infra.

## Derived Rules

- Masked rendering (API boundary only): `sk-a***gh` = first 3 + `***` + last 3 for len > 8, else `***`; never stored, never sent to runtime consumers.
- Upsert guard: submitted secret equal to the masked form (or empty) → keep stored ciphertext.

## State Transitions

- Provider secret value: `plaintext (legacy) --019 bootstrap--> ciphertext` (idempotent; ciphertext never reverts to plaintext except full `down`+`up`).
- Tenant auth channel: `tenants.api_keys` -> (removed) -> virtual_keys only.

## Out of Scope

- KMS/Vault-backed keys, per-tenant encryption keys.
- Encrypting `virtual_keys.key_hash` (already irreversible) or tenant `dictionaries`/`pii_config`.

## No-Change Stub

Not applicable — this feature changes the persisted model (see above).