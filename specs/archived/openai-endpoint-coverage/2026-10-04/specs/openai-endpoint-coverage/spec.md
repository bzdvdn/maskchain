# OpenAI Endpoint Coverage

## Scope Snapshot

- In scope: expose the remaining OpenAI-compatible JSON text endpoints — `GET /v1/models`, `POST /v1/moderations`, `POST /v1/rerank`, and Anthropic `POST /v1/messages/count_tokens` — through the existing auth, shield, model-access, and budget chain.
- Out of scope: images, audio, responses, batches, files, fine-tuning, assistants, realtime, videos, vector stores, MCP/A2A, and cross-format translation.

## Goal

Clients and SDKs that already speak the OpenAI (or Anthropic) wire format can use MaskChain without changing call paths or endpoints. Today MaskChain mounts only `chat/completions`, `completions`, `embeddings`, and `messages`; this feature adds the JSON text surface those clients expect, with tenant content shielding applied to the text fields each endpoint carries, and with the same key/budget enforcement as chat. Success is visible when a client calls `/v1/models`, `/v1/moderations`, `/v1/rerank`, or `/v1/messages/count_tokens` and receives the provider-shaped response with PII masked before the provider sees it.

## Primary User Flow

1. Starting point: an application configured with `base_url = <gateway>` and a virtual key calls `GET /v1/models` to discover models, then `POST /v1/rerank` (or `/v1/moderations`, `/v1/messages/count_tokens`).
2. Main interaction: the gateway authenticates the key, checks model access and budget, masks the endpoint's text fields per tenant policy, and forwards the request to the routed provider at the matching endpoint path.
3. Outcome: the client receives the provider's response unchanged (moderation categories, rerank scores, token count).
4. Failure/fallback path: an endpoint shape the provider does not serve returns an explicit `UNSUPPORTED_ENDPOINT` error instead of being sent to a different endpoint.

## User Stories

- P1 Story: as an integrator, I can list models in the OpenAI shape and call moderation/rerank/count-tokens through MaskChain so my existing client works unchanged.
- P2 Story: as an operator, the text these endpoints carry is shielded by the same tenant PII policy as chat, and my budgets still apply.

## MVP Slice

- `GET /v1/models` in OpenAI format, filtered by the key's model scopes, plus `POST /v1/moderations` and `POST /v1/rerank` proxied with text masking. Must satisfy `AC-001`, `AC-002`, `AC-003`, `AC-004`.

## First Deployable Outcome

- `curl GET /v1/models` returns `{"object":"list","data":[{"id":...,"object":"model",...}]}` scoped to the key.
- `curl POST /v1/rerank` returns provider scores, and the provider receives masked `query`/`documents[].text`.
- `curl POST /v1/moderations` returns provider categories from masked input.
- Demonstrable with a recording mock provider and with a real OpenAI-compatible provider.

## Scope

- `GET /v1/models` and `GET /api/v1/models` (existing self-service shape stays as-is; `/v1/models` is added in the OpenAI shape).
- `POST /v1/moderations`, `POST /v1/rerank`, `POST /v1/messages/count_tokens`, each mounted on both `/v1` and `/api/v1` prefixes.
- Text-field masking for each endpoint via the existing detector pipeline.
- The existing middleware order (auth → model access → shield → usage → budget → export → handler) extended with an input-shield suitable for non-chat request shapes.
- Path fidelity: each request reaches its matching provider endpoint.

## Context

- `RoutingProxyHandler` already derives the upstream path from the request (`src/internal/api/provider_handler.go`) and the path allowlist lives in `src/internal/adapters/provider/upstream.go`; new endpoints must extend that allowlist.
- `GET /api/v1/models` exists behind virtual-key auth (`src/internal/api/self_handler.go`) and returns a bare array of `{id, allowed}`; it is preserved.
- Chat masking operates on `messages`; non-chat shapes need field-level masking (`input`, `query`, `documents[].text`, Anthropic `messages`).
- Embeddings established the pattern of a dedicated input-shield chain that skips chat-only stages (session/conversation/cache/SSE) (`src/internal/api/server.go`).

