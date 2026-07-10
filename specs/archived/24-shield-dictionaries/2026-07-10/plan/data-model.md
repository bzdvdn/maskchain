# Shield Dictionaries — Модель данных

## Scope

- Связанные `AC-*`: AC-002, AC-006
- Связанные `DEC-*`: DEC-001, DEC-002, DEC-005
- Статус: `changed`

## Сущности

### DM-001 dictionary_entries

- Назначение: хранит словарь для профиля. Один профиль = одна запись.
- Источник истины: PostgreSQL, таблица `dictionary_entries`
- Инварианты:
  - `profile_slug` уникален — один словарь на профиль
  - `name` не пуст
  - `entries` — массив строк, может быть пустым ([]), не null
  - `match_mode` — одно из: `exact`, `contains`, `regex`, `fuzzy`
- Связанные `AC-*`: AC-002, AC-006
- Связанные `DEC-*`: DEC-002, DEC-005
- Поля:
  - `profile_slug` - TEXT, PK, ссылается на profile_slug в profiles (логический FK, без ON DELETE CASCADE — ошибка удаления профиля со словарём обрабатывается приложением)
  - `name` - TEXT, NOT NULL
  - `entries` - JSONB, NOT NULL DEFAULT '[]'::jsonb — массив строк
  - `match_mode` - TEXT, NOT NULL, CHECK (match_mode IN ('exact','contains','regex','fuzzy'))
  - `created_at` - TIMESTAMPTZ, NOT NULL, DEFAULT now()
  - `updated_at` - TIMESTAMPTZ, NOT NULL, DEFAULT now()
- Жизненный цикл:
  - создаётся при добавлении словаря в профиль (Save)
  - обновляется при изменении словаря (Save — upsert по profile_slug)
  - удаляется при удалении словаря из профиля (Delete)
  - каскадного удаления при удалении профиля нет — ошибка обрабатывается приложением
- Замечания по консистентности:
  - запись всегда целостна: entries не null, match_mode валиден
  - stale-состояние невозможно — словарь загружается атомарно вместе с профилем

## Связи

- `DM-001 -> Profile`: 1:1, profile_slug связывает словарь с профилем. Profile — владелец, Dictionary не существует без Profile.

## Производные правила

- При загрузке Profile через ProfileRepository: если записи в dictionary_entries нет — Profile.Dictionaries = nil (не ошибка)

## Переходы состояний

- Жизненный цикл достаточно прост — Create (insert), Update (upsert), Delete. State machine не требуется.

## Вне scope

- Индексы全文 поиска по entries — не требуется, Aho-Corasick работает in-memory
- Audit/log таблица изменений словаря — PostMVP
