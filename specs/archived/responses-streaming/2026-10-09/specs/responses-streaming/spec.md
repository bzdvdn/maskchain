# Responses Streaming Events

## Scope Snapshot

- In scope: support `stream: true` on `POST /v1/responses` by relaying the provider's Responses SSE events while unmasking the streamed text with a carry buffer for split placeholders.
- Out of scope: background mode, `GET /v1/responses/{id}`, tool-argument unmasking, WebRTC realtime, and cross-format translation.

## Goal

The non-streaming Responses endpoint already exists but rejects `stream: true`. Streaming clients (agents, chat UIs) expect OpenAI Responses events (`response.output_text.delta`, `response.output_text.done`, `response.completed`, ...) with the original text restored. This feature forwards the provider's event stream, unmasks each event's text using the request's mask mapping, and handles placeholders that arrive split across chunks, so a streaming Responses client sees the same values as a non-streaming one.

## Primary User Flow

1. Starting point: a client sends `POST /v1/responses` with `stream: true`, `model`, `input`, and optional `instructions`.
2. Main interaction: the gateway authenticates, checks model access and budget, masks the request text, opens the provider stream, and re-emits each Responses event with its text unmasked.
3. Outcome: the client receives a valid Responses event stream ending with `response.completed` (or `response.failed`), with placeholders replaced by the originals.
4. Failure/fallback path: an unrouted model gives `NO_ROUTE`; a provider that cannot serve `/v1/responses` gives `UNSUPPORTED_ENDPOINT` before any bytes are streamed.

## User Stories

- P1 Story: as a streaming Responses client, I get incremental `response.output_text.delta` events with originals restored.
- P2 Story: as an operator, the same shield applies to streamed Responses traffic and budgets still gate it.
- P3 Story: as a client, I receive well-formed Responses SSE frames (event name + data), not chat-style `data: ...`/`[DONE]`.

## MVP Slice

- `stream: true` is accepted; events are relayed and text is unmasked per event with a carry buffer. Must satisfy `AC-001`–`AC-005`, `AC-008`.

## First Deployable Outcome

- A streamed `/v1/responses` call yields `response.output_text.delta` events whose text contains the original values.
- A recording mock shows the provider received masked `instructions`/`input` and streamed placeholders.

## Scope

- Accept `stream: true` for `/v1/responses` on `/v1` and `/api/v1` instead of rejecting it.
- Relay Responses SSE events, preserving the event framing clients expect.
- Event-aware unmasking of `response.output_text.delta`/`.done` and content-part text, with a carry buffer across chunks.
- Auth, model-access, budget, and streaming usage accounting.

## Context

- The non-streaming MVP stores `STREAMING_UNSUPPORTED`; the shield already builds the placeholder→original mapping and exposes an unmask writer for non-streaming responses.
- The egress SSE parser (`src/internal/adapters/egress/stream.go`) yields only `data:` payloads (stripping the `data: ` prefix and the `event:` line); each Responses payload still contains a `type` field, so the event name can be reconstructed. It also treats `data: [DONE]` as terminal, which Responses does not send.
- The chat streaming path (`streamFromProvider`) emits chat-style `data: ...` + `[DONE]`, which is the wrong schema for Responses.
- Responses streaming usage is nested (`response.completed` → `response.usage.{input_tokens,output_tokens}`), not a top-level `usage` object.
- `maskTexts` already returns the placeholder→original mapping.

## Dependencies

- The non-streaming responses feature (shipped): `ResponsesShieldMiddleware`, route mounting, path allowlist, `maskTexts` mapping.
- Routing/fallback/streaming egress and the budget/usage middleware.

## Requirements

- RQ-001 `stream: true` MUST be accepted and proxied as a Responses SSE stream (the `STREAMING_UNSUPPORTED` rejection is removed).
- RQ-002 Request masking MUST be identical to the non-streaming path (`instructions` + `input`), and the resulting mapping MUST be available on the response path.
- RQ-003 Each event's text (`response.output_text.delta.delta`, `response.output_text.done.text`, and content-part `text`) MUST be unmasked to the original values.
- RQ-004 A placeholder split across two or more chunks MUST be reassembled via a carry buffer before unmasking; no partial placeholder may leak to the client.
- RQ-005 Emitted frames MUST be valid Responses SSE: the event name is preserved (from the payload `type`) and the data payload is re-serialized; the stream MUST end on `response.completed`/`response.failed` (no chat `[DONE]`).
- RQ-006 Non-JSON / keepalive lines that carry no unmaskable text MUST be relayed or safely dropped without corrupting the stream.
- RQ-007 Virtual-key auth, model access, and budget MUST apply before the stream opens; usage MUST be accounted when the stream reports it.
- RQ-008 The endpoint MUST be mounted on `/v1` and `/api/v1`; an unsupported provider shape MUST return `UNSUPPORTED_ENDPOINT` before streaming.
- RQ-009 The non-streaming Responses behavior MUST be unchanged.

