# Provider Path Fidelity Plan

## Phase Contract

Inputs: `specs/active/provider-path-fidelity/spec.md` and targeted repo context.
Outputs: `plan.md` (data model and contracts noted inline).
Stop if: the spec is too vague to plan safely.

## Goal

Make the passthrough adapters (`openai`, `proxy`, `ollama`, `anthropic`) build their upstream URL from the incoming request path instead of hardcoded constants, through one shared, safety-checked composition helper. The api layer keeps its current middleware chain; only the `ProviderRequest` contract and the fallback error classification change. Gemini/Bedrock keep their translators untouched.

## MVP Slice

- `POST /v1/embeddings` and `POST /v1/completions` reach the matching provider endpoint on OpenAI-shaped providers, and `base_url` ending in `/v1` composes without a duplicated segment.
- Must satisfy `AC-001`, `AC-002`, `AC-005`.

## First Validation Path

1. Unit-test the new `provider.ResolveUpstreamURL` table: base URLs with/without `/v1`, each mounted path, hostile paths.
2. Run an adapter test with the existing mock egress that records the outbound URL, asserting `/v1/embeddings` and `/v1/completions`.
3. Manual: point an `openai`/`proxy` provider at a local OpenAI-compatible mock and `curl POST /v1/embeddings`.

## Scope

- New shared URL-composition + path-safety helper in the provider adapter package.
- Adapters `openai`, `proxy`, `ollama`, `anthropic` use it (Call and Stream).
- `ProviderRequest` gains an explicit query field; the handler populates it.
- Fallback classification + handler response for unsupported endpoint shapes.
- Untouched: routing selection/order, shield/usage/budget/export middleware, Gemini/Bedrock translators, DB schema, UI.

## Performance Budget

- URL composition: <1 ms p95, no additional network or DB calls.
- Allocations: no allocation growth beyond a small string concat per request; no new buffers in the hot path.
- State `none` for memory/throughput beyond the above — the change is path string handling.

## Implementation Surfaces

- New: `src/internal/adapters/provider/upstream.go` — `ResolveUpstreamURL` + path sanitizer + per-`api_type` support check.
- Modified: `src/internal/adapters/provider/{openai,proxy,ollama,anthropic}.go` — build request via the helper; propagate an error return.
- Modified: `src/internal/ports/provider.go` — add `RawQuery` field and `ErrUnsupportedEndpoint` sentinel.
- Modified: `src/internal/domain/routing/service/fallback.go` — continue the chain on unsupported-endpoint and surface the sentinel when exhausted.
- Modified: `src/internal/api/provider_handler.go` — set `RawQuery`, map the sentinel to an explicit 4xx error, log the chosen upstream path.
- Tests: new `upstream_test.go`; extend `provider_test.go`, `proxy_test.go`, `ollama_test.go`, `provider_handler_test.go`, and fallback tests.
- New surface justification: existing per-adapter URL strings have no shared seam, so a single helper is required to avoid duplicating safety/join logic four times.

## Bootstrapping Surfaces

- none — the provider adapter package, port struct, and tests already exist.

## Architecture Impact

- Local: provider adapters become path-driven; one new helper file.
- Cross-boundary: `ports.ProviderRequest` gains an additive field; `ports` gains a sentinel error. Existing implementations (Gemini/Bedrock/stub) ignore both.
- Compatibility: `base_url` with or without `/v1` composes correctly (no migration); mounted chat behavior unchanged; embeddings/completions change from broken to correct, which is the feature.

## Acceptance Approach

- AC-001 → OpenAI-shaped adapters resolve the incoming `/v1/embeddings`; surfaces `upstream.go` + adapters; proof: adapter test with recording mock asserts the outbound path, plus real-provider smoke.
- AC-002 → same helper resolves `/v1/completions`; surfaces `upstream.go` + adapters; proof: adapter test asserts `/v1/completions`.
- AC-003 → chat path passes through unchanged; surfaces `upstream.go` + adapters; proof: existing chat proxy tests stay green.
- AC-004 → Anthropic adapter keeps `/v1/messages` + `anthropic-version`; surfaces `anthropic.go`; proof: anthropic test asserts path and header.
- AC-005 → join rule de-duplicates `/v1`; surfaces `upstream.go`; proof: table test asserts `.../v1/embeddings` with no `v1/v1`.
- AC-006 → sentinel + fallback + handler mapping; surfaces `ports`, `fallback.go`, `provider_handler.go`; proof: handler test asserts explicit error and zero mock calls for an OpenAI-only route receiving `/v1/messages`.
- AC-007 → fallback/stream use the same derived path; surfaces `fallback.go` + adapters; proof: fallback test records the path for Call and SSE.
- AC-008 → `RawQuery` forwarded; surfaces `ports` + handler + helper; proof: recording mock asserts `foo=bar`.
- AC-009 → sanitizer rejects traversal/scheme-relative/absolute paths; surfaces `upstream.go`; proof: unit tests per hostile input; host always equals `base_url`.

## Data and Contracts

- Data model: no change — no entities, tables, or migrations.
- Contract change: `ports.ProviderRequest` adds `RawQuery string` (additive, backwards-compatible); `ports` adds `ErrUnsupportedEndpoint error`. No HTTP request/response schema change for clients; unsupported routes return the existing error envelope with a new machine code.
- Config: no schema change; `base_url` semantics documented in `Assumptions` of the spec.

