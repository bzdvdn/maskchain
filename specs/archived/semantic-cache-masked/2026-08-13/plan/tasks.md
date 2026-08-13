# Semantic Cache over Masked Data — Tasks

## Phase Contract

Inputs: plan.md, spec.md, data-model.md (semantic-cache-masked).
Outputs: ordered executable tasks with AC coverage, Surface Map, Implementation Context.
Stop if: any AC-* cannot be mapped to executable work without guessing.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/domain/cache/ (new) | T1.1, T3.3, T3.4 |
| src/internal/adapters/repository/cache/ (new) | T1.2, T3.4 |
| src/internal/app/cache/ (new) | T2.1, T3.1, T3.2 |
| src/internal/api/middleware/cache.go (new) | T2.2, T3.5 |
| src/internal/api/server.go | T2.2 |
| src/cmd/internal/bootstrap/ | T2.2 |
| src/internal/infra/config/config.go | T1.2 |
| src/internal/infra/metrics/ | T2.3 |
| deployments/docker-compose/config.yaml | T4.2 |
| deployments/helm/maskchain/values.yaml | T4.2 |
| src/internal/domain/cache/cache_test.go | T1.1, T3.3, T3.4 |
| src/internal/api/middleware/cache_test.go | T2.4, T3.5 |

## Implementation Context

- MVP goal: chat-path middleware after shield that serves repeated semantically-similar masked requests from Valkey cache without a provider call.
- Key invariants: only masked forms in key/value (RQ-001/002); tenant in key (RQ-003, AC-003); hit skips provider and does not re-bill spend (DEC-001); no writes in last 5% of hard limit (AC-008).
- Errors/fallback: embedding/Valkey failure degrades to transparent passthrough — no 5xx to client, `maskchain_cache_error_total` (AC-006).
- Contracts: key `cache:{tenant}:{sha256(masked-embedding)}`; value = masked wire JSON; config `data.cache.*` (DM); no public HTTP changes.
- Proof signals: unit tests + integration test hitting `maskchain_cache_*` metrics; Valkey inspection shows no raw substrings.
- Out of scope: streaming cache (passthrough for `stream=true`, RQ-010 variant A), policy invalidation, UI page, raw caching.
- References: DEC-001..004, DM, RQ-001..010.

## Phase 1: Foundation

Goal: domain entities, ports, Valkey store and config so the middleware has stable ground.

- [x] T1.1 Add domain/cache package — CacheKey (tenant + sha256(masked-embedding)), CacheEntry (masked JSON, created_at, expires_at), Embedder port (Embed(text) []float32), Store port (Get/Set with TTL); unit tests for key derivation and entry serialization. Touches: src/internal/domain/cache/ (new), src/internal/domain/cache/cache_test.go
  Proof: code src/internal/domain/cache/entity.go CacheKey
  Proof: code src/internal/domain/cache/repository.go Embedder
  Proof: test src/internal/domain/cache/cache_test.go TestCacheKeyDeterministic
- [x] T1.2 Add ValkeyStore adapter + DataConfig — Valkey GET/SET with per-tenant TTL and size guard (skip entries > max_entry_bytes); config keys `data.cache.enabled|ttl|similarity_threshold|budget_guard_percent|max_entry_bytes|embedding.*` with defaults; validation. Touches: src/internal/adapters/repository/cache/ (new), src/internal/infra/config/config.go
  Proof: code src/internal/adapters/repository/cache/valkey.go ValkeyCacheStore
  Proof: code src/internal/infra/config/config.go CacheConfig
  Proof: code src/internal/infra/config/validator.go validateDataCache

## Phase 2: MVP Slice

Goal: end-to-end hit/miss behavior on the chat path with external embedding.

- [x] T2.1 Add SemanticCacheService + ExternalEmbedder — compute masked-embedding, lookup, on hit return masked response; on miss write masked response with tenant TTL; embedder calls OpenAI-compatible `external_url` with timeout. Touches: src/internal/app/cache/ (new)
  Proof: code src/internal/app/cache/service.go SemanticCacheService
  Proof: code src/internal/app/cache/embedder.go ExternalEmbedder
- [x] T2.2 Add SemanticCacheMiddleware + wiring — hook after shield and before usage/budget/egress in RegisterProxyRoute; skip when disabled, `stream=true`, or not chat; expose RegisterCacheMiddleware on Server; bootstrap constructor wires service+store+config; per-tenant enable from config. Touches: src/internal/api/middleware/cache.go (new), src/internal/api/server.go, src/cmd/internal/bootstrap/
  Proof: code src/internal/api/middleware/cache.go SemanticCacheMiddleware
  Proof: code src/internal/api/server.go RegisterCacheMiddleware
  Proof: code src/cmd/internal/bootstrap/cache.go NewSemanticCacheService
  Proof: code src/cmd/gateway/run.go
  Proof: code src/cmd/all/gateway.go
