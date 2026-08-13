# Semantic Cache over Masked Data — Plan

## Phase Contract

Inputs: spec and inspect (pass), minimal repo context (middleware chain, config, Valkey infra).
Outputs: plan.md, data-model.md.
Stop if: the spec is too vague to plan safely.

## Goal

Implement a semantic LLM-response cache over masked prompts, in-line on the chat path: spend/latency savings without weakening Content Shield and without storing raw data. The cache key is derived from the embedding of the masked input, never from raw text; per-tenant isolation is enforced in the key; budget-safe writes are guarded.

## Context

Mask placeholders are non-deterministic (`[MASK_<id>.<n>]`, `mask.NewShortID()` in `middleware/shield.go`), so the cache key is built from the embedding of the masked input, not from string equality. The cache sits in the chain after shield (masking) and before usage/budget/egress (`src/internal/api/server.go` RegisterProxyRoute). Existing stack: DDD (domain/app/adapters), Valkey already in use (`valkey-go`), viper/mapstructure config — see `data-model.md`.

## Requirements

RQ-001..RQ-010 from `spec.md` are adopted as-is, not duplicated here. Key points: only masked data in key/value (RQ-001/002), per-tenant isolation (RQ-003), hit/miss behavior (RQ-004/005), two embedding sources (RQ-006), disable (RQ-007), budget-safe (RQ-008), metrics (RQ-009), streaming passthrough (RQ-010).

## Acceptance Criteria

- AC-001 hit without provider call (`maskchain_cache_hit_total`, correct unmask)
- AC-002 miss proxies and stores
- AC-003 per-tenant isolation (tenant in key)
- AC-004/005 only masked forms in Valkey (key and value)
- AC-006 embedding fallback when external is down
- AC-007 disable — ordinary path
- AC-008 budget-safe: no writes in the last 5% of hard limit
- Approach and evidence per AC — in the "Acceptance Approach" section.

## Open Questions

One open question from the spec (streaming cache, variant A/B, RQ-010): in MVP — explicit passthrough for `stream=true`. Product confirmation — after first release.

## Assumptions

Valkey available; egress unmask path correct and untouched by cache; similarity threshold 0.90 global (default); streaming outside cache in MVP; external embedding is an optional dependency.

## MVP Slice

- Cache middleware on the chat path (non-streaming), Valkey storage, one embedding (external) + fallback, enable/TTL per tenant, metrics.
- Closes: `AC-001` (hit), `AC-002` (miss+write), `AC-003` (tenant isolation), `AC-004`/`AC-005` (masked-only), `AC-007` (disable).

## First Validation Path

- Local compose: enable `data.cache.enabled: true` for a tenant; send the same masked request twice; verify the second is served from cache (log/metric `maskchain_cache_hit_total`), provider not called, unmask correct.

## Scope

- `src/internal/domain/cache/` — entity, KeyPort, Store port, Embedder port (new packages behind adapters).
- `src/internal/adapters/repository/cache/` — ValkeyStore (key = hash(tenant+embedding), value = masked JSON).
- `src/internal/app/cache/` — SemanticCacheService (compute, lookup, write, budget-guard), ExternalEmbedder + SelfContainedEmbedder.
- `src/internal/api/middleware/cache.go` — SemanticCacheMiddleware (after shield, before usage/budget/egress).
- `src/internal/infra/config/config.go` + defaults — `DataConfig` (`data.cache.*`).
- `src/internal/infra/metrics/` — cache metrics.
- Wiring: `src/cmd/internal/bootstrap/` + `src/internal/api/server.go` (RegisterCacheMiddleware), `src/cmd/all/{admin,gw}.go`.
- Untouched boundaries: egress unmask path, shield pipeline, budget enforcement, existing routes, shield contracts.

## Performance Budget

- p95 middleware overhead on miss <5% (SC-002) — Valkey lookup sub-ms, embedding is the dominant cost.
- Memory: no cache entries for responses >1MB (configurable limit, spec edge case); allocations bounded by masked-body size.
- External embedding latency: must not block the data plane (fallback + timeout), see AC-006.

## Implementation Surfaces

- New packages `domain/cache` (ports: `Embedder`, `Store`; entities: `CacheKey` — tenant + masked-embedding; `CacheEntry` — masked JSON, TTL), `adapters/repository/cache` (ValkeyStore), `app/cache` (Service, embedders).
- Changed: `server.go` (add `RegisterCacheMiddleware`), `middleware/` (new `cache.go`), `infra/config/` (DataConfig), `infra/metrics/`, bootstrap package (constructor), `config.yaml`/`config-runtime.yaml`/helm-values (examples).
- Why new surfaces: `domain/cache` and `app/cache` do not exist; the Valkey client (`valkey-go`) is reused, but a dedicated store type for cache key/value is required (existing budget/session repos are different shapes).

## Bootstrapping Surfaces

- `src/internal/adapters/repository/cache/` — first; the Valkey client connection already exists, only a new store node is needed.
- `none` further bootstrap — the config framework and middleware chain are already defined.

## Architecture Impact

- Adds a middleware hook in the chain (does not replace existing ones).
- Clean domain ports (DDD).
- Config extends (`DataConfig`) without breaking changes; defaults valid.
- Cache does not change the wire response format — passthrough.

