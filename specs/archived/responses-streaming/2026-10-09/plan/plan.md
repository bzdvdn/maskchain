# Responses Streaming Events Plan

## Phase Contract

Inputs: `specs/active/responses-streaming/spec.md` and targeted repo context.
Outputs: `plan.md` (data model noted inline).
Stop if: the spec is too vague to plan safely.

## Goal

Accept `stream: true` on `/v1/responses` and relay the provider's Responses SSE events with the text unmasked. A dedicated Responses handler streams events, reconstructing the event name from the payload `type`, unmasking text fields with a carry buffer for split placeholders, and ending on `response.completed`/`response.failed`. The shield stops rejecting streaming and instead exposes the mask mapping to the streaming path.

## MVP Slice

- `stream: true` accepted; events relayed; delta/`done` text unmasked with a carry buffer. Must satisfy `AC-001`–`AC-005`, `AC-008`.

## First Validation Path

1. Unit-test the streaming unmasker (full, split, and trailing-partial placeholder).
2. Recording-mock stream test: provider emits masked deltas; client sees originals and valid frames.
3. `curl -N POST /v1/responses` with `stream:true`.

## Scope

- Remove the `STREAMING_UNSUPPORTED` rejection; store the mask mapping for the response path.
- A Responses handler that relays non-stream and streams the streaming case.
- Event-aware unmask with a carry buffer; valid Responses SSE framing; terminal event handling.
- Streaming usage accounting (nested `response.usage`).

## Performance Budget

- Streaming memory stays proportional to the current chunk plus the carry buffer (bounded by the longest placeholder).
- No buffering of the whole stream.

## Implementation Surfaces

- `src/internal/api/responses_handler.go` (new) — `ResponsesHandler.HandleResponses` (non-stream relay + streaming relay) and the `unmaskStream` carry-buffer helper.
- `src/internal/api/middleware/responses_shield.go` — drop the stream rejection; store the mapping in context; wrap the buffered writer only for non-stream.
- `src/internal/api/middleware/usage_capture.go` — parse nested `response.usage` in the SSE payload.
- `src/internal/api/server.go` — register and mount the Responses handler in the responses chain.
- `src/cmd/gateway/run.go`, `src/cmd/all/gateway.go` — construct/register the Responses handler.
- Tests: `responses_handler_test.go` (new), `responses_shield_test.go`, `usage_capture_test.go`, `server_test.go`.

## Bootstrapping Surfaces

- none — routing, streaming egress, usage middleware, and the shield mapping already exist.

## Architecture Impact

- Local: one new handler + shield behavior change + a small usage-parser addition.
- Cross-boundary: the responses chain's terminal handler changes from the chat handler to the Responses handler.
- Compatibility: non-streaming Responses is preserved (the shield still wraps the buffered unmask writer for it).

## Acceptance Approach

- AC-001 → streaming relay in `responses_handler.go`; surfaces `server.go`, `responses_handler.go`; proof: stream test returns `text/event-stream` without `STREAMING_UNSUPPORTED`.
- AC-002 → request masking unchanged in the shield; surfaces `responses_shield.go`; proof: mock received masked prompt.
- AC-003 → per-event unmask; surfaces `responses_handler.go`; proof: concatenated deltas contain originals.
- AC-004 → carry buffer; surfaces `responses_handler.go`; proof: split-placeholder unit + stream test.
- AC-005 → frame reconstruction + terminal event; surfaces `responses_handler.go`; proof: frame parsing test, no `[DONE]`.
- AC-006 → chain keeps model-access/budget before streaming; surfaces `server.go`; proof: 401/403/429 with zero calls.
- AC-007 → usage parser reads nested `response.usage`; surfaces `usage_capture.go`; proof: parser unit test.
- AC-008 → both prefixes; surfaces `server.go`; proof: parity test; unsupported before bytes.

## Data and Contracts

- Data model: no change — no entities, tables, or migrations.
- Contracts: `stream: true` now yields Responses SSE; a new context key carries the mask mapping to the response path; the responses chain's handler becomes `ResponsesHandler`.
- No change to the request/response shapes themselves.

## Implementation Strategy

