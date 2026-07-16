# 131-analytics-pipeline

## Scope Snapshot

- In scope: сбор и агрегация метрик использования LLM-запросов — middleware для захвата токенов, асинхронная запись в UsageStore, воркер агрегации, Prometheus-метрики.
- Out of scope: UI-дашборды для метрик, биллинг/инвойсинг, экспорт метрик в внешние системы (кроме Prometheus), анализ логов запросов (уже покрыто audit/incidents).

## Цель

Операторы получают прозрачность затрат по tenant и модели: после внедрения pipeline каждый LLM-запрос автоматически регистрирует потреблённые токены и стоимость, данные агрегируются по часам/дням, а Prometheus-метрики позволяют отслеживать нагрузку и расходы в реальном времени.

## Основной сценарий

1. Клиент отправляет chat/completions запрос через gateway.
2. Proxy handler обрабатывает запрос, получает ответ от LLM провайдера.
3. UsageMiddleware (post-processing middleware) читает response body, парсит поле `usage` (OpenAI-формат), вычисляет стоимость через CostRate.
4. TokenUsage передаётся в буферизованный канал async worker.
5. Async worker раз в 5 секунд делает batch insert в UsageStore (PostgreSQL).
6. Aggregation worker раз в N минут материализует per-hour/per-day агрегаты и чистит сырые данные старше retention.
7. Prometheus-метрики обновляются синхронно в middleware: токены, стоимость, количество запросов по tenant и модели.

## User Stories

- P1 (Operator): "Я хочу видеть, сколько токенов и денег тратит каждый tenant на каждую модель, без задержки более 10 секунд."
- P2 (Operator): "Я хочу мониторить нагрузку и тренды через Prometheus/Grafana, имея метки tenant и model."

## MVP Slice

Middleware, async worker с batch insert в UsageStore и базовые Prometheus-метрики. Aggregation worker — второй проход.

## First Deployable Outcome

После одного запроса через gateway в Prometheus появляются метрики `maskchain_tokens_total`, `maskchain_cost_total`, `maskchain_request_total` с корректными tenant/model лейблами.

## Scope

- UsageMiddleware — Gin middleware, захватывает response body, парсит usage, вычисляет cost, обновляет метрики, отправляет TokenUsage в канал.
- Token counting — приоритет: поле `usage` из response body (OpenAI-формат); fallback: оценка через tiktoken.
- Async worker — горутина с буферизованным каналом, batch insert каждые 5 секунд через `UsageStore.RecordBatch`.
- Aggregation worker — горутина, материализует per-hour и per-day агрегаты в отдельную таблицу.
- Cleanup worker — горутина (может быть частью aggregation worker), удаляет сырые данные старше retention.
- Prometheus-метрики:
  - `maskchain_tokens_total{tenant,model,type="input|output"}`
  - `maskchain_cost_total{tenant,model}`
  - `maskchain_request_total{tenant,model}`
- Конфигурация: retention периода для сырых данных, интервал агрегации, размер batch, таймауты.
- Регистрация UsageMiddleware в Server (gateway) через отдельный метод `RegisterUsageMiddleware`.
- CostRate registry — конфигурация цен по модели (через config file или env), с fallback нулевой стоимости если модель неизвестна.
- Поддержка только non-streaming запросов в MVP; на streaming-запросах middleware проверяет поле `stream` в request body и пропускает обработку (не парсит response, не отправляет TokenUsage).

## Контекст

- Response body перехватывается через обёртку `gin.ResponseWriter` (паттерн аналогичный `envelopeWriter` в `middleware/envelope.go`).
- Tenant ID доступен через `middleware.TenantFromContext(c)`.
- Domain-сущности (TokenUsage, UsageStore, CostRate, Aggregation, UsageRecord) уже реализованы в `src/internal/domain/analytics/` (задача 130).
- Response body для streaming-запросов не содержит usage в каждом chunk; usage может прийти в финальном chunk `[DONE]` или отсутствовать — обработка streaming не входит в MVP.
- Repository map: `src/internal/infra/metrics/` содержит Prometheus-определения; `src/internal/api/` — middleware-регистрация.
- CostRate предполагает конфигурацию через viper/cobra.

## Зависимости

