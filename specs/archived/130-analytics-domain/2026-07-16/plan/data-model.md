# Analytics Domain Layer Модель данных

## Scope

- Связанные `AC-*`: `AC-001`, `AC-003`, `AC-005`
- Связанные `DEC-*`: `DEC-001`, `DEC-003`
- Статус: `no-change`
- Причина: фича определяет entity и value objects в domain-слое (in-memory); persistence, таблицы БД, API-контракты не затрагиваются. Data model будет определена в фазе реализации адаптера.

## No-Change Stub

- Статус: `no-change`
- Причина: фича не добавляет и не меняет persisted entities, value objects, state transitions или contract-relevant payload shapes
- Revisit triggers:
  - появляется новое сохраняемое состояние (адаптер/БД)
  - API/event payload shape нужно отслеживать
  - миграция схемы БД для UsageStore
