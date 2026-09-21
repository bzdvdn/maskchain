# Usage & Spend Accounting Integrity Plan

## Phase Contract

Inputs: `spec.md` + minimal repo context.
Outputs: `plan.md` (no data-model/contracts expansion).
Stop if: spec is vague — not the case; decisions below are pinned by the spec's answered questions.

## Goal

Make accounting complete on every proxy path by (a) capturing provider usage from both JSON and SSE responses through one shared tee-writer, (b) recording it after the response (stream included), (c) routing `/completions` through the real proxy with the full middleware chain, and (d) resolving cost via explicit → fallback → zero with metrics for the last two. Existing response transparency and non-streaming behavior are preserved.

## MVP Slice

- Shared usage capture + post-stream accounting + fallback rate + missing-usage metric.
- AC covered: AC-001, AC-002, AC-004, AC-006, AC-007.

## First Validation Path

1. Configure a tenant budget with a low hard limit and a routed streaming model.
2. Send `stream: true`; observe `GET /api/v1/budgets/:id` `spent` rise and history gain an entry.
3. Send again; observe `429 BUDGET_EXCEEDED`.
4. `curl /metrics | grep maskchain_usage_missing_total` and `..._cost_rate_fallback_total`.

## Scope

- `middleware/budget.go`, `middleware/usage.go`, new `middleware/usage_capture.go`.
- `domain/analytics/cost_rate.go` (fallback resolution).
- `infra/config` (`analytics.default_cost_rate`, `analytics.stream_usage`), `infra/metrics`.
- `api/server.go` + `api/provider_handler.go` (`/completions` chain, `stream_options` injection).
- `cmd/*` wiring for the fallback rate.
- Untouched: budget domain/API shape, routing selection, UI, DLP detectors, admin endpoints.

## Performance Budget

- Streaming responses: no client-visible latency added (accounting runs after the stream ends); SC-002.
- Capture memory: bounded per response — line buffer for SSE and a size cap for non-streaming bodies; no full-stream buffering.
- No new allocations on the hot path beyond the existing writer wrapper.

## Implementation Surfaces

- `src/internal/api/middleware/usage_capture.go` — new: tee-writer with usage extraction from JSON bodies and SSE streams, bounded buffers.
- `src/internal/api/middleware/budget.go` — use capture for streaming and non-streaming; record spend after response.
- `src/internal/api/middleware/usage.go` — use capture; feed analytics for streaming too.
- `src/internal/domain/analytics/cost_rate.go` — `Resolve(model)` returning a rate plus its source: explicit, fallback, or missing.
- `src/internal/infra/config/config.go` and `src/internal/infra/config/defaults.go` — `AnalyticsConfig.DefaultCostRate`, `StreamUsage`.
- `src/internal/infra/metrics/metrics.go` — `UsageMissingTotal`, `CostRateFallbackTotal`, `CostRateMissingTotal`.
- `src/internal/api/server.go` — `/completions` uses the chat chain + routing handler.
- `src/internal/api/provider_handler.go` — inject `stream_options.include_usage` for OpenAI-shaped streaming.
- `src/cmd/gateway/run.go`, `src/cmd/all/gateway.go`, `src/cmd/all/admin.go`, `src/cmd/admin/run.go` — pass fallback rate into the registry.

## Bootstrapping Surfaces

- none — all packages exist; only new file is `usage_capture.go`.

## Architecture Impact

- Local: two middlewares share one capture helper; metrics added.
- Integration: streaming responses are now inspected; `/completions` becomes a real proxy (behavior change vs stub).
- No DB migration; config-only schema addition; no UI change.

## Acceptance Approach

- AC-001 → capture SSE usage, compute cost, increment counter + spend entry; proof: budget middleware test + `budgets/:id/history`.
- AC-002 → existing pre-flight check now sees streamed spend; proof: test sending a second request after a streamed one returns 429.
- AC-003 → usage middleware feeds `TokenUsage` for streams; proof: middleware test + `analytics/tokens`.
- AC-004 → when capture finds no usage, no record and `UsageMissingTotal` increments; proof: test + `/metrics`.
- AC-005 → `/completions` mounted with the chat chain and routing handler; proof: route tests for 403/429/200 + spend record.
- AC-006 → `Resolve` returns fallback rate and `CostRateFallbackTotal` increments; proof: registry + middleware tests.
- AC-007 → no rate + no fallback → cost 0 and `CostRateMissingTotal`; proof: registry + middleware tests.

## Data and Contracts

