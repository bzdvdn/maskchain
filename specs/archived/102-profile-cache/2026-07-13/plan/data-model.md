# Profile Cache — Модель данных

## Scope

- Связанные `AC-*`: none (изменений модели нет)
- Связанные `DEC-*`: none
- Статус: `no-change`
- Явно: фича не добавляет и не меняет persisted entities, value objects, state transitions или contract-relevant payload shapes

## No-Change Stub

- Статус: `no-change`
- Причина: `ProfileCache` — кэширующий слой поверх существующего `PostgresProfileRepo`. Все данные продолжают храниться в таблице `profiles` (PG), без новых таблиц, колонок или индексов. PubSub-канал `profile.invalidate:<slug>` — runtime-контракт, не persisted data.
- Revisit triggers:
  - появляется новое сохраняемое состояние (напр., persistent cache warming, persisted invalidation log)
  - появляются новые инварианты или lifecycle states для Profile entity
  - API/event payload shape меняется
