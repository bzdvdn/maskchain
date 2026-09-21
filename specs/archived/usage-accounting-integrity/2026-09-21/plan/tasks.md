# Usage & Spend Accounting Integrity Tasks

## Phase Contract

Inputs: `plan.md` + `spec.md` (decisions pinned).
Outputs: ordered executable tasks with coverage mapping.
Stop if: acceptance coverage cannot be mapped — not the case.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/api/middleware/usage_capture.go | T1.1, T2.1, T2.2 |
| src/internal/api/middleware/budget.go | T2.1 |
| src/internal/api/middleware/usage.go | T2.2 |
| src/internal/infra/metrics/metrics.go | T1.2 |
| src/internal/infra/config/config.go | T1.3 |
| src/internal/infra/config/defaults.go | T1.3 |
| src/internal/domain/analytics/cost_rate.go | T1.4 |
| src/cmd/gateway/run.go | T2.3 |
| src/cmd/all/gateway.go | T2.3 |
| src/cmd/all/admin.go | T2.3 |
| src/cmd/admin/run.go | T2.3 |
| src/internal/api/provider_handler.go | T2.4, T3.1 |
| src/internal/api/server.go | T3.1, T3.2 |
| src/internal/api/middleware/usage_capture_test.go | T4.1 |
| src/internal/api/middleware/budget_middleware_test.go | T4.1 |
| src/internal/api/middleware/usage_test.go | T4.1 |
| src/internal/domain/analytics/cost_rate_test.go | T4.1 |
| src/internal/api/server_test.go | T4.2 |
| src/internal/api/provider_handler_test.go | T4.2 |
| README.md | T4.3 |
| examples/config.yaml | T4.3 |

## Implementation Context

- MVP Goal: streamed and `/completions` traffic accrues spend/usage exactly like non-streaming, with fallback cost rate and visible gaps.
- Acceptance Boundaries: AC-001..AC-007.
- Key Rules: account **after** the response (no mid-stream abort, DEC-001); never invent tokens (DEC-006); one shared capture for JSON+SSE (DEC-002).
- Data/Domain Invariants: cost = explicit rate → `analytics.default_cost_rate` → `0`; each matched budget increments once; client response stays byte-transparent; non-streaming behavior unchanged.
- Errors/Codes: pre-flight `429 BUDGET_EXCEEDED` unchanged; `403 MODEL_ACCESS_DENIED` on `/completions`; accounting failures are logged, never client-visible.
- Contracts/Protocols: new config `analytics.default_cost_rate.{input_price_per_1k,output_price_per_1k}` and `analytics.stream_usage` (default `true`); `stream_options.include_usage` injected only for OpenAI-shaped streaming (`/chat/completions`, `/completions`), never `/messages`; `/completions` + `/v1/completions` become real proxies.
- Scope Boundaries: do not implement mid-stream abort or local token estimation; do not change budget domain/API, routing selection, or UI.
- Proof Signals: `go test` for middleware/analytics/api packages; `maskchain_usage_missing_total`, `maskchain_cost_rate_fallback_total`, `maskchain_cost_rate_missing_total` exposed; budget history entry for a streamed request.
- References: DEC-001..DEC-006 (plan.md), RQ-001..RQ-007 (spec.md).

## Phase 1: Foundation

Goal: shared capture, metrics, config, and fallback resolution that later phases depend on.

- [x] T1.1 Add shared usage capture — outcome: one helper extracts provider usage from a JSON body and from SSE (OpenAI final chunk, Anthropic `message_delta`), reports absence, and keeps buffers bounded. Touches: src/internal/api/middleware/usage_capture.go
      Proof: code src/internal/api/middleware/usage_capture.go usageCapture
- [x] T1.2 Add accounting metrics — outcome: `usage_missing_total`, `cost_rate_fallback_total`, `cost_rate_missing_total` are registered and appear at `/metrics`. Touches: src/internal/infra/metrics/metrics.go
      Proof: code src/internal/infra/metrics/metrics.go UsageMissingTotal
- [x] T1.3 Add fallback cost-rate config — outcome: `analytics.default_cost_rate` and `analytics.stream_usage` parse with sane defaults and validate. Touches: src/internal/infra/config/config.go, src/internal/infra/config/defaults.go
      Proof: code src/internal/infra/config/config.go AnalyticsConfig
- [x] T1.4 Add fallback resolution to the registry — outcome: `Resolve(model)` returns a rate plus its source (explicit/fallback/missing). Touches: src/internal/domain/analytics/cost_rate.go
      Proof: code src/internal/domain/analytics/cost_rate.go Resolve

