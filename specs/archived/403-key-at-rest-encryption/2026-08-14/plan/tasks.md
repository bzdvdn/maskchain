# At-rest Key Encryption, Virtual-Key Auth Cache, and Legacy Tenant Key Removal — Tasks

## Phase Contract

Inputs: `plan.md`, `data-model.md`, minimal repo context.
Outputs: ordered executable tasks with AC coverage.
Stop if: plan vague or an AC cannot map to work — none found.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/infra/config/config.go | T1.1, T5.1 |
| src/internal/infra/config/validator.go | T1.1, T4.5, T5.1 |
| src/internal/infra/config/defaults.go | T1.1 |
| src/internal/adapters/repository/postgres/routing_registry.go | T2.1, T2.2, T2.3, T3.1(n/a), T5.1 |
| src/internal/adapters/repository/postgres/virtual_key_repo.go | T3.1 |
| src/internal/api/middleware/virtualkey_cache.go | T3.1, T3.2 |
| src/internal/api/middleware/virtualkey_auth.go | T3.2 |
| src/internal/api/handler/admin/virtual_key_handler.go | T3.1 |
| src/internal/api/dto/routing.go | T2.3 |
| src/internal/api/handler/admin/routing_handler.go | T2.3 |
| src/internal/api/dto/tenant.go | T4.2 |
| src/internal/api/handler/admin/tenant_handler.go | T4.2 |
| src/internal/domain/shield/entity/tenant.go | T4.1 |
| src/internal/app/compliance/apply.go | T4.1 |
| src/internal/adapters/repository/tenant/in_memory.go | T4.3 |
| src/internal/api/middleware/auth.go | T4.3 |
| src/internal/api/server.go | T4.3 |
| src/internal/adapters/repository/postgres/tenant.go | T4.1, T4.4 |
| src/internal/adapters/repository/postgres/migrations/019_*.sql | T2.2, T4.4 |
| src/internal/adapters/repository/postgres/migrations/020_*.sql | T4.4 |
| src/cmd/gateway/run.go, src/cmd/admin/run.go, src/cmd/all/{gateway,admin}.go | T1.1, T3.2, T4.3 |
| ui/src/pages/Routing.tsx, ui/src/api/routing.ts | T2.4 |
| ui/src/pages/Tenants/{TenantForm,TenantDetail}.tsx, ui/src/api/tenants.ts | T4.2 |
| src/internal/adapters/repository/conversation/pg_conversation_store_test.go (scratch-DB infra pattern) | T5.1 |

## Implementation Context

- MVP goal: provider secrets encrypted at rest + masked in admin API/UI + legacy plaintext re-encrypted (AC-001..004 first).
- Invariants/semantics:
  - Secrets stored as `base64(AES-GCM(nonce||ct))` under `MASKCHAIN_KEYS_KEY` (reuse `infra/crypto.Encryptor`; it already returns nonce(12)||ct).
  - Decrypt ONLY at repository read boundary (`ListProviders`); runtime consumers (egress/provider adapters) keep plaintext.
  - Masking `sk-a***gh` = first 3 + `***` + last 3 (len > 8) else `***`; applies only to admin DTO responses.
  - Upsert guard: submitted key equal to masked form or empty → preserve stored ciphertext (DEC-003).
  - Virtual-key auth reads from in-process cache; authoritative state stays in DB; cache invalidated on admin create/update/delete (DEC-004).
- Errors/codes: missing/invalid `MASKCHAIN_KEYS_KEY` with DB → startup fails closed (AC-008). Decrypt failure → auth/read error surfaced.
- Contracts/protocol: tenant admin API drops `api_keys` (breaking); routing provider API returns masked keys; virtual-key one-time plaintext return unchanged; runtime provider headers unchanged.
- Scope boundaries: do not add KMS/Vault, do not encrypt `virtual_keys.key_hash`, do not change egress/provider adapters, do not remove `tenants.<slug>.api_keys` YAML seed (kept as seed only).
- Proof signals: integration tests with `TEST_DATABASE_URL` on scratch DB; `go build/vet/test` + `golangci-lint`; UI `tsc -b` + `vitest run` + `vite build`.
- References: DEC-001..006 in plan.md; DM-001..003 in data-model.md.

