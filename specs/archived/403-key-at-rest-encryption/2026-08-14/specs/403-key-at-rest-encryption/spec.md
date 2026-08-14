# At-rest Key Encryption, Virtual-Key Auth Cache, and Legacy Tenant Key Removal

## Scope Snapshot

- In scope: store all LLM-provider secrets and tenant keys encrypted at rest, serve provider keys partially masked to the admin UI, cache virtual-key lookups in-process with periodic refresh, auto-create virtual keys from YAML, and remove legacy raw `tenant.api_keys` in favor of virtual keys.
- Out of scope: key rotation workflows, key management UI beyond masking, multi-tenant crypto key isolation (KYMS/KMS), TLS/mTLS changes.

## Goal

Admins and operators need provider credentials and tenant keys that are encrypted at rest (not plaintext JSON) and a gateway whose request-time key lookup does not hit PostgreSQL on every call. This feature makes `routing_providers` secrets and tenant keys impossible to read as plaintext from the database, returns provider keys to the admin UI partially masked, resolves authenticated virtual keys from an in-process periodically-refreshed cache, seeds virtual keys from YAML automatically, and deletes the legacy raw `tenants.api_keys` path so virtual keys become the single source of tenant authentication.

## Main Scenario

1. Operator sets `MASKCHAIN_KEYS_KEY` (AES-256-GCM, base64 32 bytes) and configures tenants + routing providers in YAML.
2. On startup, `BackfillVirtualKeys` auto-creates virtual keys for each configured raw `tenants.<slug>.api_keys`, and the provider registry seeds providers with secrets encrypted before they are written to PostgreSQL.
3. A gateway request authenticates through `VirtualKeyAuth`: candidate key hashes are resolved from an in-process cache refreshed periodically from the DB (with immediate invalidation on admin key mutations); the tenant is set and request proceeds.
4. Admin opens Providers page: key columns show a masked form (e.g. `sk-abv***xyz`); editing a provider without changing keys preserves the stored secret.
5. Fallback: if `MASKCHAIN_KEYS_KEY` is missing while DB-backed routing/tenancy is enabled, startup fails closed with a clear error rather than storing plaintext.

## User Stories

- P1 Story: admin can add a provider with an API key, and the DB never contains the plaintext key; the UI always displays masked keys.
- P2 Story: tenants authenticate only via virtual keys, which are auto-created from YAML and served from a periodically refreshed in-process cache, so per-request DB lookups disappear.
- P3 Story: legacy raw tenant API keys are gone — no entity field, no column, no plaintext anywhere.

## MVP Slice

Encryption at rest for provider secrets + provider-key masking + migration of existing rows, shipped first. This closes AC-001, AC-006, AC-007, AC-008. The virtual-key cache (AC-003, AC-004, AC-005) and legacy-removal (AC-002) follow as the second deployment slice.

## First Deployable Outcome

After the first implementation pass a reviewer can: read the `routing_providers` table and see ciphertext for `api_keys`/AWS secrets; call `GET /api/v1/routing/providers` and see only masked keys; edit a provider without altering keys and confirm the stored secret is unchanged; and run the migration against a scratch DB with plaintext rows to confirm they become encrypted.

## Scope

- Add `MASKCHAIN_KEYS_KEY`-backed AES-256-GCM encryptor for provider secrets (`infra/crypto` reuse) wired into the Postgres registry repository.
- Encrypt at write: `routing_providers.api_keys`, `aws_access_key_id`, `aws_secret_access_key`, `additional_headers` in `UpsertProvider`, `SeedFromYAML`, and a migration that re-encrypts existing plaintext rows.
- Decrypt at read: `ListProviders` returns plaintext internally so egress/proxy adapters work unchanged; admin DTO responses return masked keys only.
- Masking rule for provider keys exposed via admin API/UI (first 3 + `***` + last 3 for keys with len > 8, otherwise `***`); `UpsertProvider` must not overwrite a secret when the submitted value equals the masked representation or is empty.
- Remove legacy in-memory raw-key auth: `middleware.Auth`, `InMemoryRepository` key index/`FindByAPIKey`, and the gateway no-DB auth fallback; `VirtualKeyAuth` becomes the only auth path and requires a DB.
- Remove `tenant.api_keys`: entity field + `NewTenant` param, `tenants.api_keys` column (migration drop), `CreateTenantRequest`/`UpdateTenantRequest`/`TenantResponse` DTO fields, TenantForm/TenantDetail UI fields, and the `compliance/apply.go` `NewTenant` call site.
- Keep `tenants.<slug>.api_keys` in YAML config strictly as the seed source for `BackfillVirtualKeys` (auto-create virtual keys, idempotent); it is not persisted in the tenant row.
- In-process `VirtualKeyCache` (hash → tenant + virtual key) with periodic refresh from the DB (default interval 30–60s) and immediate invalidation on admin create/update/delete; `VirtualKeyAuth` reads from the cache.

