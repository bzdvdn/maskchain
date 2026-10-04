# OpenAI Endpoint Coverage Plan

## Phase Contract

Inputs: `specs/active/openai-endpoint-coverage/spec.md`, `inspect.md` (concerns, no blockers), path-fidelity feature (shipped).
Outputs: `plan.md` (data model and contracts noted inline).
Stop if: the spec is too vague to plan safely.

## Goal

Extend the existing model-based proxy path to the remaining JSON text endpoints. The router already resolves a model, picks a provider, applies path fidelity, and runs the shield/budget chain; this feature adds the missing routes, a non-chat input shield for their text fields, and an OpenAI-shaped `GET /v1/models`. No new provider clients and no request/response translation are needed.

## MVP Slice

- `GET /v1/models` in OpenAI format scoped by key, plus `POST /v1/moderations` and `POST /v1/rerank` proxied with request masking. Must satisfy `AC-001`, `AC-002`, `AC-003`, `AC-004`.

## First Validation Path

1. Unit-test the new text-shield field adapters (moderations `input` string/array; rerank `query` + `documents[].text`).
2. Hit each route with a recording mock provider and assert path + masked body + scoped models.
3. Manual: `curl GET /v1/models`, `curl POST /v1/rerank` against a local OpenAI-compatible mock.

## Scope

- New routes on both `/v1` and `/api/v1`: `GET /models` (OpenAI format on `/v1`), `POST /moderations`, `POST /rerank`, `POST /messages/count_tokens`.
- A generic non-chat input shield with per-endpoint field adapters.
- Extend the provider path allowlist.
- Untouched: chat/completions/embeddings behavior, provider clients, routing selection, DB schema, UI.

## Performance Budget

- Added latency for the new endpoints: same order as the embeddings shield; p95 overhead <15% over a direct provider call measured against a recording mock (SC-001 baseline).
- Allocations: no body copy beyond the single read/rewrite the existing shields already do.

## Implementation Surfaces

- `src/internal/api/self_handler.go` — add `HandleModelsOpenAI` (`{"object":"list","data":[...]}`), keeping `HandleModels` unchanged.
- `src/internal/api/middleware/shield_text.go` (new) — `TextInputShieldMiddleware(kind, engine, cfg, log)` with extractors/rewriters for moderations, rerank, count_tokens.
- `src/internal/api/middleware/shield_embeddings.go` — extract the shared dictionary+PII masking core so the new middleware reuses it (no behavior change).
- `src/internal/api/provider_handler.go` — reuse the model-based handler for the new POST routes (`model` + path derivation already generic).
- `src/internal/api/server.go` — mount the new routes with the non-chat chain and register the new shield; add `GET /v1/models`.
- `src/internal/adapters/provider/upstream.go` — extend `upstreamPathFor` allowlists.
- `src/cmd/gateway/run.go`, `src/cmd/all/gateway.go` — wire the new shield + `/v1/models` (mirror `RegisterEmbeddingsShield` / `RegisterSelfHandler`).
- Tests: `shield_text_test.go` (new), `self_handler_test.go`, `provider_handler_test.go`, `upstream_test.go`.

## Bootstrapping Surfaces

- none — the middleware, server, registry, and shield engine already exist.

## Architecture Impact

- Local: one new middleware + one new self-service handler method; the proxy handler is reused as-is.
- Cross-boundary: `upstreamPathFor` gains endpoints; `ProviderRequest` is unchanged.
- Compatibility: additive only. `/api/v1/models` shape is untouched; new endpoints do not alter chat/embeddings.

## Acceptance Approach

