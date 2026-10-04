# Media Endpoints (Images & Audio) Tasks

## Phase Contract

Inputs: `specs/active/media-endpoints/plan.md`, `spec.md`.
Outputs: ordered executable tasks with coverage mapping.
Stop if: tasks would be vague or acceptance coverage cannot be mapped.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/adapters/provider/upstream.go | T1.1 |
| src/internal/adapters/provider/upstream_test.go | T1.3 |
| src/internal/ports/provider.go | T1.2 |
| src/internal/adapters/provider/openai.go | T1.2 |
| src/internal/adapters/provider/proxy.go | T1.2 |
| src/internal/adapters/provider/ollama.go | T1.2 |
| src/internal/adapters/provider/provider_test.go | T1.3 |
| src/internal/api/media_handler.go | T2.1, T3.2 |
| src/internal/api/media_handler_test.go | T2.2, T3.2 |
| src/internal/api/middleware/media_model_access.go | T2.3 |
| src/internal/api/middleware/media_model_access_test.go | T2.4 |
| src/internal/api/server.go | T3.1 |
| src/internal/api/server_test.go | T3.4 |
| src/cmd/gateway/run.go | T3.3 |
| src/cmd/all/gateway.go | T3.3 |
| Makefile | T4.1 |

## Implementation Context

- MVP goal: `POST /v1/images/generations` (JSON) and `POST /v1/audio/speech` (JSON in, binary out) proxied with auth, model access, and budget (AC-001, AC-003, AC-005, AC-006, AC-010).
- Invariants/semantics: the request body is read once and forwarded byte-identical; the original `Content-Type` (multipart boundary) is preserved; the model is extracted from a JSON `model` field or the multipart `model` form field; NO shield/masking runs on media; reduced chain = auth → mediaModelAccess → budget → handler; binary responses use `c.Data` with the provider status/`Content-Type` and skip the JSON envelope.
- Errors/codes: missing model → 400 `VALIDATION_ERROR`; disallowed model → 403 `MODEL_ACCESS_DENIED`; budget → 429 `BUDGET_EXCEEDED`; unsupported shape → 400 `UNSUPPORTED_ENDPOINT`; no route → `NO_ROUTE`.
- Contracts/protocols: routes on `/v1` and `/api/v1`: `/images/generations`, `/images/edits`, `/audio/speech`, `/audio/transcriptions`; `ports.ProviderRequest` gains additive `ContentType`.
- Scope boundaries: do not add shielding/masking to media; do not add variations/translations/responses/batches/files/fine-tuning; no media format conversion.
- Proof signals: extractor tests (JSON/multipart); byte-equality of multipart body and binary response; 403 for a disallowed multipart model with zero calls; route/prefix/unsupported/budget tests.
- References: DEC-001..DEC-008, RQ-001..RQ-010 (no mandatory re-read).

## Phase 1: Foundation

Goal: add the media paths to the allowlist and let adapters forward the original Content-Type.

- [x] T1.1 Add `/v1/images/generations`, `/v1/images/edits`, `/v1/audio/speech`, `/v1/audio/transcriptions` to the OpenAI-shaped allowlist (DEC-008, AC-008, AC-009). Touches: src/internal/adapters/provider/upstream.go
      Proof: code src/internal/adapters/provider/upstream.go openAIShapedEndpoints
- [x] T1.2 Add `ContentType` to `ProviderRequest` and make the OpenAI-shaped adapters use it (fallback `application/json`) (DEC-003, AC-003, AC-002). Touches: src/internal/ports/provider.go, src/internal/adapters/provider/openai.go, src/internal/adapters/provider/proxy.go, src/internal/adapters/provider/ollama.go
      Proof: code src/internal/ports/provider.go ContentType
- [x] T1.3 Prove allowlist acceptance/rejection and content-type propagation (AC-003, AC-009). Touches: src/internal/adapters/provider/upstream_test.go, src/internal/adapters/provider/provider_test.go
      Proof: test src/internal/adapters/provider/provider_test.go TestPassthroughContentType