## Context

- Virtual keys already store only SHA-256 hashes and return the plaintext secret exactly once at creation (`CreateVirtualKeyResponse.Key`) — this stays unchanged.
- Runtime consumers (egress, provider adapters in `src/internal/adapters/provider/`) consume `ProviderConfig.APIKeys` from `ListProviders`; they must keep receiving plaintext, so decryption belongs at the repository read boundary.
- `BackfillVirtualKeys` already creates virtual keys from `cfg.Tenants[slug].APIKeys`; it must keep working unchanged as the auto-create mechanism.
- The gateway already defaults to `VirtualKeyAuth` when a DB pool exists; removing the fallback means DB-less startup must no longer enable tenant authentication.
- `infra/crypto.Encryptor` (AES-256-GCM, fail-closed `New`) is the established at-rest encryption pattern; a second key env var (`MASKCHAIN_KEYS_KEY`) prevents key reuse across concerns.

## Dependencies

- PostgreSQL for tenants, routing registry, virtual keys (required — no in-memory auth fallback).
- Existing `infra/crypto.Encryptor` implementation (reused, not forked).
- `none` external services beyond the current stack (no KMS/Vault in scope).

## Requirements

- RQ-001 System MUST store all provider secrets (`api_keys`, AWS access/secret, `additional_headers`) encrypted at rest using `MASKCHAIN_KEYS_KEY` AES-256-GCM, decrypting only at the repository read boundary.
- RQ-002 System MUST return provider keys to the admin API/UI in masked form and MUST NOT overwrite an unchanged stored secret on provider edit.
- RQ-003 System MUST cache virtual-key lookups in-process with a periodic DB refresh and invalidate on admin key mutations, so request-time authentication does not query PostgreSQL per request.
- RQ-004 System MUST auto-create virtual keys from `tenants.<slug>.api_keys` YAML seed values on startup, idempotently.
- RQ-005 System MUST remove legacy raw-key tenant authentication and `tenants.api_keys` persistence; tenant authentication MUST resolve exclusively through virtual keys backed by a database.
- RQ-006 System MUST fail closed at startup when DB-backed routing/tenancy is enabled but `MASKCHAIN_KEYS_KEY` is absent or invalid.

## Out of Scope

- Key rotation / re-keying UI or automation (a future feature may add re-encryption tooling).
- KMS / HashiCorp Vault integration.
- Encrypting virtual-key hashes (already irreversible SHA-256) or tenant PII config.
- Envoy/data-plane changes and TLS/mTLS hardening.
- Remote key-header parity beyond what `VirtualKeyAuth` already supports.

## Acceptance Criteria

### AC-001 Provider secrets stored encrypted at rest

- Why: operators must be able to read the DB without recovering provider credentials.
- **Given** `MASKCHAIN_KEYS_KEY` set and a provider with `api_keys: ["sk-openai-abc..."]` upserted,
- **When** the row is inspected directly in `routing_providers` (raw SQL),
- **Then** `api_keys` (and any AWS secret fields) contain AES-GCM ciphertext, never the plaintext value; `ListProviders` returns the decrypted plaintext to the runtime consumer.
- Evidence: integration test asserting the DB column value ≠ plaintext and `ListProviders` round-trips plaintext.

### AC-002 Migration re-encrypts existing plaintext rows

- Why: deployments that already store plaintext must be migrated without manual DML.
- **Given** a scratch DB with `routing_providers` rows holding plaintext keys and the new migration applied,
- **When** the migration runs,
- **Then** every existing secret column is converted to ciphertext under `MASKCHAIN_KEYS_KEY` and startup read-back decrypts correctly.
- Evidence: migration test on scratch DB verifying ciphertext after `up` and plaintext after `down` (or a `down` that leaves encrypted flag consistent).

### AC-003 Provider keys masked in admin API

