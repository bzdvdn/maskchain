# Critical Test Coverage

## Scope Snapshot

- In scope: закрытие пробелов модульного и интеграционного тестирования на critical path (auth → shield → routing → egress → response) для существующих handler'ов, middleware и server.
- Out of scope: изменение production-кода, E2E-тесты вне Go, performance/load-тесты, UI-тесты, документация.

## Цель

Разработчик и reviewer получают уверенность, что critical path покрыт наблюдаемыми тестами: fallback и health-aware routing, PII blocking и graceful degradation shield, полный mask/unmask pipeline, health endpoints и graceful shutdown сервера, а также интеграционный тест полного цикла. Успех фичи будет виден по отсутствию критических пробелов в тестовом покрытии перечисленных сценариев.

## Основной сценарий

1. Стартовая точка: существующие тесты покрывают отдельные компоненты изолированно, но имеют критические пробелы (graceful shutdown не тестируется, HandleUnmask не тестируется, нет сквозного integration-теста).
2. Основное действие: для каждого компонента (provider_handler, shield middleware, mask_handler, server) добавляются недостающие тесты, а также создаётся integration-тест, соединяющий всю цепочку.
3. Результат: `go test ./...` проходит; каждый критический сценарий имеет observable proof.
4. Ошибка/fallback-путь: если тест требует минимального изменения production-кода (например, экспорт unexported функции), это допускается; если требуется архитектурное изменение — выносится в открытые вопросы.

## User Stories

- P1 Story: как разработчик, я хочу видеть, что graceful shutdown сервера тестируется, чтобы быть уверенным в production-hardening.
- P2 Story: как разработчик, я хочу видеть, что полный цикл mask → unmask тестируется, чтобы избежать регрессии pipeline.
- P3 Story: как reviewer, я хочу видеть integration-тест, проверяющий цепочку auth → shield → routing → egress, чтобы доверять сквозному сценарию.

## MVP Slice

Наименьший срез, дающий независимую ценность: тесты graceful shutdown, HandleUnmask и один integration-тест golden path. AC-001, AC-002, AC-007.

## First Deployable Outcome

После первого implementation pass можно прогнать `go test ./src/internal/api/...` и `go test ./src/internal/api/middleware/...` и убедиться, что новые тесты проходят, а критические пробелы закрыты.

## Scope

- `src/internal/api/provider_handler_test.go` — fallback, health-aware выбор, tenant-scoped routing, таймауты провайдера, ProxyCompletionHandler, nil routingHandler, пропагация X-Request-ID
- `src/internal/api/middleware/shield_test.go` — graceful degradation (все варианты default_action), отключённый shield, cancel контекста, ScanResult=(nil,nil), X-Shield-Incident-ID на degraded path
- `src/internal/api/mask_handler_test.go` — HandleUnmask (успех, пустой результат, ошибка Get), HandleMask (ошибка Save, ошибка preprocessor), полный mask→unmask цикл
- `src/internal/api/server_test.go` — Shutdown() graceful, 404/Method Not Allowed, маршрут `/metrics`, route registration вызовы
- Сквозной integration-тест (в одном из существующих test-файлов) — полный цикл: request → auth middleware → shield middleware → routing handler → egress response (с mock-заглушками)

## Контекст

- Все тесты выполняются в памяти (in-process) с mock-зависимостями; external services (PostgreSQL, Valkey, внешние API) не поднимаются.
- Auth middleware проверяет X-API-Key; для integration-теста допускается отключение auth через конфигурацию.
- Shield middleware может быть отключён через конфигурацию (enabled: false).
- Routing зависит от ProviderRegistry, RouteSelector, и FallbackHandler из `src/internal/domain/routing/service/`.
- Mask handler зависит от MaskRepository (storage interface).
- Graceful shutdown использует `signal.NotifyContext`; в тесте контекст отменяется программно.

## Зависимости

- Зависит от существующих port interfaces в `src/internal/ports/`.
- Зависит от существующих mock-реализаций (если есть) или test helpers.
- `none` внешних и меж-спековых зависимостей.

## Требования

- RQ-001 Система ДОЛЖНА иметь тест, проверяющий, что Server.Shutdown() корректно завершает активные запросы в течение таймаута.
- RQ-002 Система ДОЛЖНА иметь тест, проверяющий полный цикл mask → unmask через HTTP handler (POST /api/v1/shield/mask → POST /api/v1/shield/unmask).
- RQ-003 Система ДОЛЖНА иметь тест, проверяющий graceful degradation shield middleware при ошибке Scan (default_action=allow → 200, default_action=block → 403) с X-Shield-Incident-ID.
- RQ-004 Система ДОЛЖНА иметь тест, проверяющий fallback chain в routing handler (primary unhealthy → fallback успешен → 200).
- RQ-005 Система ДОЛЖНА иметь integration-тест, проверяющий сквозной сценарий: запрос проходит auth → shield (clean) → routing (health-aware выбор) → egress (mock response) → ответ 200 с корректными заголовками.
- RQ-006 Система ДОЛЖНА иметь тест на ProxyCompletionHandler (не-chat completion endpoint).
- RQ-007 Система ДОЛЖНА иметь тест на HandleUnmask с существующим mask_id (успешное восстановление), несуществующим mask_id (пустой результат), и ошибкой storage.Get().
- RQ-008 Система ДОЛЖНА иметь тест на HandleMask при ошибке storage.Save().

## Вне scope