## Implementation Strategy

- DEC-001 Central URL composition helper
  Why: one place owns the `/v1` de-dup and path safety, testable without a provider; four adapters stay thin; alternative (per-adapter URL builds) duplicates logic and safety checks. Not: a new interface/abstraction — a package function suffices.
  Tradeoff: adapters gain an error return path; `proxy`/`ollama` `buildRequest` change signature.
  Affects: `src/internal/adapters/provider/upstream.go`, four adapters.
  Validation: `upstream_test.go` table tests + adapter recording tests.
- DEC-002 Sentinel `ErrUnsupportedEndpoint` with fallback continue
  Why: keeps `api_type` support knowledge inside adapters, not the api layer; lets a mixed chain fall through to a compatible provider; ensures the handler returns an explicit error instead of generic 503. Not: pre-filtering providers in the handler (leaks adapter knowledge); not: reusing `NO_HEALTHY_PROVIDER`.
  Tradeoff: `fallback.go` gains one classification branch; a new exported sentinel widens the `ports` API.
  Affects: `src/internal/ports/provider.go`, `src/internal/domain/routing/service/fallback.go`, `src/internal/api/provider_handler.go`.
  Validation: fallback test (continue on unsupported) + handler test (explicit 4xx, zero calls).
- DEC-003 Explicit `RawQuery` field
  Why: keeps the path sanitizer free of query parsing and preserves query bytes exactly; alternative (embed `?query` in `URL`) complicates validation and risk of double-encoding. Not: reusing `Path`/`URL`.
  Tradeoff: additive port field touched by every adapter signature reader (ignored where unused).
  Affects: `src/internal/ports/provider.go`, `src/internal/api/provider_handler.go`, adapters.
  Validation: AC-008 recording test.
- DEC-004 Preserve Anthropic `chat/completions → /v1/messages` alias
  Why: existing deployments route chat to Anthropic providers; dropping the alias would be a silent breaking change beyond this feature's intent. Not: dropping the alias per the spec Open Question default.
  Tradeoff: one documented exception to strict path fidelity; Anthropic still rejects other shapes (e.g. `/v1/embeddings`).
  Affects: `src/internal/adapters/provider/anthropic.go`, spec Open Question resolution.
  Validation: anthropic tests assert alias path and embeddings rejection.
- DEC-005 Strict path sanitizer
  Why: the path originates from the client; rejecting `..`, `//`, `://`, and non-`/` prefixes prevents SSRF/host override; the upstream host is always `base_url`. Not: trusting `net/url` parsing alone.
  Tradeoff: marginally stricter than the minimal spec; no legitimate mounted path is affected.
  Affects: `src/internal/adapters/provider/upstream.go`.
  Validation: AC-009 unit tests.

## Incremental Delivery

### MVP (First Value)

- Add `upstream.go` + `RawQuery`; wire `openai`/`proxy`/`ollama`; keep existing tests green.
- MVP readiness: `AC-001`, `AC-002`, `AC-005` pass; `AC-003` regression test passes.

### Iterative Expansion

- Anthropic adapter via the helper with alias preservation → `AC-004`.
- Sentinel + fallback + handler mapping → `AC-006`, `AC-007`.
- Query forwarding + sanitizer coverage → `AC-008`, `AC-009`.

## Sequencing Notes

- `upstream.go` + `ports` field first (compile dependency for adapters).
- Adapter edits can proceed in parallel once the helper exists.
- Fallback/handler changes come after adapters return the sentinel, otherwise `AC-006` is untestable.
- No flag needed: the change only affects previously broken paths and preserves chat behavior.

## Risks

- Mixed-provider chain now advances on an unsupported shape, changing which provider serves a request.
  Mitigation: only triggers for shapes the previous provider cannot serve (previously a wrong-endpoint call); covered by an explicit fallback test.
- Existing tests/mocks assert the hardcoded `/v1/chat/completions` URL.
  Mitigation: update the recording mocks; `AC-003` pins the unchanged chat path.
- `base_url` edge cases (trailing slash, `/v1`, subpath).
  Mitigation: table-driven tests for each combination.
- Anthropic alias divergence from the spec's default Open Question answer.
  Mitigation: DEC-004 documents the rationale; revisit if the operator prefers an explicit error.

## Rollout and Compatibility

- No DB migration or backfill. Configs with or without `/v1` in `base_url` keep working.
- Behavior change is a fix for `/v1/embeddings` and `/v1/completions`; `/v1/chat/completions` and `/v1/messages` are unchanged.
- Operational follow-up: the chosen upstream path is logged and `X-Provider` remains the response header for observability.

## Validation

- Automated: `go test ./src/internal/adapters/provider/... ./src/internal/api/... ./src/internal/domain/routing/...` plus `make test` before archive.
- New tests: `upstream_test.go` (composition + sanitizer), adapter recording tests (`AC-001/002/004/008`), handler test (`AC-006`), fallback test (`AC-007`).
- Manual: local OpenAI-compatible mock + `curl POST /v1/embeddings` and `/v1/completions`.
- Proves: `AC-001`–`AC-009`; `DEC-001`–`DEC-005`.

## Constitution Compliance

- no conflicts. Preserves native-only data plane, tenant-driven policy chain, and passthrough (no translation) posture; no weakening of Content Shield.
