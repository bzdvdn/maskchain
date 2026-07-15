# Cleanup Profile Repository — План

## Phase Contract

Inputs: spec.md, inspect.md (pass), repo-контекст `.speckeep/`.
Outputs: plan.md, data-model.md.
Stop if: spec слишком расплывчата — нет, spec детализирована.

## Цель

Удалить ProfileRepository, PostgresProfileRepo, ProfileHandler, Profile entity, ProfileID/ProfileSlug value objects,
и весь код, завязанный на них (DictionaryCacheWarmer, профильные методы DictionaryCache, ListByProfile, ScanPipelineFactory.Build,
MaskEntry.ProfileID, Incident.ProfileSlug, профильный фильтр в DTO/handler).

Профили как сущность упразднены — всё через tenant-словари. Удаление безопасно: код мёртвый (не завайрен в runtime) или
дублируется tenant-механизмами.

## MVP Slice

Удаление **всех** профильных поверхностей за один инкремент. Никакого поэтапного ввода — это чистка мёртвого кода,
где единственный критерий: `go build ./... && go vet ./... && go test ./...` проходят.

Закрываемые AC: все 14.

## First Validation Path

```
go build ./... && go vet ./... && go test ./... && go build ./...
```

Если компиляция и тесты проходят — удаление корректно. Дополнительно: `grep -r 'ProfileRepository' src/` не даёт результатов.

## Scope

### В зоне
- `src/internal/domain/shield/repository.go` — удалить ProfileRepository interface, ProfileSlug из IncidentFilter
- `src/internal/domain/shield/entity/profile.go` — удалить весь файл (Profile entity и option-функции)
- `src/internal/domain/shield/value/profile_id.go` — удалить
- `src/internal/domain/shield/value/profile_slug.go` — удалить
- `src/internal/domain/shield/entity/incident.go` — удалить profileSlug поле, ProfileSlug() метод, параметр из NewAuditIncident
- `src/internal/domain/shield/dictionary/dictionary.go` — удалить profileSlug поле, ProfileSlug() метод, параметр из NewDictionary
- `src/internal/domain/shield/dictionary/repository.go` — удалить весь файл (DictionaryRepository interface — мёртв, tenant-словари через PostgresTenantRepo.GetDictionaries/UpdateDictionaries)
- `src/internal/domain/shield/dictionary/repository_test.go` — удалить весь файл
- `src/internal/domain/shield/dictionary/dictionary_test.go` — убрать ProfileSlug-ассершены и FindByProfileSlug тесты
- `src/internal/domain/shield/detector/dictionary_detector_test.go` — убрать использование NewProfileSlug
- `src/internal/adapters/repository/postgres/profile.go` — удалить весь файл (264 строки)
- `src/internal/adapters/repository/postgres/dictionary.go` — удалить весь файл (PostgresDictionaryRepo — мёртв, единственный потребитель PostgresProfileRepo)
- `src/internal/adapters/repository/postgres/incident.go` — удалить ListByProfile, profileSlug из SQL/scan, ProfileSlug фильтр из List
- `src/internal/adapters/repository/dictionary/` — удалить весь пакет целиком (cached.go, valkey.go, lru.go, warm.go, pubsub.go, metrics.go, и все *_test.go). Весь пакет dead code: ни один production-файл не импортирует `dictionaryrepo`, DictionaryCache нигде не завайрен.
- `src/internal/api/dto/incident.go` — удалить ProfileSlug из IncidentResponse, IncidentFilterParams, ExportQuery
- `src/internal/api/handler/incident/handler.go` — удалить ProfileSlug фильтрацию, toResponse вызов ProfileSlug()
- `src/internal/api/handler/incident/handler_test.go` — обновить mock (убрать ListByProfile, ProfileSlug)
- `src/internal/api/handler/incident/export.go` — удалить ProfileSlug фильтрацию и вывод
- `src/internal/api/handler/profile/` — удалить весь package (handler.go + handler_test.go)
- `src/internal/api/server.go` — удалить RegisterProfileHandler
- `src/internal/api/admin.go` — удалить RegisterProfileHandler
- `src/internal/app/usecase/shield/pipeline_factory.go` — удалить Build(ctx, profile)
- `src/internal/domain/entity/mask_entry.go` — удалить ProfileID поле
- `src/internal/api/middleware/shield_test.go` — убрать NewProfileSlug, заменить на TenantID
- `src/internal/domain/shield/reaction/alert_test.go` — убрать ListByProfile из mock
- `src/internal/adapters/repository/postgres/postgres_integration_test.go` — убрать ProfileSlug тесты

