# Media Endpoints (Images & Audio)

## Scope Snapshot

- In scope: pass through the OpenAI-compatible media endpoints — `POST /v1/images/generations`, `/v1/images/edits`, `/v1/audio/speech`, `/v1/audio/transcriptions` — with auth, model access, and budget enforced.
- Out of scope: `/v1/images/variations` (deprecated), `/v1/audio/translations`, `/v1/responses`, content shielding of media requests, and media analysis.

## Goal

MaskChain is used as a drop-in proxy, so clients that call image and audio endpoints must reach their provider unchanged. These endpoints carry binary and multipart payloads (image files, audio files) and produce binary responses (generated images by URL/JSON, synthesized audio). The gateway must forward the request byte-for-byte (preserving the multipart boundary and Content-Type), apply the same key/model/budget gates as text endpoints, and return the provider response with its original status and Content-Type.

**Deliberate exception to Content Shield:** media requests are **not** masked. Masking an image prompt or TTS input would change the generated output and break the client's expectation. The tenant shield is therefore not applied to media endpoints; protection comes from auth, model access, and budgets. This is a conscious operator choice, not a default to revisit silently.

## Primary User Flow

1. Starting point: a client sends an image or audio request to the gateway with a virtual key.
2. Main interaction: the gateway authenticates, resolves the model (from JSON or multipart), checks model access and budget, and forwards the raw request body to the routed provider at the matching endpoint path.
3. Outcome: the client receives the provider's response (JSON image metadata or binary audio) with the provider's status and Content-Type.
4. Failure/fallback path: an unauthorized or disallowed request fails before any provider call; a provider that does not serve the endpoint returns an explicit `UNSUPPORTED_ENDPOINT` error.

## User Stories

- P1 Story: as an integrator, my image/audio calls work through MaskChain unchanged.
- P2 Story: as an operator, keys, model scopes, and budgets still apply to media traffic.
- P3 Story: as a security-conscious operator, I understand media prompts are not masked and can restrict these endpoints through model scopes.

## MVP Slice

- `POST /v1/images/generations` (JSON passthrough) and `POST /v1/audio/speech` (JSON in, binary out), with auth, model access, and budget. Must satisfy `AC-001`, `AC-003`, `AC-005`, `AC-006`, `AC-010`.

## First Deployable Outcome

