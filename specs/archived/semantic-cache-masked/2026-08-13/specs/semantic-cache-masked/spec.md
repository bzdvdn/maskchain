# Semantic Cache over Masked Data

## Scope Snapshot

- In scope: a semantic LLM-response cache over masked prompts — saves tokens/budget and is a differentiation unreachable by competitors (redact-only / NER-only gateways cannot compute a cache over replaced data deterministically).
- Out of scope: caching raw prompts/raw responses, compliance packs (401), zero-retention (402), changes to the masking pipeline.

## Goal

A tenant operator (FinOps / Platform engineer) gets reduced spend on repeated requests without weakening Content Shield: the cache operates exclusively on masked prompts, so confidential data never enters the cache index or value. Success is measured by hit-rate ≥40% on repeated requests and zero cross-tenant cache intersection.

## Main Scenario

1. Starting point: a chat request arrives at the existing middleware chain (auth → rate limit → shield).
2. Main action: after the prompt is masked (shield), the system computes a semantic key from the masked prompt, looks it up per tenant; on a hit it returns the masked response from cache (provider is skipped); on a miss it proxies to the provider and stores the masked response in cache.
3. Result: the client receives a response without an LLM API call; on egress the response goes through the standard unmask.
4. Failure/fallback path: any cache failure (embedding unavailable, Valkey down, TTL expired) degrades to transparent passthrough to the provider — no error to the client, metric `maskchain_cache_error_total` incremented.

## User Stories

- P1: an admin enables caching for a tenant (per-tenant flag + TTL); repeated semantically-similar masked requests are served from cache without a provider call and without raw-data leakage.
- P2: an admin sees hit-rate and token savings in metrics and can disable the cache per tenant or when the tenant budget is close to its limit.
- As this is an infrastructure optimization without a UI screen, P2 is delivered via metrics/config, not a new page.

## MVP Slice

- One middleware hook after shield-masking on the chat path, Valkey storage, one embedding source (external OpenAI-compatible) with a self-contained fallback, per-tenant enable + TTL.
- Must close `AC-001`, `AC-002`, `AC-003` first.

## First Deployable Outcome

- After the first implementation pass: local compose with cache enabled for a tenant; two identical masked requests — the second does not reach the provider (verified by log/metric), and the response is correct after unmask.

## Scope

- Semantic cache in the data plane (gateway), only after masking (hook between shield middleware and egress).
- Storage in Valkey: key = hash(tenant + masked-embedding), value = masked response; per-tenant TTL.
- Embedding: external OpenAI-compatible by default + self-contained fallback (no external runtime deps).
- Per-tenant enable/disable + TTL in config (`data.cache.*`).
- Budget-safe: when a tenant is close to hard-limit the cache stops writing new entries (hits still served).
- Metrics: hit-rate, token savings, errors, per-tenant.
- The existing unmask path in egress stays unchanged.

## Context

- Mask placeholders are not deterministic between requests: `[MASK_<docID>.<n>]`, where `docID` is a UUIDv7-ish short id (`src/internal/domain/shield/mask/`). Therefore the cache must compare masked inputs semantically, not by string equality of masked text.
- The chat-path middleware chain is: auth (tenant) → rate limit → shield; the cache hook goes after shield, before egress.
- Spend is already accounted for by `BudgetMiddleware` (spend increment after response); a cache hit must not double-count tokens — provider-path spend is not incremented on a cache hit.
- MaskChain is passthrough, not translation (principle 5); the cache transforms nothing, it caches an already-masked wire response.

## Dependencies

- `70-routing-engine` / `112-proxy-streaming-wiring` — integration points (already in code).
- Valkey — available project infra (rate limiting, mask cache already use it).
- External embedding API (optional) — OpenAI-compatible endpoint with a config key; self-contained fallback works without it.
- `none` inter-spec dependencies on unfinished features.

## Requirements

- RQ-001 The system MUST compute a semantic key exclusively from the masked prompt (after shield), never from raw data.
- RQ-002 The system MUST store only the masked response and masked key in cache; raw prompts/raw responses in cache are forbidden.
- RQ-003 The cache MUST be per-tenant isolated: a tenant A request cannot receive a tenant B response (scope in key).
- RQ-004 On a match (above the semantic-similarity threshold) the system MUST return the masked response from cache, skipping the provider call, and run it through the standard unmask.
- RQ-005 On a miss the system MUST proxy the request to egress and store the masked response in cache with the tenant TTL.
- RQ-006 The embedding source MUST be configurable: external OpenAI-compatible or self-contained fallback; when external is unavailable, degrade seamlessly without errors to the client.
- RQ-007 A tenant MUST be able to disable the cache; with caching disabled traffic takes the ordinary path.
- RQ-008 When a tenant budget is close to hard-limit, the system MUST stop writing new cache entries (only serving existing hits).
- RQ-009 The system MUST publish metrics: hit-rate, saved tokens, cache errors, per-tenant hits.
- RQ-010 Streaming responses MUST NOT be served from cache by default (client gets passthrough); the streaming-cache variant is an open question (default A).