## Acceptance Approach

- AC-001: hit → metric + no provider call + correct unmask. Surfaces: middleware/cache.go, ValkeyStore, metric. Proof: integration test + metric.
- AC-002: miss → ordinary path, write to cache, second request hits. Proof: miss metric + second request hit.
- AC-003: per-tenant isolation — tenant in key; integration test with two tenants. Proof: test, `maskchain_cache_key_total{tenant}`.
- AC-004: Valkey contains only masked keys, no raw substrings. Proof: cache-entry inspection test.
- AC-005: value holds only masked forms. Proof: serialize/deserialize unit, redis view.
- AC-006: external unavailable → fallback/off, client gets 2xx from provider. Proof: mock unavailable external, `maskchain_cache_error_total`.
- AC-007: disable → ordinary path. Proof: config test.
- AC-008: budget in last 5% → stop writing, existing hits stay. Proof: limit-proximity test, `maskchain_cache_write_blocked_total`.

## Data and Contracts

- New `domain/cache` entities: `CacheEntry` (masked JSON), `CacheKey` (tenant+embedding hash). Stored in Valkey under `cache:{tenant}:{hash}` — not in PG; data-model change for the project.
- Contracts: the new `cache.go` middleware uses the new `data.cache.*` config structure. Public APIs unchanged (no new HTTP endpoints — management via config only).
- `data-model.md` — see below (carrier of the data changes in this feature).

## Implementation Strategy

- DEC-001 Place middleware after shield, before usage/budget
  Why: the cache must operate on masked text only, while spend/budget is not double-counted (a cache-hit response is not re-billed), and usage looks like the ordinary path — minimal chain disruption.
  Tradeoff: a cache hit skips usage/budget processing; requires a budget-gate guard so it stays non-blocking.
  Affects: server.go (chain), middleware/cache.go, budget-gate interplay.
  Validation: integration test on repeated requests shows no double spend counting.

- DEC-002 Embedding behind boundaries (external + self-contained)
  Why: external embedding gives default quality; self-contained keeps native-only and offline capability (constitution principle 4) without external runtimes.
  Tradeoff: two embedder implementations to maintain; self-contained is less accurate on unknown terms but deterministic.
  Affects: app/cache/embedder_*.go, config (data.cache.embedding.source), tests.
  Validation: unit tests for both; AC-006 fallback test.

- DEC-003 Cache key from hash (tenant + masked-embedding)
  Why: only masked data in key/value (diff thesis AC-004/005); mask IDs are not deterministic across requests → bind to embedding of the masked input, not to strings.
  Tradeoff: requires an embedder over masked text; on dictionary changes semantic duplicates are possible — TTL covers.
  Affects: domain/cache (CacheKey), app/cache service, ValkeyStore.
  Validation: AC-003 isolation unit test; cache entry inspected with no raw fragments.

- DEC-004 Budget-safe guard inside cache service (non-blocking)
  Why: standard behavior — do not create entries when close to hard-limit (last 5%), hits still served; does not block traffic.
  Tradeoff: a dependency on applied limits (budget context in middleware), write-blocked only.
  Affects: app/cache service, budget gate, metrics (cache_write_blocked).
  Validation: budget-threshold test; metric.

## Incremental Delivery

### MVP (First Value)

- `domain/cache` entity + port + `ValkeyStore`, embedder (external), service (hit/miss/write, budget-guard), middleware registration, config, metrics; enabled at chain head. Criterion: AC-001..005, AC-007 green in integration tests; manual compose probe.

### Iterative Expansion

- Step 2: self-contained fallback (AC-006, RQ-006) + external-unavailable test.
- Step 3: AC-008 budget guard + metrics (SC-001/SC-002 via synthetic benchmark later).

## Implementation Order

- First: bootstrap surfaces + domain/cache + ValkeyStore (no middleware) + unit tests.
- Then: embedder + service + middleware, registration, integration hit/miss test.
- Then: fallback + budget guard + metrics/config, final run.
- Parallel: config/metrics/cmd-wiring after the domain settles.

## Risks

- Risk 1: external embedding adds an external dependency → fallback + timeout, configurable off (AC-006).
- Risk 2: false hits on tenant dictionary changes → TTL and global threshold 0.90; semantic duplicates negligible.
- Risk 3: embedding latency on each miss → bounded sync/async; SC-002.
- Risk 4: double spend billing on hit → no increment on cache hit, test.
- Risk 5: streaming not covered in MVP (RQ-010 variant A) → explicit bypass for `stream=true`, stated in spec.

## Rollout and Compatibility

- Feature behind flag (`enabled: false` default), no migrations/backfill.
- Config examples in compose/helm/runtime; after release — monitor cache metrics.
- Backward compatible: all existing routes/contracts unchanged.

## Verification

- Automated: unit (cache service, embedder, Valkey parse), integration (hit/miss/isolation/no-raw/disable/budget) — go test.
- Manual: compose probe — same masked request twice; check metrics.
- AC: each of the 8 AC covered by a test; proof per evidence section of spec.

## Constitution Compliance

- No conflicts: native-only (self-contained fallback), Content Shield not weakened (masked-only storage), passthrough (cache over wire response, no translation).