### Миграция
- `migrations/008_cleanup.up.sql`: DROP TABLE profiles, DROP TABLE dictionary_entries, ALTER TABLE mask_entries DROP COLUMN profile_id
- `migrations/008_cleanup.down.sql`: CREATE TABLE profiles (...), CREATE TABLE dictionary_entries (...), ALTER TABLE mask_entries ADD COLUMN profile_id TEXT

### Вне scope
- `incidents.profile_slug` колонка в БД — не дропается (данные сохраняются)
- UI — не меняется (получает пустой profile_slug в ответе; фильтр по profile_slug не срабатывает)
- `src/internal/domain/tenant/tenant.go` — `ProfileSlug()` поле (другая сущность, конфиг tenant, не shield Profile)
- `src/internal/infra/config/config.go` — DictionaryCacheConfig остаётся (используется для конфига валкея)

## Performance Budget

`none` — чистка не создаёт новых путей; аллокации уменьшатся на profile-структуры.

## Implementation Surfaces

### Удаляемые целиком
- `src/internal/domain/shield/entity/profile.go` — Profile entity
- `src/internal/domain/shield/value/profile_id.go`
- `src/internal/domain/shield/value/profile_slug.go`
- `src/internal/domain/shield/dictionary/repository.go` — DictionaryRepository interface
- `src/internal/domain/shield/dictionary/repository_test.go`
- `src/internal/adapters/repository/postgres/profile.go` — PostgresProfileRepo
- `src/internal/adapters/repository/postgres/dictionary.go` — PostgresDictionaryRepo
- `src/internal/api/handler/profile/handler.go`
- `src/internal/api/handler/profile/handler_test.go`
- `src/internal/adapters/repository/dictionary/` — весь пакет (cached.go, valkey.go, lru.go, warm.go, pubsub.go, metrics.go, cached_test.go, warm_test.go, lru_test.go, pubsub_test.go, valkey_test.go, metrics_test.go)

### Редактируемые
- `src/internal/domain/shield/repository.go` — ProfileRepository interface, ProfileSlug из IncidentFilter
- `src/internal/domain/shield/entity/incident.go` — profileSlug поле, ProfileSlug(), NewAuditIncident params
- `src/internal/domain/shield/dictionary/dictionary.go` — profileSlug, ProfileSlug, NewDictionary
- `src/internal/domain/shield/dictionary/dictionary_test.go`
- `src/internal/domain/shield/detector/dictionary_detector_test.go`
- `src/internal/adapters/repository/postgres/incident.go` — ListByProfile, profileSlug SQL/scan, фильтр
- `src/internal/api/dto/incident.go` — ProfileSlug поля
- `src/internal/api/handler/incident/handler.go` — ProfileSlug фильтр, toResponse
- `src/internal/api/handler/incident/handler_test.go`
- `src/internal/api/handler/incident/export.go` — ProfileSlug фильтр, CSV/JSON
- `src/internal/api/server.go` — RegisterProfileHandler
- `src/internal/api/admin.go` — RegisterProfileHandler
- `src/internal/app/usecase/shield/pipeline_factory.go` — Build(*Profile)
- `src/internal/domain/entity/mask_entry.go` — ProfileID поле
- `src/internal/api/middleware/shield_test.go`
- `src/internal/domain/shield/reaction/alert_test.go`
- `src/internal/adapters/repository/postgres/postgres_integration_test.go`

### Новая
- `migrations/008_cleanup.up.sql`
- `migrations/008_cleanup.down.sql`

## Bootstrapping Surfaces

`none` — структура директорий не меняется.

## Влияние на архитектуру

- Profile как доменная сущность исчезает; словари живут в tenant-контексте (JSONB в таблице tenants).
- DictionaryRepository, PostgresDictionaryRepo и весь dictionaryrepo/ пакет (DictionaryCache, Valkey, LRU, Warmer, PubSub) удаляются как dead code. Tenant-словари ходят напрямую через PostgresTenantRepo.GetDictionaries() → JSONB.
- IncidentRepository теряет ListByProfile (мёртвый, нигде не вызван).
- ScanPipelineFactory теряет Build(profile); остаётся BuildFromRules (tenant-rules based).
- Компиляция и тесты — единственный критерий корректности.

