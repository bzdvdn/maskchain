# Dependency-aware health/readiness probes — Модель данных

## Scope

- Связанные `AC-*`: `AC-007`
- Связанные `DEC-*`: `DEC-001`
- Статус: `no-change`
- Явно: фича не добавляет и не меняет persisted entities, value objects, state transitions или contract-relevant payload shapes. Health-ответы — transient JSON, не хранятся и не реплицируются.

## No-Change Stub

- Статус: `no-change`
- Причина: фича не требует новых persisted сущностей, не меняет схемы БД, не добавляет event-контрактов. Единственное новое поле — `server.health_check.critical_deps` в конфиге (viper/yaml), не является data model.
- Revisit triggers:
  - появляется необходимость сохранять историю health-проверок для аудита;
  - health-статус начинает влиять на persisted state (например, автоматическое отключение dependency в конфиге).
