# 112 Proxy Streaming Wiring

## Scope Snapshot

- In scope: проксирование SSE-потока от LLM-провайдера до клиента через gateway, включая детекцию streaming-запроса, форвардинг чанков и обработку cancellation.
- Out of scope: изменение протокола провайдеров, buffering-стратегии, трансформация содержимого чанков (Content Shield scan streaming будет отдельной фичей).

## Цель

Пользователи gateway (LLM-клиенты) получают токены по SSE в реальном времени при `stream: true` в теле запроса. Сейчас HandleChatCompletion вызывает только `Call()` и возвращает полный ответ — streaming-запросы либо игнорируют параметр `stream`, либо падают. После фичи клиент получает `text/event-stream` с чанками от первого здорового провайдера, cancellation от клиента прерывает upstream-запрос. Фича считается успешной, когда существующие интеграционные тесты покрывают streaming-сценарий и клиент получает корректный SSE-поток.

## Основной сценарий

1. Клиент отправляет POST `/v1/chat/completions` с `"stream": true` в JSON body.
2. RoutingProxyHandler детектирует streaming-запрос по полю `Stream bool` в `chatRequest`.
3. Middleware `WrapSSE()` устанавливает заголовки `Content-Type: text/event-stream` и `Transfer-Encoding: chunked`, включает flush-per-chunk.
4. `FallbackHandler.Stream()` проходит по списку провайдеров, вызывает `client.Stream()` на первом здоровом.
5. RoutingProxyHandler форвардит чанки через `gin.Context.Stream()` — каждый `ProviderChunk.Data` пишется в виде SSE-сообщения `data: <chunk>\n\n`.
6. При получении `ProviderChunk.Done == true` стрим закрывается штатно.
7. При cancellation (клиент разорвал соединение) контекст отменяется, upstream-запрос прерывается, стрим закрывается.

## User Stories

- P1 (потокобезопасный streaming): разработчик интеграции отправляет `stream: true` и получает SSE-поток через gateway без потери токенов и без зависаний.
- P2 (graceful degradation): при падении первого провайдера в середине стрима, fallback переключается на следующего без видимого разрыва для клиента (best effort — ошибка пишется в стрим, если переключение невозможно).

## MVP Slice

Базовый streaming: детекция `stream: true` → выбор провайдера → `FallbackHandler.Stream()` → форвардинг чанков через `gin.Context.Stream()` → обработка `Done`. Закрывает AC-001, AC-002, AC-003, AC-004. P2 (fallback внутри стрима) выносится в следующий срез.

## First Deployable Outcome

После первого implementation pass можно отправить curl-запрос с `"stream": true` и наблюдать потоковые токены в терминале без потери соединения. Наличие `X-Provider` header с именем провайдера в первом чанке.

## Scope

- Добавление поля `Stream bool` в `chatRequest` struct
- `Middleware.WrapSSE()` — установка SSE-заголовков, flush-per-chunk
- RoutingProxyHandler.HandleChatCompletion: ветвление на `stream: true` → вызов `h.fallback.Stream()` → форвардинг через `c.Stream()`
- `FallbackHandler.Stream()` — последовательный перебор провайдеров, вызов `client.Stream()`
- Обработка cancellation: клиентский контекст отменяется → контекст провайдера отменяется
- Обработка ошибок в середине стрима: `ProviderChunk.Err != nil` → пишем SSE-сообщение с ошибкой, закрываем стрим
- Форвардинг upstream headers (`X-Provider`, etc.) в первый SSE-чанк (как metadata перед data)
- Интеграционные тесты streaming-сценария в `provider_handler_test.go`

## Контекст

- `ProviderClient` интерфейс уже содержит `Stream(ctx, req) (<-chan ProviderChunk, error)`
- `ProviderChunk` содержит `Data []byte`, `Err error`, `Done bool`
- OpenAI и Anthropic адаптеры уже реализуют `Stream()` с SSE-парсингом
- Egress-клиент (`adapters/egress/client.go`) содержит `streamSSE()` — реальная реализация
- `FallbackHandler` существует только с `Call()` — нужно добавить `Stream()`
- Gateway использует Gin — `c.Stream()` и `c.Request.Context()` доступны
- chatRequest сейчас содержит только `Model string` — добавить `Stream bool`

## Зависимости

- Зависит от интерфейса `ProviderClient.Stream()` (реализован в 71-egress-streaming)
- `specs/active/70-routing-engine/` — FallbackHandler и RouteSelector (существуют)

## Требования

- RQ-001 Gateway ДОЛЖЕН детектировать streaming-запрос по полю `"stream": true` в JSON теле запроса.
- RQ-002 Gateway ДОЛЖЕН устанавливать `Content-Type: text/event-stream` и `Transfer-Encoding: chunked` для streaming-ответов.
- RQ-003 Gateway ДОЛЖЕН форвардить каждый чанк от провайдера клиенту в формате `data: <chunk>\n\n`.
- RQ-004 Gateway ДОЛЖЕН отменять upstream-контекст при разрыве соединения клиентом.
- RQ-005 Gateway ДОЛЖЕН записывать ошибку провайдера в стрим (SSE-сообщение с `error`) и закрывать стрим при ошибке в середине потока.
- RQ-006 FallbackHandler ДОЛЖЕН поддерживать `Stream()` с последовательным перебором провайдеров (best-effort, без переключения в середине стрима).

