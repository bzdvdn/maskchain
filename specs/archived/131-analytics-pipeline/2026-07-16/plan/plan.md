# 131-analytics-pipeline План

## Phase Contract

Inputs: spec (pass inspect), repo context: domain/analytics, api/server, infra/metrics, infra/config, cmd/gateway, app/worker.
Outputs: plan.md, data-model.md.
Stop if: spec too vague — passed inspect.

## Цель

Собрать и агрегировать метрики использования LLM-запросов. Middleware перехватывает response body, парсит usage, обновляет Prometheus-метрики синхронно, отправляет TokenUsage в буферизованный канал. Async worker раз в 5 секунд делает batch insert в PostgreSQL через UsageStore. Aggregation worker материализует per-hour/per-day агрегаты. Cleanup worker удаляет сырые данные старше retention.

## MVP Slice

UsageMiddleware + Prometheus-метрики + async worker с batch insert (AC-001, AC-002, AC-003, AC-005, AC-006, AC-007). Aggregation worker (AC-004) и cleanup (AC-008) — второй проход.

## First Validation Path

1. Поднять gateway с PostgreSQL.
2. Отправить `POST /api/v1/chat/completions` c `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}` через mock-провайдер, возвращающий usage.
3. Проверить `curl localhost:8080/metrics | grep maskchain_tokens_total` — метрики с tenant/model лейблами.
4. Проверить `SELECT * FROM usage_raw` — запись в БД через <10с.

## Scope

- `src/internal/api/middleware/` — новый файл `usage.go` (UsageMiddleware, response body capture, parsing, metrics update, channel send)
- `src/internal/domain/analytics/` — расширение `UsageStore` (RecordBatch, DeleteOlderThan), CostRate registry (конструктор из конфига)
- `src/internal/infra/metrics/` — новые Prometheus-метрики (tokens_total, cost_total, request_total)
- `src/internal/infra/config/` — `AnalyticsConfig` + `CostRateConfig` секция
- `src/internal/app/analytics/` — новый пакет: async worker, aggregation worker, cleanup worker
- `src/cmd/gateway/main.go` — DI wiring: создание CostRate registry, запуск workers, регистрация middleware
- PostgreSQL migrations — таблицы `usage_raw`, `usage_agg_hourly`, `usage_agg_daily`
- `src/internal/adapters/repository/analytics/` — PostgreSQL реализация UsageStore

## Performance Budget

- Middleware overhead: <1ms p99 на захват body + парсинг + обновление метрик (бенчмарк на单元-тесте)
- Batch insert: <50ms на 100 записей
- Memory: буфер канала — 1000 записей; пиковое потребление <16MB сверх базового

## Implementation Surfaces

| Surface | Почему | Новая/Сущ. |
|---------|--------|-------------|
| `src/internal/api/middleware/usage.go` | Middleware для перехвата response body | Новая |
| `src/internal/domain/analytics/` | UsageStore.RecordBatch, DeleteOlderThan, CostRate registry | Сущ. — расширение |
| `src/internal/infra/metrics/` | 3 новых Prometheus-метрики | Сущ. — расширение |
| `src/internal/infra/config/` | AnalyticsConfig, CostRateConfig | Сущ. — расширение |
| `src/internal/app/analytics/` | AsyncWorker, AggregationWorker, CleanupWorker | Новая |
| `src/internal/adapters/repository/analytics/` | PostgreSQL реализация UsageStore | Новая |
| `src/cmd/gateway/main.go` | DI wiring | Сущ. — расширение |
| `deployments/docker-compose/migrations/` | Таблицы usage_raw, usage_agg_hourly, usage_agg_daily | Сущ. — расширение |

## Bootstrapping Surfaces

1. PostgreSQL миграции (таблицы для сырых и агрегированных данных)
2. `src/internal/adapters/repository/analytics/` — PgUsageStore
3. `src/internal/app/analytics/` — workers

## Влияние на архитектуру

- UsageStore port расширяется двумя методами (`RecordBatch`, `DeleteOlderThan`) — обратная совместимость: существующий интерфейс не ломается, добавляются новые методы.
- CostRate registry — новый компонент в domain layer (фабрика из конфига).
- Analytics workers следуют паттерну `worker.CleanupWorker` (ticker-based, context cancellation).
- Нет изменений в API-контрактах, routing, shield, или tenant management.

