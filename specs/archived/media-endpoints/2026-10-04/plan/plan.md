# Media Endpoints (Images & Audio) Plan

## Phase Contract

Inputs: `specs/active/media-endpoints/spec.md` and targeted repo context.
Outputs: `plan.md` (data model noted inline).
Stop if: the spec is too vague to plan safely.

## Goal

Add a byte-preserving passthrough path for the OpenAI-compatible media endpoints. A dedicated media handler reads the raw request body, extracts the model (JSON or multipart) for access/budget, forwards the untouched bytes and original `Content-Type` through the existing routing/fallback machinery, and writes the provider response (JSON or binary) with its status and `Content-Type`. No shield stage runs on these routes.

## MVP Slice

- `POST /v1/images/generations` (JSON) and `POST /v1/audio/speech` (JSON in, binary out) with auth, model access, and budget. Must satisfy `AC-001`, `AC-003`, `AC-005`, `AC-006`, `AC-010`.

## First Validation Path

1. Unit-test the model extractor for JSON and multipart bodies.
2. Recording-mock tests per endpoint asserting the upstream path and byte-identical body/`Content-Type`.
3. `curl POST /v1/images/generations` and `/v1/audio/speech` against a local mock.

## Scope

- Four routes on `/v1` and `/api/v1`: images/generations, images/edits, audio/speech, audio/transcriptions.
- A media passthrough handler and a media-aware model-access check.
- `Content-Type` propagation through the provider request.
- Reduced chain (no shield/session/conversation/cache/SSE/export); budget enforced.
- Untouched: text endpoints, routing/fallback, shield engine, DB schema.

## Performance Budget

- Passthrough adds one body buffer (already required to read for model extraction); no JSON re-encode for multipart.
- No new allocations proportional to response size beyond a single buffer copy when forwarding.

## Implementation Surfaces

- `src/internal/api/media_handler.go` (new) — `HandleMediaPassthrough` + `ExtractModel` (JSON/multipart).
- `src/internal/api/middleware/media_model_access.go` (new) — model access that also reads multipart.
- `src/internal/ports/provider.go` — add `ContentType` to `ProviderRequest`.
- `src/internal/adapters/provider/openai.go`, `proxy.go`, `ollama.go` — honor `req.ContentType` (fallback `application/json`).
- `src/internal/adapters/provider/upstream.go` — add the four paths to the OpenAI-shaped allowlist.
- `src/internal/api/server.go` — `RegisterMedia` + route mounting with the reduced chain.
- `src/cmd/gateway/run.go`, `src/cmd/all/gateway.go` — wire the media model-access middleware.
- Tests: `media_handler_test.go` (new), `media_model_access_test.go` (new), `upstream_test.go`, `server_test.go`.

## Bootstrapping Surfaces

- none — routing, provider clients, and middleware scaffolding already exist.

## Architecture Impact

- Local: one new handler + one new middleware; a small additive field on `ProviderRequest`.
- Cross-boundary: provider adapters gain a content-type override; the path allowlist grows.
- Compatibility: additive. No text endpoint behavior changes; no schema change.

## Acceptance Approach

- AC-001 → media handler forwards JSON to `/v1/images/generations`; surfaces `media_handler.go`; proof: recording mock shows path + unchanged body.
- AC-002 → multipart forwarded byte-for-byte; surfaces `media_handler.go`; proof: mock body/`Content-Type` equal the sent ones.
- AC-003 → binary response passthrough; surfaces `media_handler.go`, adapters; proof: response bytes + `Content-Type` equal the provider's.
- AC-004 → transcription multipart forwarded; surfaces `media_handler.go`; proof: mock body equality + JSON transcript returned.
- AC-005 → `ExtractModel` for JSON/multipart; surfaces `media_handler.go`, `media_model_access.go`; proof: 403 for a disallowed multipart model with zero calls.
- AC-006 → no shield in the media chain; surfaces `server.go`; proof: mock receives the original unterminated prompt.
- AC-007 → auth/model-access/budget on media; surfaces `server.go`; proof: 401/403/429 tests.
- AC-008 → both prefixes; surfaces `server.go`, `upstream.go`; proof: same path for both.
- AC-009 → allowlist rejects unsupported shapes; surfaces `upstream.go`, `media_handler.go`; proof: 400 `UNSUPPORTED_ENDPOINT`.
- AC-010 → no JSON envelope on binary; surfaces `media_handler.go`; proof: byte-equality test.

## Data and Contracts

- Data model: no change — no entities, tables, or migrations.
- Contracts: new routes; `ProviderRequest` gains an additive `ContentType` field; `RegisterMedia` on the server.
- Response contract: media responses bypass the JSON response envelope (`SkipEnvelopeKey`) like proxy responses.

## Implementation Strategy