## Вне scope

- Content Shield scanning streaming-чанков (будет в отдельной фиче Shield + streaming)
- Retry/logic переключения провайдера в середине активного стрима (при падении — ошибка в стрим, fallback только до начала стрима)
- Трансформация/модификация содержимого SSE-сообщений
- gRPC streaming (только HTTP/1.1 SSE)
- buffering-стратегии (chunk размер, backpressure)

## Критерии приемки

### AC-001 Детекция streaming-запроса

- Почему это важно: без детекции gateway не может выбрать ветку Stream() vs Call()
- **Given** POST-запрос к `/v1/chat/completions` с `"stream": true` в JSON body
- **When** RoutingProxyHandler.HandleChatCompletion парсит body
- **Then** `chatRequest.Stream == true`
- Evidence: unit-тест, где `chatRequest` десериализуется с `stream: true` и `Stream` поле установлено в `true`

### AC-002 WrapSSE middleware устанавливает корректные заголовки

- Почему это важно: клиент должен понимать, что ответ — SSE-поток
- **Given** запрос, проходящий через `Middleware.WrapSSE()`
- **When** middleware отрабатывает
- **Then** `Content-Type: text/event-stream` и `Transfer-Encoding: chunked` установлены в ResponseWriter
- Evidence: тест проверяет заголовки ответа после прохождения middleware

### AC-003 Streaming-запрос форвардит чанки провайдера клиенту

- Почему это важно: основная ценность — клиент получает токены по SSE
- **Given** настроенный RoutingProxyHandler с mock-провайдером, который возвращает 2 чанка через Stream()
- **When** HandleChatCompletion получает запрос с `"stream": true`
- **Then** клиент получает `data: <chunk1>\n\ndata: <chunk2>\n\n` с `Content-Type: text/event-stream`
- Evidence: интеграционный тест проверяет body ответа как SSE-поток с ожидаемыми чанками

### AC-004 Cancellation клиентом прерывает upstream

- Почему это важно: предотвращает утечку ресурсов при отключении клиента
- **Given** активный streaming-запрос, провайдер шлёт чанки
- **When** клиент разрывает соединение
- **Then** upstream-контекст отменяется, стрим закрывается без отправки остатка
- Evidence: тест с context cancel, проверяющий что `Stream()` на провайдере получил cancelled context

### AC-005 Ошибка в середине стрима пишется в поток

- Почему это важно: клиент должен узнать об ошибке, а не получить оборванный стрим без объяснения
- **Given** активный streaming-запрос
- **When** провайдер возвращает `ProviderChunk` с `Err != nil`
- **Then** в стрим пишется `data: {"error": "<err>"}\n\n` и стрим закрывается
- Evidence: тест проверяет, что при ошибке в чанке клиент получает SSE-сообщение с error и стрим завершается

### AC-006 FallbackHandler.Stream перебирает провайдеров

- Почему это важно: resilience — при недоступности первого провайдера gateway пробует следующих
- **Given** список провайдеров `[A, B]`, где A.Stream() возвращает ошибку соединения
- **When** FallbackHandler.Stream() вызывается с этим списком
- **Then** вызывается B.Stream(), и клиент получает чанки от B
- Evidence: unit-тест с mock-провайдерами, где A.Stream() возвращает ошибку, B.Stream() возвращает чанки

## Допущения

- Клиент устанавливает заголовки для SSE корректно (или не устанавливает — gateway всё равно шлёт `text/event-stream`)
- `ProviderChunk.Done == true` — единственный штатный способ завершения стрима
- Размер чанка не требует фрагментации на уровне gateway
- Все провайдеры возвращают SSE в формате OpenAI-совместимого API (`data: {...}\n\n`)
- Fallback происходит только ДО начала стрима; переключение в середине — не требуется

## Критерии успеха

- SC-001 Время до первого токена (TTFT) при streaming через gateway не более чем на 10% выше, чем прямой вызов провайдера (измеряется в интеграционном тесте с реальным/заглушечным провайдером).
- SC-002 Все существующие non-streaming тесты продолжают проходить без изменений.

## Краевые случаи

- Пустой стрим (провайдер сразу вернул `Done`): gateway закрывает стрим с `data: [DONE]\n\n`
- Запрос с `"stream": false` или без поля `stream` — поведение не меняется (идёт через Call)
- Провайдер не поддерживает streaming (нет реализации Stream или падает с ошибкой) — FallbackHandler.Stream пробует следующего
- Двойной `"stream": true` + `"stream": false` — валидация: последнее значение побеждает (стандартное поведение JSON unmarshal)
- Гигантский чанк (>1MB) — передаётся как есть; фрагментация не требуется
- Клиент закрыл соединение до первого чанка — upstream-контекст отменяется, стрим не начинается

## Открытые вопросы

1. Нужен ли heartbeat/keepalive (комментарий `: keepalive\n\n`) при долгом отсутствии чанков? — Если да, то какой интервал? Решение: пока `none`, heartbeat будет добавлен при необходимости в отдельной фиче.
2. Формат error-сообщения в SSE: достаточно ли `data: {"error": "..."}\n\n` или нужен OpenAI-совместимый `data: {"error": {"message": "..."}}\n\n`? — Решение: OpenAI-совместимый формат для совместимости с клиентами.
