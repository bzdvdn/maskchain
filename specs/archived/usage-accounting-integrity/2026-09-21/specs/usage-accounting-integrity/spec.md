# Usage & Spend Accounting Integrity

## Scope Snapshot

- In scope: make spend/usage accounting complete and non-bypassable across every LLM proxy path, including streaming (SSE) and the legacy `/completions` endpoint, and give operators a configurable cost rate for models without an explicit rate.
- Out of scope: mid-stream request abortion, token estimation without provider-reported usage, RBAC/SSO, and any DLP detector changes.

## Goal

Operators rely on MaskChain budgets to cap spend and on analytics to see real usage. Today streaming requests and the legacy `/completions` endpoint bypass spend/usage accounting, and models without a configured cost rate silently cost `0`, so budgets never trigger. This feature makes accounting truthful for every request that reaches a provider: a tenant that only uses streaming or an unrated model must still accrue spend, be blocked by a hard limit, and appear in analytics.

## Primary User Flow

1. Starting point: an operator has configured a budget (tenant/key/model) and/or cost rates; clients call `/api/v1/chat/completions` or `/api/v1/completions` with `stream: true` or `false`.
2. Main interaction: a request completes (streamed or not) and the provider reports token usage.
3. Outcome: spend counters and spend history increase by the computed cost, analytics tokens/cost include the request, and the next request is blocked with `429` once a hard limit is reached.
4. Failure/fallback path: when the provider reports no usage, the request is not counted and an observability signal is emitted; when a model has no explicit cost rate, the configured fallback rate is used, or `0` with a signal if no fallback is configured.

## User Stories

- P1 Story: as an operator, I can cap streaming spend so `stream: true` cannot be used to bypass a budget.
- P2 Story: as an operator, I can see streaming usage in analytics like any other request.
- P3 Story: as an operator, I can set one fallback cost rate so new/unlisted models still accrue spend.

## MVP Slice

- AC-001 (streaming spend recorded), AC-002 (streaming hard limit enforced on the next request), AC-006 (fallback cost rate). These three close the money hole with the least surface.

## First Deployable Outcome

- After the first pass, a streamed chat completion against a tenant with a monthly hard limit increments spend and the following request returns `429`; the same is observable in `GET /api/v1/budgets/:id/history`.

## Scope

- Streaming (`text/event-stream`) accounting on `/api/v1/chat/completions` and `/api/v1/messages`.
- The legacy `/api/v1/completions` endpoint (and its `/v1/completions` alias) served as a real provider proxy with the same middleware guarantees as chat completions.
- A configurable fallback cost rate for models without an explicit rate.
- Observability signals for the two "cannot account" cases: missing provider usage and missing cost rate.

## Context

- The gateway is a transparent proxy; the provider is the only authoritative source of token usage. OpenAI-compatible providers report usage in the final SSE chunk when `stream_options.include_usage` is set; Anthropic reports usage in `message_delta`. Ollama may report none.
- `BudgetMiddleware` and `UsageMiddleware` already parse usage for non-streaming responses and currently short-circuit with `c.Next()` for streaming (`middleware/budget.go`, `middleware/usage.go`).
- `RoutingProxyHandler` derives the upstream path from the request path, so the same handler can serve `/completions`.
- Constitution: native-only data plane, no external runtime dependencies; accounting must not add a Python/tokenizer service dependency.

## Dependencies

- Existing `budget` domain (`Budget`, `SpendCounter`, `BudgetRepository`, `AlertNotifier`) and its Valkey/Postgres adapters.
- Existing `analytics.CostRateRegistry` and `TokenUsage` pipeline.
- Existing provider streaming path (`FallbackHandler.Stream`) and the routing proxy handler.
- No new external service or library.

## Requirements

- RQ-001 For any request whose response is streamed, when the provider reports usage, the gateway MUST increment the matched budget spend counter, record a spend entry, and feed analytics with the reported tokens and computed cost.
- RQ-002 The gateway MUST enforce a budget hard limit for streaming clients by rejecting the next request once recorded spend reaches the limit (no mid-stream abortion).
- RQ-003 When a streamed response carries no provider usage, the gateway MUST NOT invent tokens or cost, and MUST emit an observable signal that accounting was skipped.
- RQ-004 `/api/v1/completions` (and `/v1/completions`) MUST be served as a real provider proxy and pass through the same model-access, shield, usage and budget guarantees as `/api/v1/chat/completions`.
- RQ-005 Cost computation MUST use an explicit per-model rate when configured; otherwise it MUST use a configurable fallback rate; otherwise it MUST use `0` and emit an observable signal.
- RQ-006 Fallback-rate usage and missing-usage skips MUST be visible as metrics (and a structured log line) so operators can detect unaccounted traffic.
- RQ-007 Existing non-streaming accounting behavior and response transparency (client sees the unmasked, unmodified provider response) MUST be preserved.

## Out of Scope

- Mid-stream request abortion when a limit is crossed.
- Token estimation without provider-reported usage.
- Budget scope/period/alert model changes and the admin budget API.
- Provider/routing selection changes.
- UI redesign; the existing Budgets/Analytics views are expected to reflect the corrected data without new screens.

