# 112 Proxy Streaming Wiring — Модель данных

## Scope

- Статус: `no-change`
- Причина: фича не добавляет и не меняет persisted entities, value objects, state transitions или contract-relevant payload shapes. `chatRequest.Stream bool` — runtime-only поле для десериализации JSON-запроса, не сохраняется и не влияет на доменные сущности.
- Revisit triggers:
  - появляется новое сохраняемое состояние (напр., streaming-сессия в базе)
  - streaming требует особых инвариантов или lifecycle tracking на уровне данных