## Acceptance Approach

| AC | Подход | Поверхности |
|---|---|---|
| AC-001 | Удалить ProfileRepository interface | `repository.go` |
| AC-002 | Удалить PostgresProfileRepo | `postgres/profile.go` |
| AC-003 | Удалить ProfileHandler package | `handler/profile/*` |
| AC-004 | Удалить Profile entity + value objects | `entity/profile.go`, `value/profile_id.go`, `value/profile_slug.go` |
| AC-005 | Удалить RegisterProfileHandler из server + admin | `server.go:Line98-106`, `admin.go:Line82-90` |
| AC-006 | Удалить ScanPipelineFactory.Build(ctx, profile) | `pipeline_factory.go:Line35-75` |
| AC-007 | Удалить DictionaryCache + DictionaryRepository + PostgresDictionaryRepo + весь dictionaryrepo/ пакет | `adapters/repository/dictionary/`, `domain/shield/dictionary/repository.go`, `adapters/repository/postgres/dictionary.go` |
| AC-008 | Удалить DictionaryCacheWarmer | `warm.go`, `warm_test.go` (входит в dictionaryrepo/ пакет) |
| AC-009 | Удалить ListByProfile из IncidentRepository | `repository.go:Line52`, `postgres/incident.go:Line75-90` |
| AC-010 | Удалить ProfileID из MaskEntry | `mask_entry.go` — поле |
| AC-011 | Migration 008 дропает profiles/dictionary_entries + profile_id | `migrations/008_cleanup.*.sql` |
| AC-012 | Incident.ProfileSlug удалён (колонка БД сохранена) | `incident.go` entity, SQL scan |
| AC-013 | IncidentFilterParams без ProfileSlug | `dto/incident.go` — три struct |
| AC-014 | Нет @sk- ссылок на профильные задачи | `grep -v '@sk-'` check |

## Данные и контракты

- `data-model.md` — статус changed (сущности Profile удалены, Dictionary/Incident/MaskEntry потеряли профильные поля)
- **API контракты меняются**: IncidentResponse больше не содержит `profile_slug`; бэкенд не принимает `profile_slug` как query param в `GET /api/v1/incidents`. Совместимость: обратная не гарантируется (UI — out of scope).
- **Event/contracts**: нет затронутых.

## Стратегия реализации

### DEC-001 Удаление снизу вверх
  Why: удалять сперва value objects и entity, потом интерфейсы, потом реализации, потом API. Ни одна удаляемая сущность не имеет runtime-потребителей (кроме тестов), но порядок важен для компиляции.
  Tradeoff: одни и те же файлы могут конфликтовать при параллельном удалении. Рекомендуется не более 2 параллельных потоков.
  Affects: все поверхности.
  Validation: компиляция после каждого блока.

### DEC-002 DictionaryCache + DictionaryRepository + PostgresDictionaryRepo — удаляем целиком
  Why: весь dictionaryrepo/ пакет (cached.go, valkey.go, lru.go, warm.go, pubsub.go, metrics.go) — dead code. Ни один production-файл не импортирует этот пакет. DictionaryCache реализует ProfileRepository (который удаляется), но для tenant-словарей используется прямой path: PostgresTenantRepo.GetDictionaries() → JSONB → entity.Tenant.Dictionaries() → middleware. DictionaryRepository и PostgresDictionaryRepo тоже мертвы — единственный потребитель PostgresProfileRepo, который удаляется. Tenant-словари живут через TenantRepository (JSONB колонка, не связанная с профилями).
  Tradeoff: остаётся конфиг DictionaryCacheConfig в config.go (не потребляется production-кодом, но удаление конфига — вне scope spec-а). Пакет dictionary/domain/dictionary.go (entity Dictionary) живёт — используется Tenant entity, middleware и tenant handler.
  Affects: `domain/shield/dictionary/repository.go`, `adapters/repository/postgres/dictionary.go`, `adapters/repository/dictionary/` (весь пакет)
  Validation: `go build ./...` проходит без dictionaryrepo/ пакета, tenant handler CRUD работает (через PostgresTenantRepo JSONB).

