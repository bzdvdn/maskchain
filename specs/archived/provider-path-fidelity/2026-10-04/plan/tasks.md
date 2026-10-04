# Provider Path Fidelity Tasks

## Phase Contract

Inputs: `specs/active/provider-path-fidelity/plan.md`, `spec.md`.
Outputs: ordered executable tasks with coverage mapping.
Stop if: tasks would be vague or acceptance coverage cannot be mapped.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/ports/provider.go | T1.1 |
| src/internal/adapters/provider/upstream.go | T1.2, T2.1, T2.2, T2.3, T3.1 |
| src/internal/adapters/provider/upstream_test.go | T1.3 |
| src/internal/adapters/provider/openai.go | T2.1 |
| src/internal/adapters/provider/proxy.go | T2.2 |
| src/internal/adapters/provider/ollama.go | T2.3 |
| src/internal/adapters/provider/anthropic.go | T3.1 |
| src/internal/api/provider_handler.go | T2.4, T3.3 |
| src/internal/domain/routing/service/fallback.go | T3.2 |
| src/internal/adapters/provider/provider_test.go | T2.5, T3.4 |
| src/internal/adapters/provider/proxy_test.go | T2.5 |
| src/internal/adapters/provider/ollama_test.go | T2.5 |
| src/internal/api/provider_handler_test.go | T2.5, T3.5 |
| src/internal/domain/routing/service/service_test.go | T3.5 |
| README.md, examples/config.yaml | T4.2 |

## Implementation Context

- MVP goal: `/v1/embeddings` and `/v1/completions` reach the matching provider endpoint on OpenAI-shaped providers, with `base_url` `/v1` de-dup (AC-001, AC-002, AC-005).
- Invariants/semantics: passthrough adapters derive the upstream path from `ProviderRequest.URL` (request path minus `/api`), never a hardcoded constant; upstream host is always provider `base_url`; chat behavior must not regress.
- Errors/codes: new sentinel `ports.ErrUnsupportedEndpoint`; handler returns HTTP 400 with code `UNSUPPORTED_ENDPOINT` naming provider + path; do not reuse `NO_HEALTHY_PROVIDER` for this case.
- Contracts/protocol: `ports.ProviderRequest` gains `RawQuery string`; handler populates it from `c.Request.URL.RawQuery`; helper appends it verbatim.
- Scope boundaries: do not add new endpoint families or catch-all routing; do not translate OpenAI↔Anthropic; do not touch Gemini/Bedrock translators or routing selection.
- Proof signals: unit tests for path composition + sanitizer; adapter tests with a recording mock asserting outbound path/query; handler test asserting explicit error and zero provider calls; fallback test asserting same path on primary+fallback for Call and SSE.
- References: DEC-001..DEC-005, RQ-001..RQ-007 (no mandatory re-read).

## Phase 1: Foundation

Goal: establish the additive port contract and the shared URL-composition helper that later phases depend on.

- [x] T1.1 Add `RawQuery` field and `ErrUnsupportedEndpoint` sentinel to the provider port — the port exposes both without breaking existing implementers (DEC-002, DEC-003). Touches: src/internal/ports/provider.go
      Proof: code src/internal/ports/provider.go ErrUnsupportedEndpoint
- [x] T1.2 Implement `ResolveUpstreamURL(apiType, baseURL, req)` with `/v1` de-dup, per-`api_type` support check, and a sanitizer rejecting `..`, `//`, `://`, and non-`/` paths (DEC-001, DEC-005, RQ-001..RQ-003, RQ-006). Touches: src/internal/adapters/provider/upstream.go
      Proof: code src/internal/adapters/provider/upstream.go ResolveUpstreamURL
- [x] T1.3 Add table-driven unit tests for `ResolveUpstreamURL` — join matrix (`/v1`, no `/v1`, trailing slash), each mounted path, query append, and every hostile input (AC-005, AC-008, AC-009). Touches: src/internal/adapters/provider/upstream_test.go
      Proof: test src/internal/adapters/provider/upstream_test.go TestResolveUpstreamURL

## Phase 2: MVP Slice

Goal: deliver correct `/v1/embeddings` and `/v1/completions` routing on OpenAI-shaped providers.

- [x] T2.1 Wire `OpenAIClient` Call and Stream through the helper, propagating its error (AC-001, AC-002, AC-003). Touches: src/internal/adapters/provider/openai.go
      Proof: code src/internal/adapters/provider/openai.go Call
- [x] T2.2 Wire `ProxyClient.buildRequest` through the helper and return its error (AC-001, AC-002, AC-003). Touches: src/internal/adapters/provider/proxy.go
      Proof: code src/internal/adapters/provider/proxy.go buildRequest
