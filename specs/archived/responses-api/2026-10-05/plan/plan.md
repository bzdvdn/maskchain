# Responses API Support Plan

## Phase Contract

Inputs: `specs/active/responses-api/spec.md` and targeted repo context.
Outputs: `plan.md` (data model noted inline).
Stop if: the spec is too vague to plan safely.

## Goal

Add `/v1/responses` (non-streaming) as a proxied endpoint that reuses the model-based routing handler and the chat unmask machinery. A Responses-specific shield extracts `instructions` and `input` text, masks it reversibly, and installs the same placeholder→original response writer used by chat to restore `output_text`/`output[].content[].text`. Streaming is rejected explicitly in this MVP.

## MVP Slice

- `POST /v1/responses` (non-stream) on both prefixes: routing + auth + model access + budget, request masking, response unmasking, usage accounting. Must satisfy `AC-001`–`AC-005`, `AC-007`, `AC-008`.

## First Validation Path

1. Unit-test the Responses extractor (string `input`, message items, `input_text` blocks, `instructions`).
2. Recording-mock test: provider receives masked text; client gets the original back.
3. `curl POST /v1/responses` against a local mock; `stream:true` returns `STREAMING_UNSUPPORTED`.

## Scope

- `/v1/responses` and `/api/v1/responses` (non-streaming).
- Reversible masking of `instructions` + `input`; unmask of `output_text` + `output[].content[].text`.
- Passthrough of all other fields; usage accounting via the existing middleware.
- Untouched: chat/embeddings/media, routing/fallback, DB schema.

## Performance Budget

- One body read/rewrite for the shield (same as chat); unmask is a map replace over the response body.
- No new per-request allocation beyond the existing shield path.

## Implementation Surfaces

- `src/internal/api/middleware/responses_shield.go` (new) — extractor, mask/rewrite, stream rejection, unmask writer wiring.
- `src/internal/api/middleware/shield_text.go` — `maskTexts` also returns the placeholder→original mapping (reused by chat-style unmask).
- `src/internal/api/middleware/shield_embeddings.go` — adapt the existing caller to the new return.
- `src/internal/api/server.go` — `RegisterResponsesShield` + mount `/responses` on both prefixes.
- `src/internal/adapters/provider/upstream.go` — add `/v1/responses` to the allowlist.
- `src/cmd/gateway/run.go`, `src/cmd/all/gateway.go` — wire the Responses shield.
- Tests: `responses_shield_test.go` (new), `server_test.go`, `upstream_test.go`, `usage_capture_test.go`.

## Bootstrapping Surfaces

- none — the shield engine, mask mapping, and response writer already exist.

## Architecture Impact

- Local: one new middleware + one shared-core signature change.
- Cross-boundary: the allowlist grows; the proxy handler is reused unchanged.
- Compatibility: additive; no behavior change for existing endpoints.

## Acceptance Approach

- AC-001 → reuse `RoutingProxyHandler` for non-stream; surfaces `server.go`, `responses_shield.go`; proof: mock shows `/v1/responses` and the response body.
- AC-002 → chain keeps model-access + budget; surfaces `server.go`; proof: 403/429 with zero calls.
- AC-003 → extractor masks `instructions`/`input`; surfaces `responses_shield.go`; proof: mock receives placeholders.
- AC-004 → unmask writer restores originals; surfaces `responses_shield.go`, `shield_text.go`; proof: client body contains originals.
- AC-005 → body rewrite preserves other fields; surfaces `responses_shield.go`; proof: mock body keeps `previous_response_id`/`store`/`tools`.
- AC-006 → `stream:true` rejected before dispatch; surfaces `responses_shield.go`; proof: `STREAMING_UNSUPPORTED`, zero calls.
- AC-007 → existing usage middleware reads `input_tokens`/`output_tokens`; surfaces `usage_capture.go`; proof: usage record test.
- AC-008 → both prefixes + allowlist; surfaces `server.go`, `upstream.go`; proof: same path for both; `UNSUPPORTED_ENDPOINT` otherwise.

## Data and Contracts

- Data model: no change — no entities, tables, or migrations.
- Contracts: new routes `/v1/responses`, `/api/v1/responses`; `maskTexts` gains a mapping return (internal).
- Reuse: `wrapUnmaskWriter` and the placeholder→original map from the chat path.

## Implementation Strategy