- `src/internal/domain/analytics/` — готовая domain-модель (TokenUsage, UsageStore, CostRate, Aggregation). UsageStore требует расширения: метод `RecordBatch(ctx, []TokenUsage) error` для batch-записи, метод `DeleteOlderThan(ctx, time.Time) error` для cleanup.
- PostgreSQL — для хранения сырых и агрегированных записей через UsageStore.
- Prometheus client_golang — уже в проекте.
- [optional] `github.com/pulumi/pulumi-tiktoken` или эквивалент для fallback token counting — не зафиксировано в spec, будет выбрано в plan.

## Требования

- RQ-001 Система ДОЛЖНА захватывать response body каждого non-streaming `/api/v1/chat/completions` запроса после обработки proxy handler.
- RQ-002 Система ДОЛЖНА извлекать `usage.prompt_tokens`, `usage.completion_tokens` из response body в OpenAI-формате.
- RQ-003 Система ДОЛЖНА вычислять стоимость запроса через CostRate на основе модели tenant-а.
- RQ-004 Система ДОЛЖНА асинхронно записывать TokenUsage в UsageStore с batch insert каждые 5 секунд.
- RQ-005 Система ДОЛЖНА материализовать per-hour и per-day агрегаты из сырых данных UsageStore.
- RQ-006 Система ДОЛЖНА удалять сырые данные старше настроенного retention периода.
- RQ-007 Система ДОЛЖНА экспортировать Prometheus-метрики с лейблами tenant, model, type (input/output).

## Вне scope

- UI-дашборды для отображения метрик.
- Streaming-запросы (usage в streaming chunks).
- Экспорт метрик в внешние системы (кроме Prometheus).
- Биллинг/инвойсинг на основе собранных данных.
- Rate limiting на основе analytics (только мониторинг).

## Критерии приемки

### AC-001 UsageMiddleware захватывает response body и парсит usage

- Почему это важно: без захвата тела ответа невозможно получить реальное потребление токенов.
- **Given** настроенный gateway с UsageMiddleware
- **When** клиент отправляет POST `/api/v1/chat/completions` с `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`
- **Then** middleware читает response body, извлекает `usage.prompt_tokens` и `usage.completion_tokens`, создаёт `TokenUsage` с корректным TenantID, моделью, токенами и стоимостью
- Evidence: unit-тест middleware с mock-обработчиком, возвращающим известный usage JSON; assert на созданный TokenUsage

### AC-002 Async worker batch-insert каждые 5 секунд

- Почему это важно: синхронная запись каждого запроса в БД добавит latency; batch insert решает проблему.
- **Given** запущенный async worker с буферизованным каналом; UsageStore имеет метод `RecordBatch(ctx, []TokenUsage) error`
- **When** 10 TokenUsage записей отправлены в канал за 3 секунды
- **Then** через 5 секунд все 10 записей записаны в UsageStore одним batch insert (один вызов `RecordBatch` со всеми 10 записями)
- Evidence: mock UsageStore с счётчиком вызовов `RecordBatch`; проверка что batch содержит все 10 записей и был ровно 1 вызов

### AC-003 Prometheus-метрики обновляются при каждом запросе

- Почему это важно: операторы должны видеть нагрузку в реальном времени.
- **Given** настроенный Prometheus registry
- **When** middleware обрабатывает запрос tenant-1 на модели gpt-4
- **Then** метрики `maskchain_tokens_total{tenant="tenant-1",model="gpt-4",type="input"}`, `maskchain_tokens_total{tenant="tenant-1",model="gpt-4",type="output"}`, `maskchain_cost_total{tenant="tenant-1",model="gpt-4"}`, `maskchain_request_total{tenant="tenant-1",model="gpt-4"}` инкрементированы на корректные значения
- Evidence: unit-тест с prometheus testutil; проверка значений метрик через `prometheus.ToOpenMetrics()`

### AC-004 Aggregation worker материализует per-hour и per-day агрегаты

- Почему это важно: для оперативных и исторических отчётов нужны предвычисленные агрегаты.
- **Given** UsageStore содержит сырые записи за последние 48 часов
- **When** aggregation worker запускается
- **Then** созданы агрегированные записи в отдельной таблице: per-hour для каждого часа с данными и per-day для каждого дня с данными
- Evidence: интеграционный тест с PostgreSQL UsageStore; проверка записей в агрегационных таблицах с корректными TotalTokens, TotalCost, RequestCount

