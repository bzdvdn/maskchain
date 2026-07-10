# Persistence Layer Data Model

## Scope

- Связанные `AC-*`: `AC-001`, `AC-002`, `AC-004`
- Связанные `DEC-*`: `DEC-001`, `DEC-003`, `DEC-005`
- Статус: `changed`
- Изменения: добавлены таблицы `profiles` и `incidents`, схема `dictionary_entries` изменена (JSONB entries → строки entry_value + match_mode)

## Сущности

### DM-001 profiles

- Назначение: хранилище профилей Content Shield
- Источник истины: PostgreSQL (единственное persisted-состояние профиля)
- Инварианты: slug UNIQUE; tenant_id NOT NULL; preprocessors JSONB (NULL/nullable)
- Связанные `AC-*`: `AC-001`, `AC-002`
- Связанные `DEC-*`: `DEC-003` (slug generation)
- Поля:
  - `id` - UUIDv7, PK, auto-generated
  - `slug` - VARCHAR, UNIQUE NOT NULL, внешний ключ для dictionary_entries/incidents, auto-generated (DEC-003)
  - `name` - VARCHAR NOT NULL, имя профиля
  - `tenant_id` - VARCHAR NOT NULL, tenant owner
  - `preprocessors` - JSONB, nullable, preprocessor definitions
  - `status` - VARCHAR NOT NULL DEFAULT 'active', статус профиля
  - `version` - INTEGER NOT NULL DEFAULT 1, оптимистичная блокировка/версионирование
  - `created_at` - TIMESTAMPTZ NOT NULL DEFAULT now()
  - `updated_at` - TIMESTAMPTZ NOT NULL DEFAULT now()
- Жизненный цикл:
  - Создаётся: ProfileRepo.Save() нового профиля
  - Обновляется: ProfileRepo.Save() существующего профиля (version increment)
  - Удаляется: ProfileRepo.Delete() с каскадным удалением словарей и инцидентов
- Замечания по консистентности:
  - slug генерируется один раз при создании, не меняется
  - preprocessors читаются/пишутся как JSONB целиком (no partial update)
  - version увеличивается при каждом обновлении

### DM-002 dictionary_entries

- Назначение: словарные записи профиля (entry_value + match_mode), привязанные по profile_slug
- Источник истины: PostgreSQL
- Инварианты: profile_slug REFERENCES profiles(slug); match_mode IN ('exact','contains','regex','fuzzy')
- Связанные `AC-*`: `AC-001`
- Поля:
  - `id` - BIGSERIAL PK
  - `profile_slug` - VARCHAR NOT NULL, FK → profiles(slug)
  - `entry_value` - TEXT NOT NULL, значение записи словаря
  - `match_mode` - VARCHAR NOT NULL, режим сопоставления (exact/contains/regex/fuzzy)
  - `created_at` - TIMESTAMPTZ NOT NULL DEFAULT now()
- Жизненный цикл:
  - Создаётся: DictionaryRepo.Save() для profile_slug
  - Удаляется: каскадно при удалении профиля или явно DictionaryRepo.Delete()
  - Обновляется: replace (DELETE + INSERT), не in-place update
- Замечания по консистентности:
  - Смена schema с "одна строка = весь словарь JSONB" → "одна строка = одна entry_value"
  - FK-constraint гарантирует ссылочную целостность

### DM-003 incidents

- Назначение: лог детектированных инцидентов Content Shield
- Источник истины: PostgreSQL
- Инварианты: profile_slug REFERENCES profiles(slug)
- Связанные `AC-*`: `AC-004`
- Поля:
  - `id` - BIGSERIAL PK
  - `profile_slug` - VARCHAR NOT NULL, FK → profiles(slug)
  - `request_id` - VARCHAR NOT NULL, идентификатор HTTP-запроса
  - `detector_type` - VARCHAR NOT NULL, тип детектора (regex/keyword/presidio/dictionary)
  - `entry_value` - TEXT, nullable; сработавшее значение словаря/паттерна
  - `severity` - VARCHAR NOT NULL, уровень (low/medium/high/critical)
  - `action` - VARCHAR NOT NULL, предпринятое действие (allow/block/review/log)
  - `raw_snippet` - TEXT, nullable; фрагмент контекста
  - `timestamp` - TIMESTAMPTZ NOT NULL DEFAULT now()
- Жизненный цикл:
  - Создаётся: IncidentRepo.Save() при детекции инцидента
  - Читается: ListByProfile, ListByTenant
  - Удаляется: каскадно при удалении профиля, или ротация (future)
- Замечания по консистентности:
  - Индекс по timestamp (RQ-009) и profile_slug
  - Только append-only (нет update); исторические данные не меняются

## Связи

- `profiles(slug) 1──N dictionary_entries(profile_slug)`: один профиль → много словарных записей (каскадное удаление)
- `profiles(slug) 1──N incidents(profile_slug)`: один профиль → много инцидентов (каскадное удаление)
- `profiles` — владелец; словари и инциденты — зависимые сущности

## Производные правила

- none: все поля хранятся явно, вычисляемых полей нет

## Переходы состояний

- Profile: `active` → `archived` | `disabled` (не в MVP — задел на будущее)
- Dictionary entry: только replace (DELETE + INSERT), без state transition
- Incident: append-only, без переходов

## Вне scope

- Материализованные view или денормализованные агрегаты
- Пагинация для списков (LIMIT/OFFSET — задача implement, не model)
- Версионирование/история изменений профилей
- Хранение результатов сканирования (scan_result)
