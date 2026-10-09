# Responses Streaming Events Tasks

## Phase Contract

Inputs: `specs/active/responses-streaming/plan.md`, `spec.md`.
Outputs: ordered executable tasks with coverage mapping.
Stop if: tasks would be vague or acceptance coverage cannot be mapped.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/api/middleware/responses_shield.go | T1.1 |
| src/internal/api/middleware/responses_shield_test.go | T1.2 |
| src/internal/api/responses_handler.go | T2.1 |
| src/internal/api/responses_handler_test.go | T2.2 |
| src/internal/api/server.go | T3.1 |
| src/cmd/gateway/run.go | T3.2 |
| src/cmd/all/gateway.go | T3.2 |
| src/internal/api/server_test.go | T3.3 |
| src/internal/api/middleware/usage_capture.go | T3.4 |
| src/internal/api/middleware/usage_capture_test.go | T3.4 |
| Makefile | T4.1 |

## Implementation Context

- MVP goal: accept `stream:true` on `/v1/responses` and relay provider Responses SSE events with text unmasked and split placeholders reassembled (AC-001…AC-005, AC-008).
- Invariants/semantics: the shield stores the placeholder→original mapping in the gin context and wraps the buffered unmask writer only for non-stream requests; the streaming relay turns each egress data payload into `event: <type>\n` + `data: <json>\n\n`; it unmasks the text of `response.output_text.delta` (`delta`) and `response.output_text.done` (`text`) and content-part text; a carry buffer holds a trailing fragment that is a proper prefix of a known placeholder; the stream ends on `response.completed`/`response.failed` (no `[DONE]`).
- Errors/codes: `NO_ROUTE` 400; `MODEL_ACCESS_DENIED` 403; `BUDGET_EXCEEDED` 429; `UNSUPPORTED_ENDPOINT` 400 before any bytes stream.
- Contracts/protocols: `POST /v1/responses` `stream:true` → `text/event-stream` Responses frames; a context key carries the mask mapping from shield to handler.
- Scope boundaries: do not unmask tool-call arguments or multimodal content; do not add background mode, stored responses, WebRTC, or Chat↔Responses translation.
- Proof signals: carry-buffer unit tests (full/split/trailing-partial); stream relay test (frames, event names, no `[DONE]`, terminal event); shield accepts stream and exposes the mapping; nested `response.usage` parse test; route/gate tests.
- References: DEC-001..DEC-007, RQ-001..RQ-009 (no mandatory re-read).

## Phase 1: Foundation

Goal: the shield accepts streaming and hands the mapping to the handler.

- [x] T1.1 Remove the `STREAMING_UNSUPPORTED` rejection; store the mask mapping in the gin context and wrap the buffered unmask writer only for non-stream requests (DEC-002, AC-002). Touches: src/internal/api/middleware/responses_shield.go
      Proof: code src/internal/api/middleware/responses_shield.go ResponsesShieldMiddleware
- [x] T1.2 Update the shield tests: `stream:true` is accepted, the mapping is exposed to the downstream handler, and non-stream unmasking still works (AC-002, AC-003). Touches: src/internal/api/middleware/responses_shield_test.go
      Proof: test src/internal/api/middleware/responses_shield_test.go TestResponsesShieldAcceptsStreamAndExposesMapping

## Phase 2: Streaming handler (MVP)

Goal: relay and unmask the Responses event stream.

- [x] T2.1 Implement `ResponsesHandler` (non-stream relay reused from the model-based flow, plus a streaming relay) and the `unmaskStream` carry-buffer helper; reconstruct `event:`/`data:` frames from the payload `type`, end on `response.completed`/`response.failed`, and flush the carry at stream end (DEC-001, DEC-003, DEC-004, DEC-005, DEC-007, AC-001, AC-003, AC-004, AC-005). Touches: src/internal/api/responses_handler.go
      Proof: code src/internal/api/responses_handler.go HandleResponses
- [x] T2.2 Add tests: carry-buffer unmask (full/split/trailing-partial), streamed relay produces valid frames with no `[DONE]`, and terminal-event handling (AC-001, AC-003, AC-004, AC-005). Touches: src/internal/api/responses_handler_test.go
      Proof: test src/internal/api/responses_handler_test.go TestUnmaskStreamerCarry

## Phase 3: Routes, wiring, usage

Goal: mount the handler, gate the stream, and account usage.

- [x] T3.1 Register the Responses handler and use it as the responses chain terminal handler on both prefixes (DEC-001, AC-001, AC-006, AC-008). Touches: src/internal/api/server.go
      Proof: code src/internal/api/server.go RegisterResponsesHandler
- [x] T3.2 Construct/register the Responses handler in both binaries (AC-001). Touches: src/cmd/gateway/run.go, src/cmd/all/gateway.go
      Proof: code src/cmd/gateway/run.go RegisterResponsesHandler
- [x] T3.3 Add route/gate tests: both prefixes stream; model-access 403 / budget 429 before the stream; unsupported → 400 (AC-006, AC-008). Touches: src/internal/api/server_test.go
      Proof: test src/internal/api/server_test.go TestResponsesStreamRouteParity
- [x] T3.4 Parse nested `response.usage` in the SSE usage parser and add a test (DEC-006, AC-007). Touches: src/internal/api/middleware/usage_capture.go, src/internal/api/middleware/usage_capture_test.go
      Proof: code src/internal/api/middleware/usage_capture.go parsePayload

## Phase 4: Validation

Goal: prove the feature end to end and leave the package reviewable.

- [x] T4.1 Run the affected suites and `make test`; fix regressions (SC-001, SC-002). Touches: Makefile
      Proof: chore Makefile test

## Acceptance Coverage

- AC-001 -> T2.1, T2.2, T3.1, T3.2
- AC-002 -> T1.1, T1.2
- AC-003 -> T1.2, T2.1, T2.2
- AC-004 -> T2.1, T2.2
- AC-005 -> T2.1, T2.2
- AC-006 -> T3.1, T3.3
- AC-007 -> T3.4
- AC-008 -> T3.1, T3.3

## Notes

- MVP is Phase 2; Phase 1 is its prerequisite, Phase 3 exposes and accounts.
- Single carry buffer (one active text stream) is the MVP; multi-output-item nuance is documented in the plan.
- Every `[x]` task needs a `Proof:` line on the next line; no proof means not done.
