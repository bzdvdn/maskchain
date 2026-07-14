# 112 Proxy Streaming Wiring — План

## Phase Contract

Inputs: spec, inspect (pass), repo context (provider_handler.go, fallback.go, ports/provider.go, middleware/*)
Outputs: plan.md, data-model.md (no-change)
Stop if: spec расплывчата — spec прозрачна, inspect pass.

## Цель

Добавить streaming-ветку в RoutingProxyHandler.HandleChatCompletion. Работа полностью локальна: api-слой (handler, middleware), domain routing service (FallbackHandler), без изменений persistence, инфраструктуры, протокола провайдеров. Безопасна — non-streaming path не трогается.

## MVP Slice

Базовый streaming (AC-001, AC-002, AC-003, AC-004): детекция → WrapSSE middleware → FallbackHandler.Stream() → форвардинг чанков → cancellation. AC-005 (ошибка в стриме) и AC-006 (fallback-перебор) входят в первый же implementation pass, т.к. они — часть того же Handler-цикла.

## First Validation Path

curl-запрос с `"stream": true` к локальному gateway (stub-провайдер) → в терминале видны `data: {...}\n\n` чанки. `curl -N -X POST ... -d '{"model":"gpt-4","stream":true}'`

## Scope

- `src/internal/api/provider_handler.go` — chatRequest struct, HandleChatCompletion branching
- `src/internal/api/middleware/` — новый файл sse.go с WrapSSE()
- `src/internal/domain/routing/service/fallback.go` — новый метод Stream()
- `src/internal/api/provider_handler_test.go` — новые тесты
- `src/internal/domain/routing/service/service_test.go` — тесты FallbackHandler.Stream()

## Performance Budget

- none — streaming через gateway не добавляет буферизации; latency добавляется только на форвардинг (1 write per chunk, Go channel overhead). P99 latency overhead < 5ms для типичного чанка.

## Implementation Surfaces

1. **`src/internal/api/provider_handler.go`** (существующий) — chatRequest: добавить `Stream bool`, HandleChatCompletion: добавить ветвление
2. **`src/internal/api/middleware/sse.go`** (новый) — WrapSSE() middleware
3. **`src/internal/domain/routing/service/fallback.go`** (существующий) — FallbackHandler.Stream()
4. **`src/internal/api/provider_handler_test.go`** (существующий) — тесты streaming
5. **`src/internal/domain/routing/service/service_test.go`** (существующий) — тесты FallbackHandler.Stream()

## Bootstrapping Surfaces

- none — все директории существуют

## Влияние на архитектуру

Локальное: только handler-слой и routing service. ProviderClient.Stream() уже объявлен и реализован. Данные не сохраняются. Non-streaming path не затронут.

## Acceptance Approach

- AC-001 (детекция) → добавить `Stream bool` в chatRequest; unit-тест десериализации JSON с `"stream": true`
- AC-002 (WrapSSE) → новый middleware-файл; unit-тест заголовков ResponseWriter
- AC-003 (форвардинг) → FallbackHandler.Stream() + ветвление в HandleChatCompletion; интеграционный тест c mock-провайдером через gin test context
- AC-004 (cancellation) → HandleChatCompletion использует `c.Request.Context()` для upstream; тест с cancel через контекст
- AC-005 (ошибка в стриме) → проверка `ProviderChunk.Err` в цикле форвардинга; тест с mock, возвращающим chunk с Err
- AC-006 (fallback-перебор) → FallbackHandler.Stream() перебирает провайдеры; unit-тест с моками A(ошибка) → B(успех)

## Данные и контракты

- ProviderClient.Stream() контракт не меняется (interface уже существует)
- ProviderChunk не меняется
- Единственное изменение данных: `chatRequest.Stream bool` — runtime-only, не persisted
- Формат ответа клиенту: SSE (`text/event-stream`), не меняет API contract (добавляется режим ответа)
- см. `data-model.md`

## Стратегия реализации

- DEC-001 WrapSSE как middleware, а не inline в хендлере
  Why: разделение ответственности — middleware занимается wire-форматом, handler — бизнес-логикой. Если в будущем понадобится SSE в других ручках, middleware переиспользуется.
  Tradeoff: дополнительный слой в цепочке middleware. Для одного роута это незначительно.
  Affects: `src/internal/api/middleware/sse.go`, `src/internal/api/server.go` (регистрация middleware на роут)
  Validation: AC-002

- DEC-002 FallbackHandler.Stream() следует паттерну Call() — последовательный перебор с isRetriableError
  Why: единообразие — разработчик, читающий FallbackHandler, видит одинаковую логику для Call и Stream. Изменение только в вызове клиента и возврате канала.
  Tradeoff: нет переключения провайдера в середине стрима (осознанное ограничение — вне scope спеки).
  Affects: `src/internal/domain/routing/service/fallback.go`
  Validation: AC-006

- DEC-003 Upstream-заголовки форвардятся через metadata-чанк перед data-чанками
  Why: SSE не имеет concept "headers" после установки соединения. Единственный способ передать X-Provider и другие заголовки — включить их в первое SSE-сообщение как JSON metadata.
  Tradeoff: клиент должен уметь парсить metadata-сообщение. Альтернатива (подмешивать заголовки в каждый чанк) нарушает SSE-спецификацию.
  Affects: `src/internal/api/provider_handler.go` (шаг форвардинга)
  Validation: AC-003 (наличие X-Provider в первом чанке проверяется интеграционным тестом)

## Incremental Delivery

### MVP (Первая ценность)

1. `FallbackHandler.Stream()` — метод, перебирающий провайдеры и возвращающий канал ProviderChunk. Тесты.
2. `chatRequest.Stream bool` + десериализация.
3. `WrapSSE()` middleware с тестами.
4. RoutingProxyHandler.HandleChatCompletion: ветка `if req.Stream { ... }` с форвардингом чанков, cancellation, error handling.
5. Интеграционные тесты streaming-сценария.

Все AC (AC-001–AC-006) закрываются одним implementation pass — объём мал (5 files touched).

### Итеративное расширение

- none — фича поставляется целиком одним инкрементом

## Порядок реализации

1. FallbackHandler.Stream() + тесты — независим, не зависит от handler/middleware
2. chatRequest.Stream + WrapSSE middleware + тесты — можно параллельно с п.1
3. HandleChatCompletion streaming ветка + тесты — зависит от п.1 и п.2
4. Интеграционные тесты

## Риски

- Риск 1: неправильная обработка `c.Stream()` — Gin требует особого паттерна (нельзя писать после `c.Stream()`).
  Mitigation: тест с mock-провайдером, проверяющий полный цикл форвардинга и закрытия стрима.
- Риск 2: утечка горутины при cancellation — если клиент отключился, но上游 провайдер продолжает слать чанки в канал.
  Mitigation: context cancellation проверяется в цикле форвардинга; канал провайдера дрейнится при отмене (реализация streamSSE уже обрабатывает ctx.Done()).

## Rollout и compatibility

- Специальных rollout-действий не требуется. Новая функциональность активируется при `"stream": true` в теле запроса. Non-streaming запросы не меняют поведения.
- Feature flag не нужен — изменение backward compatible.

## Проверка

- Unit-тесты: chatRequest десериализация (AC-001), WrapSSE headers (AC-002), FallbackHandler.Stream (AC-006)
- Интеграционные тесты: HandleChatCompletion с mock-провайдером (AC-003, AC-005), cancellation (AC-004)
- Manual: curl с `-N` для визуальной проверки SSE-потока
- SC-002: прогон существующих тестов — все non-streaming тесты должны пройти

## Соответствие конституции

- нет конфликтов — Go + Gin, DDD (FallbackHandler в domain/service), Clean Architecture (middleware в api), comments=en, docs=ru