## Out of scope

- Caching raw prompts or raw responses in any form.
- Compliance packs / data-retention (401/402) — separate features.
- Per-tool-call caching in MCP traffic.
- Event-driven invalidation on policy changes (detector change invalidates cache — deferred; rely on TTL).
- A UI page for cache settings (managed via config/Admin-API metrics only).

## Acceptance Criteria

### AC-001 Semantic hit without provider call

- Why it matters: the core value — spend/latency savings.
- **Given** a tenant with cache enabled and an already-cached response for masked prompt X
- **When** a semantically equivalent masked prompt X' arrives (different UUID placeholder, same content)
- **Then** the response is served from cache, the provider call is not made, and the client receives a correct unmasked response
- Evidence: `maskchain_cache_hit_total` incremented; no provider call for this request in provider logs; the response matches the cached one after unmask.

### AC-002 Miss proxies and stores

- Why it matters: the cache must not break the ordinary path.
- **Given** a tenant with cache enabled and a new masked prompt Y absent from cache
- **When** the request completes fully
- **Then** the response comes from the provider, is stored in cache, and the client receives the full correct response
- Evidence: `maskchain_cache_miss_total` incremented; a repeated Y request yields a hit (AC-001).

### AC-003 Per-tenant cache isolation

- Why it matters: multi-tenant security.
- **Given** two tenants A and B, each with its own masking dictionary and cache enabled
- **When** a tenant A request is semantically similar to a cached response of tenant B
- **Then** tenant A does not receive tenant B's response (cache key includes tenant scope and/or dictionaries)
- Evidence: integration test — a run under both tenant keys never crosses cache; `maskchain_cache_key_total{tenant}` shows the split.

### AC-004 No raw data in cache keys

- Why it matters: the diff thesis "cache over masked".
- **Given** a configured dictionary masking PII-like values and cache enabled
- **When** a request with sensitive data is processed and enters the cache
- **Then** the cache storage (Valkey) contains no raw fragments; key and value hold only masked forms
- Evidence: a test inspecting Valkey entries under the cache prefix finds zero raw-value substrings; runs in CI.

### AC-005 No raw data in cache values

- Why it matters: the cached value must carry no confidentiality.
- **Given** a masked response (containing `[MASK_...]` placeholders) stored in cache
- **When** an administrative inspection of Valkey contents occurs
- **Then** the cache value has no raw strings, only the masked form
- Evidence: unit test on entry serialize/deserialize; manual check via `redis-cli GET`.

### AC-006 Embedding fallback and resilience

- Why it matters: an external dependency must not break the data plane.
- **Given** the external embedding API is configured but temporarily unavailable (timeout/5xx)
- **When** a request arrives with cache enabled
- **Then** the system uses the self-contained fallback or disables cache read/write, and the client gets a normal provider response
- Evidence: test mocking an unavailable external embedding; `maskchain_cache_error_total` incremented, no 5xx to the client.

### AC-007 Cache disable

- Why it matters: per-tenant controllability.
- **Given** a tenant with cache disabled
- **When** a request arrives
- **Then** the path is untouched by the cache and the provider is always called
- Evidence: config-disable test; a provider call is made for every request when caching is off.

### AC-008 Budget-safe cache write

- Why it matters: limit safety.
- **Given** a tenant budget in the last 5% of its hard limit (spent ≥ 0.95 × hard_limit)
- **When** a request arrives
- **Then** new cache entries are not created while existing hits are still served
- Evidence: limit-proximity test; manual observation of `maskchain_cache_write_blocked_total`.

## Assumptions

- The existing unmask path in egress correctly reveals placeholders; the cache does not change the wire response format.
- Valkey is available (as for rate limiting / mask cache).
- Default semantic-similarity threshold = 0.90 (cosine), configurable globally via `data.cache.similarity_threshold`; empirical calibration after a first production period is tracked via success criteria, it does not block implementation.
- Streaming responses are not served from cache by default (passthrough) until RQ-010 is refined.
- The external embedding API is an optional dependency; the self-contained fallback has no external runtimes.

## Success Criteria

- SC-001 Hit-rate ≥40% on repeated (semantically similar) requests in a synthetic benchmark.
- SC-002 The added middleware overhead at p95 <5% on a miss (path without cache hop).
- SC-003 Zero cross-tenant cache-hit intersections in integration tests.

## Edge Cases

- Empty cache (first tenant request) — ordinary miss, no errors.
- TTL expires between read and write — race handled correctly (write-on-fact, possible duplication).
- Embedding unavailable on read/write — fallback, no error surfaced to the client.
- Response larger than the cache-entry size threshold — not written to cache (configurable limit).
- Tenant dictionary/rule changes — cache is not force-invalidated (TTL covers it; deferred to out-of-scope).

## Open Questions

- Streaming cache support (variant A/B from RQ-010) — default A, to confirm product-wise.