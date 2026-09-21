---
report_type: verify
slug: embeddings-passthrough
status: pass
docs_language: en
generated_at: 2026-09-21
---

# Verify Report: embeddings-passthrough

## Scope

- snapshot: an OpenAI-compatible embeddings endpoint masks the request input per tenant policy before the provider call, returns vectors unchanged, and accounts tokens/spend like chat.
- verification_mode: default
- artifacts:
  - CONSTITUTION.md
  - specs/active/embeddings-passthrough/spec.md
  - specs/active/embeddings-passthrough/plan.md
  - specs/active/embeddings-passthrough/tasks.md
- inspected_surfaces:
  - src/internal/api/middleware/shield_embeddings.go (+ test)
  - src/internal/api/server.go
  - src/cmd/gateway/run.go, src/cmd/all/gateway.go
  - src/internal/api/server_test.go, src/internal/api/provider_handler_test.go
  - README.md, examples/config.yaml

## Verdict

- status: pass
- archive_readiness: safe
- summary: all 8 tasks carry `Proof:` lines, 10 embeddings tests pass, every AC maps to passing evidence, and the chat shield path was not modified.

## Checks

- task_state: completed=8, open=0 (verify-task-state.sh: `OK: all tasks are marked complete`; `PROOFS_MISSING=0`).
- acceptance_evidence:
  - AC-001 -> `TestEmbeddingsRouteParityAndAuth` (both prefixes 200 with key, 401 without); wiring `RegisterEmbeddingsShield` in server.go and both binaries.
  - AC-002 -> `TestEmbeddingsShieldMasksDictionary`, `TestEmbeddingsShieldMasksPII`, `TestEmbeddingsShieldArrayInput`, `TestEmbeddingsShieldDoesNotUnmaskResponse`.
  - AC-003 -> `TestEmbeddingsShieldBlocks` (403, `X-Shield-Status: blocked`, handler not called).
  - AC-004 -> `TestEmbeddingsShieldRejectsUnsupportedInput` (token array, empty string, empty array, number → 400, no provider call).
  - AC-005 -> `TestEmbeddingsUsageAccounting` (1000 prompt tokens, output 0, input-only cost 2); budget stage runs on `/embeddings` in `TestEmbeddingsRouteParityAndAuth`.
  - AC-006 -> `TestRoutingHandlerEmbeddingsFallback` (fallback provider, upstream path `/v1/embeddings`), `TestRoutingHandlerEmbeddingsNoRoute` (400 `NO_ROUTE`).
- implementation_alignment:
  - `EmbeddingsShieldMiddleware` parses `{model,input}`, rejects non-text input, applies tenant dictionaries then PII rules, rewrites only `input`, and never wraps the response writer.
  - `RegisterProxyRoute` mounts `/embeddings` on `/api/v1` and `/v1` only when the embeddings shield is registered, with model-access, usage and budget but without chat-only session/conversation/cache/SSE stages.
  - The routing proxy handler is reused unchanged and derives `/v1/embeddings` from the request path.

## Errors

- none

## Warnings

- none

## Questions

- none

## Not Verified

- Live provider embeddings call: no network call was made, so real provider embeddings responses (including providers that omit `usage`) were not observed end-to-end.
- Budget spend increment for an embeddings request through the real `BudgetMiddleware` at the route level was not asserted directly; the budget stage is proven to run on `/embeddings`, and the budget arithmetic is covered by the shared middleware tests.
- The chat shield path was inspected for non-interference (no edits), but a full chat regression suite was not re-run beyond the package tests.

## Next Step

- safe to archive
