---
status: no-change
slug: 118-api-consistency
---

# Data Model: 118-api-consistency

## Status

- status: no-change

## Причина

Фича затрагивает только API-слой представления (envelope, pagination format, response types).
Никакие доменные сущности, агрегаты, value objects, таблицы PostgreSQL или ключи Valkey не меняются.
Изменения касаются исключительно `dto`-пакетов и middleware в `src/internal/api/`.