## Phase 1: Foundation

Goal: config surface + fail-closed key validation exist before any encryption wiring.

- [x] T1.1 Add `MASKCHAIN_KEYS_KEY` config wiring + fail-closed validation — outcome: config exposes keys key; validator errors when DB-backed routing/tenancy enabled and key absent/invalid (32-byte base64). Touches: src/internal/infra/config/config.go, src/internal/infra/config/validator.go, src/internal/infra/config/defaults.go, src/cmd/{gateway,admin}/run.go, src/cmd/all/{gateway,admin}.go
      Proof: code src/internal/infra/config/config.go KeysKeyEnvVar
      Proof: code src/internal/infra/config/config.go CryptoConfig
      Proof: code src/internal/infra/config/validator.go validateAtRestKeysKey
- [x] T1.2 Add key-env test — outcome: unit test proves fail-closed (AC-008). Touches: src/internal/infra/config/config_test.go
      Proof: test src/internal/infra/config/config_test.go TestMissingKeysKeyFailsClosed
      Proof: test src/internal/infra/config/config_test.go TestInvalidKeysKeyFailsClosed
      Proof: test src/internal/infra/config/config_test.go TestValidKeysKeyPasses

## Phase 2: MVP Slice — provider secrets at rest

Goal: AC-001..004 — secrets encrypted under keys key, masked in admin API/UI, legacy rows migrated.

- [x] T2.1 Implement repository encryption boundary — outcome: `UpsertProvider`/`SeedFromYAML` seal `api_keys`, `aws_access_key_id`, `aws_secret_access_key`, `additional_headers`; `ListProviders` decrypts. Touches: src/internal/adapters/repository/postgres/routing_registry.go
      Proof: code src/internal/adapters/repository/postgres/routing_registry.go sealBytes
      Proof: code src/internal/adapters/repository/postgres/routing_registry.go openBytes
      Proof: code src/internal/adapters/repository/postgres/routing_registry.go UpsertProvider
      Proof: code src/internal/adapters/repository/postgres/routing_registry.go SeedFromYAML
      Proof: code src/internal/adapters/repository/postgres/routing_registry.go ListProviders
- [x] T2.2 Add re-encryption of legacy plaintext rows — outcome: Go bootstrap (post-`RunMigrations`) detects plaintext rows via decrypt attempt and seals them; idempotent. Touches: src/internal/adapters/repository/postgres/routing_registry.go, src/cmd/{gateway,admin}/run.go, src/cmd/all/{gateway,admin}.go
      Proof: code src/internal/adapters/repository/postgres/routing_registry.go ReencryptProviderSecrets
      Proof: code src/cmd/gateway/run.go run
      Proof: code src/cmd/admin/run.go run
      Proof: code src/cmd/all/gateway.go buildGatewayServer
- [x] T2.3 Add provider-key masking + upsert guard — outcome: `dto.ProviderToResponse` masks keys; `UpsertProvider` skips overwrite when submitted value is masked/empty; handler passes through. Touches: src/internal/api/dto/routing.go, src/internal/api/handler/admin/routing_handler.go
      Proof: code src/internal/api/dto/routing.go MaskSecret
      Proof: code src/internal/api/dto/routing.go ProviderToResponse
      Proof: code src/internal/adapters/repository/postgres/routing_registry.go preserveMaskedSecrets
      Proof: code src/internal/adapters/repository/postgres/routing_registry.go UpsertProvider
- [x] T2.4 Wire provider masking into UI — outcome: Routing Providers page shows masked keys; add/edit keeps existing secrets when not changed. Touches: ui/src/pages/Routing.tsx, ui/src/api/routing.ts
      Proof: code ui/src/api/routing.ts isMaskedKey
      Proof: code ui/src/pages/Routing.tsx Routing
      Proof: code ui/src/pages/Routing.tsx CrudModal

## Phase 3: Virtual-key auth cache

Goal: AC-005, AC-006 — 0-DB auth after warm-up, auto-seed idempotency.