## Acceptance Approach

| AC | Подход | Surfaces | Наблюдение |
|----|--------|----------|------------|
| AC-001 | Unit-тест: mock обработчик возвращает JSON с usage; middleware парсит, создаёт TokenUsage | middleware/usage.go, domain/analytics | Assert на TokenUsage поля |
| AC-002 | Unit-тест: async worker с mock UsageStore; 10 записей за 3с, проверка 1 вызова RecordBatch | app/analytics/async_worker.go, domain/analytics (UsageStore mock) | Счётчик RecordBatch = 1, все 10 записей |
| AC-003 | Unit-тест: middleware обновляет prometheus registry; проверка через testutil | middleware/usage.go, infra/metrics | prometheus.ToOpenMetrics содержит ожидаемые лейблы |
| AC-004 | Интеграционный тест: PgUsageStore с сырыми данными → запуск AggregationWorker → проверка таблиц agg_hourly/agg_daily | app/analytics/agg_worker.go, adapters/repository/analytics | Записи в agg_* таблицах |
| AC-005 | Unit-тест: response без usage → fallback tiktoken или warning | middleware/usage.go | Warning в логе или вычисленные токены |
| AC-006 | Unit-тест: Server.RegisterUsageMiddleware добавляет middleware в цепочку | api/server.go, middleware/usage.go | Middleware вызывается на /chat/completions |
| AC-007 | Интеграционный тест: config с cost_rates → запуск → запрос → проверка стоимости | infra/config, domain/analytics (CostRate), middleware | Стоимость в TokenUsage соответствует ценам из конфига |
| AC-008 | Интеграционный тест: PgUsageStore с записями старше/младше retention → cleanup → проверка | app/analytics/cleanup_worker.go, adapters/repository/analytics | Старые записи удалены, новые сохранены |

## Данные и контракты

- **UsageStore port**: добавляются `RecordBatch(ctx, []TokenUsage) error` и `DeleteOlderThan(ctx, time.Time) error` — обратно совместимо.
- **CostRate registry**: новая сущность в domain, создаётся из конфига на старте.
- **Таблицы**: `usage_raw` (сырые записи), `usage_agg_hourly` (per-hour агрегаты), `usage_agg_daily` (per-day) — см. `data-model.md`.
- **Конфиг**: новая секция `analytics` в Config struct.
- API-контракты не меняются.
- `data-model.md` — changed (3 новые таблицы).

## Стратегия реализации

### DEC-001 UsageMiddleware как post-processing response body writer

- **Why**: middleware должен читать response body *после* proxy handler, когда ответ уже сформирован. Gin middleware с обёрткой `ResponseWriter` (паттерн из `envelope.go`) позволяет перехватить body без изменения handler.
- **Tradeoff**: двойное буферирование body (оригинал + обёртка) — ~1-4KB на запрос overhead. Принимаем для прозрачности.
- **Affects**: `middleware/usage.go`, `api/server.go`
- **Validation**: unit-тест AC-001

### DEC-002 Async worker с буферизованным каналом и ticker-based flush

- **Why**: отдельная горутина с каналом буфера 1000 записей накапливает TokenUsage; ticker каждые 5 секунд (или при переполнении буфера) вызывает `RecordBatch`. Минимизирует latency для API-запросов (асинхронный write-behind).
- **Tradeoff**: до 5 секунд задержки между запросом и появлением данных в БД. Для операторов приемлемо (P1: "без задержки более 10 секунд"). При падении процесса — потеря несохранённых записей в буфере.
- **Affects**: `app/analytics/async_worker.go`
- **Validation**: AC-002

### DEC-003 CostRate registry из конфига, not from DB

- **Why**: цены меняются редко, конфиг + env — достаточно. DB-based добавит latency и сложность без явной потребности.
- **Tradeoff**: изменение цен требует перезапуска gateway. Для enterprise сценария приемлемо.
- **Affects**: `infra/config`, `domain/analytics`
- **Validation**: AC-007

### DEC-004 Aggregation worker и cleanup worker — один общий ticker