- AC-001 → `HandleModelsOpenAI` returns the list shape; surfaces `self_handler.go`, `server.go`; proof: handler test asserts `object=="list"` and each entry's `id`/`object`/`created`/`owned_by`.
- AC-002 → reuse `vk.AllowsModel` filtering in the new handler; surfaces `self_handler.go`; proof: test with a scoped key returns only allowed ids.
- AC-003 → moderations extractor masks `input` (string/array); surfaces `shield_text.go`; proof: recording mock sees masked input + `/v1/moderations`.
- AC-004 → rerank extractor masks `query` + `documents[].text`; surfaces `shield_text.go`; proof: recording mock sees masked fields + `/v1/rerank`; response passthrough.
- AC-005 → allowlist adds `/v1/messages/count_tokens`; count_tokens extractor masks message text; surfaces `upstream.go`, `shield_text.go`; proof: mock sees path + masked body, response `input_tokens`.
- AC-006 → route chain keeps auth + `modelAccessMw`; surfaces `server.go`; proof: 401/403 tests with zero provider calls.
- AC-007 → route chain keeps `budgetMw` for the three POST endpoints; surfaces `server.go`; proof: 429 at hard limit with zero calls.
- AC-008 → allowlist rejects unsupported shapes via existing `UNSUPPORTED_ENDPOINT`; surfaces `upstream.go`, `provider_handler.go`; proof: 400 + zero calls.
- AC-009 → routes mounted on both prefixes; query forwarded by the existing handler; surfaces `server.go`, `provider_handler.go`; proof: mock records same path + query for both prefixes.

## Data and Contracts

- Data model: no change — no entities, tables, or migrations.
- Contracts: new HTTP routes added; existing routes/response shapes unchanged. `provider.ProviderRequest` unchanged.
- `GET /v1/models` entry: `id`, `object:"model"`, `created:0`, `owned_by:"maskchain"` (resolves inspect W-001).

## Implementation Strategy

- DEC-001 Reuse the model-based proxy handler for all JSON endpoints
  Why: `HandleChatCompletion` already resolves model, selects/fallbacks providers, derives the upstream path, and preserves the response; endpoints differ only in shielding. Not: a handler per endpoint.
  Tradeoff: the handler name stays chat-centric; document it as the generic proxy handler (rename optional, deferred).
  Affects: `provider_handler.go`, `server.go`.
  Validation: per-endpoint route tests assert the correct upstream path.
- DEC-002 One generic non-chat input shield with per-endpoint adapters
  Why: the embeddings shield already proves dictionary+PII masking of plain text works; extracting its core avoids a third copy and keeps block/error semantics identical (resolves inspect W-006).
  Not: duplicating `EmbeddingsShieldMiddleware` per endpoint.
  Tradeoff: refactor touches the embeddings path — mitigate by keeping identical behavior and running the embeddings tests.
  Affects: `shield_text.go` (new), `shield_embeddings.go` (extract), `server.go`.
  Validation: new extractor tests + existing embeddings tests stay green.
- DEC-003 Extend the provider path allowlist per `api_type`
  Why: path fidelity is the single source of truth for supported shapes; unsupported providers should fail explicitly.
  Not: bypassing the allowlist or adding new clients.
  Tradeoff: providers without these endpoints return `UNSUPPORTED_ENDPOINT` (intended).
  Affects: `upstream.go`.
  Validation: `upstream_test.go` cases for each new path.
- DEC-004 Add `/v1/models` as a separate OpenAI-shaped handler
  Why: additive and non-breaking; internal callers rely on the existing `{id,allowed}` array.
  Not: changing `/api/v1/models`.
  Tradeoff: two shapes for the same resource.
  Affects: `self_handler.go`, `server.go`, wiring.
  Validation: handler tests for both shapes.
- DEC-005 Budget/model-access scope excludes the catalog
  Why: `GET /v1/models` makes no provider call and incurs no spend; gating it on budget is nonsensical (resolves inspect W-002).
  Not: applying `budgetMw` to the catalog route.
  Tradeoff: none; catalog remains behind auth only.
  Affects: `server.go`.
  Validation: catalog reachable at hard budget; POST endpoints return 429.