- [x] T3.1 Add in-process VirtualKeyCache — outcome: map `key_hash → (tenant, vk)`; periodic refresh (default 30s) via repo; invalidate(keyID/tenantID) exposed. Touches: src/internal/api/middleware/virtualkey_cache.go, src/internal/adapters/repository/postgres/virtual_key_repo.go
      Proof: code src/internal/api/middleware/virtualkey_cache.go VirtualKeyCache
      Proof: code src/internal/api/middleware/virtualkey_cache.go Start
      Proof: code src/internal/api/middleware/virtualkey_cache.go Refresh
      Proof: code src/internal/api/middleware/virtualkey_cache.go InvalidateKey
      Proof: code src/internal/api/middleware/virtualkey_cache.go InvalidateTenant
      Proof: code src/internal/adapters/repository/postgres/virtual_key_repo.go List
- [x] T3.2 Switch VirtualKeyAuth to cache + wire cache bootstrap — outcome: middleware resolves from cache, missing entry falls back to repo; gateway/admin bootstrap owns cache lifecycle. Touches: src/internal/api/middleware/virtualkey_auth.go, src/internal/api/middleware/virtualkey_cache.go, src/cmd/gateway/run.go, src/cmd/all/gateway.go
      Proof: code src/internal/api/middleware/virtualkey_auth.go VirtualKeyAuth
      Proof: code src/internal/api/middleware/virtualkey_auth.go lookupVirtualKey
      Proof: code src/cmd/gateway/run.go initTenants
      Proof: code src/cmd/all/run.go run
      Proof: code src/cmd/all/gateway.go buildGatewayServer
- [x] T3.3 Invalidate cache on admin key mutations — outcome: create/update/delete in VirtualKeyHandler invalidate cache (no stale auth after revoke). Touches: src/internal/api/handler/admin/virtual_key_handler.go
      Proof: code src/internal/api/handler/admin/virtual_key_handler.go NewVirtualKeyHandler
      Proof: code src/internal/api/handler/admin/virtual_key_handler.go Create
      Proof: code src/internal/api/handler/admin/virtual_key_handler.go Update
      Proof: code src/internal/api/handler/admin/virtual_key_handler.go Delete
      Proof: code src/internal/api/handler/admin/virtual_key_handler.go invalidateKey
      Proof: code src/cmd/all/admin.go buildAdminServer

## Phase 4: Legacy removal + fail-closed

Goal: AC-007, AC-008 — single virtual-key auth path, no tenant raw keys.

- [x] T4.1 Remove apiKeys from tenant entity + data path — outcome: no `apiKeys`/`APIKeys()` on entity, `NewTenant` drops param, repo scan/persist drops column reads, `compliance/apply.go` call fixed. Touches: src/internal/domain/shield/entity/tenant.go, src/internal/adapters/repository/postgres/tenant.go, src/internal/app/compliance/apply.go
  Proof: code src/internal/domain/shield/entity/tenant.go NewTenant
  Proof: code src/internal/adapters/repository/postgres/tenant.go scanTenant
  Proof: code src/internal/app/compliance/apply.go Apply
- [x] T4.2 Remove api_keys from tenant DTO + UI — outcome: Create/Update/Response DTOs and TenantForm/TenantDetail no longer carry `api_keys`. Touches: src/internal/api/dto/tenant.go, src/internal/api/handler/admin/tenant_handler.go, ui/src/pages/Tenants/{TenantForm,TenantDetail}.tsx, ui/src/api/tenants.ts
  Proof: code src/internal/api/dto/tenant.go TenantResponse
  Proof: code src/internal/api/handler/admin/tenant_handler.go CreateTenant
- [x] T4.3 Remove legacy in-memory raw-key auth — outcome: `middleware/Auth`, `TenantProvider`-based raw-key path, and `InMemoryRepository`/`FindByAPIKey` deleted; `VirtualKeyAuth` is the only auth path, requires DB. Touches: src/internal/api/middleware/auth.go, src/internal/adapters/repository/tenant/in_memory.go, src/internal/api/server.go, src/cmd/gateway/run.go, src/cmd/all/gateway.go
  Proof: code src/internal/api/middleware/auth.go (Auth removed)
  Proof: code src/internal/adapters/repository/tenant/in_memory.go (deleted)
  Proof: code src/cmd/gateway/run.go initTenants
  Proof: code src/cmd/all/gateway.go buildGatewayServer
