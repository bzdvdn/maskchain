# Prompt Injection Shield Модель данных

## Scope

- Связанные `AC-*`: `none` (нет изменений persisted модели)
- Связанные `DEC-*`: `DEC-001`
- Статус: `no-change`

## No-Change Stub

- Статус: `no-change`
- Причина: фича не добавляет и не меняет persisted entities, value objects, state transitions или contract-relevant payload shapes. Новый `DetectorType` — это константа в Go-коде (не в БД). All built-in patterns — compile-time строковые литералы (не persist). Tenant-level override — через существующую модель `entity.Detector + Pattern`.
- Revisit triggers:
  - если built-in patterns потребуется хранить в БД для auto-update
  - если появится новый persisted state (например, "pattern override set" как tenant resource)