## Non-Goals

- Background mode, stored responses, cancellation, WebRTC/realtime.
- Unmasking tool-call arguments or binary/multimodal content.
- Chat Completions ↔ Responses translation.

## Acceptance Criteria

### AC-001 Streaming is accepted and relayed

- Why this matters: streaming clients must work.
- **Given** a route to an OpenAI-shaped provider for model `m`
- **When** a client sends `POST /v1/responses` with `stream: true`
- **Then** the gateway streams the provider's Responses events and does not return `STREAMING_UNSUPPORTED`.
- Evidence: the response is `text/event-stream`; the first events are `response.created`/`response.in_progress`.

### AC-002 Request text is masked for the stream

- Why this matters: DLP applies to streaming too.
- **Given** a tenant whose policy masks emails
- **When** the client streams a request with an email in `input`/`instructions`
- **Then** the provider receives the masked prompt.
- Evidence: the recording mock's received body contains a placeholder, not the email.

### AC-003 Delta text is unmasked

- Why this matters: the client must see original values incrementally.
- **Given** masked text and a provider stream that emits a placeholder in `response.output_text.delta`
- **When** the client reads the deltas
- **Then** the concatenated delta text contains the original values.
- Evidence: collecting `delta` fields yields the email; no placeholder remains.

### AC-004 Split placeholders are reassembled

- Why this matters: deltas can split a placeholder.
- **Given** the provider emits `[MASK.` in one delta and `0]` in the next
- **When** the gateway relays the stream
- **Then** the client never sees the partial placeholder and sees the original value once.
- Evidence: no emitted delta contains a partial placeholder; the original appears exactly once.

### AC-005 Frames are valid Responses SSE

- Why this matters: clients parse event names, not chat deltas.
- **Given** a streamed call
- **When** the client parses the SSE frames
- **Then** each frame has the provider's event name and a JSON data payload, and the stream ends with `response.completed` (no `[DONE]`).
- Evidence: frame parsing yields event names; no `[DONE]` frame.

### AC-006 Auth, model access, budget gate the stream

- Why this matters: streaming must not bypass guardrails.
- **Given** a missing/invalid key, a disallowed model, or an over-budget tenant
- **When** `stream: true` is requested
- **Then** the gateway returns 401/403/429 before opening the stream, with no provider call.
- Evidence: statuses; the mock records zero calls.

### AC-007 Streaming usage is accounted

- Why this matters: spend tracking must cover streamed Responses.
- **Given** a stream that ends with `response.completed` carrying `response.usage.{input_tokens,output_tokens}`
- **When** the stream finishes
- **Then** a usage record with the mapped token counts is emitted.
- Evidence: an emitted usage record with the expected counts.

### AC-008 Both prefixes and unsupported shapes

- Why this matters: `/v1` and `/api/v1` clients both exist; wrong shapes must fail loudly.
- **Given** a supported request
- **When** it is streamed to `/api/v1/responses` and `/v1/responses`
- **Then** both relay the stream; a provider that cannot serve it returns `UNSUPPORTED_ENDPOINT` with no bytes streamed.
- Evidence: same behavior for both prefixes; explicit error otherwise.

## Assumptions

- Responses providers stream the OpenAI Responses event schema.
- The event `type` is present in each data payload, so the event name can be reconstructed when the upstream `event:` line is not preserved.
- The mask mapping from the request is available while relaying the stream.
- Only text fields are unmasked; structured/tool fields pass through.

## Success Criteria

- SC-001 Streaming adds no unbounded buffering: memory stays proportional to the chunk size plus the carry buffer.
- SC-002 Non-streaming Responses and all existing suites are unchanged.

## Edge Cases

- Placeholder split across the final two chunks: the carry buffer is flushed at stream end.
- Provider stream error mid-flight: the error is surfaced and the stream closes without corrupting the carry buffer.
- Empty deltas / keepalive comments: relayed or dropped without unmask.
- No maskable text in the request: events pass through unmodified.
- Client disconnects: the upstream stream is cancelled and resources released.

## Open Questions

- Emit explicit `event: <type>` lines in addition to the data payload, or data-only frames? (Clients differ.)
- Should streaming usage be captured from the nested `response.completed.response.usage` (requires extending the SSE usage parser)?
- Are `response.output_item.done` / tool-call events in scope for unmasking, or only text deltas?
- Streaming timeout/keepalive policy for long Responses streams.