- [x] T4.4 Add 020 migration dropping `tenants.api_keys` — outcome: column dropped; down restores it; applied after code stops reading the column. Touches: src/internal/adapters/repository/postgres/migrations/020_tenants_drop_api_keys.{up,down}.sql, src/internal/adapters/repository/postgres/tenant.go
  Proof: code src/internal/adapters/repository/postgres/migrations/020_tenants_drop_api_keys.up.sql
  Proof: code src/internal/adapters/repository/postgres/migrations/020_tenants_drop_api_keys.down.sql
- [x] T4.5 Update fail-closed validation for key removal — outcome: validator errors when DB absent but tenants configured (virtual-key auth requires DB). Touches: src/internal/infra/config/validator.go, src/internal/infra/config/config_test.go
  Proof: code src/internal/infra/config/validator.go validateAtRestKeysKey
  Proof: test src/internal/infra/config/config_test.go TestTenantsWithoutDBFailsClosed

## Phase 5: Verification

Goal: prove all ACs; leave package reviewable.

- [x] T5.1 Add integration + unit tests for encryption/masking/cache/removal — outcome: real-DB tests prove ciphertext at rest, masked responses, upsert key preservation, migration of legacy rows, cache call-count + invalidation, idempotent YAML seed, entity/DTO grep-negative. Touches: src/internal/adapters/repository/conversation/pg_conversation_store_test.go (pattern reuse), src/internal/adapters/repository/postgres/routing_registry_test.go, src/internal/api/handler/admin/routing_handler_test.go, src/internal/api/middleware/virtualkey_auth_test.go, src/internal/api/middleware/virtualkey_cache_test.go, src/internal/api/handler/admin/virtual_key_handler_test.go, src/internal/api/handler/admin/tenant_handler_test.go, src/internal/app/compliance/apply_test.go
  Proof: code src/internal/api/middleware/virtualkey_cache_test.go TestVirtualKeyAuthServesFromCacheAfterWarmup
  Proof: test src/internal/adapters/repository/postgres/routing_registry_integration_test.go TestPgRegistrySecretsCiphertextAtRest
  Proof: test src/internal/adapters/repository/postgres/routing_registry_integration_test.go TestPgRegistryReencryptLegacyRows
  Proof: code src/internal/api/handler/admin/routing_handler_test.go TestRoutingHandlerListMasksSecrets
  Proof: test src/internal/adapters/repository/postgres/virtual_key_backfill_integration_test.go TestPgVirtualKeyBackfillIdempotent
  Proof: test src/internal/api/dto/tenant_test.go TestTenantDTOGrepNegativeAPIKeys
  Proof: code src/internal/adapters/repository/postgres/routing_registry_test.go TestSealOpenJSONRoundTrip
- [x] T5.2 Run repo-wide verification — outcome: `go build/vet/test ./...` (in-scope packages green), `golangci-lint ./...` clean, UI `tsc -b`/`vitest run`/`vite build` clean, migration runner applies 019/020 on scratch DB. Touches: repo-wide (see Surface Map)
  Proof: test src/internal/adapters/repository/postgres/routing_registry_integration_test.go (real-DB ciphertext/re-encrypt/idempotent)
  Proof: code src/internal/adapters/repository/postgres/migrations/020_tenants_drop_api_keys.up.sql (applied on scratch DB, version 20)
  Proof: test src/internal/api/dto/tenant_test.go TestTenantDTOGrepNegativeAPIKeys

## AC Coverage

- AC-001 -> T2.1, T5.1
- AC-002 -> T2.2, T5.1
- AC-003 -> T2.3, T5.1
- AC-004 -> T2.3, T5.1
- AC-005 -> T3.1, T3.2, T3.3, T5.1
- AC-006 -> T3.2, T5.1
- AC-007 -> T4.1, T4.2, T4.3, T4.4, T5.1
- AC-008 -> T1.1, T1.2, T5.1

## Notes

- Phase order is dependency-driven: foundation → MVP(secrets) → cache → removal → verify.
- Fail-closed validation lives in Phase 1 (T1.1+T1.2); AC-008 covered there and by T5.1.
- Validation is split from implementation (Phase 5); each `[x]` task requires a `Proof:` line during implement.
- No phase skipped; edge cases (short keys, empty edit, DB-less run) covered inside T2.3/T3.2/T4.3 + T5.1 tests.