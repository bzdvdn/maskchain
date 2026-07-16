# 131-analytics-pipeline Модель данных

## Scope

- Связанные `AC-*`: `AC-001`, `AC-002`, `AC-004`, `AC-008`
- Связанные `DEC-*`: `DEC-003`, `DEC-004`
- Статус: `changed`
- Domain-сущности в `src/internal/domain/analytics/` не меняются (TokenUsage, UsageRecord, Aggregation, CostRate). Добавляются три PostgreSQL таблицы для хранения.

## Таблицы

### DM-001 usage_raw

- Назначение: сырые записи каждого LLM-запроса, источник истины для агрегации.
- Источник истины: async worker (batch insert).
- Инварианты: `id` уникален, `tenant_id` + `model` + `recorded_at` не NULL.
- Связанные `AC-*`: `AC-002`
- Связанные `DEC-*`: `DEC-002`
- Поля:
  - `id` - UUIDv7, PK, генерируется в middleware
  - `tenant_id` - VARCHAR(255), NOT NULL, tenant slug
  - `model` - VARCHAR(255), NOT NULL, имя модели
  - `input_tokens` - BIGINT, NOT NULL, >= 0
  - `output_tokens` - BIGINT, NOT NULL, >= 0
  - `cost` - NUMERIC(12,6), NOT NULL, >= 0
  - `recorded_at` - TIMESTAMPTZ, NOT NULL, время запроса
- Индексы: `(tenant_id, recorded_at)`, `(recorded_at)` для cleanup
- Жизненный цикл:
  - создаётся: async worker вызывает RecordBatch -> INSERT в usage_raw
  - обновляется: никогда (append-only)
  - удаляется: cleanup worker (DELETE WHERE recorded_at < retention)
- Замечания по консистентности:
  - идемпотентность: `id` уникален, повторный INSERT с тем же id — conflict (можно ON CONFLICT DO NOTHING)
  - out-of-order: возможны запросы с out-of-order recorded_at (до 5 секунд). Для агрегации не критично — окна по recorded_at.

### DM-002 usage_agg_hourly

- Назначение: материализованные per-hour агрегаты для быстрых запросов операторов.
- Источник истины: aggregation worker (upsert).
- Инварианты: `(tenant_id, model, hour)` уникален.
- Связанные `AC-*`: `AC-004`
- Связанные `DEC-*`: `DEC-004`
- Поля:
  - `tenant_id` - VARCHAR(255), NOT NULL
  - `model` - VARCHAR(255), NOT NULL
  - `hour` - TIMESTAMPTZ, NOT NULL, начало часа (truncated)
  - `total_input_tokens` - BIGINT, NOT NULL, >= 0
  - `total_output_tokens` - BIGINT, NOT NULL, >= 0
  - `total_cost` - NUMERIC(14,6), NOT NULL, >= 0
  - `request_count` - BIGINT, NOT NULL, >= 0
  - `updated_at` - TIMESTAMPTZ, NOT NULL, время последнего обновления
- Индексы: `(tenant_id, hour)`, `(model, hour)`
- Жизненный цикл:
  - создаётся/обновляется: aggregation worker вычисляет SUM по usage_raw за час и UPSERT
  - удаляется: только при ручной архивации (не автоматически)
- Замечания по консистентности:
  - worker идемпотентен: per-hour агрегат пересчитывается каждый цикл

### DM-003 usage_agg_daily

- Назначение: материализованные per-day агрегаты для дашбордов и отчётов.
- Источник истины: aggregation worker (upsert).
- Инварианты: `(tenant_id, model, day)` уникален.
- Связанные `AC-*`: `AC-004`
- Связанные `DEC-*`: `DEC-004`
- Поля:
  - `tenant_id` - VARCHAR(255), NOT NULL
  - `model` - VARCHAR(255), NOT NULL
  - `day` - DATE, NOT NULL
  - `total_input_tokens` - BIGINT, NOT NULL, >= 0
  - `total_output_tokens` - BIGINT, NOT NULL, >= 0
  - `total_cost` - NUMERIC(14,6), NOT NULL, >= 0
  - `request_count` - BIGINT, NOT NULL, >= 0
  - `updated_at` - TIMESTAMPTZ, NOT NULL
- Индексы: `(tenant_id, day)`, `(model, day)`
- Жизненный цикл: аналогично DM-002, но по дням.

## Связи

- `usage_raw` -> `usage_agg_hourly`: worker агрегирует по hour(recorded_at), GROUP BY tenant_id, model.
- `usage_raw` -> `usage_agg_daily`: worker агрегирует по date(recorded_at), GROUP BY tenant_id, model.
- `usage_agg_hourly` и `usage_agg_daily` независимы, не ссылаются друг на друга.

## Производные правила

- Aggregation worker: `SELECT tenant_id, model, date_trunc('hour', recorded_at) AS hour, SUM(input_tokens), SUM(output_tokens), SUM(cost), COUNT(*) FROM usage_raw WHERE recorded_at >= $1 GROUP BY tenant_id, model, hour` — UPSERT в usage_agg_hourly.
- Аналогично для per-day с `date_trunc('day', recorded_at)`.

## Переходы состояний

- Простая модель append-only: запись создаётся, живёт до cleanup, удаляется. Aggregates пересчитываются каждый цикл.

## Вне scope

- Valkey/кэш для агрегатов — только PostgreSQL.
- Партиционирование usage_raw — отложено (если >100M записей).
- Audit log удаления сырых данных — достаточно лога worker.