## Phase 2: MVP Slice

Goal: post-response accounting for streaming and non-streaming, with fallback pricing.

- [x] T2.1 Account spend after the response in budget middleware — outcome: a completed streamed request increments the matched budget counter and records a spend entry; absent usage records nothing and increments the missing-usage metric (AC-001, AC-002, AC-004, AC-007). Touches: src/internal/api/middleware/budget.go, src/internal/api/middleware/usage_capture.go
      Proof: code src/internal/api/middleware/budget.go Handler
- [x] T2.2 Feed analytics for streaming responses — outcome: streamed requests produce a `TokenUsage` record and fallback/missing cost-rate metrics fire (AC-003, AC-006, AC-007). Touches: src/internal/api/middleware/usage.go, src/internal/api/middleware/usage_capture.go
      Proof: code src/internal/api/middleware/usage.go Handler
- [x] T2.3 Wire the fallback rate from config — outcome: every gateway/admin process builds the cost-rate registry with the configured fallback. Touches: src/cmd/gateway/run.go, src/cmd/all/gateway.go, src/cmd/all/admin.go, src/cmd/admin/run.go
      Proof: code src/cmd/gateway/run.go newCostRateRegistry
- [x] T2.4 Request provider usage on OpenAI-shaped streams — outcome: when `analytics.stream_usage` is enabled and the client omitted it, outgoing `/chat/completions` and `/completions` bodies carry `stream_options.include_usage: true`; `/messages` is untouched (AC-001). Touches: src/internal/api/provider_handler.go
      Proof: code src/internal/api/provider_handler.go withStreamUsage

## Phase 3: Core Implementation

Goal: close the legacy `/completions` bypass.

- [x] T3.1 Serve `/completions` as a real proxy with the chat chain — outcome: `POST /api/v1/completions` and `/v1/completions` pass model-access, shield, usage and budget and reach the routed provider (AC-005). Touches: src/internal/api/server.go, src/internal/api/provider_handler.go
      Proof: code src/internal/api/server.go RegisterProxyRoute
- [x] T3.2 Keep a safe fallback when no routing handler is wired — outcome: `/completions` still runs the full middleware chain with the stub handler instead of the reduced chain (AC-005). Touches: src/internal/api/server.go
      Proof: code src/internal/api/server.go RegisterProxyRoute

## Phase 4: Validation

Goal: prove behavior and leave the package reviewable.

- [x] T4.1 Add unit coverage for capture, accounting, and fallback — outcome: tests cover OpenAI/Anthropic/JSON/no-usage extraction, streaming spend increment, streaming usage record, and fallback/missing rate resolution (AC-001, AC-002, AC-003, AC-004, AC-006, AC-007). Touches: src/internal/api/middleware/usage_capture_test.go, src/internal/api/middleware/budget_middleware_test.go, src/internal/api/middleware/usage_test.go, src/internal/domain/analytics/cost_rate_test.go
      Proof: test src/internal/api/middleware/usage_capture_test.go TestSSEUsageParserOpenAI
- [x] T4.2 Add route coverage for `/completions` parity — outcome: tests assert `403 MODEL_ACCESS_DENIED`, `429 BUDGET_EXCEEDED`, and a successful call with a spend record on both `/api/v1/completions` and `/v1/completions` (AC-005). Touches: src/internal/api/server_test.go, src/internal/api/provider_handler_test.go
      Proof: test src/internal/api/server_test.go TestCompletionsUsesFullChain
- [x] T4.3 Document the new config and behavior — outcome: README and the full config reference describe `default_cost_rate`, `stream_usage`, and streaming accounting semantics. Touches: README.md, examples/config.yaml
      Proof: docs README.md

## Acceptance Coverage

- AC-001 -> T1.1, T2.1, T2.4, T4.1
- AC-002 -> T2.1, T4.1
- AC-003 -> T2.2, T4.1
- AC-004 -> T1.1, T1.2, T2.1, T4.1
- AC-005 -> T3.1, T3.2, T4.2
- AC-006 -> T1.3, T1.4, T2.2, T2.3, T4.1
- AC-007 -> T1.2, T1.4, T2.1, T2.2, T4.1

## Notes

- Phases are ordered by dependency: capture/metrics/config first, accounting second, route parity third, validation last.
- T4.3 is documentation only and must not change runtime behavior.
- No data model or migration work: config-only schema addition.