## Dependencies

- Path-fidelity feature (shipped): `provider.ResolveUpstreamURL`, `ports.ProviderRequest.RawQuery`, `ports.ErrUnsupportedEndpoint`.
- Existing detector/registry and tenant PII policy.
- Existing virtual-key auth, model-access, usage, and budget middleware.
- External providers: OpenAI-compatible `/v1/moderations` and `/v1/rerank`; Anthropic `/v1/messages/count_tokens`.

## Requirements

- RQ-001 `GET /v1/models` MUST return the OpenAI list shape (`{"object":"list","data":[{"id":...,"object":"model",...}]}`) for models routed to the authenticated tenant.
- RQ-002 `GET /v1/models` MUST include only models the presenting virtual key is allowed to use; with an unscoped key it returns all routed models.
- RQ-003 `POST /v1/moderations` MUST forward to the routed provider's `/v1/moderations`, masking each `input` string per tenant policy before the call.
- RQ-004 `POST /v1/rerank` MUST forward to the routed provider's `/v1/rerank`, masking `query` and every `documents[].text` per tenant policy before the call.
- RQ-005 `POST /v1/messages/count_tokens` MUST forward to the routed Anthropic provider's `/v1/messages/count_tokens`, masking message/system text per tenant policy before the call.
- RQ-006 Every new endpoint MUST enforce virtual-key authentication, model access, and the pre-request budget check, and MUST account usage/cost when the provider response reports usage.
- RQ-007 An endpoint shape the selected provider does not serve MUST return an explicit `UNSUPPORTED_ENDPOINT` error and MUST NOT be forwarded to a different endpoint.
- RQ-008 Each endpoint MUST be mounted on both `/v1` and `/api/v1` prefixes.
- RQ-009 Masked text MUST be the text the provider receives; these endpoints MUST NOT unmask provider output (they carry no user-origin text echo on the success path).

## Non-Goals

- Image, audio, video, responses, batches, files, fine-tuning, assistants, realtime, vector-store, and MCP/A2A endpoints.
- OpenAI↔Anthropic request/response translation.
- Streaming on the new endpoints (all are single JSON responses).
- Changing the existing `GET /api/v1/models` response shape.
- Provider-side model discovery inside `/v1/models` (listing is route-driven).

## Acceptance Criteria

### AC-001 Model catalog uses the OpenAI shape

- Why this matters: OpenAI SDKs and tooling parse a fixed list shape.
- **Given** a tenant with routed models `m1`, `m2` and a valid key
- **When** the client sends `GET /v1/models`
- **Then** the response body is `{"object":"list","data":[...]}` and every entry has `id`, `object:"model"`, `created`, and `owned_by`, containing `m1` and `m2`.
- Evidence: a request returns HTTP 200 with the documented JSON shape; a schema assertion on `object == "list"` and each `data[].object == "model"` passes.

### AC-002 Model catalog respects key scope

- Why this matters: a key must not discover models it cannot use.
- **Given** a key restricted to model `m1` while the tenant routes `m1` and `m2`
- **When** the client sends `GET /v1/models` with that key
- **Then** the `data` list contains `m1` and not `m2`.
- Evidence: response `data[].id` for the scoped key equals `["m1"]`; an unrestricted key returns both.

### AC-003 Moderations are forwarded and masked

- Why this matters: moderation input may carry PII that must not leave the boundary.
- **Given** a route to an OpenAI-shaped provider for model `mod` and a tenant whose policy masks emails
- **When** the client sends `POST /v1/moderations` with `{"input":"email alice@example.com"}`
- **Then** the provider receives `/v1/moderations` with the email replaced by a placeholder and the client receives the provider's categories response unchanged.
- Evidence: a recording mock shows the masked input string and the endpoint path; a real provider returns `results[].categories`.

