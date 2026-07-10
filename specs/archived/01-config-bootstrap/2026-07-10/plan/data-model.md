# Config Bootstrap Модель данных

## Scope

- Связанные `AC-*`: нет (AC-001–AC-005 не затрагивают persisted data model)
- Связанные `DEC-*`: нет
- Статус: `no-change`

## No-Change Stub

- Статус: `no-change`
- Причина: фича добавляет runtime-конфигурацию (Config struct в памяти), не создаёт и не меняет persisted entities, value objects, state transitions или contract-relevant payload shapes
- Revisit triggers:
  - появляется новое сохраняемое состояние (БД, кеш, файл)
  - API/event payload shape нужно отслеживать здесь