- DEC-001 A dedicated `ResponsesHandler` for stream vs non-stream
  Why: the chat handler emits chat-style `data: ...`/`[DONE]`, the wrong schema for Responses. Not: branching inside the chat handler.
  Tradeoff: a second streaming relay to maintain.
  Affects: `responses_handler.go`, `server.go`, cmd wiring.
  Validation: AC-001/AC-005 tests.
- DEC-002 Shield stores the mapping and wraps the buffered writer only for non-stream
  Why: the buffered `dictUnmaskWriter` would hold the entire stream, defeating streaming. Not: buffering then flushing.
  Tradeoff: two unmask mechanisms (buffered for non-stream, per-event for stream).
  Affects: `responses_shield.go`.
  Validation: AC-002/AC-003 tests.
- DEC-003 Carry-buffer unmask for split placeholders
  Why: a delta can contain a partial placeholder; a held tail that is a proper prefix of a known placeholder is completed by the next chunk. Not: assuming whole placeholders per delta.
  Tradeoff: one placeholder of look-ahead latency; single carry (one active text stream).
  Affects: `responses_handler.go`.
  Validation: split-placeholder unit test.
- DEC-004 Reconstruct SSE frames from the payload (`event:` name from `type`)
  Why: the egress parser yields only `data:` payloads; every Responses payload carries `type`. Not: a new raw-SSE egress path.
  Tradeoff: comment/keepalive lines are dropped; a raw passthrough would need egress changes.
  Affects: `responses_handler.go`.
  Validation: AC-005 test.
- DEC-005 End on `response.completed`/`response.failed`; no `[DONE]`
  Why: Responses terminates with a terminal event, not chat's sentinel. Not: appending `[DONE]`.
  Tradeoff: clients relying on `[DONE]` are non-standard.
  Affects: `responses_handler.go`.
  Validation: AC-005 test.
- DEC-006 Extend the SSE usage parser for nested `response.usage`
  Why: Responses nests usage under the completed event's `response` object; the parser already handles a nested `message.usage`. Not: a parallel usage path.
  Tradeoff: one more nesting case in the parser.
  Affects: `usage_capture.go`.
  Validation: AC-007 unit test.
- DEC-007 Reuse the existing egress streaming client unchanged
  Why: it already streams `data:` payloads and cancels on disconnect; event names are reconstructable. Not: a Responses-specific egress parser.
  Tradeoff: no explicit `event:` line passthrough from upstream (we synthesize it).
  Affects: none (documented).
  Validation: stream tests.

## Incremental Delivery

### MVP (First Value)

- Shield accepts stream and stores the mapping; `ResponsesHandler` streams and unmasks with the carry buffer; frame reconstruction; terminal event.
- MVP readiness: `AC-001`–`AC-005`, `AC-008` pass.

### Iterative Expansion

- Auth/model-access/budget stream tests → `AC-006`.
- Nested usage parsing → `AC-007`.

## Sequencing Notes

- Shield change (accept stream + expose mapping) before the handler can depend on it.
- Handler before server/chain wiring.
- Usage-parser change is independent and can land in parallel.

## Risks

- Carry buffer across multiple output items could mis-order text.
  Mitigation: single carry for the MVP; document; revisit if multi-item text streams appear.
- Usage middleware buffers the SSE for parsing, which adds memory for long streams.
  Mitigation: bounded parser state (partial line + last usage), same as chat.
- Dropping upstream `event:` lines may surprise clients that read them.
  Mitigation: synthesize `event:` from `type`; covered by AC-005.
- Client disconnect must cancel the upstream stream.
  Mitigation: relay uses the request context; existing egress cancels on ctx done.

## Rollout and Compatibility

- Additive behavior: `stream:true` stops erroring and streams. Non-streaming unchanged. No migration/flag.
- Operational follow-up: `X-Provider` header and shield status headers still apply.

## Validation

- Automated: `go test ./src/internal/api/... ./src/internal/api/middleware/...` plus `make test`.
- New tests: carry-buffer unmask (full/split/trailing-partial), stream relay (frames, terminal event, no `[DONE]`), shield accepts stream, usage nested parse, route gate.
- Proves: `AC-001`–`AC-008`; `DEC-001`–`DEC-007`.

## Constitution Compliance

- no conflicts. Content Shield remains applied (masking + streaming unmask), tenant-scoped; native-only data plane; no new dependencies.
