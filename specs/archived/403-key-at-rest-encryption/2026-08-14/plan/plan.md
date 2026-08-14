# At-rest Key Encryption, Virtual-Key Auth Cache, and Legacy Tenant Key Removal — Implementation Plan

## Phase Contract

Inputs: `spec.md`, minimal repo context (routing registry, tenant entity, virtual key auth, migrations).
Outputs: `plan.md`, `data-model.md`.
Stop if: AC ambiguous — none found; spec passed inspect.

## Goal

Implement the spec: encrypt provider secrets at rest under `MASKCHAIN_KEYS_KEY`, mask provider keys in admin API/UI, add an in-process periodically-refreshed virtual-key auth cache, keep YAML→virtual-key auto-seed, and remove legacy raw tenant keys (entity, column, DTO, UI, in-memory auth) so virtual keys are the only tenant auth path (DB required).

## MVP Slice

Provider secret encryption + masking + re-encryption migration. Covers AC-001, AC-002, AC-003, AC-004.
Second slice: virtual-key cache + YAML auto-seed already-exists coverage (AC-005, AC-006) and legacy removal (AC-007, AC-008).

## First Validation Path

1. Run repo integration tests against a scratch DB (`TEST_DATABASE_URL`): `TestPgRegistryProviderSecretEncryption`, `TestPgRegistryMigrateLegacySecrets`, `TestUpsertProviderPreservesMaskedSecret`.
2. `GET /api/v1/routing/providers` returns masked keys (`sk-a***gh`), DB row shows ciphertext.
3. `go build ./... && go vet ./... && golangci-lint run ./...` clean; migration runner applies 019/020 cleanly.

## Scope

- `routing_registry.go`: encrypt/decrypt provider secrets via `crypto.Encryptor` (encrypt in `UpsertProvider`/`SeedFromYAML`, decrypt in `ListProviders`); legacy-secret re-encryption bootstrap.
- `dto/routing.go` + `handler/admin/routing_handler.go`: masked key rendering in `ProviderToResponse`; `UpsertProvider` skips overwriting unchanged/empty secrets.
- `infra/config`: add `MASKCHAIN_KEYS_KEY` wiring + fail-closed validation.
- `middleware/virtualkey_auth.go` + new in-process `VirtualKeyCache` (hash→tenant+vk), periodic refresh + invalidation on admin key mutations.
- Remove: `middleware.Auth`, `in_memory.go` (`InMemoryRepository`, `FindByAPIKey`), `Tenant.apiKeys`/`APIKeys()`/`NewTenant` param, `tenants.api_keys` column, DTO/UI api_keys fields, `compliance/apply.go` call site, no-DB auth fallback.
- Keep: `BackfillVirtualKeys` (YAML→virtual keys), `tenants.<slug>.api_keys` in YAML strictly as seed.
- Untouched: egress/provider adapters (they consume plaintext from `ListProviders`), virtual-key hashing/one-time-return contract, tenant PII/dictionaries columns.

## Performance Budget

- Auth path: `FindByKeyHash` per authenticated request → 0 DB calls after cache warm-up (SC-001).
- Cache refresh interval default 30s, refresh query is a single `SELECT` over `virtual_keys`.
- `none` for other paths (admin CRUD, seeding — not hot).

## Implementation Surfaces

- `src/internal/adapters/repository/postgres/routing_registry.go` — existing; add encryption boundary.
- `src/internal/domain/routing/config.go` — existing; no shape change (secrets stay in `ProviderConfig`).
- `src/internal/api/dto/routing.go` — existing; add masked rendering in `ProviderToResponse`.
- `src/internal/api/handler/admin/routing_handler.go` — existing; masked upsert guard.
- `src/internal/infra/config/{config,validator,defaults}.go` — existing; `MASKCHAIN_KEYS_KEY` + validation.
- `src/internal/api/middleware/virtualkey_auth.go` — existing; switch to cache.
- `src/internal/api/middleware/virtualkey_cache.go` — **new**; in-process cache (mirrors `TenantProvider` pattern).
- `src/internal/api/handler/admin/virtual_key_handler.go` — existing; invalidate cache on create/update/delete.
- `src/internal/domain/shield/entity/tenant.go` — existing; remove `apiKeys`.
- `src/internal/api/middleware/auth.go` — existing; delete file.
- `src/internal/adapters/repository/tenant/in_memory.go` — existing; delete file.
- `src/internal/adapters/repository/postgres/tenant.go`, `src/internal/api/dto/tenant.go`, `src/internal/api/handler/admin/tenant_handler.go` — existing; drop api_keys.
- `src/internal/app/compliance/apply.go` — existing; fix `NewTenant` call.
- `src/cmd/{gateway,admin,all}/run.go|*.go` — existing; key env wiring, cache bootstrap, no-DB fallback removal.
- `src/internal/adapters/repository/postgres/migrations/019_*.sql`, `020_*.sql` — **new**.
- `ui/src/pages/Tenants/TenantForm.tsx`, `TenantDetail.tsx`, `ui/src/api/tenants.ts` — existing; remove api_keys fields.
- `ui/src/pages/Routing.tsx`, `ui/src/api/routing.ts` — existing; masked display.
- `src/internal/adapters/repository/postgres/virtual_key_repo.go` — existing; add cache-refresh query reuse.
- `src/internal/api/server.go` / `src/internal/api/admin.go` — existing; route/auth wiring.

