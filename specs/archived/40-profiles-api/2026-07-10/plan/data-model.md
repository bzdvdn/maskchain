---
status: no-change
---

# Data Model: 40-profiles-api

## Domain Model

- `entity.Profile` — существует, не меняется (entity/profile.go)
- `dictionary.Dictionary` — существует, не меняется (dictionary/dictionary.go)
- `preprocessor.PreprocessorDef` — существует, не меняется (preprocessor/processor.go)

## Persistence

- PostgreSQL таблицы `profiles` и `dictionary_entries` — существуют, миграции не требуются
- Репозиторий `PostgresProfileRepo` — существует, не меняется

## DTO (new — только для API слоя)

Добавляются request/response структуры в `src/internal/api/dto/profile.go`:
- `CreateProfileRequest` — slug, name, description, dictionaries, preprocessors
- `UpdateProfileRequest` — name, description, dictionaries, preprocessors
- `ProfileResponse` — полная структура (id, slug, name, status, dictionaries, preprocessors, timestamps)
- `ProfileListItem` — краткая (slug, name, status)
- `PatchDictionaryRequest` — action, name, entries
- `ErrorResponse` — error, code, details (опционально)

DTO не являются частью data model — это контракты API.
