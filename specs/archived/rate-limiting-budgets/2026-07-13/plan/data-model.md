# Rate Limiting & Token Budgets Модель данных

## Scope

- Связанные `AC-*`: `all`
- Связанные `DEC-*`: `DEC-001`, `DEC-002`
- Статус: `no-change`

## No-Change Stub

- **Статус**: `no-change`
- **Причина**: фича не добавляет и не меняет persisted entities, value objects, state transitions или contract-relevant payload shapes. Все данные — временные счётчики в Valkey с TTL (sorted set + INCR counters). PostgreSQL/schemas не затрагиваются.
- **Revisit triggers**:
  - появляется новое сохраняемое состояние (например, история rate-limit событий в БД)
  - появляются новые инварианты или lifecycle states для существующих entities
  - API/event payload shape нужно отслеживать именно здесь
