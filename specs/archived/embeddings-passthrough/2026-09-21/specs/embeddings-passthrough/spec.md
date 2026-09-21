# Embeddings Passthrough with Input Shield

## Scope Snapshot

- In scope: an OpenAI-compatible embeddings endpoint served by the gateway that masks the request input per tenant before it reaches the provider, routes by model, and records tokens/spend — with no unmask on the response.
- Out of scope: reversible restoration of embeddings (vectors are not invertible), token-array/base64 inputs, output scanning, and embeddings-specific caching.

## Goal

Teams that build RAG and search pipelines send embeddings traffic outside MaskChain today, so sensitive text reaches the provider unmasked and that traffic is invisible to budgets and analytics. This feature makes MaskChain the single gateway for embeddings as well: the tenant's existing PII rules and dictionaries are applied to the input text before the provider call, the provider's vectors are returned unchanged, and embeddings tokens count toward usage and budgets like any other request.

## Primary User Flow

1. Starting point: a tenant has virtual keys, a routed embeddings model, and shield policy (PII rules and dictionaries) configured.
2. Main interaction: a client calls the embeddings endpoint with a model and text input.
3. Outcome: the provider receives masked input, the client receives the provider's vectors unchanged, and the request is counted in analytics and spend.
4. Failure/fallback path: when a tenant rule says block, the request is rejected before the provider is called; when the model has no route, the client gets the same routing error as chat; when the input shape is unsupported, the client gets a clear validation error.

## User Stories

- P1 Story: as a platform engineer, I can point RAG ingestion at MaskChain and know raw PII never leaves my network.
- P2 Story: as an operator, I can see embeddings traffic and its token spend in the same dashboards as chat.
- P3 Story: as a security engineer, I can enforce the same tenant policy for embeddings as for chat without a second configuration.

## MVP Slice

- AC-001 (endpoint parity + auth), AC-002 (input masking), AC-005 (accounting). These deliver the core value: masked, accounted embeddings traffic.

## First Deployable Outcome

- After the first pass, `POST /api/v1/embeddings` with a tenant key returns vectors while the provider log shows masked input, and the request appears in usage and budget spend.

## Scope

- Gateway routes `POST /api/v1/embeddings` and the OpenAI-compatible `/v1/embeddings` alias, behind the same auth, model-access, shield, usage, budget and routing chain as chat completions.
- Shield handling for the embeddings request shape: `input` as a string or an array of strings.
- One-way masking: detected PII/secrets and dictionary terms are replaced (or blocked) per the tenant's existing policy; the response is never unmasked.
- Token and spend accounting using the provider-reported prompt tokens.
- Model routing and fallback for embeddings models.

## Out of Scope

- Restoring original values from an embeddings response (mathematically impossible).
- Token-array or base64 `input` forms (they are not text and cannot be masked).
- Scanning or transforming the response vectors.
- Embeddings-specific caching, batching, or `/v1/batches`.
- New detectors or policy types beyond the existing tenant `pii_config` and dictionaries.

## Context

- The gateway already proxies `/api/v1/chat/completions` and `/api/v1/messages` with a shared middleware chain; `/v1` aliases are served directly (no redirect).
- `ShieldMiddleware` currently parses a chat-shaped body (`messages`) and would treat an embeddings body as empty; it must learn the embeddings `input` shape.
- Tenant policy already exists as `pii_config` (enabled, default_action, rules with actions mask/block) plus per-tenant dictionaries, and is applied identically to chat.
- Embeddings responses are non-streaming and carry `usage.prompt_tokens`; the existing usage/budget capture path already reads prompt tokens for non-streaming responses.
- Cost rates are per-model input/output; embeddings have no completion tokens, so only the input rate contributes.

## Dependencies

- Existing shield middleware and tenant policy (PII config + dictionaries).
- Existing routing selector/fallback, model-access, usage and budget middleware.
- Existing cost-rate registry with fallback.
- No new external dependency; the endpoint is provider-agnostic and OpenAI-compatible.

## Requirements

