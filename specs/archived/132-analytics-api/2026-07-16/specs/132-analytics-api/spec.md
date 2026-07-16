# Analytics API

## Scope Snapshot

- In scope: REST API для просмотра аналитики использования LLM — токены, стоимость, трафик, сводка по тенанту + Grafana dashboard + OpenAPI спецификация.
- Out of scope: запись usage-данных (реализована в 131-analytics-pipeline); агрегация в реальном времени (используются materialized агрегаты); алертинг; экспорт в PDF.

## Цель

Администратор и тенант получают read-only HTTP API для просмотра статистики использования LLM через gateway: сколько токенов потреблено, сколько потрачено, какая задержка. Успех: после деплоя любой тенант может открыть `/api/v1/analytics/tokens?period=week` и увидеть потребление по моделям.

## Основной сценарий

1. Система уже собирает usage-данные через middleware и async worker (131-analytics-pipeline). AggregationWorker материализует почасовые и подневные агрегаты.
2. Тенант/auth middleware определяет `TenantID` из контекста (JWT/API-ключ).
3. HTTP-запрос к `GET /api/v1/analytics/tokens?period=day&model=llama3.2` проходит авторизацию, хендлер вызывает `UsageStore.QueryByTenant` с фильтром по `TenantID`.
4. Admin-server возвращает JSON-ответ в стандартном envelope `ApiResponse{Data, Pagination}`.
5. Разработчик/админ открывает Grafana dashboard с предсобранными панелями.

## User Stories

- P1 (MVP): API endpoints чтения агрегированных метрик по токенам и стоимости для своего тенанта.
- P2: Grafana dashboard с визуализацией трендов (daily active tenants, model distribution, latency heatmap).
- P3: CSV/JSON экспорт для reporting и compliance.

## MVP Slice

4 GET endpoints + авторизация по тенанту + OpenAPI spec. AC-001–AC-005.

## First Deployable Outcome

```shell
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/analytics/tokens?period=week
# → 200 { "data": { "records": [...], "totals": {...} }, "pagination": { "page": 1, "per_page": 50, "total": 12 } }
```

## Scope

- 4 REST endpoints на admin server:
  - `GET /api/v1/analytics/tokens` — токены по tenant/model за период
  - `GET /api/v1/analytics/cost` — стоимость по tenant/model
  - `GET /api/v1/analytics/traffic` — количество запросов, latency P50/P95/P99
  - `GET /api/v1/analytics/tenants/:slug/summary` — сводка по конкретному тенанту (admin-only)
- Tenant-scoped авторизация: тенант видит только свою аналитику, admin видит всё
- CSV/JSON export через query-параметр `?format=csv|json`
- Grafana dashboard provisioning (JSON) в `deployments/grafana/dashboards/analytics.json`
- OpenAPI spec в `api/analytics.yaml` или встроенная swagger-документация
- Query-параметры: `period` (day/week/month), `from`/`to`, `model`, `page`/`per_page`

## Контекст

- Usage-данные уже записываются в `usage_agg_hourly` и `usage_agg_daily` (131-analytics-pipeline)
- Admin server уже работает на порту 8080 со стандартным middleware stack (auth, envelope, metrics)
- `UsageStore` port уже включает `QueryByTenant`, `QueryByModel`, `AggregateByDay`
- `PgUsageStore` уже реализует все методы порта
- Тенанты хранятся в PostgreSQL, auth middleware предоставляет `TenantID` в `gin.Context`
- Latency пока не записывается в analytics — потребуется расширение `usage_raw` или отдельная метрика

## Зависимости

- 131-analytics-pipeline (завершён) — `usage_agg_hourly`/`usage_agg_daily` таблицы и `UsageStore` порт
- Auth middleware — tenant context из JWT
- Grafana (если dashboard деплоится) — внешняя зависимость для визуализации

## Требования

- RQ-001 Система ДОЛЖНА возвращать агрегированные токены (input+output) по tenant + model за day/week/month
- RQ-002 Система ДОЛЖНА возвращать стоимость (total_cost) по tenant + model за период
- RQ-003 Система ДОЛЖНА возвращать traffic-метрики: request_count, latency P50/P95/P99 (где latency доступна)
- RQ-004 Tenant ДОЛЖЕН видеть только свои данные; admin ДОЛЖЕН видеть все тенанты
- RQ-005 API ДОЛЖЕН поддерживать пагинацию (page/per_page) и экспорт (format=csv|json)
- RQ-006 Grafana dashboard ДОЛЖЕН быть provisioned как JSON в репозитории

## Вне scope

