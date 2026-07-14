# 112 Proxy Streaming Wiring — Задачи

## Phase Contract

Inputs: plan.md, data-model.md (no-change), repo context (provider_handler.go, fallback.go, server.go, tests)
Outputs: tasks.md с покрытием всех AC
Stop if: — все AC привязываются к исполнимых задачам.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/domain/routing/service/fallback.go` | T1.1 |
| `src/internal/domain/routing/service/service_test.go` | T1.2 |
| `src/internal/api/provider_handler.go` | T2.1, T2.3 |
| `src/internal/api/middleware/sse.go` | T2.2 |
| `src/internal/api/server.go` | T2.2 |
| `src/internal/api/provider_handler_test.go` | T3.1 |

## Implementation Context

- Цель MVP: проксировать SSE-поток от провайдера до клиента при `"stream": true` в теле запроса
- Инварианты/семантика:
  - `chatRequest.Stream == true` → ветка `Stream()`; иначе → существующая ветка `Call()`
  - FallbackHandler.Stream() перебирает провайдеров последовательно, без переключения в середине стрима
  - `ProviderChunk.Done == true` завершает стрим; `ProviderChunk.Err != nil` пишет ошибку и закрывает
  - Клиентский контекст (`c.Request.Context()`) управляет upstream — при cancel стрим прерывается
- Ошибки/коды:
  - ошибка в стриме → `data: {"error":{"message":"<err>"}}\n\n` + закрытие (OpenAI-совместимый формат)
  - пустой стрим → `data: [DONE]\n\n`
  - upstream cancellation → стрим закрывается без дополнительного сообщения
- Контракты/протокол:
  - `WrapSSE()` middleware: устанавливает `Content-Type: text/event-stream`, `Transfer-Encoding: chunked`, флашит ResponseWriter
  - формат SSE: `data: <json>\n\n`
  - первый чанк — metadata `{"provider":"<name>","headers":{...}}`
- Границы scope: не делаем Content Shield scan streaming-чанков, не делаем gRPC streaming, не делаем переключение провайдера в середине стрима
- Proof signals: curl `-N` с `stream: true` возвращает SSE-поток; все non-streaming тесты проходят
- References: DEC-001 (middleware), DEC-002 (fallback паттерн), DEC-003 (metadata-чанк)

## Фаза 1: FallbackHandler.Stream

Цель: добавить метод Stream() в FallbackHandler — последовательный перебор провайдеров с isRetriableError.

- [x] T1.1 Реализовать FallbackHandler.Stream() — сигнатура `Stream(ctx context.Context, providers []string, req *ports.ProviderRequest) (<-chan ports.ProviderChunk, string, error)`, перебирает провайдеры, вызывает `client.Stream()`, возвращает канал + имя провайдера. Touches: `src/internal/domain/routing/service/fallback.go`
- [x] T1.2 Добавить unit-тесты FallbackHandler.Stream(): успешный стрим, fallback A→B при ошибке A, все провайдеры недоступны. Touches: `src/internal/domain/routing/service/service_test.go`

## Фаза 2: chatRequest + WrapSSE middleware

Цель: подготовить handler-слой к streaming — детекция запроса и middleware для SSE-формата.

- [x] T2.1 Добавить поле `Stream bool` в `chatRequest` struct. Touches: `src/internal/api/provider_handler.go`
- [x] T2.2 Создать `WrapSSE()` middleware в `src/internal/api/middleware/sse.go`: устанавливает `Content-Type: text/event-stream`, `Transfer-Encoding: chunked`, вызывает `c.Writer.Flush()`. Зарегистрировать на роуте `/v1/chat/completions` перед HandleChatCompletion в `RegisterProxyRoute`. Touches: `src/internal/api/middleware/sse.go`, `src/internal/api/server.go`
- [x] T2.3 Написать unit-тест для WrapSSE (заголовки после прохождения middleware). Touches: `src/internal/api/middleware/middleware_test.go`
- [x] T3.1 Реализовать streaming-ветку в HandleChatCompletion:
  1. при `chatRequest.Stream == true` вызывает `h.fallback.Stream()`
  2. в `c.Stream()` (Gin streaming writer) форвардит чанки: metadata-сообщение с X-Provider, затем `data: <chunk.Data>\n\n`
  3. при `chunk.Done` — `data: [DONE]\n\n` + закрытие
  4. при `chunk.Err != nil` — `data: {"error":{"message":"<err>"}}\n\n` + закрытие
  5. cancellation через ctx.Done() прерывает цикл и закрывает стрим
  Touches: `src/internal/api/provider_handler.go`
- [x] T4.1 Написать интеграционные тесты в `provider_handler_test.go`:
  - streaming с 2 чанками mock-провайдера → проверка SSE-body (AC-003)
  - cancellation → проверка отмены upstream (AC-004)
  - ошибка в чанке → проверка SSE error сообщения (AC-005)
  - non-streaming запрос (`stream: false`) → поведение не изменилось (AC-001, SC-002)
  Touches: `src/internal/api/provider_handler_test.go`

## Покрытие критериев приемки

- AC-001 → T2.1, T4.1
- AC-002 → T2.2, T2.3
- AC-003 → T3.1, T4.1
- AC-004 → T3.1, T4.1
- AC-005 → T3.1, T4.1
- AC-006 → T1.1, T1.2

## Заметки

- T1.1 и T1.2 независимы от T2.x; можно выполнять параллельно.
- T2.1 тривиален (одно поле) — можно объединить с T3.1 на implement.
- mockProviderClient в service_test.go уже имеет Stream()-заглушку; mockPortClient в provider_handler_test.go — тоже.
- WrapSSE middleware регистрируется только на streaming-роут, не затрагивая существующие middleware.
