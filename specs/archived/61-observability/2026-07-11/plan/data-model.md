# 61-observability Модель данных

## No-Change Stub

- Статус: `no-change`
- Причина: фича не добавляет и не меняет persisted entities, value objects, state transitions или contract-relevant payload shapes. Все изменения — инфраструктурные (OTel SDK init, Prometheus метрики, slog adapter, middleware, docker-compose). Config секция `otel` не влияет на data model.
- Revisit triggers:
  - появляется новое сохраняемое состояние (например, метрики в timeseries БД)
  - появляются новые инварианты или lifecycle states
  - API/event payload shape нужно отслеживать именно здесь