### AC-004 Rerank is forwarded and masked

- Why this matters: rerank queries/documents often contain confidential text.
- **Given** a route to an OpenAI-shaped provider for model `rr` and a masking policy
- **When** the client sends `POST /v1/rerank` with `query` and `documents[].text`
- **Then** the provider receives `/v1/rerank` with `query` and every `documents[].text` masked, and the client receives the provider's ranked results.
- Evidence: a recording mock shows masked `query`/`documents[].text` and the path; the response contains `results[]` with `index`/`relevance_score`.

### AC-005 Anthropic count_tokens is forwarded and masked

- Why this matters: token counts must reflect what the provider will actually see.
- **Given** a route to an `anthropic` provider for model `claude-x`
- **When** the client sends `POST /v1/messages/count_tokens` with `messages`
- **Then** the provider receives `/v1/messages/count_tokens` with the message text masked and the client receives `{"input_tokens": <n>}`.
- Evidence: a recording mock shows the path and masked body; the response contains an `input_tokens` integer.

### AC-006 Auth and model access are enforced

- Why this matters: the new surface must not bypass tenant isolation.
- **Given** no/invalid key, or a key not allowed the requested model
- **When** the client calls any new endpoint
- **Then** the gateway returns 401 for missing/invalid auth or 403 for a disallowed model and makes no provider call.
- Evidence: responses have the corresponding status and the recording mock records zero calls.

### AC-007 Budget is enforced before the provider call

- Why this matters: spend limits must apply to all endpoints.
- **Given** a tenant at its hard budget limit
- **When** the client calls any new endpoint
- **Then** the gateway returns 429 without calling the provider.
- Evidence: response status 429 with the budget error code; zero provider calls recorded.

### AC-008 Unsupported shapes fail explicitly

- Why this matters: a shape a provider cannot serve must not be silently rerouted.
- **Given** a model routed only to a provider without `/v1/rerank`
- **When** the client sends `POST /v1/rerank`
- **Then** the gateway returns the explicit `UNSUPPORTED_ENDPOINT` error and the provider receives no request to another endpoint.
- Evidence: response status 400 with the `UNSUPPORTED_ENDPOINT` code; zero upstream calls.

### AC-009 Both prefixes and query preservation

- Why this matters: existing clients use `/v1` and internal callers use `/api/v1`.
- **Given** a supported request
- **When** it is sent to `/api/v1/<endpoint>` and to `/v1/<endpoint>` (with a query string)
- **Then** both reach the same provider endpoint and the query string is preserved.
- Evidence: a recording mock shows the same path and query for both prefixes.

## Assumptions

- Providers serving moderations/rerank are OpenAI-compatible; `count_tokens` is Anthropic-specific.
- The existing detector pipeline can mask a plain string field without chat context.
- The success responses of these endpoints do not echo user-origin text, so no unmask is required.
- `/api/v1/models` keeps its current `{id, allowed}` array shape; only `/v1/models` is OpenAI-shaped.

## Success Criteria

- SC-001 Added latency to new endpoints stays within the existing proxy overhead (<15% p95 over direct provider call).
- SC-002 All existing suites keep passing; new endpoints add no regression to chat/embeddings.

## Edge Cases

- `moderations.input` given as an array of strings: every element masked.
- `rerank` with empty `documents`: forwarded unchanged.
- Request body missing the expected field: existing-style `400 VALIDATION_ERROR` without a provider call.
- Provider returns 4xx/5xx: the provider error body is surfaced unchanged.
- Model not routed: `400 NO_ROUTE`.

## Open Questions

- `owned_by` value in `/v1/models` entries: a constant (`maskchain`) or the primary provider name of the route? (Default assumed: `maskchain`.)
- Should `/api/v1/models` eventually adopt the OpenAI shape (a breaking change) or remain the current array?
- Should masking of `/v1/moderations` input be mandatory or opt-out per tenant (some operators want to moderate the original text)? (Default assumed: apply the standard tenant policy.)
