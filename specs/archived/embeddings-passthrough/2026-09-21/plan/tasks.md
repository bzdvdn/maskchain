# Embeddings Passthrough with Input Shield Tasks

## Phase Contract

Inputs: `plan.md` + `spec.md` (decisions pinned).
Outputs: ordered executable tasks with coverage mapping.
Stop if: acceptance coverage cannot be mapped — not the case.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/api/middleware/shield_embeddings.go | T1.1 |
| src/internal/api/middleware/shield_embeddings_test.go | T3.1 |
| src/internal/api/server.go | T1.2 |
| src/cmd/gateway/run.go | T2.1 |
| src/cmd/all/gateway.go | T2.2 |
| src/internal/api/server_test.go | T3.2 |
| src/internal/api/provider_handler_test.go | T3.3 |
| README.md | T3.4 |
| examples/config.yaml | T3.4 |

## Implementation Context

- MVP Goal: `POST /api/v1/embeddings` (+ `/v1/embeddings`) masks the text `input` per tenant policy, returns provider vectors unchanged, and is accounted/budgeted like chat.
- Acceptance Boundaries: AC-001..AC-006.
- Key Rules: separate embeddings shield middleware, no branches inside chat shield (DEC-001); reuse tenant policy with no unmask and no system-prompt injection (DEC-003); reject non-text input early (DEC-004); reuse the routing handler unchanged (DEC-005).
- Invariants: mask is one-way (never unmask vectors); dict masking uses `[MASK.N]` counters like chat; block rules abort before the provider; `X-Shield-Status` headers set (clean/suspicious/blocked).
- Errors/Codes: unsupported/empty input → `400 VALIDATION_ERROR`; block rule → `403` with `X-Shield-Status: blocked`; unauthenticated → `401`; unrouted model → existing `NO_ROUTE`; disallowed model → `403 MODEL_ACCESS_DENIED`.
- Contracts/Protocols: request `{model, input}` where `input` is a string or array of strings; response is the provider's OpenAI-compatible embeddings payload passed through; chain order = model-access, session, embeddings shield, usage, budget, routing handler.
- Scope Boundaries: do not change chat/messages shield behavior or the tenant policy schema; do not support token-array/base64 input; do not add embeddings caching or batching.
- Proof Signals: middleware tests (mask/block/shapes/no provider call), route tests (both prefixes, 401), accounting test (usage + budget), fallback/no-route test, `go test` green.
- References: DEC-001..DEC-005 (plan.md), RQ-001..RQ-007 (spec.md).

## Phase 1: Foundation

Goal: the embeddings shield and its route exist.

- [x] T1.1 Implement the embeddings shield middleware — outcome: it accepts `input` as a string or array of strings, rejects other shapes with a validation error, applies tenant dictionaries and PII rules (mask or block) to the input, rewrites the body, sets shield headers, and never unmasks. Touches: src/internal/api/middleware/shield_embeddings.go
      Proof: code src/internal/api/middleware/shield_embeddings.go EmbeddingsShieldMiddleware
- [x] T1.2 Register the embeddings route — outcome: `POST /api/v1/embeddings` and `/v1/embeddings` are served with model-access, session, embeddings shield, usage, budget and the routing handler. Touches: src/internal/api/server.go
      Proof: code src/internal/api/server.go RegisterEmbeddingsShield

## Phase 2: MVP Slice

Goal: the endpoint is live in both binaries.

- [x] T2.1 Wire the embeddings shield in the gateway binary — outcome: the gateway builds the embeddings shield middleware and registers the route before the proxy routes. Touches: src/cmd/gateway/run.go
      Proof: code src/cmd/gateway/run.go RegisterEmbeddingsShield
- [x] T2.2 Wire the embeddings shield in the combined binary — outcome: the combined binary does the same as the gateway. Touches: src/cmd/all/gateway.go
      Proof: code src/cmd/all/gateway.go RegisterEmbeddingsShield

## Phase 3: Validation

Goal: prove behavior and leave the package reviewable.

- [x] T3.1 Add embeddings shield unit coverage — outcome: tests assert input masking reaches the provider-bound body, block rules abort with 403 and no provider call, unsupported/empty input returns a validation error, and the response is not unmasked (AC-002, AC-003, AC-004). Touches: src/internal/api/middleware/shield_embeddings_test.go
      Proof: test src/internal/api/middleware/shield_embeddings_test.go TestEmbeddingsShieldMasksDictionary
- [x] T3.2 Add route and accounting coverage — outcome: tests assert both prefixes resolve, an unauthenticated call returns 401, and a completed embeddings request records usage and increments budget spend (AC-001, AC-005). Touches: src/internal/api/server_test.go
      Proof: test src/internal/api/server_test.go TestEmbeddingsRouteParityAndAuth
- [x] T3.3 Add routing coverage for embeddings — outcome: tests assert fallback to the next provider when the primary is unavailable and the no-route error for an unrouted model (AC-006). Touches: src/internal/api/provider_handler_test.go
      Proof: test src/internal/api/provider_handler_test.go TestRoutingHandlerEmbeddingsFallback
- [x] T3.4 Document the endpoint and an example route — outcome: README describes the embeddings endpoint and `examples/config.yaml` shows an embeddings model route. Touches: README.md, examples/config.yaml
      Proof: docs README.md

## Acceptance Coverage

- AC-001 -> T1.2, T2.1, T2.2, T3.2
- AC-002 -> T1.1, T3.1
- AC-003 -> T1.1, T3.1
- AC-004 -> T1.1, T3.1
- AC-005 -> T1.2, T2.1, T2.2, T3.2
- AC-006 -> T1.2, T3.3

## Notes

- Phases are dependency-driven: middleware → route → wiring → tests/docs.
- No data model, migration, or config schema change; embeddings models use existing routing rules.
- T3.4 is documentation only and must not change runtime behavior.
