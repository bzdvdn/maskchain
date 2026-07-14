# Rate Limit Wiring — Модель данных

## Scope

- Связанные `AC-*`: none
- Связанные `DEC-*`: none
- Статус: `no-change`

## No-Change Stub

- Статус: `no-change`
- Причина: фича не добавляет и не меняет persisted entities, value objects, state transitions или contract-relevant payload shapes. Все доменные типы (`budget.RateLimit`, `budget.TokenBudget`, `budget.RateLimitRepository`, `budget.TokenBudgetRepository`) уже определены в фазе `rate-limiting-budgets`. Конфигурационные типы (`RateLimitConfig`, `RateLimitOverride`) уже определены в `config.go`. Добавляется только HTTP header `Retry-After` — это transport-level изменение, не затрагивающее модель данных.
- Revisit triggers:
  - появляется новое сохраняемое состояние (Valkey key format уже определён)
  - появляются новые инварианты или lifecycle states для budget сущностей
  - API/event payload shape требует отслеживания