- [x] T2.3 Wire `OllamaClient.buildRequest` through the helper and return its error (AC-001, AC-002, AC-003). Touches: src/internal/adapters/provider/ollama.go
      Proof: code src/internal/adapters/provider/ollama.go buildRequest
- [x] T2.4 Populate `RawQuery` from the incoming request in `RoutingProxyHandler` for primary and fallback `ProviderRequest` values (DEC-003, AC-008). Touches: src/internal/api/provider_handler.go
      Proof: code src/internal/api/provider_handler.go HandleChatCompletion
- [x] T2.5 Prove the MVP with recording-mock adapter tests asserting outbound `/v1/embeddings`, `/v1/completions`, unchanged `/v1/chat/completions`, and query preservation (AC-001, AC-002, AC-003, AC-005, AC-008). Touches: src/internal/adapters/provider/provider_test.go, src/internal/adapters/provider/proxy_test.go, src/internal/adapters/provider/ollama_test.go, src/internal/api/provider_handler_test.go
      Proof: test src/internal/adapters/provider/provider_test.go TestPassThroughPathFidelity

## Phase 3: Core Implementation

Goal: complete Anthropic handling, unsupported-shape errors, and fallback/stream path fidelity.

- [x] T3.1 Route `AnthropicClient` Call and Stream through the helper, preserve the `chat/completions → /v1/messages` alias, and reject other shapes (DEC-004, AC-004, AC-006). Touches: src/internal/adapters/provider/anthropic.go
      Proof: code src/internal/adapters/provider/anthropic.go Call
- [x] T3.2 Make `FallbackHandler.Call` and `Stream` treat `ErrUnsupportedEndpoint` as "try next provider", surfacing the sentinel when the chain is exhausted (DEC-002, AC-007). Touches: src/internal/domain/routing/service/fallback.go
      Proof: code src/internal/domain/routing/service/fallback.go Call
- [x] T3.3 Map the sentinel in `RoutingProxyHandler` to HTTP 400 `UNSUPPORTED_ENDPOINT` naming provider + path, and log the chosen upstream path (DEC-002, RQ-007, AC-006). Touches: src/internal/api/provider_handler.go
      Proof: code src/internal/api/provider_handler.go respondUnsupportedEndpoint
- [x] T3.4 Add Anthropic tests for the messages path + `anthropic-version` header, the chat alias, and rejection of an unsupported shape (AC-004, AC-006). Touches: src/internal/adapters/provider/provider_test.go
      Proof: test src/internal/adapters/provider/provider_test.go TestAnthropicPathFidelity
- [x] T3.5 Add fallback and handler tests: zero provider calls + explicit 4xx for an OpenAI-only route receiving `/v1/messages`, and identical derived path across primary/fallback for Call and SSE (AC-006, AC-007). Touches: src/internal/domain/routing/service/service_test.go, src/internal/api/provider_handler_test.go
      Proof: test src/internal/api/provider_handler_test.go TestRoutingHandlerUnsupportedEndpoint

## Phase 4: Validation

Goal: prove the feature end to end and leave the package reviewable.

- [x] T4.1 Run the affected suites with the race detector and the full `make test`; fix any regression (SC-001, SC-002). Touches: src/internal/adapters/provider/, src/internal/api/, src/internal/domain/routing/service/
      Proof: chore Makefile test
- [x] T4.2 Document the `base_url` join rule (single `/v1`) in the config example so operators do not double the version segment (AC-005). Touches: README.md, examples/config.yaml
      Proof: docs README.md

## Acceptance Coverage

- AC-001 -> T1.2, T1.3, T2.1, T2.2, T2.3, T2.5
- AC-002 -> T1.2, T1.3, T2.1, T2.2, T2.3, T2.5
- AC-003 -> T2.1, T2.2, T2.3, T2.5
- AC-004 -> T3.1, T3.4
- AC-005 -> T1.2, T1.3, T2.5, T4.2
- AC-006 -> T3.1, T3.2, T3.3, T3.5
- AC-007 -> T3.2, T3.5
- AC-008 -> T1.1, T1.3, T2.4, T2.5
- AC-009 -> T1.2, T1.3

## Notes

- MVP is Phase 2; Phases 3–4 expand coverage without reworking the helper.
- Every `[x]` task needs a `Proof:` line on the next line (kind path [anchor]); no proof means not done.
- Order matters: Phase 1 helper/port must compile before adapter edits; sentinel must exist before Phase 3 handler mapping.
