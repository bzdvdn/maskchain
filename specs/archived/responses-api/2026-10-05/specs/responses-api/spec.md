# Responses API Support

## Scope Snapshot

- In scope: proxy `POST /v1/responses` (non-streaming) through routing/auth/model-access/budget with reversible masking of the request text and unmasking of the response text.
- Out of scope: streaming Responses events, background mode, and stored-response management beyond passing `previous_response_id`/`store` through unchanged.

## Goal

MaskChain is a drop-in gateway, so clients that use OpenAI's Responses API must reach their provider through it. This feature adds `/v1/responses` as a first-class proxied endpoint on both `/v1` and `/api/v1`: the request is routed like chat, the tenant's shield masks the prompt text (`instructions` and `input`), the provider's response is unmasked back to the client, and spend/usage is accounted. Unlike media endpoints, Responses carries text, so the Content Shield applies. Success is visible when a Responses client gets the original prompt values restored in the reply and the provider never sees the originals.

## Primary User Flow

1. Starting point: a client sends `POST /v1/responses` with `model`, `input`, and optional `instructions` (`stream` omitted/false).
2. Main interaction: the gateway authenticates, checks model access and budget, masks the request text per tenant policy, and forwards the request to the routed provider's `/v1/responses`.
3. Outcome: the client receives the provider's response with `output_text`/`output[].content[].text` restored to the original values.
4. Failure/fallback path: `stream: true` is rejected with a clear `STREAMING_UNSUPPORTED` error in this MVP; an unrouted model gives `NO_ROUTE`.

## User Stories

- P1 Story: as an integrator using the Responses API, my calls work through MaskChain unchanged (non-streaming).
- P2 Story: as an operator, the same tenant shield that protects chat also protects Responses prompts, with originals restored to the client.
- P3 Story: as an operator, keys, model scopes, and budgets apply to Responses traffic.

## MVP Slice

- Non-streaming `POST /v1/responses` proxied with request masking and response unmasking. Must satisfy `AC-001`–`AC-005`, `AC-008`.

## First Deployable Outcome

- A non-streaming `/v1/responses` call returns the provider response with masked prompt values restored.
- A recording mock shows the provider received masked `instructions`/`input`.

## Scope

- `POST /v1/responses` on `/v1` and `/api/v1`.
- Model extraction from the `model` field; auth, model-access, budget.
- Reversible masking of `instructions` and `input` text; unmask of `output_text` and `output[].content[].text`.
- Passthrough of `previous_response_id`, `store`, `tools`, `text`, `metadata`, and any unknown fields.
- Usage/cost accounting from the Responses `usage` payload.

## Context

- The model-based proxy handler (`RoutingProxyHandler.HandleChatCompletion`) already reads `model`, selects a provider, derives the upstream path, and forwards the body; the Responses request carries `model`.
- The chat shield is `messages`-shaped and unmasks via a placeholder→original mapping; Responses uses `instructions` + `input` and exposes `output_text`/`output[].content[].text`, so a Responses-aware extractor/rewriter is needed.
- The streaming branch of the proxy emits Chat-Completions SSE (`data: ...`, `[DONE]`), which is not the Responses event schema; streaming is deferred.
- Usage middleware parses Chat `prompt_tokens`/`completion_tokens`; Responses reports `input_tokens`/`output_tokens`.
- Path fidelity (`ResolveUpstreamURL`) requires `/v1/responses` in the allowlist.

## Dependencies

- Path fidelity, virtual-key auth, model access, budget, routing/fallback, the shield engine and mask mapping (all shipped).

## Requirements

- RQ-001 `POST /v1/responses` MUST be forwarded non-streaming to the routed provider's `/v1/responses` and return the provider response unchanged except for unmasking.
- RQ-002 The model MUST be read from the `model` field and MUST be subject to model access and the pre-request budget check.
- RQ-003 The tenant shield MUST mask the text in `instructions` and in `input` (a string, or message items whose content contains `input_text`/`text`) before the provider call.
- RQ-004 The response text in `output_text` and `output[].content[].text` MUST be unmasked to the original values using the request's mask mapping.
- RQ-005 All other request fields (`previous_response_id`, `store`, `tools`, `text`, `metadata`, unknown keys) MUST be forwarded byte-meaning-equivalent and unmodified.
- RQ-006 A request with `stream: true` MUST return an explicit `STREAMING_UNSUPPORTED` error in this MVP and MUST NOT call the provider.
- RQ-007 Usage/cost MUST be accounted when the response reports `usage` (Responses `input_tokens`/`output_tokens`).
- RQ-008 The endpoint MUST be mounted on `/v1` and `/api/v1`; an unsupported shape for the selected provider MUST return `UNSUPPORTED_ENDPOINT`.