### AC-005 Fallback token counting при отсутствии usage в response

- Почему это важно: не все провайдеры возвращают usage в OpenAI-формате; система должна работать с fallback.
- **Given** response body не содержит поле `usage` (или оно null)
- **When** middleware обрабатывает такой ответ
- **Then** middleware пытается вычислить токены через tiktoken (или другой tokenizer) на основе prompt из request body и completion из response body; если и это невозможно — записывает нулевые токены и логирует warning
- Evidence: unit-тест с mock-ответом без usage; проверка что токены вычислены или залогировано предупреждение

### AC-006 UsageMiddleware можно зарегистрировать на Server

- Почему это важно: middleware должен подключаться стандартным способом.
- **Given** Server (gateway) создан
- **When** вызывается `s.RegisterUsageMiddleware(analyticsMw)`
- **Then** middleware добавлен в цепочку обработки gin.Engine
- Evidence: тест, проверяющий что middleware вызывается для запросов к `/api/v1/chat/completions`

### AC-007 CostRate registry конфигурируется через config

- Почему это важно: операторы должны иметь возможность добавлять/менять цены без перекомпиляции.
- **Given** конфигурационный файл содержит секцию `analytics.cost_rates` с ценами для gpt-4
- **When** gateway стартует
- **Then** CostRate для gpt-4 загружен и используется при вычислении стоимости
- Evidence: интеграционный тест: конфиг с известными ценами → запрос → проверка вычисленной стоимости

### AC-008 Cleanup worker удаляет сырые данные старше retention

- Почему это важно: без cleanup сырые данные бесконечно растут, увеличивая стоимость хранения и замедляя запросы.
- **Given** UsageStore содержит сырые записи старше и младше настроенного retention периода (например, 7 дней)
- **When** cleanup worker запускается
- **Then** все записи старше retention удалены, записи младше retention сохранены
- Evidence: интеграционный тест с PostgreSQL UsageStore; проверка количества записей до и после cleanup

## Допущения

- Response body для non-streaming запросов всегда JSON и парсится без ошибок (ошибки парсинга логируются, метрики не обновляются).
- Tenant ID всегда доступен в контексте Gin для авторизованных запросов.
- Модель в request body совпадает с моделью, используемой для CostRate lookup.
- Batch insert в UsageStore — идемпотентный (каждый TokenUsage имеет уникальный идентификатор).
- Aggregation worker использует тот же UsageStore (PostgreSQL) — отдельного хранилища для агрегатов не требуется.
- Retention периода по умолчанию — 7 дней для сырых данных; агрегаты хранятся бессрочно.
- CostRate с нулевой ценой — валидное состояние для моделей без стоимости.
- UsageMiddleware проверяет `stream` поле request body: если `true` — пропускает без обработки (не парсит response, не обновляет метрики).

## Критерии успеха

- SC-001 Batch insert latency: <50ms на batch из 100 записей при штатной нагрузке.
- SC-002 Middleware overhead: <1ms добавленной задержки на p99 (захват body + парсинг).
- SC-003 Prometheus-метрики доступны на `/metrics` эндпоинте в течение 1 секунды после первого запроса.

## Краевые случаи

- Response body с `usage: null` или отсутствующим `usage` — fallback через tokenizer или warning + zero tokens.
- Запрос без tenant (unauthenticated) — метка `tenant="unknown"`.
- CostRate для модели отсутствует — стоимость = 0, метрики обновляются.
- Batch insert превышает размер (буфер переполнен) — запись синхронно, warning в лог.
- Aggregation worker не успевает за потоком данных — накопление сырых записей, следующая итерация обработает больше.

## Открытые вопросы

- `none` — вопросы обсуждены и закрыты:
  - **tiktoken wrapper**: `github.com/pulumi/tiktoken-go` — наиболее поддерживаемый Go-аналог; решение за plan по критериям: размер бинарника (wasm vs pure Go) и скорость на first call.
  - **unique ID для TokenUsage**: UUIDv7 как у других сущностей в проекте; обеспечивает идемпотентность batch insert и удобен для дедупликации в агрегации.
  - **Хранение агрегатов**: отдельная таблица materialized aggregates (per-hour, per-day) — упрощает запросы и cleanup, не требует изменения UsageRecord.
  - **Streaming**: отложено до отдельной фичи; middleware пропускает streaming-запросы.