- Запись/редактирование usage-данных (read-only API)
- Алертинг на основе метрик
- Real-time streaming метрик (Server-Sent Events)
- Сравнение периодов (period-over-period)
- Управление тенантами через analytics API
- Экспорт в XLSX/PDF

## Критерии приемки

### AC-001 Tokens endpoint

- Почему это важно: базовая метрика — сколько токенов потребляет каждый тенант/модель
- **Given** существующие usage-записи в `usage_agg_daily` за последние 7 дней для tenantA и model=gpt-4
- **When** tenantA отправляет `GET /api/v1/analytics/tokens?period=week`
- **Then** ответ содержит records с `{tenant_id, model, total_input_tokens, total_output_tokens, period_start, period_end}` и totals
- Evidence: тест проверяет, что хендлер возвращает правильные суммы из `UsageStore.QueryByTenant`, отсортированные по дате

### AC-002 Cost endpoint

- Почему это важно: стоимость — ключевой бизнес-показатель
- **Given** usage-записи с `total_cost` для tenantA
- **When** tenantA отправляет `GET /api/v1/analytics/cost?period=month`
- **Then** ответ содержит records с `{tenant_id, model, total_cost, request_count, period_start, period_end}`
- Evidence: тест с mock UsageStore проверяет корректный JSON

### AC-003 Traffic endpoint

- Почему это важно: понимание нагрузки и производительности
- **Given** usage-записи с request_count и latency (если доступно)
- **When** tenantA отправляет `GET /api/v1/analytics/traffic?period=day`
- **Then** ответ содержит `{request_count, avg_latency_ms, p50_latency_ms, p95_latency_ms, p99_latency_ms}` (null если данных нет)
- Evidence: тест проверяет структуру ответа и fallback для отсутствующих latency

### AC-004 Tenant summary (admin-only)

- Почему это важно: admin видит сводку по конкретному тенанту
- **Given** admin-пользователь и записи для tenantB
- **When** admin отправляет `GET /api/v1/analytics/tenants/tenantB/summary`
- **Then** ответ содержит сводку: `{tenant_id, total_tokens, total_cost, request_count, model_breakdown: [{model, tokens, cost, pct}]}`
- Evidence: тест с admin-контекстом проверяет сводку; tenant (не admin) получает 403

### AC-005 Пагинация и экспорт

- Почему это важно: API должен работать с большими объёмами данных
- **Given** более 100 записей для запроса
- **When** `GET /api/v1/analytics/tokens?page=1&per_page=10&format=csv`
- **Then** ответ: в JSON-режиме — пагинированные records; в CSV-режиме — `Content-Type: text/csv` с headers
- Evidence: тест проверяет `pagination` в JSON и CSV content-type в export

### AC-006 Grafana dashboard

- Почему это важно: визуализация трендов для оператора
- **Given** Grafana с datasource на Prometheus/Gateway API
- **When** оператор открывает Analytics dashboard
- **Then** dashboard показывает панели: tokens/day, cost/day, top models, requests/day, latency heatmap
- Evidence: dashboard JSON лежит в `deployments/grafana/dashboards/analytics.json` с валидной структурой Grafana

## Допущения

- UsageStore уже подключён к PG в admin main.go (131-analytics-pipeline DI wiring)
- Auth middleware устанавливает `TenantID` и `IsAdmin` в gin.Context
- Latency данные пока не записываются — P50/P95/P99 возвращают null до реализации записи latency
- CSV экспорт использует стандартный `text/csv` Content-Type с BOM для Excel-совместимости
- Dashboard JSON совместим с Grafana 10+

## Критерии успеха

- SC-001 Запрос к /analytics/tokens за 30 дней возвращает ответ <500ms p95 при 10k записей
- SC-002 Dashboard открывается в Grafana без ошибок provisioning
- SC-003 Экспорт 10k records в CSV завершается <2s

## Краевые случаи

- Пустой период: 0 records с корректным totals={0}
- Неизвестный tenant slug в /tenants/:slug/summary: 404
- Тенант пытается посмотреть чужой slug: 403
- Невалидный period: 400 Bad Request
- Огромный запрос без пагинации: ограничить per_page=1000
- CSV с специальными символами: правильное экранирование
- Grafana datasource недоступен: dashboard отображается с error-панелью (Grafana-side)

## Открытые вопросы

1. Latency пока не записывается — сохранить ли latency в usage_raw или читать из отдельного источника? Решение: добавить latency поля в usage_raw в будущей spec; пока API возвращает null.
2. CSV-экспорт — потоковый (streaming) или полная буферизация? Решение: буферизация для MVP (до 10k records); streaming при необходимости.
3. Grafana dashboard — JSON модель или YAML? Решение: JSON (стандарт Grafana provisioning).
