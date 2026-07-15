# Cleanup Profile Repository — Задачи

## Phase Contract

Inputs: plan.md, data-model.md, repo-контекст.
Outputs: tasks.md с фазами, Touches, Surface Map, покрытие AC.
Stop if: хотя бы один AC нельзя привязать к исполнимым задачам.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `domain/shield/entity/profile.go` | T1.2 |
| `domain/shield/entity/incident.go` | T3.1 |
| `domain/shield/value/profile_id.go` | T1.1 |
| `domain/shield/value/profile_slug.go` | T1.1 |
| `domain/shield/dictionary/dictionary.go` | T2.2 |
| `domain/shield/dictionary/repository.go` | T2.1 |
| `domain/shield/dictionary/repository_test.go` | T2.1 |
| `domain/shield/dictionary/dictionary_test.go` | T2.3 |
| `domain/shield/detector/dictionary_detector_test.go` | T2.3 |
| `domain/shield/repository.go` | T1.3 |
| `domain/entity/mask_entry.go` | T3.4 |
| `adapters/repository/postgres/profile.go` | T1.4 |
| `adapters/repository/postgres/dictionary.go` | T2.1 |
| `adapters/repository/postgres/incident.go` | T3.3 |
| `adapters/repository/dictionary/` (весь пакет) | T2.1 |
| `adapters/repository/postgres/postgres_integration_test.go` | T4.1 |
| `api/dto/incident.go` | T3.2 |
| `api/handler/incident/handler.go` | T3.2 |
| `api/handler/incident/export.go` | T3.2 |
| `api/handler/incident/handler_test.go` | T3.6 |
| `api/handler/profile/` (весь пакет) | T1.5 |
| `api/server.go` | T1.6 |
| `api/admin.go` | T1.6 |
| `api/middleware/shield_test.go` | T4.1 |
| `app/usecase/shield/pipeline_factory.go` | T3.5 |
| `domain/shield/reaction/alert_test.go` | T4.1 |
| `adapters/repository/postgres/migrations/008_cleanup.up.sql` | T4.2 |
| `adapters/repository/postgres/migrations/008_cleanup.down.sql` | T4.2 |

## Implementation Context

- **Цель MVP:** удалить ProfileRepository и весь связанный код; убедиться, что `go build && go vet && go test` проходят
- **Инварианты:**
  - Dictionary entity (`domain/shield/dictionary/dictionary.go`) живёт — используется Tenant entity, middleware, tenant handler
  - Tenant-словари ходят через `PostgresTenantRepo.GetDictionaries()` (JSONB колонка), не через `DictionaryRepository`
  - `incidents.profile_slug` колонка в БД не дропается (DEC-003)
  - `domain/tenant/tenant.go` ProfileSlug — другая сущность, не трогать
- **Ключевые решения:**
  - DEC-001: удаление снизу вверх (value → entity → interface → impl → handler)
  - DEC-002: DictionaryCache + DictionaryRepository + PostgresDictionaryRepo — удаляются целиком (dead code)
  - DEC-003: колонка `incidents.profile_slug` сохраняется
- **Контракты:**
  - API `GET /api/v1/incidents` больше не принимает `profile_slug` query param
  - API `GET /api/v1/incidents/:id` больше не возвращает `profile_slug` в JSON
  - API `GET /api/v1/incidents/export` больше не принимает/возвращает `profile_slug`
- **Proof signals:**
  - `go build ./...` — 0 errors
  - `go vet ./...` — 0 warnings
  - `go test ./...` — все проходят
  - `grep -r 'ProfileRepository' src/` — 0 matches
  - `test -d src/internal/adapters/repository/dictionary/` — false (пакет удалён)
- **Вне scope:**
  - `domain/tenant/tenant.go` — ProfileSlug не трогаем
  - `infra/config/config.go` — DictionaryCacheConfig не удаляем
  - UI — не меняем

## Фаза 1: Profile сущности, интерфейс и PostgresProfileRepo

Цель: удалить core Profile entity, value objects, интерфейс ProfileRepository и его Postgres реализацию.

- [x] T1.1 Удалить Profile value objects (`value/profile_id.go`, `value/profile_slug.go`) — никаких внешних зависимостей, чистые value objects.
  Touches: `src/internal/domain/shield/value/profile_id.go`, `src/internal/domain/shield/value/profile_slug.go`
  AC: AC-004
  DEC: DEC-001