- DEC-006 Rerank response is passed through without unmask
  Why: masking is one-way on input; success responses carry no user-origin text echo for the providers in scope; if a provider echoes documents, placeholders are returned (safe) (resolves inspect W-003).
  Not: building an unmask path for echoed documents.
  Tradeoff: a provider that echoes documents shows placeholders to the client.
  Affects: `shield_text.go`, spec assumption.
  Validation: recording-mock rerank test asserts input masking and response passthrough.
- DEC-007 Moderations input masking follows tenant policy by default
  Why: consistent with chat/embeddings and with the shield-as-core constitution principle (resolves inspect W-004).
  Not: a per-endpoint mask opt-out in this feature.
  Tradeoff: an operator who wants to moderate original text must disable tenant PII/dictionaries for that tenant.
  Affects: `shield_text.go`.
  Validation: moderations test with a masking tenant.
- DEC-008 Usage/cost accounting reuses the existing middleware unchanged
  Why: moderations/rerank/count_tokens generally report no usage; the existing middleware already no-ops when usage is absent; budget pre-check still applies.
  Not: a new accounting model for these endpoints.
  Tradeoff: cost for these calls may be 0.
  Affects: none (documented).
  Validation: usage middleware tests unchanged; new endpoints do not error on missing usage.
- DEC-009 Non-chat endpoints skip session/conversation/cache/SSE stages
  Why: those stages are chat-specific; embeddings already uses a reduced chain.
  Not: reusing the full chat chain.
  Tradeoff: no conversation logging/session for these calls.
  Affects: `server.go`.
  Validation: route-chain tests confirm the reduced chain.

## Incremental Delivery

### MVP (First Value)

- Extend the allowlist; add `shield_text.go` with moderations/rerank adapters; mount the two POST routes; add `GET /v1/models`.
- MVP readiness: `AC-001`, `AC-002`, `AC-003`, `AC-004` pass.

### Iterative Expansion

- Add `count_tokens` adapter + anthropic allowlist → `AC-005`.
- Add chain/auth/budget/unsupported/prefix tests → `AC-006`–`AC-009`.

## Sequencing Notes

- Allowlist + shield core first (compile dependency for routes).
- `GET /v1/models` is independent of the shield work and can land in parallel.
- Wiring in both `run.go` and `all/gateway.go` must stay in sync.

## Risks

- Refactoring the embeddings shield core could regress embeddings masking.
  Mitigation: extract without behavior change; rely on existing embeddings tests.
- `model_access` requires a `model` field; a malformed body returns 400 before the provider.
  Mitigation: covered by existing validation behavior and a test.
- Not all OpenAI-shaped providers serve `/v1/rerank` or `/v1/moderations`.
  Mitigation: explicit `UNSUPPORTED_ENDPOINT` (AC-008).
- Missing wiring in one of the two binaries.
  Mitigation: apply identical wiring in `src/cmd/gateway/run.go` and `src/cmd/all/gateway.go`; smoke both.
- Query/path divergence between prefixes.
  Mitigation: existing handler already derives path from request; test both prefixes.

## Rollout and Compatibility

- Additive routes; no migration or flag.
- Behavior change: none for existing endpoints. New endpoints inherit tenant policy and budgets.
- Operational follow-up: `X-Provider` and shield headers already emitted; observe new route metrics.

## Validation

- Automated: `go test ./src/internal/api/... ./src/internal/adapters/provider/...` plus `make test` before archive.
- New tests: `shield_text_test.go` (extractors), `self_handler_test.go` (both shapes), `provider_handler_test.go` (routes/paths/prefixes), `upstream_test.go` (allowlist).
- Manual: local OpenAI-compatible mock for `/v1/models`, `/v1/moderations`, `/v1/rerank`.
- Proves: `AC-001`–`AC-009`; `DEC-001`–`DEC-009`.

## Constitution Compliance

- no conflicts. Content Shield stays core (applied to new text surfaces); tenant-scoped policy preserved; native-only data plane; no new runtime dependencies.
