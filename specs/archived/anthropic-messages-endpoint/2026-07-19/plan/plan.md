# Anthropic Native Messages Endpoint — План

## Phase Contract

Inputs: spec, repo context (provider_handler.go, server.go, anthropic.go, ports/provider.go).
Outputs: plan, data-model.
Stop if: нет.

## Цель

Зарегистрировать `/api/v1/messages` на gateway, добавить поле `Path` в `ProviderRequest`, обновить `AnthropicClient` для passthrough-режима при `Path == "/v1/messages"`.

**Ключевое наблюдение:** `AnthropicClient` уже работает как passthrough (не конвертирует формат). Основная работа — API-слой: новый роут + передача path.

## MVP Slice

Один инкремент: все AC закрываются одним набором изменений.

## First Validation Path

```bash
go build ./...
go test -race -count=1 ./src/internal/adapters/provider/...
# ручная проверка: curl -X POST http://localhost:8080/api/v1/messages \
#   -H "Authorization: Bearer sk-test" \
#   -d '{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"hi"}]}'
```

## Scope

- `src/internal/ports/provider.go` — добавить поле `Path` в `ProviderRequest`
- `src/internal/api/server.go` — зарегистрировать `POST /api/v1/messages` (+ `/v1/messages` → 301 redirect)
- `src/internal/api/provider_handler.go` — передавать `c.Request.URL.Path` в `ProviderRequest.Path`; переиспользовать `HandleChatCompletion` для обоих путей
- `src/internal/adapters/provider/anthropic.go` — при `req.Path == "/v1/messages"` не менять поведение (уже passthrough), при `/v1/chat/completions` — то же самое (обратная совместимость)
- Тесты handler + anthropic адаптера

**Вне scope:** shield/usage middleware не требуют изменений (применяются на уровне группы роутов).

## Performance Budget

`none` — добавление поля Path не создаёт аллокаций на горячем пути; новый роут использует те же middleware.

## Implementation Surfaces

| Surface | Зачем меняется | Тип |
|---------|----------------|-----|
| `src/internal/ports/provider.go` | + поле `Path string` в `ProviderRequest` | существующая |
| `src/internal/api/server.go` | + `primary.POST("/messages", chain...)` + `/v1/messages` → 301 redirect | существующая |
| `src/internal/api/provider_handler.go` | заполнять `Path` из `c.Request.URL.Path`; извлечь логику выбора path для upstream | существующая |
| `src/internal/adapters/provider/anthropic.go` | проверить `req.Path` для явного passthrough (сейчас уже passthrough, но неявно) | существующая |

## Bootstrapping Surfaces

`none` — все файлы существуют.

## Влияние на архитектуру

**Локальное.** Поле `Path` добавляется в порт, но все существующие адаптеры его игнорируют (кроме AnthropicClient). Routing/fallback/selector не меняются.

## Acceptance Approach

| AC | Подход | Surfaces | Наблюдение |
|----|--------|----------|------------|
| AC-001 | server_test.go: проверить что POST `/api/v1/messages` не 404; middleware chain intact | server.go | go test |
| AC-002 | anthropic_test.go: `TestAnthropicClient_NativePassthrough` — тело отправляется как есть, ответ возвращается как есть | anthropic.go | go test |
| AC-003 | provider_handler_test.go: проверить `ProviderRequest.Path` после вызова handler | provider_handler.go | go test |
| AC-004 | Существующие `TestAnthropicClient_Call` и `TestAnthropicClient_Stream` проходят | anthropic.go | go test |
| AC-005 | `go build ./...` + все существующие тесты провайдеров проходят | все | go build + go test |
| AC-006 | shield middleware включён в цепочку `/api/v1/messages` — проверить через server_test.go | server.go | go test |

## Данные и контракты

См. `data-model.md`. Единственное изменение: поле `Path` в `ports.ProviderRequest`.

## Стратегия реализации

### DEC-001: Path как явный сигнал, а не авто-детекция

Why: детекция формата тела (Anthropic vs OpenAI) по content — ненадёжна (оба используют JSON с `messages`). Path — стабильный признак от клиентского SDK.

Tradeoff: требуется coordination между handler и adapter. Не работает, если клиент шлёт Anthropic-формат на `/chat/completions` (но это нестандартно).

Affects: ports/provider.go, provider_handler.go, anthropic.go
Validation: unit test проверяет Path

### DEC-002: Один handler для обоих путей

Why: routing/fallback/streaming логика идентична. Разница только в `Path` поле → adapter решает passthrough или convert.

Tradeoff: handler должен парсить `model` из тела (работает для обоих форматов — оба имеют `{"model":..., "stream":...}`).

Affects: server.go (регистрация двух роутов на один handler), provider_handler.go
Validation: оба пути обрабатываются без 404

### DEC-003: AnthropicClient уже passthrough — изменения минимальны

Why: текущий `AnthropicClient.Call/Stream` форвардит `req.Body` как есть в Anthropic API и возвращает ответ как есть. Никакой конвертации формата нет. Изменение: явная проверка `req.Path` для документации контракта.

Tradeoff: если в будущем AnthropicClient начнёт конвертировать OpenAI→Anthropic (`/chat/completions`), Path станет единственным разделителем режимов.

Affects: anthropic.go
Validation: существующие тесты проходят

## Incremental Delivery

### MVP (Первая ценность)

Весь scope — один инкремент. Новый роут + Path + Anthropic passthrough.

Критерий: `go build ./...`, все тесты проходят.

## Порядок реализации

1. `ports/provider.go` — добавить `Path` (нет зависимостей)
2. `provider_handler.go` — заполнять `Path` из запроса
3. `server.go` — зарегистрировать `/api/v1/messages`
4. `anthropic.go` — явная проверка Path (опционально, т.к. уже passthrough)
5. Тесты

## Риски

| Риск | Mitigation |
|------|------------|
| Shield middleware обходится через новый path | Одна middleware chain на оба роута (см. server.go: `chain` применяется одинаково к `/chat/completions` и `/messages`) |
| Handler парсит body только для `model` + `stream` — Anthropic SDK может добавить поля, которые изменят поведение | Handler игнорирует неизвестные поля (json.Unmarshal не строгий). Все поля остаются в `body` для upstream. |

## Rollout и compatibility

- Новый роут — обратно совместим (старые клиенты продолжают использовать `/api/v1/chat/completions`)
- Поле `Path` — опционально, zero-value для существующего кода
- Feature flag не требуется

## Проверка

- `go build ./...` — компиляция
- `go test -race -count=1 ./src/internal/adapters/provider/...` — адаптер тесты
- `go test -race -count=1 ./src/internal/api/...` — handler + server тесты
- `go vet ./...` — статический анализ

## Соответствие конституции

Нет конфликтов. Изменения только в API слое и порте; core domain (Content Shield) не меняется.