- [x] T1.2 Удалить Profile entity (`entity/profile.go`) — файл целиком, включая все option-функции и доступоры.
  Touches: `src/internal/domain/shield/entity/profile.go`
  AC: AC-004
  DEC: DEC-001

- [x] T1.3 Удалить ProfileRepository interface из `repository.go` (строки 11-18). Удалить поле `ProfileSlug *string` из `IncidentFilter` (строка 24).
  Touches: `src/internal/domain/shield/repository.go`
  AC: AC-001, AC-009
  DEC: DEC-001

- [x] T1.4 Удалить PostgresProfileRepo (`postgres/profile.go`) — весь файл.
  Touches: `src/internal/adapters/repository/postgres/profile.go`
  AC: AC-002
  DEC: DEC-001

- [x] T1.5 Удалить ProfileHandler package (`handler/profile/handler.go`, `handler/profile/handler_test.go`) — всю директорию.
  Touches: `src/internal/api/handler/profile/handler.go`, `src/internal/api/handler/profile/handler_test.go`
  AC: AC-003
  DEC: DEC-001

- [x] T1.6 Удалить `RegisterProfileHandler` из `server.go` (строки 98-106) и `admin.go` (строки 82-90).
  Touches: `src/internal/api/server.go`, `src/internal/api/admin.go`
  AC: AC-005
  DEC: DEC-001

## Фаза 2: Словарные репозитории и кеш

Цель: удалить DictionaryRepository, PostgresDictionaryRepo, и весь dictionaryrepo/ пакет (DictionaryCache, Valkey, LRU, Warmer, PubSub). Обновить Dictionary entity.

- [x] T2.1 Удалить DictionaryRepository interface (`dictionary/repository.go`), PostgresDictionaryRepo (`postgres/dictionary.go`), и весь пакет `adapters/repository/dictionary/` (cached.go, valkey.go, lru.go, warm.go, pubsub.go, metrics.go + все *_test.go).
  Touches: `src/internal/domain/shield/dictionary/repository.go`, `src/internal/domain/shield/dictionary/repository_test.go`, `src/internal/adapters/repository/postgres/dictionary.go`, `src/internal/adapters/repository/dictionary/`
  AC: AC-007, AC-008
  DEC: DEC-002

- [x] T2.2 Обновить Dictionary entity (`dictionary/dictionary.go`) — удалить поле `profileSlug`, метод `ProfileSlug()`, параметр `profileSlug` из `NewDictionary()`.
  Touches: `src/internal/domain/shield/dictionary/dictionary.go`
  AC: AC-004
  DEC: DEC-001, DEC-002

- [x] T2.3 Обновить `dictionary_test.go` (убрать ProfileSlug assertion и FindByProfileSlug тесты) и `dictionary_detector_test.go` (убрать NewProfileSlug).
  Touches: `src/internal/domain/shield/dictionary/dictionary_test.go`, `src/internal/domain/shield/detector/dictionary_detector_test.go`
  AC: AC-004
  DEC: DEC-001

## Фаза 3: Incident, MaskEntry, DTO и API

Цель: удалить профильные поля из Incident entity, MaskEntry, DTO, handler, export и pipeline_factory.

- [x] T3.1 Обновить Incident entity (`entity/incident.go`) — удалить поле `profileSlug`, метод `ProfileSlug()`, параметр `profileSlug` из `NewAuditIncident()`.
  Touches: `src/internal/domain/shield/entity/incident.go`
  AC: AC-012
  DEC: DEC-001, DEC-003

- [x] T3.2 Обновить DTO, handler и export — удалить `ProfileSlug` из `IncidentResponse`, `IncidentFilterParams`, `ExportQuery`; удалить ProfileSlug фильтрацию и вывод из `handler.go` и `export.go`.
  Touches: `src/internal/api/dto/incident.go`, `src/internal/api/handler/incident/handler.go`, `src/internal/api/handler/incident/export.go`
  AC: AC-012, AC-013
  DEC: DEC-001

- [x] T3.3 Обновить PostgresIncidentRepo (`postgres/incident.go`) — удалить метод `ListByProfile` (строки 75-90), удалить `profile_slug` из SQL запросов и scan (строки 55-71, 170-208), удалить ProfileSlug фильтр из `List` (строки 128-131), удалить `incident.ProfileSlug()` из `Save` (строка 33).
  Touches: `src/internal/adapters/repository/postgres/incident.go`
  AC: AC-009, AC-012
  DEC: DEC-001