## Phase 2: Media handler and access (MVP)

Goal: forward JSON media requests and binary responses, and check model access for JSON and multipart.

- [x] T2.1 Implement `ExtractModel` (JSON `model` / multipart `model` field) and `HandleMediaPassthrough` (raw body forward, provider status/`Content-Type` response, binary via `c.Data`) (DEC-001, DEC-002, DEC-006, AC-001, AC-003, AC-005, AC-010). Touches: src/internal/api/media_handler.go
      Proof: code src/internal/api/media_handler.go HandleMedia
- [x] T2.2 Add media handler tests: JSON generations passthrough, binary speech byte-equality + `Content-Type`, and prompt-unmodified (no shield) (AC-001, AC-003, AC-006, AC-010). Touches: src/internal/api/media_handler_test.go
      Proof: test src/internal/api/media_handler_test.go TestMediaHandlerImageGeneration
- [x] T2.3 Implement a media-aware model-access middleware that reads the model from JSON or multipart without altering the body (DEC-004, AC-005). Touches: src/internal/api/middleware/media_model_access.go
      Proof: code src/internal/api/middleware/media_model_access.go MediaModelAccess
- [x] T2.4 Add media model-access tests: allowed, disallowed multipart, missing model (AC-005). Touches: src/internal/api/middleware/media_model_access_test.go
      Proof: test src/internal/api/middleware/media_model_access_test.go TestMediaModelAccessMultipart

## Phase 3: Routes, multipart, wiring

Goal: mount all four endpoints and forward multipart byte-for-byte.

- [x] T3.1 Add `RegisterMedia` and mount the four routes with the reduced chain (auth → mediaModelAccess → budget → handler), no shield/export/SSE (DEC-005, AC-006, AC-007, AC-008). Touches: src/internal/api/server.go
      Proof: code src/internal/api/server.go RegisterMedia
- [x] T3.2 Add multipart forwarding for `images/edits` and `audio/transcriptions` with byte-equality tests (DEC-002, AC-002, AC-004). Touches: src/internal/api/media_handler.go, src/internal/api/media_handler_test.go
      Proof: test src/internal/api/media_handler_test.go TestMediaHandlerMultipartEdits
- [x] T3.3 Wire the media model-access middleware in both binaries (AC-007). Touches: src/cmd/gateway/run.go, src/cmd/all/gateway.go
      Proof: code src/cmd/gateway/run.go RegisterMedia
- [x] T3.4 Add route tests: both prefixes reach the same path, unsupported shape → 400, auth 401 / budget 429 (AC-007, AC-008, AC-009). Touches: src/internal/api/server_test.go
      Proof: test src/internal/api/server_test.go TestMediaRoutesParityAndAuth

## Phase 4: Validation

Goal: prove the feature end to end and leave the package reviewable.

- [x] T4.1 Run the affected suites and `make test`; fix regressions (SC-001, SC-002). Touches: Makefile
      Proof: chore Makefile test

## Acceptance Coverage

- AC-001 -> T2.1, T2.2, T3.1
- AC-002 -> T1.2, T3.2
- AC-003 -> T1.2, T1.3, T2.1, T2.2
- AC-004 -> T3.2
- AC-005 -> T2.1, T2.3, T2.4
- AC-006 -> T2.2, T3.1
- AC-007 -> T2.3, T3.1, T3.3, T3.4
- AC-008 -> T1.1, T3.1, T3.4
- AC-009 -> T1.1, T1.3, T3.4
- AC-010 -> T2.1, T2.2

## Notes

- MVP is Phase 2; Phase 1 is its prerequisite and Phase 3 completes the surface.
- Never re-encode a multipart body: parse a copy only for the model, forward the original bytes.
- Media responses bypass the JSON envelope; write provider status + `Content-Type` + body.
- Every `[x]` task needs a `Proof:` line on the next line; no proof means not done.