## Bootstrapping Surfaces

- `infra/crypto.Encryptor` — already exists, reused as-is.
- `VirtualKeyCache` — new type; small, no external deps.
- `none` for remaining structure (surfaces already exist).

## Architecture Impact

- Auth is single-path now: `VirtualKeyAuth` only, DB required. Removes the in-memory plaintext key index — a security-relevant simplification.
- Secret lifecycle moves to repository boundary: DB holds ciphertext, `ListProviders` is the decrypt seam, DTO masks for admin, runtime consumers get plaintext.
- Migration/rollout: 019 re-encrypts existing plaintext secrets in-place (Go bootstrap after `RunMigrations`); 020 drops `tenants.api_keys`. Down migrations documented.
- Compatibility: runtime provider clients and header semantics unchanged; YAML seed path unchanged; admin API drops `api_keys` from tenant DTO (breaking only for old tenants API clients).

## Acceptance Approach

- AC-001 Provider secrets stored encrypted → encrypt in `UpsertProvider`/`SeedFromYAML`, decrypt in `ListProviders`. Surfaces: routing_registry.go, integration test `TestPgRegistryProviderSecretEncryption` (raw SQL shows ciphertext ≠ plaintext; `ListProviders` round-trips).
- AC-002 Migration re-encrypts existing rows → Go re-encryption bootstrap invoked post-`RunMigrations` when rows fail decrypt; integration test seeds plaintext, runs bootstrap, asserts ciphertext + correct read-back. Migration 019 + `TestPgRegistryMigrateLegacySecrets`.
- AC-003 Provider keys masked in admin API → `dto.ProviderToResponse` masks `sk-a***gh`; handler test `TestProviderMaskedResponse` asserts masked form + no full secret in JSON.
- AC-004 Editing without key change preserves secret → `UpsertProvider` guard: if submitted key equals masked form or empty, keep stored ciphertext; integration test `TestUpsertProviderPreservesMaskedSecret`.
- AC-005 Virtual-key auth from cache → `VirtualKeyAuth` uses `VirtualKeyCache`; unit test counts `FindByKeyHash` calls (0 after warm-up), invalidation test after revoke, periodic refresh test with injected clock. `TestVirtualKeyCacheAuth`, `TestVirtualKeyCacheInvalidation`, `TestVirtualKeyCacheRefresh`.
- AC-006 Virtual keys auto-created from YAML seed → existing `BackfillVirtualKeys` coverage extended (idempotency + `legacy` label); `TestBackfillVirtualKeysIdempotent`.
- AC-007 Legacy raw-key auth and `tenants.api_keys` removed → entity/DTO/UI/column/middleware/in_memory deletions; `go build` + grep for removed symbols; schema inspection; no-DB run logs "tenant auth unavailable". `TestTenantWithoutAPIKeys`.
- AC-008 Missing key fail-closed startup → config validation + bootstrap fail when DB-backed and key absent/invalid; config/boot test `TestMissingKeysKeyFailsClosed`.

## Data & Contracts

- See `data-model.md` (status: changed — routing_providers secret columns carry ciphertext; tenants.api_keys dropped; virtual_keys unchanged).
- Contracts: tenant admin API removes `api_keys` field (breaking); routing provider API now returns masked keys (backward-compatible shape, masked values); runtime provider clients unchanged; virtual key one-time-return contract unchanged.
- No new event contracts.

## Implementation Strategy

- DEC-001 Encrypt at repository boundary with existing `crypto.Encryptor`
  Why: reuses the established fail-closed AES-GCM pattern (no new crypto code); repository is the single write/read seam so runtime consumers stay plaintext.
  Tradeoff: re-encryption bootstrap must distinguish ciphertext from plaintext; a wrong `MASKCHAIN_KEYS_KEY` fails auth on read (fail-closed by design).
  Affects: routing_registry.go, config wiring, migrations 019.
  Validation: AC-001/AC-002 integration tests + startup fail-closed test.
- DEC-002 Mask at DTO boundary, not repository
  Why: `ListProviders` feeds runtime consumers that need plaintext; masking belongs to the admin response contract only.
  Tradeoff: an extra render step in `ProviderToResponse`; masking logic must be centralized to avoid drift.
  Affects: dto/routing.go, handler/admin/routing_handler.go.
  Validation: AC-003 handler test.
- DEC-003 Upsert guard: never store a masked/empty literal
  Why: without it, editing a provider would re-encrypt `sk-a***gh` as a literal and break the stored secret.
  Tradeoff: a provider genuinely wanting the masked string as its key is unsupported — acceptable, keys are secrets.
  Affects: routing_registry.go, handler/admin/routing_handler.go.
  Validation: AC-004 integration test.