- Why: admin UI must show enough to identify a key without leaking it.
- **Given** a provider whose stored secret decrypts to `sk-abcd-efgh`,
- **When** the admin calls `GET /api/v1/routing/providers`,
- **Then** the response key field is `sk-a***gh` (first 3 + `***` + last 3) and no full-length secret appears.
- Evidence: handler test asserting the masked representation and absence of the full secret in the JSON body.

### AC-004 Editing a provider without key changes preserves the secret

- Why: masked keys must not be silently re-encrypted as literals on every save.
- **Given** an existing provider with stored encrypted secret and its masked representation known,
- **When** the admin upserts the provider with that masked value in the key field,
- **Then** the stored secret is unchanged (not overwritten by the literal masked string).
- Evidence: integration test upserting a masked value and asserting the decrypted plaintext after read-back equals the original.

### AC-005 Virtual-key auth served from in-process cache

- Why: request-time auth must not hit PostgreSQL per call.
- **Given** a running gateway with `VirtualKeyAuth` and a populated virtual-key cache,
- **When** a request presents a valid key,
- **Then** the tenant is resolved from the cache (repo lookup not executed per request) and the request proceeds; a cache refresh interval is configured (default 30–60s) and admin create/update/delete invalidates the affected entry.
- Evidence: unit/integration test counting repo `FindByKeyHash` calls across repeated authenticated requests (1 after warm-up) and an invalidation test after revoke.

### AC-006 Virtual keys auto-created from YAML seed

- Why: provisioning keys must not require manual UI steps.
- **Given** config `tenants.acme.api_keys: ["sk-acme-1", "sk-acme-2"]` and an empty `virtual_keys` table,
- **When** the gateway/admin binary starts with a DB pool,
- **Then** two enabled virtual keys labeled `legacy` exist for tenant `acme`, and on the second start no duplicates are created.
- Evidence: existing `BackfillVirtualKeys` test extended to assert both idempotency and labels.

### AC-007 Legacy raw-key auth and `tenants.api_keys` removed

- Why: virtual keys are the single source of tenant authentication; no second (plaintext) path may remain.
- **Given** the code after this feature,
- **When** the tenant entity, DTOs, or DB schema are inspected, and gateway startup without a DB is attempted,
- **Then** there is no `APIKeys` field/method on the tenant entity, no `tenants.api_keys` column, no `FindByAPIKey`/in-memory key index, `middleware.Auth` is removed (only `VirtualKeyAuth` exists), and DB-less startup logs that tenant auth is unavailable instead of using raw keys.
- Evidence: `go build` + grep for removed symbols, schema inspection on scratch DB, and a run without DB producing the expected log line.

### AC-008 Missing key fail-closed startup

- Why: absent or invalid `MASKCHAIN_KEYS_KEY` must not silently store plaintext.
- **Given** DB-backed tenancy/routing enabled and `MASKCHAIN_KEYS_KEY` unset or not a valid 32-byte base64 key,
- **When** the binary starts,
- **Then** startup fails (config validation or bootstrap error) before any secret write, with a clear message naming the variable.
- Evidence: config/boot test asserting a non-zero exit or returned error.

## Assumptions

- `MASKCHAIN_KEYS_KEY` is provisioned alongside existing `MASKCHAIN_CONVERSATION_KEY` in deployments; both are AES-256-GCM 32-byte base64 keys.
- Existing runtime consumers of provider secrets (egress, provider adapters) keep receiving plaintext from `ListProviders`; masking applies only at the admin response boundary.
- YAML `tenants.<slug>.api_keys` remains the seed source and is not removed; only persistence/entity/UI exposure is removed.
- `VirtualKeyAuth`'s current header semantics are preserved; the cache is an internal lookup optimization, not a contract change.

## Success Criteria

- SC-001 Authenticated request paths perform zero per-request PostgreSQL `FindByKeyHash` lookups after cache warm-up.
- SC-002 No plaintext provider/tenant secret is observable in the DB schema dump or in admin API responses after rollout.

## Edge Cases

- Key of length ≤ 8: masked to `***` in admin responses.
- `additional_headers` containing secret header values: encrypted with the same fields, never echoed in list responses.
- Provider edit where the admin leaves keys blank (empty): stored secret preserved (same rule as masked value).
- DB-less startup: no tenant auth, explicit log line, providers/routes may still load from YAML in memory.
- Missing key at startup with no DB: no fail (nothing at rest); with DB: fail closed.

## Open Questions

- none