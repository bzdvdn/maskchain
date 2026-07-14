# Critical Test Coverage — Модель данных

- Статус: `no-change`
- Причина: фича не добавляет и не меняет persisted entities, value objects, state transitions или contract-relevant payload shapes. Все изменения — только `_test.go` файлы.
- Revisit triggers:
  - появляется новое сохраняемое состояние
  - появляются новые инварианты или lifecycle states
  - API/event payload shape нужно отслеживать именно здесь
