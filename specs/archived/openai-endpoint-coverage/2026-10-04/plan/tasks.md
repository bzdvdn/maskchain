# OpenAI Endpoint Coverage Tasks

## Phase Contract

Inputs: `specs/active/openai-endpoint-coverage/plan.md`, `spec.md`, `inspect.md` (concerns folded into plan).
Outputs: ordered executable tasks with coverage mapping.
Stop if: tasks would be vague or acceptance coverage cannot be mapped.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/adapters/provider/upstream.go | T1.1 |
| src/internal/adapters/provider/upstream_test.go | T1.2 |
| src/internal/api/middleware/shield_embeddings.go | T2.1 |
| src/internal/api/middleware/shield_embeddings_test.go | T2.1 |
| src/internal/api/middleware/shield_text.go | T2.1, T2.2 |
| src/internal/api/middleware/shield_text_test.go | T2.3 |
| src/internal/api/server.go | T3.1, T3.2, T3.3 |
| src/internal/api/self_handler.go | T3.2, T3.4 |
| src/internal/api/self_handler_test.go | T3.4 |
| src/internal/api/provider_handler_test.go | T3.3 |
| src/internal/api/server_test.go | T3.3 |
| src/cmd/gateway/run.go | T4.1 |
| src/cmd/all/gateway.go | T4.1 |
| README.md | T4.2 |
| Makefile | T4.3 |

## Implementation Context

- MVP goal: `GET /v1/models` (OpenAI shape, key-scoped) + `POST /v1/moderations` and `POST /v1/rerank` proxied with request masking (AC-001..AC-004).
- Invariants/semantics: reuse the existing model-based proxy handler (`HandleChatCompletion`) for the POST endpoints; non-chat endpoints use the reduced chain (modelAccess → textShield → usage → budget → export → handler) and skip session/conversation/cache/SSE; masking is one-way (no response unmask); `/api/v1/models` keeps its `{id,allowed}` array.
- Errors/codes: `UNSUPPORTED_ENDPOINT` → 400; `BUDGET_EXCEEDED` → 429; model access → 403 `MODEL_ACCESS_DENIED`; missing `model` → 400 `VALIDATION_ERROR`.
- Contracts/protocol: `GET /v1/models` returns `{"object":"list","data":[{"id":...,"object":"model","created":0,"owned_by":"maskchain"}]}` filtered by key scope; new POST routes mounted on both `/v1` and `/api/v1`; upstream allowlist gains `/v1/moderations`, `/v1/rerank` (OpenAI-shaped) and `/v1/messages/count_tokens` (anthropic).
- Scope boundaries: do not add images/audio/responses/batches/files/fine-tuning/MCP; do not translate OpenAI↔Anthropic; do not change `/api/v1/models`; do not add budget gating to the catalog.
- Proof signals: extractor unit tests; route tests with a recording mock asserting path/masked body/zero calls on rejection; handler tests for the models shape and key scope.
- References: DEC-001..DEC-009, RQ-001..RQ-009 (no mandatory re-read).

## Phase 1: Foundation

Goal: make the new endpoint shapes acceptable to the provider path resolver.

- [x] T1.1 Extend `upstreamPathFor` with `/v1/moderations` and `/v1/rerank` for OpenAI-shaped providers and `/v1/messages/count_tokens` for anthropic (DEC-003, AC-005, AC-008). Touches: src/internal/adapters/provider/upstream.go
      Proof: code src/internal/adapters/provider/upstream.go upstreamPathFor
- [x] T1.2 Add allowlist table cases for each new path and the anthropic count_tokens path, incl. cross-format rejection (AC-005, AC-008). Touches: src/internal/adapters/provider/upstream_test.go
      Proof: test src/internal/adapters/provider/upstream_test.go TestResolveUpstreamURL

## Phase 2: Non-chat input shield

Goal: mask the text fields of moderations/rerank/count_tokens using the existing detector pipeline.

