# Provider Path Fidelity

## Scope Snapshot

- In scope: the gateway forwards the endpoint path of the incoming request to the selected provider, so each mounted `/v1/*` route reaches its matching upstream endpoint instead of a hardcoded one.
- Out of scope: adding new endpoint families (images, audio, rerank, moderations), cross-format translation, provider registry, or load balancing.

## Goal

Operators and client SDKs that already speak the OpenAI or Anthropic wire format can point at MaskChain unchanged: a call to `/v1/embeddings`, `/v1/completions`, `/v1/chat/completions`, or `/v1/messages` reaches the same upstream endpoint on every passthrough provider. Today every passthrough adapter hardcodes `/v1/chat/completions` (or `/v1/messages`), so embeddings and legacy completions silently hit the wrong provider endpoint. Success is visible when a client observes the request arriving at the provider endpoint it actually called, with the correct response shape.

## Primary User Flow

1. Starting point: an OpenAI-compatible client is configured with `base_url = <gateway>` and calls `POST /v1/embeddings` with a model that resolves to an OpenAI-shaped provider.
2. Main interaction: the gateway authenticates, applies shield/usage/budget, selects the provider, and computes the upstream path from the request path.
3. Outcome: the provider receives `POST <base_url>/v1/embeddings` and the gateway returns the provider's embeddings response unchanged.
4. Failure/fallback path: if the provider does not support the requested endpoint shape, the gateway returns an explicit error naming the provider and path instead of sending the request to a different endpoint.

## User Stories

- P1 Story: as an operator, I can expose MaskChain as a drop-in proxy for the endpoints it already mounts, so existing clients work without changing their call paths.
- P2 Story: as an operator, I get a clear error when I route a request shape to a provider that cannot serve it, instead of a confusing provider-side error.

## MVP Slice

- The smallest slice: `/v1/embeddings` and `/v1/completions` reach their matching upstream endpoint on `openai`/`proxy`/`ollama` providers, and the `base_url` + path join never duplicates `/v1`. Must satisfy `AC-001`, `AC-002`, `AC-005`.

## First Deployable Outcome

- A live request to `POST /v1/embeddings` against a configured OpenAI-compatible provider returns a provider embeddings response (vector shape), not a chat-completion error.
- A request to `POST /v1/completions` reaches the provider completions endpoint.
- The behavior is demonstrable with a mock provider that records the received path, and with a real OpenAI-compatible provider.

## Scope

- Passthrough adapters (`openai`, `anthropic`, `ollama`, `proxy`) derive the upstream path from the incoming request path.
- A single, consistent join rule between provider `base_url` and the upstream path.
- An explicit provider-endpoint support check that rejects unsupported cross-format requests.
- Path safety: the upstream host is always the provider `base_url`; the client path cannot redirect the request to another host or escape the configured root.
- Existing middleware behavior (auth, shield, usage, budget, export, streaming) is preserved unchanged.

## Context

- `RoutingProxyHandler` already computes `upstreamPath = TrimPrefix(request path, "/api")` and stores it in `ProviderRequest.URL` and `ProviderRequest.Path` (`src/internal/api/provider_handler.go:157-205`); the intent is documented in `src/internal/api/server.go:162`.
- Passthrough adapters ignore that value and hardcode their upstream path: `openai.go:64,94`, `proxy.go:101`, `anthropic.go:67,102`.
- `GET /api/v1/models` and `/api/v1/me` are self-service routes and are not provider-passthrough endpoints; they are unaffected.
- Gemini and Bedrock adapters translate request/response bodies and build provider-native paths; they are not raw path-forwarding adapters.

## Dependencies

- Existing `ProviderRequest.URL` / `ProviderRequest.Path` contract in `src/internal/ports/provider.go`.
- Existing provider config (`ProviderConfig`) in `src/internal/infra/config/config.go`.
- Existing egress client and retry/circuit-breaker behavior in `src/internal/adapters/egress/`.

## Requirements

- RQ-001 A passthrough adapter MUST derive its upstream path from the incoming request path, not from a hardcoded constant.
- RQ-002 The join of provider `base_url` and the derived path MUST NOT duplicate a `/v1` segment (a `base_url` ending in `/v1` and an incoming `/v1/...` must not yield `/v1/v1/...`).
- RQ-003 Each passthrough `api_type` MUST declare which endpoint shapes it supports; a request whose shape is unsupported MUST fail with an explicit gateway error that names the provider and the requested path.
- RQ-004 The incoming query string MUST be preserved when forwarding to the provider.
- RQ-005 Fallback selection and streaming MUST use the same derived path as the primary attempt.
- RQ-006 The derived path MUST be constrained to the provider `base_url` host and root: traversal segments (`..`), scheme-relative (`//host`), and absolute-URL input MUST be rejected.
- RQ-007 The chosen upstream path and provider MUST be observable in gateway logs and the existing `X-Provider` response header.

## Non-Goals

- Adding new endpoint families or catch-all `/v1/*path` routing.
- Translating between OpenAI and Anthropic request/response formats.
- Changing routing selection, fallback ordering, or load balancing.
- Changing Gemini/Bedrock translation behavior.
- Changing `GET /api/v1/models` or `/api/v1/me`.

## Acceptance Criteria

### AC-001 Embeddings reaches the embeddings endpoint