- DEC-001 Dedicated media handler, not the chat handler
  Why: `HandleChatCompletion` JSON-parses the body and writes `application/json`, which corrupts multipart and binary responses. Not: overloading the chat handler with branches.
  Tradeoff: a second proxy entry path to maintain.
  Affects: `media_handler.go`, `server.go`.
  Validation: per-endpoint tests.
- DEC-002 Read the raw body once; parse only for the model; forward the original bytes
  Why: `ParseMultipartForm` + re-marshal changes the boundary/ordering and breaks the provider request (AC-002/AC-004). Not: rebuilding multipart.
  Tradeoff: the whole body is buffered (already needed for model extraction); size-bounded by the server limit.
  Affects: `media_handler.go`, `media_model_access.go`.
  Validation: byte-equality tests.
- DEC-003 `ProviderRequest.ContentType` override
  Why: multipart needs the original `Content-Type` (with boundary); adapters currently hardcode `application/json`. Not: a separate media provider client.
  Tradeoff: one more field every adapter must consider.
  Affects: `ports/provider.go`, three adapters.
  Validation: recording-mock content-type assertion.
- DEC-004 Media-aware model access
  Why: the existing `ModelAccess` rejects non-JSON bodies (400), which would block multipart media. Not: skipping access checks (would bypass key scopes).
  Tradeoff: a second access middleware.
  Affects: `media_model_access.go`, `server.go`, wiring.
  Validation: 403 multipart test.
- DEC-005 Reduced chain without shield/export
  Why: the spec deliberately does not modify media content; session/conversation/cache/SSE/export are chat-specific. Not: reusing the full chat chain.
  Tradeoff: media traffic is not shielded or conversation-logged (operator-accepted).
  Affects: `server.go`.
  Validation: chain test asserting shield stage absent.
- DEC-006 Binary response passthrough via `c.Data` with provider status/`Content-Type`
  Why: JSON envelope or SSE wrapping corrupts binary audio (AC-010). Not: `c.JSON`.
  Tradeoff: none material.
  Affects: `media_handler.go`.
  Validation: byte-equality + header test.
- DEC-007 Buffer responses (no client streaming) in the MVP
  Why: simplest correct passthrough; audio clips are small and bounded. Not: streaming/proxying the upstream body.
  Tradeoff: memory use for large audio; a future streaming refinement.
  Affects: `media_handler.go`.
  Validation: functional test.
- DEC-008 Extend the OpenAI-shaped path allowlist
  Why: path fidelity is the single source of supported shapes and yields `UNSUPPORTED_ENDPOINT` for the rest (AC-009). Not: bypassing the allowlist.
  Tradeoff: providers lacking an endpoint return an explicit error (intended).
  Affects: `upstream.go`.
  Validation: allowlist tests.

## Incremental Delivery

### MVP (First Value)

- Allowlist + `ContentType` field/adapters + media handler (JSON path) + `images/generations` and `audio/speech` routes + budget.
- MVP readiness: `AC-001`, `AC-003`, `AC-005`, `AC-006`, `AC-010` pass.

### Iterative Expansion

- Multipart extraction + `images/edits`, `audio/transcriptions` → `AC-002`, `AC-004`.
- Auth/model-access/budget/unsupported/prefix tests → `AC-007`, `AC-008`, `AC-009`.

## Sequencing Notes

- `ContentType` field + adapter honor first (handler depends on it).
- Model extractor before the handler (used by access + routing).
- Media model access before route wiring; both binaries wired together.

## Risks

- Accidental body re-encoding of multipart.
  Mitigation: forward raw bytes; byte-equality tests (AC-002/AC-004).
- Binary corrupted by the response envelope.
  Mitigation: `SkipEnvelopeKey` + `c.Data`; byte-equality test (AC-010).
- Media access bypass via multipart.
  Mitigation: media-aware access extracts `model` from the form before dispatch (AC-005).
- Large bodies.
  Mitigation: bounded by the server body limit; documented; streaming deferred.
- Missing wiring in one binary.
  Mitigation: identical wiring in `run.go` and `all/gateway.go`; smoke both.

## Rollout and Compatibility

- Additive routes; no migration or flag. Text endpoints unaffected. Shield is intentionally not applied to media (spec Goal).
- Operational follow-up: media requests are visible via metrics and `X-Provider`; usage is recorded only for JSON responses that report it.

## Validation

- Automated: `go test ./src/internal/api/... ./src/internal/adapters/provider/...` plus `make test`.
- New tests: model extractor (JSON/multipart), media handler per endpoint, media model access, allowlist, chain.
- Manual: local OpenAI-compatible mock for images/generations and audio/speech.
- Proves: `AC-001`–`AC-010`; `DEC-001`–`DEC-008`.

## Constitution Compliance

- conflict recorded: the constitution states Content Shield is core and not opt-in; this feature deliberately does not shield media content because masking would alter generated output (spec Goal, user decision). Resolved by explicit scope: media relies on auth, model access, and budgets; revisit if a block-only media policy is desired.
