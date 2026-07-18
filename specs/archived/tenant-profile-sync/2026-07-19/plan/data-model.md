# Tenant & Profile Config-DB Sync — Модель данных

## Scope

- Связанные AC: AC-001, AC-005, AC-006, AC-009
- Связанные DEC: DEC-001, DEC-003, DEC-004
- Статус: changed

## Сущности

### DM-001 Tenant (новая, заменяет Profile)

- Назначение: единая сущность для auth + dictionaries конфигурации контент-шилда
- Источник истины: PostgreSQL (таблица `tenants`), config.yaml при старте (seed, не перезатирает)
- Инварианты:
  - slug уникален (PRIMARY KEY)
  - api_keys — непустой массив строк
  - dictionaries — массив объектов с name, match_mode, entries
  - Каждый entry — string (плоское значение) или object (структурированное)
- Связанные AC: AC-001, AC-002, AC-003, AC-005, AC-006, AC-007
- Связанные DEC: DEC-001, DEC-004
- Поля:
  - `slug TEXT PK` — уникальный идентификатор tenant
  - `name TEXT NOT NULL` — человекочитаемое имя
  - `auth_header TEXT NOT NULL DEFAULT 'Authorization'` — HTTP-заголовок для API key
  - `api_keys JSONB NOT NULL DEFAULT '[]'` — массив API-ключей
  - `dictionaries JSONB NOT NULL DEFAULT '[]'` — массив словарей
  - `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
  - `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- Жизненный цикл:
  - create: POST /api/v1/admin/tenants + startup sync из config.yaml
  - update: PUT /api/v1/admin/tenants/:slug + PUT /api/v1/admin/tenants/:slug/dictionaries
  - delete: DELETE /api/v1/admin/tenants/:slug
- Замечания по консистентности:
  - Недопустимо: tenant с пустым slug или api_keys
  - startup sync не перезатирает существующие записи (только INSERT ON CONFLICT DO NOTHING)

### DM-002 Incident (изменение FK)

- Назначение: журнал срабатываний shield (было profile_slug → стало tenant_slug)
- Источник истины: PostgreSQL (таблица `incidents`)
- Изменение:
  - `profile_slug TEXT NOT NULL REFERENCES profiles(slug)` → `tenant_slug TEXT NOT NULL REFERENCES tenants(slug)`
  - Миграция 006 UPDATE существующих записей: `SET tenant_slug = profile_slug`
- Связанные AC: AC-009

### DM-003 Profile (удаляется)

- Статус: deleted
- Таблицы `profiles` и `dictionary_entries` удаляются миграцией 006

## Связи

- Tenant (1) → Incident (N): tenant_slug FK
- Tenant ↔ Dictionary: dictionaries JSONB (inline, без отдельной таблицы)

## Производные правила

- DictionaryCache: при загрузке tenant из БД dictionaries парсятся в `[]*dictionary.Dictionary`. Ключ кеша — tenant_slug.
- Startup sync: config tenants → INSERT ON CONFLICT DO NOTHING в БД.

## Переходы состояний

- create: POST или startup sync → активен
- update: PUT или dictionaries PUT → обновлён (updated_at = now())
- delete: DELETE → удалён (физический, без soft-delete)

## Вне scope

- Soft-delete для tenants (физический DELETE)
- Версионирование dictionaries (замена целиком)
- Audit log для изменений
