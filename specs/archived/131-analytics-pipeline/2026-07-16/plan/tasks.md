# 131-analytics-pipeline Задачи

## Phase Contract

Inputs: plan.md, data-model.md, spec.md.
Outputs: упорядоченные исполниные задачи с покрытием 8 AC.
Stop if: tasks получаются расплывчатыми или coverage не удаётся сопоставить.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/adapters/repository/postgres/migrations/011_analytics.up.sql` | T1.1 |
| `src/internal/adapters/repository/postgres/migrations/011_analytics.down.sql` | T1.1 |
| `src/internal/infra/config/config.go` | T1.2 |
| `src/internal/domain/analytics/usage_store.go` | T2.1 |
| `src/internal/domain/analytics/cost_rate.go` | T2.2 |
| `src/internal/domain/analytics/analytics_test.go` | T4.5 |
| `src/internal/adapters/repository/analytics/pg_usage_store.go` | T2.3 |
| `src/internal/adapters/repository/analytics/pg_usage_store_test.go` | T4.6 |
| `src/internal/infra/metrics/metrics.go` | T2.4 |
| `src/internal/api/middleware/usage.go` | T3.1 |
| `src/internal/api/middleware/usage_test.go` | T4.1 |
| `src/internal/app/analytics/async_worker.go` | T3.2 |
| `src/internal/app/analytics/async_worker_test.go` | T4.2 |
| `src/internal/api/server.go` | T3.3 |
| `src/cmd/gateway/main.go` | T3.3 |
| `src/internal/app/analytics/agg_worker.go` | T3.4 |
| `src/internal/app/analytics/agg_worker_test.go` | T4.3 |
| `src/internal/app/analytics/cleanup_worker.go` | T3.5 |
| `src/internal/app/analytics/cleanup_worker_test.go` | T4.4 |

## Implementation Context

- **Цель MVP:** Middleware захватывает usage из response body, обновляет Prometheus-метрики, async worker пишет batch в usage_raw. Aggregation и cleanup — второй проход.
- **Инварианты/семантика:**
  - UsageMiddleware — post-processing middleware (обёртка `gin.ResponseWriter`, как в `envelope.go`).
  - TokenUsage.id = UUIDv7, генерируется в middleware перед отправкой в канал.
  - Streaming-запросы: middleware проверяет `request.Body.stream == true` и пропускает без обработки.
  - Async worker: ticker 5s, буфер канала 1000, при переполнении — синхронный flush.
  - CostRate registry: map[string]*CostRate, грузится из конфига на старте; неизвестная модель → нулевая стоимость.
- **Ошибки/коды:** Ошибки парсинга usage логируются (zap.Warn), метрики не обновляются, TokenUsage не создаётся.
- **Контракты/протокол:**
  - UsageStore port: добавляются `RecordBatch(ctx, []TokenUsage) error` и `DeleteOlderThan(ctx, time.Time) error`.
  - Migration: 011_analytics (usage_raw, usage_agg_hourly, usage_agg_daily).
  - Config: секция `analytics` с `cost_rates: [{model, input_price_per_1k, output_price_per_1k}]`, `retention_days`, `batch_interval`.
- **Границы scope:** Не делаем: Valkey-кэш для агрегатов, партиционирование usage_raw, обработку streaming chunks.
- **Proof signals:**
  - AC-001: middleware unit-test с известным usage JSON → TokenUsage с ожидаемыми полями.
  - AC-002: async worker unit-test → 10 записей за 3с → 1 вызов RecordBatch со всеми 10.
  - AC-003: metrics unit-test → prometheus содержит маскированные метрики с tenant/model/type.
  - AC-007: config с cost_rates → CostRate загружен, стоимость вычислена.
- **References:** `DEC-001` (response writer capture), `DEC-002` (async worker), `DEC-003` (CostRate from config), `DEC-005` (tiktoken-go), `DM-001` (usage_raw), `DM-002` (usage_agg_hourly), `DM-003` (usage_agg_daily).

## Фаза 1: Основа (data & config)

Цель: создать таблицы, конфиг и port-расширения, от которых зависят все остальные фазы.

- [x] T1.1 Добавить PostgreSQL миграцию 011_analytics (usage_raw, usage_agg_hourly, usage_agg_daily) + down migration.
  Touches: `src/internal/adapters/repository/postgres/migrations/011_analytics.up.sql`, `src/internal/adapters/repository/postgres/migrations/011_analytics.down.sql`

- [x] T1.2 Добавить AnalyticsConfig, CostRateConfig в Config struct + defaults + валидацию.
  Touches: `src/internal/infra/config/config.go`
  Зависимости: T1.1

## Фаза 2: Domain + adapter

Цель: расширить UsageStore port, реализовать PgUsageStore, CostRate registry.

- [x] T2.1 Добавить в UsageStore port методы `RecordBatch(ctx, []TokenUsage) error` и `DeleteOlderThan(ctx, time.Time) error`.
  Touches: `src/internal/domain/analytics/usage_store.go`
  AC: AC-002, AC-008

- [x] T2.2 Создать CostRate registry (NewCostRateRegistry из конфига, lookup по модели, fallback с нулевой ценой).
  Touches: `src/internal/domain/analytics/cost_rate.go`
  AC: AC-007

- [x] T2.3 Реализовать PgUsageStore (Record, RecordBatch с COPY или multi-row INSERT, QueryByTenant, QueryByModel, AggregateByDay, DeleteOlderThan).
  Touches: `src/internal/adapters/repository/analytics/pg_usage_store.go`
  Зависимости: T1.1, T2.1

- [x] T2.4 Добавить Prometheus-метрики maskchain_tokens_total, maskchain_cost_total, maskchain_request_total с лейблами tenant, model, type.
  Touches: `src/internal/infra/metrics/metrics.go`
  AC: AC-003

## Фаза 3: MVP реализация

Цель: middleware + Prometheus-метрики + async worker + DI wiring (AC-001, AC-002, AC-003, AC-005, AC-006, AC-007).

- [x] T3.1 Реализовать UsageMiddleware: захват response body через обёртку ResponseWriter, парсинг usage (OpenAI-формат), fallback через tiktoken-go при отсутствии usage, вычисление стоимости через CostRate, обновление Prometheus-метрик, отправка TokenUsage в канал async worker. Проверка `stream` — пропуск streaming-запросов.
  Touches: `src/internal/api/middleware/usage.go`
  AC: AC-001, AC-003, AC-005
  Зависимости: T2.2, T2.4

- [x] T3.2 Реализовать AsyncWorker: буферизованный канал (cap 1000), ticker 5s, batch insert через UsageStore.RecordBatch, синхронный flush при переполнении буфера.
  Touches: `src/internal/app/analytics/async_worker.go`
  AC: AC-002
  Зависимости: T2.3

- [x] T3.3 Добавить `RegisterUsageMiddleware` на Server + DI wiring в gateway main.go (создание CostRate registry, PgUsageStore, AsyncWorker, запуск worker, регистрация middleware).
  Touches: `src/internal/api/server.go`, `src/cmd/gateway/main.go`
  AC: AC-006
  Зависимости: T3.1, T3.2

- [x] T3.4 Реализовать AggregationWorker: ticker-based, UPSERT в usage_agg_hourly и usage_agg_daily через AggregateByDay + прямой SQL.
  Touches: `src/internal/app/analytics/agg_worker.go`
  AC: AC-004
  Зависимости: T2.3 (после MVP)

- [x] T3.5 Реализовать CleanupWorker: ticker-based, DELETE FROM usage_raw WHERE recorded_at < retention через DeleteOlderThan.
  Touches: `src/internal/app/analytics/cleanup_worker.go`
  AC: AC-008
  Зависимости: T2.3 (после MVP)

## Фаза 4: Проверка

Цель: automated tests для всех AC + benchmark для SC-002.

- [x] T4.1 Middleware unit tests (usage_test.go): mock response body с известным usage JSON → assert TokenUsage (AC-001), prometheus testutil → verify метрики (AC-003), response без usage → verify warning (AC-005), проверка вызова middleware на /chat/completions (AC-006), benchmark 4KB body (SC-002).
  Touches: `src/internal/api/middleware/usage_test.go`
  Зависимости: T3.1, T3.3

- [x] T4.2 Async worker unit tests (async_worker_test.go): mock UsageStore, send 10 TokenUsage за 3с → assert 1 вызов RecordBatch со всеми 10 записями (AC-002), проверка sync flush при переполнении буфера.
  Touches: `src/internal/app/analytics/async_worker_test.go`
  Зависимости: T3.2

- [x] T4.3 Aggregation worker integration tests (agg_worker_test.go): seed raw usage данные → run worker → verify per-hour и per-day агрегаты в таблицах (AC-004).
  Touches: `src/internal/app/analytics/agg_worker_test.go`
  Зависимости: T3.4, T2.3

- [x] T4.4 Cleanup worker integration tests (cleanup_worker_test.go): insert records старше/младше retention → run worker → verify старые удалены, новые сохранены (AC-008).
  Touches: `src/internal/app/analytics/cleanup_worker_test.go`
  Зависимости: T3.5, T2.3

- [x] T4.5 CostRate registry unit tests (analytics_test.go): конфиг с cost_rates → NewCostRateRegistry → Lookup → verify cost computation для known и unknown моделей (AC-007).
  Touches: `src/internal/domain/analytics/analytics_test.go`
  Зависимости: T2.2

- [x] T4.6 PgUsageStore integration tests (pg_usage_store_test.go): Record, RecordBatch, QueryByTenant, QueryByModel, AggregateByDay, DeleteOlderThan с test PostgreSQL (AC-004, AC-008).
  Touches: `src/internal/adapters/repository/analytics/pg_usage_store_test.go`
  Зависимости: T2.3

## Покрытие критериев приемки

- AC-001 -> T3.1, T4.1
- AC-002 -> T2.1, T3.2, T4.2
- AC-003 -> T3.1, T4.1
- AC-004 -> T2.1, T3.4, T4.3, T4.6
- AC-005 -> T3.1, T4.1
- AC-006 -> T3.3, T4.1
- AC-007 -> T2.2, T4.5
- AC-008 -> T2.1, T3.5, T4.4, T4.6

## Заметки

- Фазы 1-2 можно безопасно параллелить с настройкой репозитория.
- T1.2 (config) нужен до T2.2 (CostRate registry).
- T2.3 (PgUsageStore) нужен до T3.2 (AsyncWorker) и T3.4/T3.5.
- T3.4 и T3.5 (aggregation + cleanup) — второй проход, независимы друг от друга.
- tiktoken-go (`github.com/pulumi/tiktoken-go`) добавляется в go.mod. Prewarm на старте для избежания ~500ms на первом запросе.
- T4.1-T4.6: задачи независимы, можно параллелить. T4.3/T4.4/T4.6 требуют test PostgreSQL (pgxpool.Docker).
- T4.1 (usage_test.go) включает SC-002 benchmark. T4.5 добавляет тесты CostRateRegistry в существующий analytics_test.go.
- Каждая задача покрывает ровно один test file — implement может выполнять их в любом порядке.
