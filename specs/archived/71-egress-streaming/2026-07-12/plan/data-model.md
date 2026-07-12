# Egress Streaming — Модель данных

## Scope

- Связанные AC-*: none
- Связанные DEC-*: DEC-004
- Статус: no-change

## No-Change Stub

- Статус: `no-change`
- Причина: фича не добавляет и не меняет persisted entities, value objects, state transitions или contract-relevant payload shapes. Конфигурация egress (таймауты, pool, retry) — инфраструктурная, не является domain data model.
- Revisit triggers:
  - появляется новое сохраняемое состояние (напр., per-provider transport metrics)
  - API/event payload shape требует отслеживания