- **Why**: агрегация и cleanup работают на одних данных; запуск последовательно в одном цикле (сначала агрегация, потом cleanup) гарантирует, что агрегаты построены до удаления сырья.
- **Tradeoff**: если cleanup упадёт, агрегация тоже не выполнится. Mitigation: логирование ошибок, следующая итерация повторит.
- **Affects**: `app/analytics/agg_worker.go`
- **Validation**: AC-004, AC-008

### DEC-005 Fallback token counting через tiktoken-go

- **Why**: `github.com/pulumi/tiktoken-go` — pure Go, без cgo/wasm, покрывает OpenAI-модели. Если модель неизвестна tiktoken — warning + zero tokens.
- **Tradeoff**: tiktoken-go ~2MB в бинарнике; загрузка BPE рангов при первом вызове ~500ms. Принимаем — фоллбэк-путь, не горячий.
- **Affects**: `middleware/usage.go`
- **Validation**: AC-005

## Incremental Delivery

### MVP (Первая ценность)

Middleware + Prometheus + async worker. AC-001, AC-002, AC-003, AC-005, AC-006, AC-007.
CostRate registry + config + PgUsageStore.
После имплементации: curl /metrics показывает метрики, в БД есть записи.

### Итеративное расширение

- Шаг 2: Aggregation worker (AC-004) — per-hour/per-day агрегаты в отдельные таблицы.
- Шаг 3: Cleanup worker (AC-008) — удаление сырых данных старше retention.

## Порядок реализации

1. PostgreSQL миграции (таблицы `usage_raw`, `usage_agg_hourly`, `usage_agg_daily`) — без них нельзя сохранять данные.
2. `AnalyticsConfig` + `CostRateConfig` в config — нужны для CostRate registry.
3. Расширение `UsageStore` port (RecordBatch, DeleteOlderThan) + PgUsageStore adapter.
4. CostRate registry + конструктор из конфига.
5. Prometheus-метрики в `infra/metrics`.
6. UsageMiddleware (body capture, парсинг, метрики, отправка в канал).
7. Async worker (буферизованный канал, batch insert по ticker).
8. Регистрация в `Server` + DI wiring в `main.go`.
9. Aggregation worker (второй проход).
10. Cleanup worker (второй проход).

Параллельно: 1-2, 3-4-5 (зависимости нет), 6-7-8 (последовательно), 9-10 (после 3-8).

## Риски

- **Буферизация body**: Response body может быть большим (сотни KB для streaming fallback). **Mitigation**: MVP только non-streaming, body ограничен размером ответа LLM (типично <100KB). При превышении — error в middleware -> body не парсится, метрики не обновляются.
- **Падение gateway с несохранёнными записями в буфере**: потеря до 5 секунд данных. **Mitigation**: документировано, приемлемо для метрик использования (не financial ledger). В будущем — WAL или синхронная запись для критических данных.
- **tiktoken-go первый вызов медленный (~500ms)**: **Mitigation**: ленивая инициализация или prewarm на старте. Prewarm предпочтительнее — предсказуемая latency для первого запроса.

## Rollout и compatibility

- Специальных rollout-действий не требуется: новые метрики, конфиг опционален (без секции analytics — CostRate нулевые, workers не запускаются, middleware не зарегистрирован).
- Feature flag не нужен — middleware не меняет существующее поведение.
- После деплоя: проверить `/metrics`, проверить `usage_raw` через psql.
- При откате: удаление секции analytics из конфига + откат миграций (DROP TABLE).

## Проверка

- Unit-тесты: AC-001, AC-002, AC-003, AC-005, AC-006 (mock UsageStore, mock ResponseWriter, prometheus testutil)
- Интеграционные тесты: AC-004, AC-007, AC-008 (PgUsageStore + test PostgreSQL)
- Middleware overhead benchmark: testing.B с симулированным response body 4KB (SC-002)
- Manual check: curl /metrics после запроса через gateway

## Соответствие конституции

- нет конфликтов. Analytics pipeline — вспомогательная система, не затрагивает Content Shield domain. PostgreSQL — предпочтительное хранилище. Go + Gin + cobra/viper — все компоненты соответствуют стеку.