- DEC-004 In-process VirtualKeyCache with periodic refresh + invalidation
  Why: request-time auth becomes 0-DB; mirrors existing `TenantProvider` hot-reload pattern; no Valkey dependency for auth.
  Tradeoff: cache staleness up to refresh interval (30s) unless explicitly invalidated on admin mutations (create/update/delete hook).
  Affects: virtualkey_auth.go, new virtualkey_cache.go, virtual_key_handler.go, gateway bootstrap.
  Validation: AC-005 tests (call-count, invalidation, refresh).
- DEC-005 Remove legacy in-memory raw-key auth entirely (DB required)
  Why: single source of truth (virtual keys), deletes the plaintext key index in memory; spec decision confirmed.
  Tradeoff: gateway without DB cannot authenticate tenants (explicit log line); an operator must run Postgres.
  Affects: auth.go (delete), in_memory.go (delete), server.go wiring, gateway run.go.
  Validation: AC-007 grep/build + no-DB run log.
- DEC-006 Re-encryption as Go bootstrap, not SQL
  Why: AES-GCM needs the key and random nonces — not feasible in SQL; bootstrap runs after `RunMigrations` when `MASKCHAIN_KEYS_KEY` present, idempotent.
  Tradeoff: bootstrap must be careful to skip already-ciphertext rows (detect via decrypt attempt).
  Affects: cmd run.go wiring, routing_registry.go (helper), migration 019.
  Validation: AC-002 integration test on scratch DB.

## Incremental Delivery

### MVP (Provider secrets at rest + masking)

- Encryptor wiring into registry repo; encrypt on write, decrypt on read.
- Masked DTO rendering + upsert guard.
- Migration 019 re-encryption bootstrap.
- Validated by AC-001..AC-004 + real-DB integration tests.

### Iterative expansion

- Slice 2: VirtualKeyCache (AC-005) + YAML seed idempotency test (AC-006).
- Slice 3: legacy removal — entity/DTO/UI/column/auth deletion (AC-007) + fail-closed key validation (AC-008).

## Implementation Order

- First: encryptor wiring + repo encryption/decryption (AC-001) — foundation for everything.
- Parallel-safe: DTO masking (AC-003/004) with the upsert guard; config key + fail-closed (AC-008).
- Then: migration 019 + re-encryption bootstrap (AC-002), then VirtualKeyCache (AC-005).
- Guarded: migration 020 `tenants.api_keys` drop only after all code paths stop reading the column (removal slice).
- Last: delete legacy auth/in-memory (AC-007) and UI field removal.

## Risks

- Risk 1: re-encryption bootstrap misclassifies rows (encrypts ciphertext or fails on plaintext).
  Mitigation: decrypt-attempt detection + idempotent design; integration test on scratch DB with mixed rows.
- Risk 2: cache staleness lets a revoked key authenticate up to refresh interval.
  Mitigation: immediate invalidation on admin create/update/delete + 30s default refresh; documented in AC-005.
- Risk 3: masked upsert accidentally re-encrypts literal.
  Mitigation: DEC-003 guard + AC-004 integration test.
- Risk 4: breaking the admin tenants API (api_keys removal) affects existing UI/clients.
  Mitigation: remove UI field in same slice; keep YAML seed untouched; breaking change isolated to admin API.

## Rollout & Compatibility

- Migration 019 runs at startup (Go bootstrap) — no manual DML.
- Migration 020 (drop `tenants.api_keys`) is a breaking schema change; applied in removal slice after code no longer reads the column.
- `MASKCHAIN_KEYS_KEY` must be provisioned before routing encryption activates; missing key with DB → startup fails (AC-008) — operators must add it to deployment configs (env example update).
- Auditability: key mutations already audit-logged; add cache-refresh log line at debug level.

## Verification

- Automated: `go test ./...` for touched packages (integration with `TEST_DATABASE_URL`), `go build/vet`, `golangci-lint`, UI `tsc -b` + `vitest run` + `vite build`.
- Manual: raw SQL inspection of `routing_providers` (ciphertext), `GET /api/v1/routing/providers` masked keys, edit-without-key-change preservation, revoke → next request rejected.
- AC coverage: 001 (TestPgRegistryProviderSecretEncryption), 002 (TestPgRegistryMigrateLegacySecrets), 003 (TestProviderMaskedResponse), 004 (TestUpsertProviderPreservesMaskedSecret), 005 (TestVirtualKeyCache*), 006 (TestBackfillVirtualKeysIdempotent), 007 (TestTenantWithoutAPIKeys + grep/build), 008 (TestMissingKeysKeyFailsClosed). DEC-001..006 validated by the same tests.

## Constitution Alignment

- No conflicts: encryption-at-rest and single auth path strengthen the "tenant-level security policy management" and "Content Shield is core" non-negotiables; UI stays tenant-management/logs only; no new framework pins.