# Responses API Support Tasks

## Phase Contract

Inputs: `specs/active/responses-api/plan.md`, `spec.md`.
Outputs: ordered executable tasks with coverage mapping.
Stop if: tasks would be vague or acceptance coverage cannot be mapped.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/api/middleware/shield_text.go | T1.1 |
| src/internal/api/middleware/shield_embeddings.go | T1.1 |
| src/internal/api/middleware/shield_text_test.go | T1.3 |
| src/internal/adapters/provider/upstream.go | T1.2 |
| src/internal/adapters/provider/upstream_test.go | T1.3 |
| src/internal/api/middleware/responses_shield.go | T2.1 |
| src/internal/api/middleware/responses_shield_test.go | T2.2 |
| src/internal/api/server.go | T3.1 |
| src/cmd/gateway/run.go | T3.2 |
| src/cmd/all/gateway.go | T3.2 |
| src/internal/api/server_test.go | T3.3 |
| src/internal/api/middleware/usage_capture_test.go | T3.4 |
| Makefile | T4.1 |

## Implementation Context

- MVP goal: non-streaming `POST /v1/responses` on both prefixes with request masking of `instructions`/`input`, response unmasking of `output_text`/`output[].content[].text`, and auth/model-access/budget (AC-001…AC-006, AC-008).
- Invariants/semantics: `stream:true` is rejected (`STREAMING_UNSUPPORTED`, 400) before any provider call; `maskTexts` returns the placeholder→original mapping, and the Responses shield installs the chat `wrapUnmaskWriter` so the response body is globally de-placed; all other request fields are forwarded unchanged; masking is reversible on the response.
- Errors/codes: `STREAMING_UNSUPPORTED` 400; `NO_ROUTE` 400; `MODEL_ACCESS_DENIED` 403; `BUDGET_EXCEEDED` 429; `UNSUPPORTED_ENDPOINT` 400.
- Contracts/protocols: routes `/v1/responses` and `/api/v1/responses`; Responses request `{model, input(string|items), instructions, stream, previous_response_id, store, tools, ...}`; response text at `output_text` and `output[].content[].text`; usage at `usage.{input_tokens,output_tokens}`.
- Scope boundaries: do not implement streaming events, background mode, `GET /v1/responses/{id}`, tool-argument unmasking, or cross-format translation.
- Proof signals: extractor tests (string/items/blocks); recording mock sees masked prompt and client sees originals; `stream:true` rejected with zero calls; route/prefix/unsupported tests; usage mapping test.
- References: DEC-001..DEC-007, RQ-001..RQ-008 (no mandatory re-read).

## Phase 1: Foundation

Goal: make the masking core reversible and register the Responses path.

- [x] T1.1 Extend `maskTexts` to also return the placeholder→original mapping and adapt the embeddings caller (DEC-002, AC-004). Touches: src/internal/api/middleware/shield_text.go, src/internal/api/middleware/shield_embeddings.go
      Proof: code src/internal/api/middleware/shield_text.go maskTexts
- [x] T1.2 Add `/v1/responses` to the OpenAI-shaped path allowlist (DEC-004, AC-008). Touches: src/internal/adapters/provider/upstream.go
      Proof: code src/internal/adapters/provider/upstream.go openAIShapedEndpoints
- [x] T1.3 Prove the masking mapping and the new allowlist entry (AC-004, AC-008). Touches: src/internal/api/middleware/shield_text_test.go, src/internal/adapters/provider/upstream_test.go
      Proof: test src/internal/api/middleware/shield_text_test.go TestMaskTextsReturnsMapping

## Phase 2: Responses shield (MVP)

Goal: mask the Responses request and unmask the response.

- [x] T2.1 Implement `ResponsesShieldMiddleware`: extract `instructions` + `input` text (string, message items, `input_text`/`text` blocks), mask and rewrite the body preserving other fields, install the unmask writer, and reject `stream:true` with `STREAMING_UNSUPPORTED` (DEC-002, DEC-003, DEC-007, AC-003, AC-004, AC-005, AC-006). Touches: src/internal/api/middleware/responses_shield.go
      Proof: code src/internal/api/middleware/responses_shield.go ResponsesShieldMiddleware
- [x] T2.2 Add Responses shield tests: extractor shapes, request masking, response unmasking, and stream rejection (AC-003, AC-004, AC-005, AC-006). Touches: src/internal/api/middleware/responses_shield_test.go
      Proof: test src/internal/api/middleware/responses_shield_test.go TestResponsesShieldMasksRequest

## Phase 3: Routes, wiring, usage

Goal: expose the endpoint and account usage.

- [x] T3.1 Add `RegisterResponsesShield` and mount `/responses` on both prefixes with the reduced chain (modelAccess → responsesShield → usage → budget → handler) (DEC-001, DEC-006, AC-001, AC-002, AC-008). Touches: src/internal/api/server.go
      Proof: code src/internal/api/server.go RegisterResponsesShield
- [x] T3.2 Wire the Responses shield in both binaries (AC-001). Touches: src/cmd/gateway/run.go, src/cmd/all/gateway.go
      Proof: code src/cmd/gateway/run.go RegisterResponsesShield
- [x] T3.3 Add route tests: both prefixes reach `/v1/responses`, model-access 403, budget 429, unsupported → 400 (AC-002, AC-008). Touches: src/internal/api/server_test.go
      Proof: test src/internal/api/server_test.go TestResponsesRouteParityAndAuth
- [x] T3.4 Add a usage test for the Responses `input_tokens`/`output_tokens` shape (DEC-005, AC-007). Touches: src/internal/api/middleware/usage_capture_test.go
      Proof: test src/internal/api/middleware/usage_capture_test.go TestExtractJSONUsageResponses

## Phase 4: Validation

Goal: prove the feature end to end and leave the package reviewable.

- [x] T4.1 Run the affected suites and `make test`; fix regressions (SC-001, SC-002). Touches: Makefile
      Proof: chore Makefile test

## Acceptance Coverage

- AC-001 -> T2.1, T3.1, T3.2
- AC-002 -> T3.1, T3.3
- AC-003 -> T2.1, T2.2
- AC-004 -> T1.1, T2.1, T2.2
- AC-005 -> T2.1, T2.2
- AC-006 -> T2.1, T2.2
- AC-007 -> T3.4
- AC-008 -> T1.2, T1.3, T3.1, T3.3

## Notes

- MVP is Phase 2; Phase 1 is its prerequisite, Phase 3 exposes and accounts.
- Streaming Responses events and tool-item masking are deliberate follow-ups (spec Open Questions), not part of this task list.
- Every `[x]` task needs a `Proof:` line on the next line; no proof means not done.