- DEC-001 Reuse the model-based proxy handler for non-streaming Responses
  Why: routing/fallback/path fidelity/model extraction already work; Responses carries `model`. Not: a dedicated Responses handler.
  Tradeoff: streaming cannot use the chat SSE path, so it is rejected (DEC-007).
  Affects: `server.go`, `responses_shield.go`.
  Validation: AC-001 route test.
- DEC-002 Extend `maskTexts` to return the mapping; reuse the chat unmask writer
  Why: one masking core, one unmask mechanism; avoids a third copy of dictionary+PII logic. Not: duplicating the core for Responses.
  Tradeoff: signature change touches the embeddings and text-shield callers (mechanical).
  Affects: `shield_text.go`, `shield_embeddings.go`, `responses_shield.go`.
  Validation: existing shield tests stay green; AC-004 test.
- DEC-003 Responses extractor handles `instructions` and `input` string/message/content blocks
  Why: the text lives in different shapes than chat `messages`. Not: reusing `chatMessage`.
  Tradeoff: tool items (`function_call_output`) are not masked in the MVP (documented).
  Affects: `responses_shield.go`.
  Validation: extractor unit tests.
- DEC-004 Add `/v1/responses` to the OpenAI-shaped allowlist
  Why: single source of supported shapes; yields `UNSUPPORTED_ENDPOINT` for others. Not: bypassing the allowlist.
  Tradeoff: providers without Responses return an explicit error.
  Affects: `upstream.go`.
  Validation: allowlist test.
- DEC-005 Reuse the existing usage middleware (no code change expected)
  Why: `extractJSONUsage` already reads `input_tokens`/`output_tokens`. Not: a Responses-specific usage path.
  Tradeoff: none; verified by a test.
  Affects: `usage_capture_test.go` (test only).
  Validation: AC-007 test.
- DEC-006 Reduced chain without session/conversation/cache/SSE
  Why: those stages are chat-shaped; the MVP proxies Responses statelessly. Not: the full chat chain.
  Tradeoff: no conversation logging/session for Responses (follow-up).
  Affects: `server.go`.
  Validation: chain test.
- DEC-007 Reject `stream:true` with `STREAMING_UNSUPPORTED` in the shield
  Why: the Responses SSE schema differs from chat and is deferred. Not: emitting chat SSE.
  Tradeoff: streamed Responses clients must wait for the follow-up feature.
  Affects: `responses_shield.go`.
  Validation: AC-006 test.

## Incremental Delivery

### MVP (First Value)

- Allowlist + `maskTexts` mapping + Responses shield (extract/mask/rewrite/unmask, reject stream) + route mounting.
- MVP readiness: `AC-001`–`AC-006`, `AC-008` pass.

### Iterative Expansion

- Usage test → `AC-007`.
- Streaming events and tool-item masking are follow-up specs (Open Questions).

## Sequencing Notes

- `maskTexts` mapping change + embeddings caller first (compile prerequisite).
- Shield extractor before route mounting.
- Wire both binaries together.

## Risks

- Global placeholder replace could alter unrelated response text.
  Mitigation: placeholders are unique tokens (`[MASK.N]`/`[[pii.N]]`); same mechanism as chat.
- `input` shape variance across clients.
  Mitigation: extractor handles string, message items, and `input_text`/`text` blocks; unknown shapes are forwarded unmasked-but-unchanged rather than corrupted.
- Streaming clients get an error instead of a partial behavior.
  Mitigation: explicit `STREAMING_UNSUPPORTED` and a follow-up spec.
- Usage shape differences.
  Mitigation: verified by test; both `prompt/completion` and `input/output` shapes handled.

## Rollout and Compatibility

- Additive route; no migration or flag. Streaming Responses clients are explicitly unsupported in the MVP.
- Operational follow-up: `X-Provider` and shield headers apply; usage recorded when the provider reports it.

## Validation

- Automated: `go test ./src/internal/api/... ./src/internal/adapters/provider/...` plus `make test`.
- New tests: extractor, request masking, response unmasking, stream rejection, route/prefix, usage mapping.
- Proves: `AC-001`–`AC-008`; `DEC-001`–`DEC-007`.

## Constitution Compliance

- no conflicts. Content Shield is applied to Responses text (masking + unmasking), tenant-scoped; native-only data plane; no new dependencies.