- `curl POST /v1/images/generations` reaches the provider and returns JSON.
- `curl POST /v1/audio/speech` returns binary audio with `audio/mpeg` (or the provider's Content-Type).
- Demonstrable with a recording mock provider and a real OpenAI-compatible provider.

## Scope

- The four endpoints mounted on `/v1` and `/api/v1`.
- Model extraction from JSON and from `multipart/form-data` (the `model` form field) for access and budget checks.
- Byte-preserving multipart forwarding (original `Content-Type` including the boundary).
- Binary response forwarding (status + `Content-Type` + body) without JSON envelope or SSE wrapping.
- Reduced chain: no session, conversation, cache, SSE, or export stages.

## Context

- `RegisterProxyRoute` mounts proxy endpoints; media endpoints use a reduced chain like the non-chat JSON endpoints.
- `RoutingProxyHandler.HandleChatCompletion` JSON-parses the body and writes `application/json`, so it cannot serve multipart or binary responses; media needs a dedicated handler.
- Provider adapters (`openai`, `proxy`, `ollama`) hardcode `Content-Type: application/json`; multipart forwarding requires the original Content-Type to win.
- `ModelAccess` parses a JSON `model` field and rejects non-JSON bodies, so multipart media needs a media-aware access check.
- Path fidelity (`provider.ResolveUpstreamURL`) requires the new paths in the allowlist.

## Dependencies

- Path fidelity, virtual-key auth, model-access, budget, and the routing fallback handler (all shipped).

## Requirements

- RQ-001 `POST /v1/images/generations` MUST forward the JSON body unchanged to the provider's `/v1/images/generations` and return the provider JSON response.
- RQ-002 `POST /v1/images/edits` MUST forward the multipart body byte-for-byte (same `Content-Type` with boundary) to the provider's `/v1/images/edits`.
- RQ-003 `POST /v1/audio/speech` MUST forward the JSON body and return the provider's binary audio response with the provider's `Content-Type` and status.
- RQ-004 `POST /v1/audio/transcriptions` MUST forward the multipart body byte-for-byte and return the provider JSON response.
- RQ-005 The model MUST be extracted for access and budget checks from a JSON `model` field or, for multipart requests, from the `model` form field; a missing model MUST fail before any provider call.
- RQ-006 Media requests MUST NOT have their content modified by the shield; the request body and `Content-Type` reach the provider unchanged.
- RQ-007 Every media endpoint MUST enforce virtual-key authentication, model access, and the pre-request budget check; usage/cost MUST be accounted when the response is JSON and reports usage.
- RQ-008 Each endpoint MUST be mounted on both `/v1` and `/api/v1` and reach its matching provider path.
- RQ-009 Endpoint shapes a provider cannot serve MUST return the explicit `UNSUPPORTED_ENDPOINT` error and MUST NOT be forwarded to a different endpoint.
- RQ-010 Binary responses MUST be forwarded with the provider's status and `Content-Type` (no JSON envelope, no chunk wrapping).

## Non-Goals

- `/v1/images/variations`, `/v1/audio/translations`, `/v1/responses`, batches, files, fine-tuning, assistants.
- Masking, redaction, or rewriting of any media request field (deliberate).
- Image/audio content analysis or moderation.
- Format conversion of media payloads (passthrough only).

## Acceptance Criteria

### AC-001 Image generation is proxied

- Why this matters: image clients must work unchanged.
- **Given** a route to an OpenAI-shaped provider for model `img`
- **When** a client sends `POST /v1/images/generations` with a prompt
- **Then** the provider receives `/v1/images/generations` with the body unchanged and the client receives the provider's image JSON.
- Evidence: a recording mock shows the endpoint path and an unmodified prompt; the response is the provider's JSON.

### AC-002 Image edit multipart is byte-preserved

- Why this matters: a re-encoded multipart body breaks the provider request.
- **Given** a route for model `img`
- **When** a client sends a `multipart/form-data` request to `/v1/images/edits`
- **Then** the provider receives the identical bytes and the same `Content-Type` boundary value.
- Evidence: the recording mock's received body and `Content-Type` equal the sent ones byte-for-byte.

### AC-003 Speech returns binary audio

- Why this matters: TTS output is audio, not JSON.
- **Given** a route for model `tts`
- **When** a client sends `POST /v1/audio/speech` with `input`
- **Then** the client receives the provider's binary body with the provider's `Content-Type` (e.g. `audio/mpeg`) and status.
- Evidence: the response bytes equal the provider bytes; `Content-Type` matches the provider's.

### AC-004 Transcription multipart is proxied

- Why this matters: STT clients upload audio files.
- **Given** a route for model `stt`
- **When** a client sends `POST /v1/audio/transcriptions` with a file part and `model`
- **Then** the provider receives the identical multipart bytes and the client receives the provider's JSON transcript.
- Evidence: the mock's body equals the sent bytes; the response contains the transcript field.

### AC-005 Model is extracted, including from multipart

- Why this matters: access and budget need the model for multipart requests.
- **Given** a multipart request whose `model` form field is restricted by the key
- **When** the client calls a media endpoint
- **Then** the gateway enforces the model scope (or passes when allowed) without altering the body.
- Evidence: a disallowed model returns 403 with zero provider calls; an allowed model proceeds.

### AC-006 Shield is not applied

- Why this matters: masking would change media output (deliberate passthrough).
- **Given** a tenant whose policy masks emails
- **When** a client sends an image prompt containing an email
- **Then** the provider receives the original, unmasked prompt.
- Evidence: the recording mock's prompt equals the sent prompt.

### AC-007 Auth, model access, and budget are enforced

- Why this matters: media must not bypass the guardrails that apply to text.
- **Given** missing/invalid key, a disallowed model, or a tenant at its hard budget limit
- **When** a media endpoint is called
- **Then** the gateway returns 401 / 403 / 429 respectively with no provider call.
- Evidence: each status is returned and the mock records zero calls.

### AC-008 Both prefixes and path fidelity

- Why this matters: `/v1` and `/api/v1` clients both exist.
- **Given** a supported media request
- **When** it is sent to `/api/v1/<endpoint>` and `/v1/<endpoint>`
- **Then** both reach the same provider path.
- Evidence: the recording mock shows the same path for both prefixes.

### AC-009 Unsupported shapes fail explicitly

- Why this matters: a wrong endpoint must not be called silently.
- **Given** a model routed to a provider that does not serve `/v1/audio/speech`
- **When** the client calls it
- **Then** the gateway returns `UNSUPPORTED_ENDPOINT` and no alternative endpoint is called.
- Evidence: response is 400 with `UNSUPPORTED_ENDPOINT`; zero calls to other paths.

### AC-010 Binary response has no envelope

- Why this matters: wrapping binary in a JSON envelope corrupts it.
- **Given** a provider returning `audio/wav` bytes
- **When** the client receives the response
- **Then** the body is exactly the provider bytes with `Content-Type: audio/wav`.
- Evidence: byte-equality of the response body and the header value.

## Assumptions

- Media providers are OpenAI-compatible for these endpoints.
- The request body is buffered in memory; media payloads are within the server's body-size limit.
- Selected providers serve at least one media endpoint; the allowlist rejects the rest.
- Usage for media is reported only in JSON responses; binary responses are accounted as zero.
- Model-scope checks apply to the requested (pre-alias) model, consistent with other endpoints.

## Success Criteria

- SC-001 Multipart and binary passthrough preserve the body bytes and `Content-Type` exactly.
- SC-002 Existing proxy behavior is unchanged; all suites pass.

## Edge Cases

- Missing `model` in JSON or multipart: `400 VALIDATION_ERROR`, no provider call.
- Provider returns 4xx/5xx (JSON error): status and body surfaced unchanged.
- No healthy provider: existing `NO_HEALTHY_PROVIDER` behavior.
- Unrouted model: `NO_ROUTE`.
- Very large media file: bounded by the server request body limit (documented).

## Open Questions

- Should the request body size limit be raised/configurable for media (audio/images can be several MB)?
- Should `/v1/audio/speech` stream bytes to the client instead of buffering?
- Should media usage be derived for providers that report duration (e.g., transcription seconds) instead of tokens?