- [x] T3.4 Обновить MaskEntry (`domain/entity/mask_entry.go`) — удалить поле `profileID`.
  Touches: `src/internal/domain/entity/mask_entry.go`
  AC: AC-010
  DEC: DEC-001

- [x] T3.5 Удалить метод `Build(ctx, profile)` из `ScanPipelineFactory` (`pipeline_factory.go`, строки 35-75). Оставить только `BuildFromRules`.
  Touches: `src/internal/app/usecase/shield/pipeline_factory.go`
  AC: AC-006
  DEC: DEC-001

- [x] T3.6 Обновить `handler_test.go` — убрать `ListByProfile` из mock и ProfileSlug-зависимые assertions.
  Touches: `src/internal/api/handler/incident/handler_test.go`
  AC: AC-012, AC-013
  DEC: DEC-001

## Фаза 4: Тесты, миграция и верификация

Цель: обновить оставшиеся тесты, создать миграцию, обновить trace-маркеры, проверить сборку.

- [x] T4.1 Обновить `shield_test.go`, `alert_test.go`, `postgres_integration_test.go` — убрать все ProfileSlug, ProfileID, ListByProfile, NewProfileSlug usage.
  Touches: `src/internal/api/middleware/shield_test.go`, `src/internal/domain/shield/reaction/alert_test.go`, `src/internal/adapters/repository/postgres/postgres_integration_test.go`
  AC: AC-004, AC-009, AC-012
  DEC: DEC-001

- [x] T4.2 Создать миграционные файлы `008_cleanup.up.sql` (DROP TABLE IF EXISTS profiles, dictionary_entries; ALTER TABLE mask_entries DROP COLUMN profile_id) и `008_cleanup.down.sql` (CREATE TABLE profiles, dictionary_entries; ALTER TABLE mask_entries ADD COLUMN profile_id). Обновить `migrations.go` если требуется.
  Touches: `src/internal/adapters/repository/postgres/migrations/008_cleanup.up.sql`, `src/internal/adapters/repository/postgres/migrations/008_cleanup.down.sql`
  AC: AC-011
  DEC: DEC-001, DEC-003

- [x] T4.3 Обновить @sk-task/@sk-test trace-маркеры на изменённых файлах: заменить ссылки на профильные задачи (`20-shield-domain`, `40-profiles-api`, `102-profile-cache`, `24-shield-dictionaries`) на актуальный контекст. На удалённых файлах маркеры не трогать.
  Touches: `src/internal/domain/shield/repository.go`, `src/internal/domain/shield/entity/incident.go`, `src/internal/domain/shield/dictionary/dictionary.go`, `src/internal/adapters/repository/postgres/incident.go`, `src/internal/api/dto/incident.go`, `src/internal/api/handler/incident/handler.go`, `src/internal/api/handler/incident/export.go`, `src/internal/app/usecase/shield/pipeline_factory.go`
  AC: AC-014
  DEC: —

- [x] T4.4 Выполнить финальную верификацию: `go build ./... && go vet ./... && go test ./... && grep -r 'ProfileRepository' src/` (ожидается 0 matches).
  Touches: `src/...` (все затронутые файлы, build-level check)
  AC: Все

## Покрытие критериев приемки

- AC-001 -> T1.3
- AC-002 -> T1.4
- AC-003 -> T1.5
- AC-004 -> T1.1, T1.2, T2.2, T2.3, T4.1
- AC-005 -> T1.6
- AC-006 -> T3.5
- AC-007 -> T2.1
- AC-008 -> T2.1
- AC-009 -> T1.3, T3.3, T4.1
- AC-010 -> T3.4
- AC-011 -> T4.2
- AC-012 -> T3.1, T3.2, T3.3, T3.6, T4.1
- AC-013 -> T3.2, T3.6
- AC-014 -> T4.3
## Заметки

- Задачи T1.x и T2.1 можно выполнять параллельно (независимые пакеты)
- T2.2 и T2.3 зависят от T2.1 (dictionary.go ссылается на repository.go types)
- T3.x зависят от T1.x (value objects, entity, repository interface)
- T4.1 зависит от всех T1.x, T2.x, T3.x
- T4.2 независима и может быть выполнена в любой момент
- T4.3 — последняя задача перед T4.4
- Удалённые файлы не требуют @sk-маркеров; маркеры обновляются только на оставшихся файлах
