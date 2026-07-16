# Analytics API План

## Phase Contract

Inputs: spec, inspect (pass).
Outputs: plan, data-model (no-change).
Stop if: нет.

## Цель

Добавить read-only REST API для аналитики LLM usage на admin server. Переиспользовать существующий `UsageStore` порт и `PgUsageStore`. Без изменений в домене, миграциях или gateway.

## MVP Slice

4 GET endpoints + tenant-scoped auth + CSV export. AC-001–AC-005. Grafana dashboard (AC-006) — post-MVP.

## First Validation Path

```shell
curl -H "Authorization: Bearer <admin-token>" \
  "http://localhost:8080/api/v1/analytics/tokens?period=week"
# → 200 { "data": { "records": [...], "totals": {...} }, "pagination": {...} }
```

## Scope

- `src/internal/api/handler/analytics/analytics_handler.go` — новый handler
- `src/internal/api/dto/analytics.go` — новые DTO
- `src/internal/api/admin.go` — регистрация routes
- `src/cmd/admin/main.go` — DI wiring (UsageStore)
- `deployments/grafana/dashboards/analytics.json` — dashboard (post-MVP)
- **Нетронуто**: gateway server, migrations, domain entities, UsageStore port

## Performance Budget

- none (запросы к `usage_agg_daily` — пара десятков rows; пагинация ограничивает per_page≤1000)

## Implementation Surfaces

- `src/internal/api/handler/analytics/analytics_handler.go` — **новая**, 4 endpoint methods + CSV export
- `src/internal/api/dto/analytics.go` — **новая**, DTO для запросов/ответов
- `src/internal/api/admin.go` — **существующая**, +1 Register-метод
- `src/cmd/admin/main.go` — **существующая**, +DI wiring
- `deployments/grafana/dashboards/analytics.json` — **новая**, Grafana provisioning (post-MVP)

## Bootstrapping Surfaces

- `src/internal/api/handler/analytics/` — создать директорию и файл handler

## Влияние на архитектуру

- Локальное: новый handler в admin server, использует существующий `domain/analytics`
- Конфигурация: не меняется
- Обратная совместимость: admin server уже имеет handler-паттерн, изменения прозрачны

## Acceptance Approach

- AC-001: `TokensHandler` → `UsageStore.QueryByTenant` + агрегация по period
- AC-002: `CostHandler` → те же данные, группировка по total_cost
- AC-003: `TrafficHandler` → request_count из агрегатов; latency = nil пока
- AC-004: `TenantSummaryHandler` — проверка IsAdmin, `AggregateByDay` + model breakdown
- AC-005: пагинация через `page`/`per_page` params; CSV через `format=csv` → `c.Writer` с CSV заголовками
- AC-006: Grafana dashboard — post-MVP после деплоя API

## Данные и контракты

- Data model не меняется — см. `data-model.md: no-change`
- `UsageStore.QueryByTenant` и `AggregateByDay` покрывают все queries
- Ответы оборачиваются в `ApiResponse` (envelope middleware)

## Стратегия реализации

- DEC-001 Единый AnalyticsHandler с 4 методами
  Why: все endpoints используют один порт (UsageStore) и общую логику tenant-scoping/period-parsing/pagination; однотипные методы проще поддерживать вместе
  Tradeoff: если endpoints эволюционируют по-разному, разделение придётся делать позже
  Affects: analytics_handler.go
  Validation: AC-001–AC-005

- DEC-002 Period через date_trunc в UsageStore + Go-агрегация
  Why: `usage_agg_daily` даёт агрегаты по дням; week/month считаем в Go суммированием записей за N дней. Не нужно множить DB-запросы
  Tradeoff: при period=месяц все 30-31 записей тянутся в память — допустимо для агрегатов (сотни records)
  Affects: analytics_handler.go
  Validation: AC-001

- DEC-003 Tenant-scoping на уровне handler, не на уровне DB
  Why: auth middleware кладёт TenantID и IsAdmin в context. Handler читает их и передаёт в UsageStore.QueryByTenant. Admin видит всё (filter="" или all)
  Tradeoff: если tenant не указан и не admin — 403. Логика в одном месте, легко тестировать
  Affects: analytics_handler.go
  Validation: AC-004

- DEC-004 CSV экспорт через ?format=csv без отдельного endpoint
  Why: единый код парсинга параметров + switch по format. CSV пишется через `c.Writer` с `Content-Type: text/csv`
  Tradeoff: смешивание форматов в одном хендлере усложняет чтение, но уменьшает дублирование
  Affects: analytics_handler.go
  Validation: AC-005

## Incremental Delivery

### MVP (Первая ценность)

1. DTO: `analytics.go` — request/response типы
2. Handler: `AnalyticsHandler` с 4 методами + CSV export
3. Admin: регистрация routes + DI wiring
4. Unit-тесты: AC-001–AC-005

### Итеративное расширение

- post-MVP: Grafana dashboard provisioning (AC-006)
- post-MVP: latency capture + latency в traffic endpoint
- post-MVP: streaming CSV для больших экспортов
- post-MVP: period-over-period сравнение

## Порядок реализации

1. **DTO** — типы без логики, нужны для handler
2. **Handler** — вся логика endpoints
3. **Admin routes** — регистрация
4. **DI wiring** — подключение в main.go
5. **Tests** — все AC

Параллелизация: DTO можно писать параллельно с handler.

## Риски

- QueryByTenant возвращает `recorded_at` как PeriodStart и PeriodEnd одновременно — это нормально для raw, но для агрегированных данных нужна корректная PeriodStart/PeriodEnd.
  Mitigation: handler вычисляет periodStart/periodEnd на основе query dates или берёт из агрегатов
- Missing latency field: если latency не в usage_agg, TrafficHandler не сможет показать latency.
  Mitigation: возвращать null, документировать в OpenAPI
- CSV с большими данными: буферизация всего ответа перед записью.
  Mitigation: per_page=1000 max; если нужно больше — streaming в post-MVP

## Rollout и compatibility

- Специальных rollout-действий не требуется
- Новые endpoints на admin server — обратно совместимы
- OpenAPI spec документирует endpoints, но не блокирует деплой

## Проверка

- `go test ./internal/api/handler/analytics/ -count=1` — AC-001–AC-005
- `go build ./...` + `go vet ./...`
- Manual curl к admin server

## Соответствие конституции

- нет конфликтов