- Data model: no change.
- Config contract: adds `analytics.default_cost_rate.{input_price_per_1k,output_price_per_1k}` and `analytics.stream_usage` (default `true`); absent fallback keeps current zero-cost behavior.
- API contract: `/api/v1/completions` and `/v1/completions` change from a static stub to a real provider proxy; no request/response schema change for successful calls.
- No `contracts/*` or `data-model.md` needed.

## Implementation Strategy

- DEC-001 Post-stream accounting, no mid-stream abort
  Why: provider cost is already incurred; no client-visible disruption; simplest correct enforcement.
  Tradeoff: one stream may overshoot the limit by its own cost; next request is blocked.
  Affects: middleware/budget.go, middleware/usage.go, usage_capture.go.
  Validation: streaming budget tests (AC-001/AC-002).
- DEC-002 One shared `usageCapture` writer for JSON and SSE
  Why: avoids two divergent parsers and unbounded buffering; one place to extend for new providers.
  Tradeoff: budget and usage middlewares each hold an instance (parse twice); acceptable, bounded.
  Affects: middleware/usage_capture.go, budget.go, usage.go.
  Validation: unit tests for OpenAI final-chunk, Anthropic `message_delta`, JSON body, and absent usage.
- DEC-003 Request provider usage via `stream_options.include_usage` (OpenAI-shaped, gated)
  Why: OpenAI-compatible providers do not report usage on streams by default; AC-001 needs it.
  Tradeoff: some providers may reject the field → `analytics.stream_usage: false` disables injection.
  Affects: provider_handler.go, config.
  Validation: manual/route test that injected bodies carry the field; config-off test.
- DEC-004 Fallback rate instead of fail-closed
  Why: keeps budgets effective for unlisted models without breaking new-model traffic.
  Tradeoff: fallback may misprice; mitigated by the fallback metric.
  Affects: config, analytics/cost_rate.go, cmd wiring.
  Validation: registry + middleware tests (AC-006/AC-007).
- DEC-005 `/completions` served by the routing proxy with the chat chain
  Why: removes the weaker path; handler already derives the upstream path from the request.
  Tradeoff: legacy endpoint now performs a real provider call (was a stub).
  Affects: server.go, provider_handler.go.
  Validation: route tests (AC-005).
- DEC-006 Accounting errors are non-fatal and observable
  Why: never fail a client response because accounting failed; make gaps visible instead.
  Tradeoff: spend can lag when a counter write fails.
  Affects: budget.go, usage.go, metrics.go.
  Validation: error-path tests + metric assertions.

## Incremental Delivery

### MVP (First Value)

- `usage_capture.go`, budget/usage middlewares, fallback rate, metrics.
- Ready when AC-001/002/004/006/007 pass via unit tests.

### Iterative Expansion

- `/completions` chain parity (AC-005).
- `stream_options` injection (hardens AC-001 for OpenAI).
- Each validated by its own tests without changing the MVP behavior.

## Sequencing Notes

- `usage_capture.go` first (shared dependency).
- Fallback rate + metrics can proceed in parallel with capture.
- `/completions` wiring and injection last; both are route/config-level and independent.
- No feature flag required; `analytics.stream_usage` is the only opt-out.

## Risks

- Provider rejects `stream_options` → config flag off; injection limited to OpenAI-shaped paths.
- Unknown SSE usage shape → capture handles OpenAI + Anthropic; anything else falls to AC-004 metric (visible, not silent).
- Overshoot by one stream → accepted and documented; hard limit still stops subsequent traffic.
- Duplicate capture parse cost → bounded buffers, post-response only.

## Rollout and Compatibility

- No migration. Existing non-streaming behavior unchanged.
- `/completions` behavior change is intentional and documented in the spec; monitoring via existing HTTP metrics.
- Watch `maskchain_usage_missing_total` / `maskchain_cost_rate_fallback_total` after rollout to size unaccounted traffic.

## Validation

- Unit: usage capture (OpenAI, Anthropic, plain JSON, and absent usage), budget streaming, usage streaming, registry fallback.
- Route: `/completions` 403 (model scope) / 429 (budget) / 200 (spend recorded); `/v1/completions` alias.
- Metrics: presence of the three new counters.
- Acceptance IDs covered: AC-001..AC-007. Decisions: DEC-001..DEC-006.

## Constitution Compliance

- no conflicts — native-only data plane preserved (no tokenizer/service added), tenant/budget storage unchanged, UI not treated as chat surface, no new external dependency.
