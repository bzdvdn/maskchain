---
report_type: verify
slug: anthropic-messages-endpoint
status: pass
docs_language: ru
generated_at: 2026-07-19
---

# Verify Report: anthropic-messages-endpoint

## Scope

- snapshot: проверка регистрации `/api/v1/messages`, поля `Path` в `ProviderRequest`, passthrough в AnthropicClient, обратной совместимости
- verification_mode: default
- artifacts:
  - CONSTITUTION.md (summary)
  - specs/active/anthropic-messages-endpoint/spec.md
  - specs/active/anthropic-messages-endpoint/plan.md
  - specs/active/anthropic-messages-endpoint/tasks.md
  - specs/active/anthropic-messages-endpoint/data-model.md
- inspected_surfaces:
  - src/internal/ports/provider.go (ProviderRequest.Path)
  - src/internal/api/provider_handler.go (Path propagation, upstream URL derivation)
  - src/internal/api/server.go (route registration, 301 redirect)
  - src/internal/adapters/provider/anthropic.go (explicit Path check)
  - src/internal/api/server_test.go (TestMessagesEndpointRegistered, TestMessagesRedirectFromV1)
  - src/internal/api/provider_handler_test.go (TestHandlerPathField)
  - provider tests (TestAnthropicClient_Call/Stream, etc.)

## Verdict

- status: pass
- archive_readiness: safe
- summary: все 6 AC подтверждены тестами, обратная совместимость сохранена, все 40 тестов проходят с -race

## Checks

- task_state: completed=7, open=0
- acceptance_evidence:

| AC-ID | Task IDs | Evidence | Verdict |
|-------|----------|----------|---------|
| AC-001 | T3.1, T4.1, T4.2 | `TestMessagesEndpointRegistered` (PASS, not 404), `TestMessagesRedirectFromV1` (PASS, 301), `server.go:96,115,124` | pass |
| AC-002 | T3.2, T4.1 | `TestAnthropicClient_Call` (PASS), `TestAnthropicClient_Stream` (PASS) — passthrough behaviour, `anthropic.go:53,90` — explicit Path check | pass |
| AC-003 | T1.1, T2.1, T4.1 | `TestHandlerPathField` (PASS) — Path="/api/v1/messages" и "/api/v1/chat/completions", URL="/v1/messages" и "/v1/chat/completions", `provider.go:11`, `provider_handler.go:100,138` | pass |
| AC-004 | T3.2, T4.1 | `TestAnthropicClient_Call` (PASS), `TestAnthropicClient_Stream` (PASS) — без изменений, passthrough как и было | pass |
| AC-005 | T1.1, T4.2 | `go build ./...` OK, all 18 provider tests pass (OpenAI, Gemini, Bedrock, Proxy, Anthropic), zero-value Path ignored by non-Anthropic adapters | pass |
| AC-006 | T3.1, T4.1 | `server.go:96,115` — `/messages` uses same `chain` as `/chat/completions` (includes shieldMiddleware), `TestMessagesEndpointRegistered` (PASS) — middleware intact | pass |

- implementation_alignment:
  - T1.1: `src/internal/ports/provider.go:11` — +`Path string` field
  - T2.1+2.2: `src/internal/api/provider_handler.go:87,100,104,135,138` — `upstreamPath` derivation, `Path` field set
  - T3.1: `src/internal/api/server.go:96,115,124` — route + redirect
  - T3.2: `src/internal/adapters/provider/anthropic.go:53,90` — `_ = req.Path` contract
  - T4.1: 3 new tests (21 total in api), all PASS

## Errors

- none

## Warnings

- AnthropicClient уже был passthrough до изменений — T3.2 добавил только явную документацию контракта. Поведение не изменилось.

## Questions

- нет

## Not Verified

- Полная end-to-end интеграция с реальным Anthropic API (зависит от API-ключа и сети) — не требуется для unit-verification.
- Shield middleware `TestMessagesEndpointRegistered` использует `routingHandler=nil` (legacy handler) — при `routingHandler!=nil` chain идентичен, т.к. код использует ту же переменную `chain`.

## Next Step

- safe to archive