- RQ-001 The gateway MUST serve `POST /api/v1/embeddings` and `/v1/embeddings` with the same auth, model-access, shield, usage and budget guarantees as chat completions.
- RQ-002 The request `input` MUST be accepted as a string or an array of strings; any other shape MUST be rejected with a clear validation error.
- RQ-003 Detected PII/secrets and dictionary terms in the input MUST be handled per the tenant's existing policy (mask by replacing with placeholders, or block), before the provider is called.
- RQ-004 The provider's embeddings response MUST be returned to the client unchanged; the gateway MUST NOT attempt to unmask vectors.
- RQ-005 Provider-reported prompt tokens for embeddings MUST be recorded in analytics and MUST increment matched budget spend, using the input cost rate only.
- RQ-006 Embeddings models MUST use the same routing and fallback selection as chat, including the existing no-route behavior.
- RQ-007 A blocked input MUST return the same shield block outcome as chat (a rejection before the provider call with the shield status surfaced).

## Non-Goals

- Making embeddings reversible or correlating masked tokens back to originals.
- Changing chat/messages behavior or the tenant policy schema.
- Adding provider-specific embeddings dialects beyond the OpenAI-compatible shape.
- Changing budget scopes or the admin budget API.

## Acceptance Criteria

### AC-001 Embeddings endpoint parity and auth

- Why this matters: the endpoint must be a first-class gateway path, not a weaker one.
- **Given** a tenant key and a routed embeddings model
- **When** the client posts to the canonical path and to the `/v1` alias
- **Then** both are served by the gateway, and a request without a valid key is rejected as unauthorized.
- Evidence: route tests showing both paths resolve and an unauthenticated call returns 401.

### AC-002 Input is masked before the provider call

- Why this matters: the whole point is that raw sensitive text never reaches the provider.
- **Given** a tenant whose policy masks email addresses and a request whose input contains an email
- **When** the client posts the embeddings request
- **Then** the text sent to the provider contains a placeholder instead of the email, while the client receives the provider's vectors unchanged.
- Evidence: a middleware/handler test capturing the provider-bound body and asserting the placeholder, plus the unchanged response.

### AC-003 Blocked input is rejected before the provider call

- Why this matters: block policy must be enforced consistently with chat.
- **Given** a tenant whose policy blocks a detected pattern and an input that matches it
- **When** the client posts the embeddings request
- **Then** the request is rejected with the shield block outcome and no provider call is made.
- Evidence: a test asserting the rejection status and that the provider was not invoked.

### AC-004 Unsupported input shapes are rejected clearly

- Why this matters: token arrays cannot be masked, so silently passing them would leak data.
- **Given** an input that is a token array (or another non-text shape)
- **When** the client posts the embeddings request
- **Then** the gateway returns a validation error naming the unsupported input shape and does not call the provider.
- Evidence: a test asserting the error status/code for a token-array input.

### AC-005 Embeddings usage and spend are recorded

- Why this matters: embeddings traffic must be visible and budgeted.
- **Given** a tenant with an active budget and a provider response carrying prompt tokens
- **When** the embeddings request completes
- **Then** analytics records the prompt tokens and the matched budget spend increases by the input-rate cost.
- Evidence: tests asserting the usage record and the budget counter/history for an embeddings request.

### AC-006 Routing and fallback apply to embeddings

- Why this matters: embeddings must reuse the same routing guarantees.
- **Given** an embeddings model with a configured provider order
- **When** the primary provider is unavailable
- **Then** the request falls through to the next provider; and an unrouted model returns the existing no-route error.
- Evidence: tests covering fallback selection and the no-route error for an embeddings request.

## Assumptions

- Providers return an OpenAI-compatible embeddings response with a `usage.prompt_tokens` field; when usage is absent the existing missing-usage behavior applies (no invented tokens).
- Embeddings are non-streaming; a `stream` field is not expected and is passed through or ignored by the provider.
- Masking input changes embedding semantics for masked spans; this is accepted and is a per-tenant policy choice.
- The existing shield body-size limit applies to embeddings requests.

## Success Criteria

- SC-001 Embeddings requests are accounted with the same record shape as chat (tokens, cost, model, tenant).
- SC-002 Input masking adds no measurable latency beyond the existing shield scan for the same text volume.

## Edge Cases

- Empty input string or an empty array: return a validation error rather than calling the provider.
- Array containing non-string elements: rejected as an unsupported shape.
- Input missing entirely: validation error.
- Very large input exceeding the shield body-size limit: rejected with the existing too-large error.
- Tenant has shield disabled: input passes through unchanged (same as chat).

## Open Questions

- none