- Изменение production-кода (исправление багов, рефакторинг, новая функциональность).
- Performance/load/stress-тесты (benchmark, k6, wrk).
- E2E-тесты с реальными external services (PostgreSQL, Valkey, OpenAI).
- UI-тесты (React/TypeScript).
- Concurrency/race-тесты (go test -race остаётся, но отдельные race-oriented тесты не добавляются).
- Документация тестовой стратегии.
- Исправление существующего TODO в TestRoutingHandlerWithMockClientsFallback.

## Критерии приемки

### AC-001 Graceful shutdown сервера тестируется

- Почему это важно: production-hardening требует уверенности, что сервер завершает работу без потери запросов.
- **Given** запущенный Server с активным обработчиком (долгий запрос)
- **When** вызывается Shutdown() с таймаутом
- **Then** сервер завершает активный запрос, после чего Shutdown возвращает nil (или ожидаемый контекстный timeout error при превышении graceful period)
- Evidence: assertion на возвращаемое значение Shutdown() и что запрос был полностью обработан.

### AC-002 Полный цикл mask → unmask тестируется

- Почему это важно: mask/unmask pipeline — core domain; регрессия недопустима.
- **Given** запущенный HTTP server с зарегистрированным mask handler
- **When** клиент отправляет POST /api/v1/shield/mask с текстом, содержащим PII, затем POST /api/v1/shield/unmask с полученным mask_id
- **Then** ответ /mask содержит mask_id и замаскированный текст; ответ /unmask содержит оригинальный текст
- Evidence: assertion на структуру ответов и восстановленный текст.

### AC-003 Graceful degradation shield middleware тестируется

- Почему это важно: отказ detector'а не должен блокировать весь gateway при default_action=allow.
- **Given** shield middleware настроен с default_action=allow и Scan возвращает ошибку
- **When** клиент отправляет POST запрос с JSON-телом
- **Then** запрос проходит к downstream handler, X-Shield-Status: error, X-Shield-Incident-ID присутствует
- Evidence: assertion на HTTP 200 и наличие заголовков.

### AC-004 Fallback chain в routing handler тестируется

- Почему это важно: отказ primary провайдера не должен прерывать сервис.
- **Given** RoutingHandler настроен с primary (unhealthy) и fallback (healthy) провайдерами
- **When** клиент отправляет chat completion запрос
- **Then** запрос направляется fallback провайдеру; ответ 200 с корректным телом
- Evidence: assertion на HTTP 200.

### AC-005 Integration-тест полного цикла

- Почему это важно: изолированные тесты не гарантируют корректную интеграцию компонентов.
- **Given** настроенный gateway (in-process) с auth middleware (valid key), shield middleware (clean), routing handler (healthy provider), egress (mock response)
- **When** клиент отправляет валидный chat completion запрос с корректным X-API-Key
- **Then** ответ 200; заголовки X-Shield-Status: clean, X-Request-ID присутствуют
- Evidence: assertion на HTTP статус и заголовки.

### AC-006 ProxyCompletionHandler тестируется

- Почему это важно: второй эндпоинт (non-chat completions) не должен быть без покрытия.
- **Given** RoutingHandler с healthy провайдером
- **When** клиент отправляет POST /v1/completions
- **Then** ответ 200
- Evidence: assertion на HTTP 200.

### AC-007 HandleUnmask с различными состояниями

- Почему это важно: unmask должен корректно обрабатывать success/not-found/error.
- **Given** mask storage с существующей записью, несуществующим ID, и с ошибкой Get
- **When** клиент отправляет POST /api/v1/shield/unmask с mask_id
- **Then** существующий ID → успешное восстановление; несуществующий → пустой результат; ошибка storage → 500
- Evidence: assertion на HTTP статус и тело ответа.

### AC-008 HandleMask при ошибке storage

- Почему это важно: ошибка сохранения не должна маскироваться.
- **Given** mask storage.Save возвращает ошибку
- **When** клиент отправляет POST /api/v1/shield/mask
- **Then** ответ 500
- Evidence: assertion на HTTP 500.

## Допущения

- Все тесты используют mock-зависимости (gomock или ручные моки).
- Shield middleware может быть протестирован с mock Scanner.
- Routing handler тестируется с mock ProviderClient.
- Mask handler тестируется с mock MaskRepository.
- Server тестируется через `httptest.NewServer` или `httptest.NewRecorder`.
- Graceful shutdown тестируется через программную отмену контекста (без реальных сигналов ОС).

## Критерии успеха

- SC-001 После добавления тестов `go test ./src/internal/api/...` и `go test ./src/internal/api/middleware/...` проходит без ошибок.
- SC-002 Количество тестовых функций в покрываемых файлах увеличивается минимум на 50% (относительно текущего base).

## Краевые случаи

- Отключённый shield (конфигурация enabled: false) — middleware пропускает запрос без сканирования.
- ScanResult = nil, err = nil — middleware не падает с nil pointer dereference.
- Cancel контекста во время shield scan — middleware корректно завершает запрос.
- nil routingHandler в RegisterProxyRoute — сервер использует legacy путь без паники.
- 404 для неизвестного пути — сервер возвращает корректный ответ.

## Открытые вопросы

- Где разместить integration-тест: в отдельном файле `integration_test.go` внутри `src/internal/api/` или в одном из существующих test-файлов? Предварительно: отдельный файл `src/internal/api/integration_test.go` с build tag `//go:build integration` для изоляции.
- Нужен ли тест на Server.Start() с реальным портом (случайный порт) или достаточно теста через httptest?
- Допустимо ли минимальное изменение production-кода (экспорт функции/типа) для тестируемости?