## Non-Goals

- Aborting an in-flight stream when a limit is crossed.
- Estimating tokens with a local tokenizer when the provider reports none.
- Changing budget scopes, periods, alert thresholds, or the admin budget API.
- Adding new provider types or changing routing/fallback selection.
- Documenting or reworking the UI beyond what already consumes analytics/budgets.

## Acceptance Criteria

### AC-001 Streamed spend is recorded

- Why this matters: budgets are only real if streaming requests count.
- **Given** a tenant with an active budget and a routed model, and a client request with `stream: true`
- **When** the stream completes and the provider reported usage
- **Then** the budget's spend counter and spend history increase by the computed cost for that request.
- Evidence: `GET /api/v1/budgets/:id` shows a higher `spent`, and `GET /api/v1/budgets/:id/history` contains an entry for the streamed request with non-zero cost and tokens.

### AC-002 Streaming cannot bypass a hard limit

- Why this matters: this is the exact bypass being closed.
- **Given** recorded spend is at or above a budget's hard limit because of a prior streamed request
- **When** the client sends another request (streamed or not)
- **Then** the gateway rejects it with HTTP `429` and error code `BUDGET_EXCEEDED` before contacting the provider.
- Evidence: the response status/code, and no new provider call in gateway logs.

### AC-003 Streamed usage appears in analytics

- Why this matters: operators must trust token/cost dashboards.
- **Given** a completed streamed chat completion with provider-reported usage
- **When** token and cost analytics are queried for the tenant/model
- **Then** the request's input/output tokens and cost are included.
- Evidence: `GET /api/v1/analytics/tokens` and `GET /api/v1/analytics/cost` reflect the streamed request (and `maskchain_tokens_total`/`maskchain_cost_total` metrics increase).

### AC-004 Missing usage is observable, not invented

- Why this matters: silent undercounting is worse than a visible gap.
- **Given** a streamed response where the provider reports no usage
- **When** the stream completes
- **Then** no spend entry and no analytics tokens are recorded for it, and a missing-usage counter increases.
- Evidence: a `maskchain_usage_missing_total` metric increment and a structured log line naming the tenant/model; budget spend unchanged.

### AC-005 `/completions` has chat-completion guarantees

- Why this matters: a second proxy path must not be a weaker path.
- **Given** a routed model and a `POST /api/v1/completions` request
- **When** the key's model scope forbids the model, the request MUST return `403 MODEL_ACCESS_DENIED`; when a hard limit is exceeded it MUST return `429`; when it succeeds it MUST record usage/spend like a chat completion.
- Evidence: the three observable status/code outcomes plus a spend/analytics record for the successful case; the `/v1/completions` alias behaves identically.

### AC-006 Fallback cost rate is applied

- Why this matters: new/unlisted models must not silently cost zero.
- **Given** `analytics.default_cost_rate` is configured and a model has no explicit rate
- **When** a request for that model completes with reported usage
- **Then** the cost is computed from the fallback rate and spend increases accordingly.
- Evidence: budget `spent`/history use the fallback-derived cost, and a `maskchain_cost_rate_fallback_total` metric increments.

### AC-007 Zero-rate behavior is explicit

- Why this matters: operators must know when cost is genuinely zero.
- **Given** no explicit rate and no configured fallback rate for a model
- **When** a request completes
- **Then** cost is `0` and a signal distinguishes "no rate" from "fallback applied".
- Evidence: a `maskchain_cost_rate_missing_total` metric increments and spend is unchanged; the signal is distinct from the fallback-rate signal.

## Assumptions

- Providers report usage for streaming when asked; OpenAI-compatible endpoints honor `stream_options.include_usage`, Anthropic reports `message_delta.usage`. Where a provider does not, AC-004 applies.
- The gateway may add `stream_options.include_usage: true` to OpenAI-compatible streaming requests when the client did not set it, since it does not change the client-visible content stream.
- Existing non-streaming accounting and budget pre-flight checks are correct and remain unchanged in semantics.
- Cost rates and fallback rate are expressed in the same per-1K-token model as existing `CostRate`.

## Success Criteria

- SC-001 A streamed request with provider usage produces the same spend/analytics record shape as the equivalent non-streaming request.
- SC-002 No increase in client-visible latency for streaming responses (accounting happens after the stream ends).
- SC-003 Every "cannot account" outcome (missing usage, missing rate) is observable via metrics, so unaccounted traffic is never silent.

## Edge Cases

- Stream ends with an error or client disconnect before `[DONE]`: if usage was reported before the break, account it; otherwise apply AC-004.
- Client sets `stream_options.include_usage: false` explicitly: gateway still accounts when usage is present; otherwise AC-004.
- Multiple matched budgets (tenant + key + model): each matched budget is incremented once, exactly as in the non-streaming path.
- Budget hard limit is crossed *by* a streamed request: the stream completes; the next request is blocked (AC-002).
- `/completions` request without a routed model: existing `NO_ROUTE` behavior is preserved.

## Open Questions

- none
