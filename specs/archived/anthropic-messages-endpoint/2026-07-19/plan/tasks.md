# Anthropic Native Messages Endpoint — Задачи

## Phase Contract

Inputs: spec (6 AC), plan (3 DEC), data-model.md.
Outputs: упорядоченные исполнимые задачи с покрытием всех 6 AC.
Stop if: coverage не удаётся сопоставить.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/ports/provider.go` | T1.1 |
| `src/internal/api/provider_handler.go` | T2.1, T2.2 |
| `src/internal/api/server.go` | T3.1 |
| `src/internal/adapters/provider/anthropic.go` | T3.2 |
| Тесты (anthropic_test.go, provider_handler_test.go, server_test.go) | T4.1 |

## Implementation Context

- Цель MVP: зарегистрировать `/api/v1/messages`, добавить `Path` в `ProviderRequest`, AnthropicClient явно читает `Path` (passthrough без изменений поведения)
- Границы приемки: AC-001 (роут), AC-002 (passthrough), AC-003 (Path от handler), AC-004 (обратная совместимость), AC-005 (другие провайдеры), AC-006 (shield)
- Ключевые правила: Path — явный сигнал (не auto-detect, DEC-001); один handler для обоих путей (DEC-002); AnthropicClient уже passthrough — изменения минимальны (DEC-003)
- Инварианты данных: `Path` не влияет на routing/selector/fallback; все не-Anthropic адаптеры игнорируют Path
- Контракты: `Path == "/api/v1/messages"` и `Path == "/api/v1/chat/completions"` — оба passthrough в AnthropicClient; upstream URL в AnthropicClient всегда `/v1/messages` (hardcoded, не зависит от `req.URL`)
- Proof signals: `go build ./...`, `go test -race -count=1 ./src/internal/...`
- Вне scope: конвертация форматов, авто-детекция тела, поддержка native-форматов других провайдеров

## Фаза 1: Data model

Цель: добавить поле `Path` в порт.

- [x] T1.1 Добавить `Path string` в `ports.ProviderRequest`. Touches: `src/internal/ports/provider.go`. AC: AC-003, AC-005. RQ: RQ-002.

## Фаза 2: Handler — propagation Path

Цель: `RoutingProxyHandler` заполняет `Path` из gin request и устанавливает правильный upstream `URL` по пути.

- [x] T2.1 В `HandleChatCompletion` установить `providerReq.Path = c.Request.URL.Path`. Touches: `src/internal/api/provider_handler.go`. AC: AC-003. RQ: RQ-002. DEC: DEC-001.

- [x] T2.2 Извлечь upstream path из request path (strip `/api` prefix) вместо хардкода `/v1/chat/completions`. Touches: `src/internal/api/provider_handler.go`. AC: AC-001. DEC: DEC-002.

## Фаза 3: Server + AnthropicClient

Цель: зарегистрировать новый роут и документировать контракт Path в AnthropicClient.

- [x] T3.1 Зарегистрировать `POST /api/v1/messages` с той же middleware chain (auth → shield → usage) и `GET /v1/messages` → 301 redirect. Touches: `src/internal/api/server.go`. AC: AC-001, AC-006.

- [x] T3.2 Добавить в `AnthropicClient.Call/Stream` явную проверку `req.Path` — контракт, что passthrough работает для обоих путей. Touches: `src/internal/adapters/provider/anthropic.go`. AC: AC-002, AC-004. RQ: RQ-003, RQ-004, RQ-005.

## Фаза 4: Проверка

Цель: тесты доказывают все 6 AC.

- [x] T4.1 Добавить тесты: `TestHandlerPathField` (Path propagation), `TestMessagesEndpointRegistered` (200 not 404), `TestMessagesRedirectFromV1` (301). Существующие тесты всех провайдеров проходят без изменений. Touches: `src/internal/api/server_test.go`, `src/internal/api/provider_handler_test.go`. AC: AC-001, AC-002, AC-003, AC-004, AC-006.

- [x] T4.2 Выполнить `go build ./...`, `go vet ./...`, `go test -race -count=1 ./internal/...`. Touches: Makefile. AC: AC-005.

## Покрытие критериев приемки

- AC-001 -> T3.1, T4.1, T4.2
- AC-002 -> T3.2, T4.1
- AC-003 -> T1.1, T2.1, T4.1
- AC-004 -> T3.2, T4.1
- AC-005 -> T1.1, T4.2
- AC-006 -> T3.1, T4.1

## Заметки

- AnthropicClient уже passthrough — T3.2 минимален (добавить явную проверку Path)
- T2.2 меняет хардкод `URL: "/v1/chat/completions"` на динамический upstream path из request path
- Фаза 3 не зависит от Фазы 1/2 для компиляции, но фазы упорядочены по логике задачи
- Все 9 существующих тестов провайдеров должны проходить без изменений (go test -race)
