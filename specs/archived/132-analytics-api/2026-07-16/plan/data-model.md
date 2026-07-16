# Data Model: Analytics API

## Status

no-change

## Обоснование

Все необходимые данные уже присутствуют:
- `usage_agg_daily` — агрегированные токены/стоимость/request_count по tenant+model+day
- `usage_agg_hourly` — часовые агрегаты (если нужны)
- `UsageRecord` и `Aggregation` — domain entities для ответов
- `UsageStore.QueryByTenant`, `AggregateByDay` — готовые методы запросов

Новых таблиц, сущностей или DTO не требуется. DTO для API-ответов создаются на уровне handler.