## Non-Goals

- Streaming Responses events (`response.output_text.delta`, etc.).
- Background mode, response retrieval (`GET /v1/responses/{id}`), cancellation, and stored-response listing.
- Tool-loop orchestration and unmasking of tool-call arguments beyond the plain text fields above.
- Cross-format translation between Responses and Chat Completions.

## Acceptance Criteria

### AC-001 Non-streaming Responses is proxied

- Why this matters: Responses clients must work unchanged.
- **Given** a route to an OpenAI-shaped provider for model `m`
- **When** a client sends `POST /v1/responses` with `model:"m"` and no `stream`
- **Then** the provider receives `/v1/responses` and the client receives the provider's response.
- Evidence: a recording mock shows the path; the response `object` is `response`.

### AC-002 Model access and budget apply

- Why this matters: Responses must not bypass the guardrails.
- **Given** a key scoped to `m`, or a tenant at its hard budget limit
- **When** the client calls `/v1/responses`
- **Then** a disallowed model returns 403 and an over-budget tenant returns 429, each with no provider call.
- Evidence: statuses 403/429; the mock records zero calls.

### AC-003 Request text is masked

- Why this matters: prompt text must not leave unmasked.
- **Given** a tenant whose policy masks emails
- **When** the client sends `instructions` and `input` containing an email
- **Then** the provider receives the email replaced by a placeholder in both.
- Evidence: the recording mock's `instructions`/`input` contain a placeholder and not the email.

### AC-004 Response text is unmasked

- Why this matters: the client must see original values.
- **Given** the same masked request and a provider response echoing the placeholder
- **When** the client receives the response
- **Then** `output_text` and `output[].content[].text` contain the original email.
- Evidence: the client response contains the email; the placeholder is absent.

### AC-005 Other fields are passed through

- Why this matters: state and tools must reach the provider.
- **Given** a request with `previous_response_id`, `store`, `tools`, and `metadata`
- **When** it is forwarded
- **Then** all these fields reach the provider unchanged.
- Evidence: the mock's received body contains identical values for these fields.

### AC-006 Streaming is rejected in the MVP

- Why this matters: the Responses stream schema differs from chat.
- **Given** a request with `stream: true`
- **When** the client calls `/v1/responses`
- **Then** the gateway returns `STREAMING_UNSUPPORTED` and makes no provider call.
- Evidence: error code present; zero provider calls.

### AC-007 Usage is accounted

- Why this matters: spend tracking must cover Responses.
- **Given** a provider response with `usage: {input_tokens, output_tokens, total_tokens}`
- **When** the call completes
- **Then** a usage record carries the mapped token counts and computed cost.
- Evidence: an emitted usage record with the expected counts.

### AC-008 Both prefixes and explicit unsupported shapes

- Why this matters: `/v1` and `/api/v1` clients both exist and wrong shapes must fail loudly.
- **Given** a supported request
- **When** it is sent to `/api/v1/responses` and `/v1/responses`
- **Then** both reach the provider's `/v1/responses`; a provider that cannot serve it returns `UNSUPPORTED_ENDPOINT`.
- Evidence: same path for both prefixes; 400 `UNSUPPORTED_ENDPOINT` otherwise.

## Assumptions

- Responses providers are OpenAI-compatible for `/v1/responses`.
- MVP is non-streaming; `stream` defaults to false and is rejected when true.
- `input` is either a string or an array of items; text lives in content blocks of type `input_text`/`text` (and `output_text` on the response side).
- The mask mapping from the request is available on the response path (same as chat).
- Key model scopes are evaluated against the requested model name.

## Success Criteria

- SC-001 Non-streaming proxying adds no measurable latency versus chat beyond the shield scan.
- SC-002 Existing suites pass; no change to chat/embeddings/media behavior.

## Edge Cases

- `input` given as a string: masked directly.
- No `instructions`/no maskable text: forwarded unchanged, no placeholders.
- Provider returns 4xx/5xx: status and body surfaced unchanged (no unmask).
- Unrouted model: `NO_ROUTE`. No healthy provider: `NO_HEALTHY_PROVIDER`.
- Response contains no echo of masked text: unmask is a no-op.

## Open Questions

- Streaming: map `response.output_text.delta` events and unmask deltas incrementally (next spec)?
- Should `store: true` be forced to `false` under a tenant `retention: none` policy?
- Should tool-call arguments carrying masked text be unmasked, or left to the client?
- Do we support `GET /v1/responses/{id}` (stored responses) or treat them as out of scope indefinitely?