- Why this matters: embeddings clients must not silently receive chat responses.
- **Given** an OpenAI-shaped provider (`openai`, `proxy`, or `ollama`) is routed for model `m`
- **When** a client sends `POST /v1/embeddings` for model `m`
- **Then** the provider receives a request to its `/v1/embeddings` path and the client receives the provider's embeddings response.
- Evidence: a mock provider recording the received path shows `/v1/embeddings`; a real provider returns a `data[].embedding` vector payload.

### AC-002 Legacy completions reaches the completions endpoint

- Why this matters: `/v1/completions` clients must not be routed to chat.
- **Given** an OpenAI-shaped provider is routed for model `m`
- **When** a client sends `POST /v1/completions` for model `m`
- **Then** the provider receives a request to its `/v1/completions` path.
- Evidence: a mock provider records `/v1/completions`; the response body has the completions shape (`choices[].text`).

### AC-003 Chat completions path is unchanged

- Why this matters: the primary endpoint must not regress.
- **Given** an OpenAI-shaped provider is routed for model `m`
- **When** a client sends `POST /v1/chat/completions` for model `m`
- **Then** the provider receives a request to its `/v1/chat/completions` path.
- Evidence: a mock provider records `/v1/chat/completions`; the existing chat proxy tests still pass.

### AC-004 Anthropic messages uses the messages endpoint

- Why this matters: the native Anthropic passthrough must keep its shape.
- **Given** an `anthropic` provider is routed for model `m`
- **When** a client sends `POST /v1/messages` for model `m`
- **Then** the provider receives a request to its `/v1/messages` path with the `anthropic-version` header.
- Evidence: a mock provider records `/v1/messages` and the `anthropic-version` header; the client receives the Anthropic response shape.

### AC-005 No duplicated `/v1` in the upstream URL

- Why this matters: a common `base_url` that already includes the version segment must not produce a 404.
- **Given** a provider `base_url` ending in `/v1` and an incoming request to `/v1/embeddings`
- **When** the gateway forwards the request
- **Then** the upstream URL contains exactly one `/v1` segment before `/embeddings`.
- Evidence: a unit test asserts the composed URL equals `<base_with_v1>/embeddings` and contains no `v1/v1`.

### AC-006 Unsupported cross-format request fails explicitly

- Why this matters: silently posting an Anthropic body to a chat endpoint produces confusing provider errors.
- **Given** a model routed only to an OpenAI-shaped provider
- **When** a client sends `POST /v1/messages` for that model
- **Then** the gateway returns an explicit error response naming the provider and the unsupported path, and does not call a different provider endpoint.
- Evidence: the response status is a 4xx/5xx with a machine-readable code and a message naming provider + path; the mock provider records zero calls.

### AC-007 Fallback and streaming preserve the path

- Why this matters: failover must not change which endpoint is used.
- **Given** a model with a primary and a fallback OpenAI-shaped provider
- **When** the primary fails and the request proceeds (non-streaming and streaming)
- **Then** the fallback provider receives the same derived path as the primary attempt.
- Evidence: a mock fallback provider records the path for both a non-streaming call and an SSE request.

### AC-008 Query string is preserved

- Why this matters: some provider endpoints depend on query parameters.
- **Given** a supported passthrough request with a query string (e.g. `/v1/chat/completions?foo=bar`)
- **When** the gateway forwards it
- **Then** the provider receives the same query string.
- Evidence: a mock provider records `foo=bar` on the received request.

### AC-009 Path input cannot escape the provider root

- Why this matters: the client-controlled path must not become an SSRF or host-override vector.
- **Given** a request path containing traversal (`..`), a scheme-relative prefix (`//evil.example`), or an absolute URL
- **When** the gateway derives the upstream path
- **Then** the request is rejected with an explicit error and no outbound call is made to a host other than the provider `base_url`.
- Evidence: unit tests for each hostile input assert rejection; the provider host in all forwarded calls equals the configured `base_url` host.

## Assumptions

- `base_url` denotes the provider API root; the derived incoming path is appended to it, with `/v1` de-duplicated. Existing configs that already include `/v1` keep working (see `AC-005`).
- The initial supported endpoint shapes per passthrough `api_type` are: OpenAI-shaped → `/v1/chat/completions`, `/v1/completions`, `/v1/embeddings`; Anthropic-shaped → `/v1/messages`.
- `ollama` is treated as OpenAI-shaped for the mounted endpoints.
- Gemini and Bedrock continue to use their existing translators and are exempt from raw path forwarding.
- The current middleware chain order for each route is unchanged.

## Success Criteria

- SC-001 A passthrough request adds no measurable routing/URL-composition latency beyond the existing proxy overhead (<1 ms at p95 on the URL-composition step).
- SC-002 All existing proxy/routing test suites continue to pass unchanged.

## Edge Cases

- `base_url` with a trailing slash, with `/v1`, and without `/v1` all compose to a valid single-version URL.
- Request with an empty body or missing `model` keeps the existing validation behavior.
- Provider returns 4xx/5xx: the provider error body is surfaced unchanged (existing behavior).
- No healthy provider: existing `NO_HEALTHY_PROVIDER` behavior is unchanged and the path is not sent anywhere.

## Open Questions

- Cross-format handling (AC-006): should Anthropic's historic `chat/completions → /v1/messages` alias be preserved as a documented exception, or does it become an explicit error like other cross-format requests? (Default assumed: explicit error, with the alias dropped.)
- `base_url` semantics: confirm the de-duplicate `/v1` join rule (AC-005) over a breaking change that requires config migration; and whether to expose an optional per-provider `path_prefix`/`upstream_path` override as an escape hatch.