### DEC-003 Колонка incidents.profile_slug не дропается
  Why: spec явно сохраняет колонку. Данные остаются в БД для будущего использования или миграции.
  Tradeoff: БД содержит колонку, которую Go-код больше не читает/пишет. Незначительный data debt.
  Affects: `migrations/008_cleanup.up.sql` — не включает `ALTER TABLE incidents DROP COLUMN profile_slug`
  Validation: `git diff` подтверждает отсутствие изменений incidents-таблицы в миграции.

## Incremental Delivery

### MVP (весь объём)
- Все 14 AC закрываются одной порцией. Никакого поэтапного расширения — это чистка.
- Критерий: `go build ./... && go vet ./... && go test ./...` pass.

## Порядок реализации

**Поток A (основной — сущности и интерфейсы)**:
1. Value objects: profile_id.go, profile_slug.go
2. Profile entity: profile.go
3. ProfileRepository interface: repository.go
4. PostgresProfileRepo: postgres/profile.go
5. ScanPipelineFactory.Build: pipeline_factory.go
6. ListByProfile: repository.go, postgres/incident.go

**Поток B (параллельно A — handler+server+export)**:
7. ProfileHandler package: handler/profile/*
8. RegisterProfileHandler: server.go, admin.go
9. Incident DTO: dto/incident.go
10. Incident handler/export: handler.go, export.go
11. Incident entity profileSlug: entity/incident.go

**Поток C (параллельно A, после B — dictionary + repos)**:
12. Dictionary entity profileSlug: dictionary/dictionary.go
13. DictionaryRepository (repository.go) — удалить весь файл
14. PostgresDictionaryRepo (postgres/dictionary.go) — удалить весь файл
15. DictionaryCache пакет (dictionaryrepo/) — удалить целиком всю директорию
16. MaskEntry ProfileID: mask_entry.go

**Поток D (итоговый)**:
17. Миграция 008_cleanup
18. Все test файлы (shield_test.go, alert_test.go, handler_test.go, postgres_integration_test.go, dictionary_test.go, dictionary_detector_test.go)
19. `go build ./... && go vet ./... && go test ./...`

## Риски

1. **Пропущенный референс на Profile в нетривиальном месте**
   Mitigation: после удаления каждого пакета запускать `go build ./...`. Финально — `go vet + go test`.

2. **DictionaryCacheConfig в config.go остаётся неиспользуемым**
   Mitigation: остаётся config-дефолт, который не потребляется. Удаление конфига — вне scope spec-а (не влияет на компиляцию/тесты).

3. **Миграция 008 накладывается на недропнутую колонку**
   Mitigation: миграция дропает только таблицы profiles/dictionary_entries и колонку profile_id. incidents.profile_slug не трогается.

4. **UI полагается на profile_slug в ответах инцидентов**
   Mitigation: UI — вне scope (spec). БД колонка сохранена, данные не потеряны. UI будет показывать пустое поле.

## Rollout и compatibility

- Миграция 008 — идемпотентна (IF EXISTS / DROP TABLE IF EXISTS).
- Специальных rollout-действий не требуется.
- После деплоя: incidents API перестаёт фильтровать по profile_slug, перестаёт возвращать profile_slug в JSON.
- Откат: `008_cleanup.down.sql` восстанавливает таблицы; старый код с ProfileRepository несовместим, поэтому откат на предыдущую версию бинарника + down migration.

## Проверка

| Шаг | Команда | Покрывает AC |
|---|---|---|
| 1 | `go build ./...` | Все |
| 2 | `go vet ./...` | Все |
| 3 | `go test ./...` | Все |
| 4 | `grep -r 'ProfileRepository' src/` — 0 matches | AC-001 |
| 5 | `test -f src/internal/domain/shield/entity/profile.go` — false | AC-004 |
| 6 | `test -d src/internal/adapters/repository/dictionary/` — false | AC-007, AC-008 |
| 7 | `test -f migrations/008_cleanup.up.sql` — true | AC-011 |
| 8 | На всех изменённых файлах `grep -v '@sk-'` не содержит @sk-task со старыми номерами профильных задач | AC-014 |

## Соответствие конституции

- нет конфликтов. Конституция требует удаления мёртвого кода (ProfileRepository), что и выполняется.
