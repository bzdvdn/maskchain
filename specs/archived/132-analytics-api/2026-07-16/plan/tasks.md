# Analytics API Задачи

## Phase Contract

Inputs: plan, data-model (no-change).
Outputs: исполнимые задачи с покрытием AC-001–AC-005.
Stop if: нет.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/api/dto/analytics.go` | T1.1 |
| `src/internal/api/handler/analytics/analytics_handler.go` | T2.1 |
| `src/internal/api/admin.go` | T2.2 |
| `src/cmd/admin/main.go` | T2.3 |
| `src/internal/api/handler/analytics/analytics_handler_test.go` | T3.1 |

## Implementation Context

- Цель MVP: 4 read-only REST endpoints на admin server для просмотра токенов, стоимости, трафика и сводки по тенанту
- Инварианты:
  - `UsageStore` порт уже реализован (`QueryByTenant`, `AggregateByDay`)
  - Auth middleware кладёт `TenantID` (string) и `IsAdmin` (bool) в `gin.Context`
  - Ответы оборачиваются `ResponseEnvelope` middleware в `ApiResponse{Data, Error, Pagination}`
  - Периоды: day = 1 день, week = 7 дней, month = 30 дней — вычисляются в handler
- Ошибки/коды:
  - 400 — невалидный period/параметр
  - 403 — tenant пытается запросить чужой slug
  - 404 — неизвестный tenant slug
  - Использовать `middleware.AbortWithError(c, status, ErrorCode*, msg)`
- Контракты/протокол: все endpoints GET, query params `period`, `from`, `to`, `model`, `page`, `per_page`, `format`
- Границы scope: не трогать gateway server, migrations, domain entities
- Proof signals: `go test ./internal/api/handler/analytics/ -count=1` pass; curl к admin server
- References: DEC-001 (единый handler), DEC-002 (period aggregation), DEC-003 (tenant-scoping), DEC-004 (CSV export)

## Фаза 1: DTO

Цель: request/response типы для аналитики.

- [x] T1.1 Создать `analytics.go` в `src/internal/api/dto/` — типы:
  - `TokensResponse{Records []TokenRecord, Totals TokenTotals}`
  - `TokenRecord{TenantID, Model, TotalInputTokens, TotalOutputTokens, PeriodStart, PeriodEnd}`
  - `TokenTotals{TotalInputTokens, TotalOutputTokens}`
  - `CostResponse{Records []CostRecord, Totals CostTotals}`, `CostRecord`, `CostTotals`
  - `TrafficResponse{RequestCount, AvgLatencyMs, P50LatencyMs, P95LatencyMs, P99LatencyMs}`
  - `TenantSummaryResponse{TenantID, TotalTokens, TotalCost, RequestCount, ModelBreakdown []ModelBreakdown}`
  - `ModelBreakdown{Model, Tokens, Cost, Percentage}`
  - `AnalyticsQuery{Period, From, To, Model, Page, PerPage, Format}` — парсинг query params
  Touches: `src/internal/api/dto/analytics.go`
  AC: AC-001, AC-002, AC-003, AC-004, AC-005

## Фаза 2: Handler + Wiring

Цель: реализация endpoints и подключение к admin server.

- [x] T2.1 Реализовать `AnalyticsHandler` в `handler/analytics/analytics_handler.go`:
  - Структура с `store domain.UsageStore`
  - `NewAnalyticsHandler(store)` — конструктор
  - `HandleTokens(c)` — AC-001: парсинг period → `QueryByTenant` → агрегация по модели → JSON/CSV
  - `HandleCost(c)` — AC-002: те же данные → группировка по total_cost
  - `HandleTraffic(c)` — AC-003: request_count из агрегатов; latency = null (поле опционально)
  - `HandleTenantSummary(c)` — AC-004: `AggregateByDay` → model breakdown с %
  - CSV export: общий helper `writeCSV(c, headers, rows)` вызывается при `format=csv`
  - Общие хелперы: `parsePeriod` (day/week→from/to), `tenantFromContext`, `paginationFromQuery`
  Tenant-scoping: если не admin → `QueryByTenant(c, tenantID, from, to)`
  Touches: `src/internal/api/handler/analytics/analytics_handler.go`
  AC: AC-001, AC-002, AC-003, AC-004, AC-005

- [x] T2.2 Добавить `RegisterAnalyticsHandler(h *AnalyticsHandler)` в `AdminServer` — routes:
  ```
  analytics.GET("/tokens", h.HandleTokens)
  analytics.GET("/cost", h.HandleCost)
  analytics.GET("/traffic", h.HandleTraffic)
  analytics.GET("/tenants/:slug/summary", h.HandleTenantSummary)
  ```
  Touches: `src/internal/api/admin.go`
  AC: AC-001, AC-002, AC-003, AC-004

- [x] T2.3 Добавить DI wiring в `cmd/admin/main.go`: создать `AnalyticsHandler` с `UsageStore` (из `PgUsageStore`) и зарегистрировать через `srv.RegisterAnalyticsHandler`. Обработать случай `pgPool == nil` (лог + nil handler = routes не добавляются).
  Touches: `src/cmd/admin/main.go`
  AC: AC-001, AC-002, AC-003, AC-004

## Фаза 3: Тесты

Цель: automated tests для AC-001–AC-005.

- [x] T3.1 Написать тесты:
  - `TestAnalyticsHandler_Tokens` — mock UsageStore, проверка JSON с records и totals (AC-001)
  - `TestAnalyticsHandler_Cost` — mock, проверка cost response (AC-002)
  - `TestAnalyticsHandler_Traffic` — mock, проверка traffic response с null latency (AC-003)
  - `TestAnalyticsHandler_CSVExport` — `?format=csv`, проверка Content-Type и CSV строк (AC-005)
  - `TestAnalyticsHandler_InvalidPeriod` — bad request → 400
  - `TestAnalyticsHandler_Pagination` — page/per_page params, проверка pagination meta (AC-005)
  Использованы `@sk-test` маркеры над каждой test function.
  Touches: `src/internal/api/handler/analytics/analytics_handler_test.go`
  AC: AC-001, AC-002, AC-003, AC-004, AC-005

- [x] T4.1 Выполнить verify: `go build ./...`, `go vet ./...`, `go test ./internal/api/handler/analytics/ -count=1`. Проверить `@sk-task`/`@sk-test` маркеры. Записать `verify.md`.
  Touches: `specs/active/132-analytics-api/verify.md`
  AC: AC-001, AC-002, AC-003, AC-004, AC-005

## Покрытие критериев приемки

- AC-001 -> T1.1, T2.1, T2.2, T2.3, T3.1, T4.1
- AC-002 -> T1.1, T2.1, T2.2, T2.3, T3.1, T4.1
- AC-003 -> T1.1, T2.1, T2.2, T2.3, T3.1, T4.1
- AC-004 -> T1.1, T2.1, T2.2, T2.3, T3.1, T4.1
- AC-005 -> T1.1, T2.1, T3.1, T4.1
- AC-006 -> post-MVP (Grafana dashboard)
