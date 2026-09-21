# Embeddings Passthrough with Input Shield Plan

## Phase Contract

Inputs: `spec.md` + minimal repo context.
Outputs: `plan.md` (no data-model/contracts expansion).
Stop if: spec is vague — not the case; decisions pinned below.

## Goal

Add `POST /api/v1/embeddings` (and `/v1/embeddings`) served by the existing routing proxy, with a dedicated shield stage that masks the text `input` using the tenant's existing policy and never unmasks. Reuse the existing model-access, usage, budget and routing middleware so embeddings traffic is accounted and governed like chat, without touching the chat shield path.

## MVP Slice

- Embeddings shield middleware + route registration + wiring.
- AC covered: AC-001, AC-002, AC-005.

## First Validation Path

1. Configure an embeddings route and a tenant with an email-masking rule.
2. `curl -H "Authorization: Bearer <key>" -d '{"model":"<emb>","input":"email a@b.com"}' /api/v1/embeddings`.
3. Provider-bound body shows a placeholder; response is the provider's vectors.
4. Usage/budget show the prompt tokens.

## Scope

- New embeddings shield middleware applying tenant dictionaries + PII rules to `input` (string or array of strings), one-way.
- Route registration on both prefixes with the shared model-access/usage/budget/routing stages.
- Wiring in the gateway and combined binaries.
- Untouched: chat/messages shield behavior, tenant policy schema, routing selection, budget model.

## Performance Budget

- One extra shield scan over the input text only; no change to chat latency (SC-002). No new allocations beyond the masked body copy.

## Implementation Surfaces

- `src/internal/api/middleware/shield_embeddings.go` — new: parse `{model,input}`, apply dict + PII masking/block, rewrite body, set shield headers.
- `src/internal/api/server.go` — new `RegisterEmbeddingsShield` setter + register `/embeddings` on `/api/v1` and `/v1` with an embeddings-specific chain.
- `src/cmd/gateway/run.go`, `src/cmd/all/gateway.go` — construct and register the embeddings shield middleware.
- `src/internal/api/middleware/shield_embeddings_test.go` — new: masking, block, input-shape validation, no-provider-call cases.
- `src/internal/api/server_test.go` — route parity/auth for `/embeddings`.
- `README.md`, `examples/config.yaml` — document the endpoint and an embeddings route example.

## Bootstrapping Surfaces

- none — the routing proxy handler, shield engine and tenant policy already exist.

## Architecture Impact

- Local: one new middleware + a route registration hook.
- Integration: embeddings become a first-class proxy path; the routing handler is reused unchanged (it derives the upstream path from the request and parses only `model`/`stream`).
- No DB migration, no config schema change (embeddings routes use the existing routing rules).

## Acceptance Approach

- AC-001 → register `/embeddings` on both prefixes with auth + model access; proof: route test (401 without key, 200 with) and alias test.
- AC-002 → embeddings shield masks input before the handler; proof: test capturing the provider-bound body and asserting the placeholder while the response is unchanged.
- AC-003 → block rule aborts before the provider; proof: test asserting 403 and no provider invocation.
- AC-004 → input-shape validation rejects token arrays/empty; proof: test asserting validation error and no provider call.
- AC-005 → existing usage/budget capture reads prompt tokens for the non-streaming embeddings response; proof: tests asserting usage record + budget counter/history.
- AC-006 → routing handler fallback/no-route applies unchanged; proof: test with an unavailable primary and an unrouted model.

## Data and Contracts

- Data model: no change.
- API contract: new `POST /api/v1/embeddings` (+ `/v1/embeddings`) accepting `{model, input}` where `input` is a string or array of strings; response is the provider's OpenAI-compatible embeddings payload passed through.
- Config contract: none; embeddings models are routed via existing `routing.rules`.
- No `contracts/*` or `data-model.md` needed.

## Implementation Strategy

- DEC-001 Separate embeddings shield middleware, not branches inside the chat shield
  Why: the chat shield is DLP-critical and chat-shaped (messages, unmask writers, system-prompt injection); a separate path avoids regressing it.
  Tradeoff: some duplicated dict/PII scan code; acceptable for isolation.
  Affects: src/internal/api/middleware/shield_embeddings.go.
  Validation: new middleware tests + existing chat shield tests unchanged.
- DEC-002 Register embeddings via a setter, not a signature change
  Why: `RegisterProxyRoute` has existing callers and tests; a setter keeps them intact.
  Tradeoff: one extra registration call at startup.
  Affects: server.go, cmd/gateway/run.go, cmd/all/gateway.go.
  Validation: route tests; existing server tests unchanged.
- DEC-003 Reuse tenant policy with no unmask and no system-prompt injection
  Why: policy consistency with chat; there is no LLM response to instruct and vectors cannot be unmasked.
  Tradeoff: masked input changes embedding semantics for masked spans (accepted, per-tenant).
  Affects: shield_embeddings.go.
  Validation: masking/block tests.
- DEC-004 Reject non-text input shapes early
  Why: token arrays/base64 cannot be masked; passing them through would leak sensitive text.
  Tradeoff: clients using token-array inputs must switch to text.
  Affects: shield_embeddings.go.
  Validation: validation test.
- DEC-005 Reuse the routing proxy handler unchanged
  Why: it already derives the upstream path and handles non-streaming fallback; it only needs `model`.
  Tradeoff: none.
  Affects: provider_handler.go (verified, no change expected).
  Validation: fallback/no-route tests.

## Incremental Delivery

### MVP (First Value)

- Middleware + route + wiring.
- Ready when AC-001/002/005 pass.

### Iterative Expansion

- AC-003 block path, AC-004 input validation, AC-006 routing/fallback.
- Each validated by its own tests.

## Sequencing Notes

- Embeddings shield first (route depends on it).
- Route registration + wiring next.
- Docs and route/fallback tests last.

## Risks

- Chat shield regression → isolated by DEC-001 (no edits to chat path).
- Silent leakage via token-array inputs → DEC-004 rejects them explicitly.
- Embeddings semantics change for masked spans → documented as a tenant policy choice; default follows existing rules.
- Provider embeddings payloads vary → the endpoint is OpenAI-compatible and passes responses through unchanged.

## Rollout and Compatibility

- Additive endpoint; no migration. Chat behavior unchanged.
- Operators add embeddings models to `routing.rules` to enable the path.
- Watch `usage_missing_total` for providers that do not return embeddings usage.

## Validation

- Unit: embeddings shield (mask, block, input shapes, no provider call on reject).
- Route: `/api/v1/embeddings` and `/v1/embeddings` parity + auth.
- Accounting: usage record + budget spend for an embeddings request.
- Acceptance IDs covered: AC-001..AC-006. Decisions: DEC-001..DEC-005.

## Constitution Compliance

- no conflicts — Content Shield remains the core and is strengthened (more traffic protected); no new dependency; tenant policy is reused; no weakening of chat DLP.