- [x] T2.1 Extract the shared dictionary+PII masking core out of `EmbeddingsShieldMiddleware` into a reusable helper with no behavior change (DEC-002). Touches: src/internal/api/middleware/shield_embeddings.go, src/internal/api/middleware/shield_text.go, src/internal/api/middleware/shield_embeddings_test.go
      Proof: code src/internal/api/middleware/shield_text.go maskTexts
- [x] T2.2 Implement `TextInputShieldMiddleware` with per-endpoint field adapters: moderations `input` (string/array), rerank `query` + `documents[].text`, count_tokens message/system text; rewrite only those fields (DEC-002, DEC-007, AC-003, AC-004, AC-005). Touches: src/internal/api/middleware/shield_text.go
      Proof: code src/internal/api/middleware/shield_text.go TextInputShieldMiddleware
- [x] T2.3 Add extractor/rewriter and block/error tests for each endpoint kind (AC-003, AC-004). Touches: src/internal/api/middleware/shield_text_test.go
      Proof: test src/internal/api/middleware/shield_text_test.go TestTextInputShieldMasksModerations

## Phase 3: Routes and model catalog

Goal: expose the new endpoints and the OpenAI-shaped model list.

- [x] T3.1 Mount `POST /moderations`, `/rerank`, `/messages/count_tokens` on `/v1` and `/api/v1` with the reduced chain, and add `RegisterTextShield` (DEC-009, AC-006, AC-007, AC-009). Touches: src/internal/api/server.go
      Proof: code src/internal/api/server.go RegisterTextShield
- [x] T3.2 Add `HandleModelsOpenAI` returning the OpenAI list shape and register `GET /v1/models` behind auth only (DEC-004, DEC-005, AC-001, AC-002). Touches: src/internal/api/self_handler.go, src/internal/api/server.go
      Proof: code src/internal/api/self_handler.go HandleModelsOpenAI
- [x] T3.3 Add route tests: correct upstream path per endpoint, both prefixes + query preservation, unsupported → 400 with zero calls, auth 401/403, budget 429 (AC-006, AC-007, AC-008, AC-009). Touches: src/internal/api/provider_handler_test.go, src/internal/api/server_test.go
      Proof: test src/internal/api/server_test.go TestTextEndpointsRouteParityAndAuth
- [x] T3.4 Add catalog tests: OpenAI shape fields, key-scope filtering, and unchanged `/api/v1/models` array (AC-001, AC-002). Touches: src/internal/api/self_handler_test.go
      Proof: test src/internal/api/self_handler_test.go TestSelfHandlerModelsOpenAI

## Phase 4: Wiring and validation

Goal: run the new behavior in both binaries and leave the package reviewable.

- [x] T4.1 Wire the text shield and `/v1/models` in both binaries, mirroring `RegisterEmbeddingsShield`/`RegisterSelfHandler` (AC-001, AC-006). Touches: src/cmd/gateway/run.go, src/cmd/all/gateway.go
      Proof: code src/cmd/gateway/run.go RegisterTextShield
- [x] T4.2 Document the new proxy endpoints and `/v1/models` shape in the README (AC-001, AC-005). Touches: README.md
      Proof: docs README.md
- [x] T4.3 Run the affected suites and the full `make test`; fix regressions (SC-001, SC-002). Touches: Makefile
      Proof: chore Makefile test

## Acceptance Coverage

- AC-001 -> T3.2, T3.4, T4.1, T4.2
- AC-002 -> T3.2, T3.4
- AC-003 -> T2.1, T2.2, T2.3, T3.1, T3.3
- AC-004 -> T2.2, T2.3, T3.1, T3.3
- AC-005 -> T1.1, T1.2, T2.2, T3.1, T4.2
- AC-006 -> T3.1, T3.3, T4.1
- AC-007 -> T3.1, T3.3
- AC-008 -> T1.1, T1.2, T3.3
- AC-009 -> T3.1, T3.3

## Notes

- Phase 2's extraction must keep embeddings behavior identical; run `shield_embeddings_test.go` after the refactor.
- Phase 1 must land before Phase 2/3 route work so unsupported shapes fail explicitly rather than hitting a wrong endpoint.
- Every `[x]` task needs a `Proof:` line on the next line; no proof means not done.