- [x] T2.3 Add cache metrics — `maskchain_cache_hit_total`, `_miss_total`, `_error_total`, `_write_blocked_total`, `_key_total{tenant}` (labels tenant). Touches: src/internal/infra/metrics/
  Proof: code src/internal/infra/metrics/metrics.go CacheHitsTotal
- [x] T2.4 Add integration tests for MVP — hit (AC-001): second semantically-equivalent masked request returns from cache, provider not called, unmask correct; miss (AC-002): provider response stored, second request hits; disable (AC-007): provider always called. Touches: src/internal/api/middleware/cache_test.go
  Proof: test src/internal/api/middleware/cache_test.go TestSemanticCacheHitSkipsProvider
  Proof: test src/internal/api/middleware/cache_test.go TestSemanticCacheMissStores
  Proof: test src/internal/api/middleware/cache_test.go TestSemanticCacheDisabledPassesThrough

## Phase 3: Expansion

Goal: fallback embedding, budget guard, isolation and no-raw proofs.

- [x] T3.1 Add SelfContainedEmbedder fallback — deterministic offline embedding when external is unavailable (timeout/5xx) or `source: self-contained`; AC-006 test with mocked unavailable external showing client gets 2xx and `_error_total`. Touches: src/internal/app/cache/, src/internal/api/middleware/cache_test.go
  Proof: code src/internal/app/cache/embedder_fallback.go FallbackEmbedder
  Proof: code src/internal/app/cache/embedder_fallback.go SelfContainedEmbedder
  Proof: test src/internal/api/middleware/cache_test.go TestSemanticCacheFallbackEmbedderDegrades
- [x] T3.2 Add budget-safe guard — block new writes when tenant spent ≥ 0.95 × hard_limit (budget_guard_percent), still serve hits; metric `_write_blocked_total`; AC-008 test. Touches: src/internal/app/cache/, src/internal/api/middleware/cache_test.go
  Proof: code src/internal/app/cache/guard.go BudgetWriteGuard
  Proof: test src/internal/api/middleware/cache_test.go TestSemanticCacheBudgetGuardBlocksWrites
- [x] T3.3 Add tenant-isolation test — two tenants, A cannot hit B's cache (tenant in key); AC-003. Touches: src/internal/domain/cache/cache_test.go
  Proof: test src/internal/domain/cache/cache_test.go TestCacheKeyTenantIsolation
- [x] T3.4 Add no-raw-data tests — inspect Valkey cache prefix entries: zero raw substrings in keys and values; AC-004/005. Touches: src/internal/domain/cache/cache_test.go, src/internal/adapters/repository/cache/
  Proof: test src/internal/domain/cache/cache_test.go TestCacheKeyContainsNoRaw
  Proof: test src/internal/domain/cache/cache_test.go TestCacheEntryMarshalLeaksNoRaw
  Proof: code src/internal/adapters/repository/cache/valkey.go ValkeyCacheStore
- [x] T3.5 Add disable-flag behavior test — `data.cache.enabled: false` path bypassed, provider always called; AC-007 coverage completes. Touches: src/internal/api/middleware/cache_test.go
  Proof: test src/internal/api/middleware/cache_test.go TestSemanticCacheDisabledPassesThrough

## Phase 4: Verification

Goal: prove the feature and leave the package reviewable.

- [x] T4.1 Run full verification — go build/vet/test ./... and golangci-lint clean; confirm all AC-001..008 evidence points pass. Touches: repo-wide (verification only)
  Proof: test src/internal/api/middleware/cache_test.go TestSemanticCacheBudgetGuardBlocksWrites
  Proof: chore src/internal/domain/cache/cache_test.go TestCacheEntryJSONShape
- [x] T4.2 Update deployment config examples — `data.cache.*` block (disabled default) in compose config.yaml and helm values.yaml. Touches: deployments/docker-compose/config.yaml, deployments/helm/maskchain/values.yaml
  Proof: docs deployments/docker-compose/config.yaml data.cache
  Proof: docs deployments/helm/maskchain/values.yaml data.cache

## Acceptance Coverage

- AC-001 -> T2.1, T2.2, T2.4
- AC-002 -> T2.1, T2.2, T2.4
- AC-003 -> T1.1, T3.3
- AC-004 -> T1.1, T3.4
- AC-005 -> T1.1, T3.4
- AC-006 -> T3.1
- AC-007 -> T2.2, T2.4, T3.5
- AC-008 -> T3.2

## Notes

- MVP first (Phase 1-2), then fallback/budget/isolation (Phase 3), then verification (Phase 4).
- Proof rows added by the implementer on `[x]` completion; every closed task needs `Proof: <kind> <path> [<anchor>]`.
