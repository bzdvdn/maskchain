# Cleanup: удаление ProfileRepository и мёртвого кода профилей

## Scope Snapshot

- **In scope:** удаление `ProfileRepository` interface, `PostgresProfileRepo`, `ProfileHandler`, profile entity, value objects `ProfileID`/`ProfileSlug`, profile-related DTO, profile-зависимостей в `DictionaryCache`, `IncidentRepository.ListByProfile`, `ScanPipelineFactory.Build(ctx, profile)`, profile routes в server/admin, и всех связанных тестов. Миграция cleanup `006` (удаление таблиц `profiles`, `dictionary_entries`) — в scope.
- **Out of scope:** tenant-сущности, tenant repository, tenant handler, tenant API — всё, что относится к текущей tenant-based архитектуре, не трогается. DictionaryCache как кеш tenant-словарей остаётся, но перестаёт реализовывать `ProfileRepository`.

## Цель

Разработчик, читающий код, больше не видит `ProfileRepository` и связанные типы, которые стали мёртвым грузом после перехода на tenant-based архитектуру (tenant-profile-sync). Кодовая база очищена от неиспользуемых интерфейсов, реализаций, entity, value objects, handler, routes, DTO и тестов, относящихся к упразднённой концепции "Profile как отдельная сущность". После cleanup код компилируется, `go vet` чист, все тесты проходят.

## Основной сценарий

1. Разработчик открывает `src/internal/domain/shield/repository.go` — видит только `TenantRepository`, `TenantResolver`, `IncidentRepository`.
2. Разработчик ищет `ProfileRepository` — grep не находит ни одного вхождения.
3. `PostgresProfileRepo` удалён, `profile.go` в `postgres/` отсутствует.
4. `ProfileHandler` и весь пакет `src/internal/api/handler/profile/` удалены.
5. `RegisterProfileHandler` отсутствует в `server.go` и `admin.go`.
6. `DictionaryCache` больше не реализует `ProfileRepository` (assertion удалён), его методы, специфичные для Profile (`Save`, `FindByID`, `ListByTenant`, `Delete`), удалены.
7. `ScanPipelineFactory.Build(ctx, profile)` удалён (не вызывался нигде, заменён на `BuildFromRules`).
8. `IncidentRepository.ListByProfile` удалён из интерфейса и реализации.
9. Значения `ProfileID`, `ProfileSlug` удалены из `src/internal/domain/shield/value/`.
10. `entity.Profile` и `ProfileOption` удалены из `src/internal/domain/shield/entity/`.
11. Миграция `006_cleanup` удаляет таблицы `profiles` и `dictionary_entries`.
12. `go build ./...` и `go test ./...` проходят без ошибок.

## MVP Slice

Весь scope — MVP. Это чистый cleanup без новой функциональности.

## First Deployable Outcome

После первого implementation pass:
- `src/internal/domain/shield/repository.go` не содержит `ProfileRepository` и `ListByProfile`
- `src/internal/adapters/repository/postgres/profile.go` удалён
- `src/internal/api/handler/profile/` удалён
- `src/internal/domain/shield/entity/profile.go` удалён
- `src/internal/domain/shield/value/profile_id.go` и `profile_slug.go` удалены
- `src/internal/adapters/repository/dictionary/cached.go` не содержит `var _ shield.ProfileRepository`
- `src/internal/app/usecase/shield/pipeline_factory.go` не содержит `Build(ctx, profile)`
- `src/internal/api/server.go` и `admin.go` не содержат `RegisterProfileHandler`
- Миграция `006_cleanup.sql` добавлена в `deployments/migrations/`
- `go build ./...` и `go test ./...` проходят

## Scope

- `src/internal/domain/shield/repository.go` — удалить `ProfileRepository`, `IncidentRepository.ListByProfile`
- `src/internal/domain/shield/entity/profile.go` — удалить весь файл
- `src/internal/domain/shield/value/profile_id.go` — удалить весь файл
- `src/internal/domain/shield/value/profile_slug.go` — удалить весь файл
- `src/internal/domain/shield/dictionary/dictionary.go` — удалить поле `profileSlug` и метод `ProfileSlug()`
- `src/internal/adapters/repository/postgres/profile.go` — удалить весь файл
- `src/internal/adapters/repository/postgres/incident.go` — удалить `ListByProfile`
- `src/internal/adapters/repository/dictionary/cached.go` — удалить `var _ shield.ProfileRepository`, удалить методы `Save`, `FindByID`, `ListByTenant`, `Delete`, `assembleDegraded`, `resolveVersion`
- `src/internal/adapters/repository/dictionary/valkey.go` — удалить `dictToCacheValue`, `cacheValueToDict`, `dictionaryCacheValue` struct (используют `ProfileID`/`ProfileSlug`)
- `src/internal/adapters/repository/dictionary/lru.go` — удалить `DictionaryMetadataFromProfile`
- `src/internal/adapters/repository/dictionary/warm.go` — удалить весь файл (DictionaryCacheWarmer — dead code, не завайрен нигде)
- `src/internal/api/handler/profile/` — удалить весь пакет (handler.go, handler_test.go, dto.go)
- `src/internal/api/dto/profile.go` — удалить весь файл
- `src/internal/api/server.go` — удалить `RegisterProfileHandler`, import profile handler
- `src/internal/api/admin.go` — удалить `RegisterProfileHandler`, import profile handler
- `src/internal/app/usecase/shield/pipeline_factory.go` — удалить `Build(ctx, profile)`
- `src/internal/domain/shield/entity/incident.go` — удалить поле `profileSlug`, метод `ProfileSlug()`, геттеры
- `src/internal/api/dto/incident.go` — удалить `ProfileSlug` из DTO и фильтра
- `src/internal/api/handler/incident/handler.go` — удалить фильтрацию по `ProfileSlug`
- `deployments/migrations/006_cleanup.sql` — создать миграцию: DROP TABLE profiles, dictionary_entries
- `src/internal/adapters/repository/postgres/postgres_integration_test.go` — удалить profile-тесты
- `src/internal/domain/shield/mask/entity.go` — удалить поле `ProfileID` (dead code, tenant-код не использует)
- `src/internal/domain/shield/reaction/alert_test.go` — удалить `ListByProfile` из mock
- `src/internal/api/handler/incident/handler_test.go` — удалить `ListByProfile` из mock
- `src/internal/api/middleware/shield_test.go` — удалить использование `NewProfileSlug` (заменить на tenant slug)
- `src/internal/adapters/repository/tenant/tenant.go` — удалить использование `NewProfileSlug` (если есть)
- `src/internal/infra/config/config.go` — удалить `ProfileSlug` из TenantConfig (если есть)

## Контекст

- Tenant-profile-sync (spec `active/tenant-profile-sync/`) уже реализовал переход на tenant-based архитектуру. Profile как сущность заменён на dictionaries внутри tenant.
- `ProfileRepository` и связанный код остались как dead code — не используются ни в gateway, ни в admin runtime, но создают путаницу и увеличивают surface для тестов.
- `DictionaryCache` был переименован из `ProfileCache`, но сохранил имплементацию `ProfileRepository` для обратной совместимости. Сейчас это не нужно.
- `ScanPipelineFactory.Build(ctx, profile)` принимает `*entity.Profile`, но нигде не вызывается — только `BuildFromRules` используется в production.

## Зависимости

- Tenant-profile-sync (выполнена): tenant-based архитектура уже работает.
- Нет внешних зависимостей.

## Требования

### RQ-001 Удаление ProfileRepository
Система ДОЛЖНА удалить `ProfileRepository` interface из `src/internal/domain/shield/repository.go`.

### RQ-002 Удаление PostgresProfileRepo
Система ДОЛЖНА удалить `PostgresProfileRepo` и весь файл `src/internal/adapters/repository/postgres/profile.go`.

### RQ-003 Удаление ProfileHandler
Система ДОЛЖНА удалить пакет `src/internal/api/handler/profile/` (handler + handler_test + dto).

### RQ-004 Удаление Profile entity и value objects
Система ДОЛЖНА удалить `entity.Profile`, `ProfileOption`, `value.ProfileID`, `value.ProfileSlug`.

### RQ-005 Очистка DictionaryCache
Система ДОЛЖНА удалить из `DictionaryCache` assertion `var _ shield.ProfileRepository` и методы, специфичные для Profile (`Save`, `FindByID`, `ListByTenant`, `Delete`, `assembleDegraded`, `resolveVersion`).

### RQ-006 Удаление ListByProfile из IncidentRepository
Система ДОЛЖНА удалить `IncidentRepository.ListByProfile(ctx, profileID)` и соответствующую реализацию.

### RQ-007 Удаление ScanPipelineFactory.Build(ctx, profile)
Система ДОЛЖНА удалить метод `Build(ctx, *entity.Profile)` из `ScanPipelineFactory`.

### RQ-008 Удаление profileSlug из Dictionary entity
Система ДОЛЖНА удалить поле `profileSlug` и метод `ProfileSlug()` из `dictionary.Dictionary`.

### RQ-009 Удаление profile-маршрутов
Система ДОЛЖНА удалить `RegisterProfileHandler` из `server.go` и `admin.go`.

### RQ-010 Миграция cleanup
Система ДОЛЖНА создать миграцию `006_cleanup.sql`, удаляющую таблицы `profiles` и `dictionary_entries`.

### RQ-011 Очистка incident DTO и handler
Система ДОЛЖНА удалить поле `ProfileSlug` из `IncidentFilterParams` и `IncidentResponse` DTO, а также фильтрацию по `ProfileSlug` в incident handler.

### RQ-012 Удаление DictionaryCacheWarmer
Система ДОЛЖНА удалить весь файл `src/internal/adapters/repository/dictionary/warm.go` и `warm_test.go`. `NewDictionaryCacheWarmer` не вызывается нигде, кроме собственных тестов.

### RQ-013 Удаление ProfileID из MaskEntry
Система ДОЛЖНА удалить поле `ProfileID *string` из `mask.MaskEntry` и все обращения к нему в mask-репозиториях. Tenant-код не использует это поле.

### RQ-014 Удаление Incident.ProfileSlug
Система ДОЛЖНА удалить поле `profileSlug string` и метод `ProfileSlug()` из `entity.Incident`, а также все использования в коде.

### RQ-015 Компиляция и тесты
После всех изменений `go build ./...` и `go test ./...` ДОЛЖНЫ проходить без ошибок.

## Вне scope

- Tenant-сущности, tenant repository, tenant handler, tenant API — не трогаются.
- DictionaryCache как механизм кеширования tenant-словарей остаётся (но без profile-методов).
- Изменение схемы БД для incidents (FK, колонки) — не в scope (только удаление таблиц profiles/dictionary_entries).

## Критерии приемки

### AC-001 ProfileRepository удалён
- Почему это важно: интерфейс больше не мозолит глаза и не вводит в заблуждение
- **Given** репозиторий `src/internal/domain/shield/repository.go`
- **When** разработчик ищет `ProfileRepository` в коде
- **Then** ни одного вхождения не найдено
- **Evidence** `grep -r 'ProfileRepository' src/` возвращает пустой результат

### AC-002 PostgresProfileRepo удалён
- Почему это важно: мёртвая реализация не сбивает с толку
- **Given** файл `src/internal/adapters/repository/postgres/profile.go`
- **When** проверяется наличие файла
- **Then** файл не существует
- **Evidence** `test -f src/internal/adapters/repository/postgres/profile.go && echo exists || echo not found` возвращает "not found"

### AC-003 ProfileHandler удалён
- Почему это важно: мёртвый handler не создаёт ложного впечатления работающего API
- **Given** пакет `src/internal/api/handler/profile/`
- **When** проверяется наличие пакета
- **Then** пакет не существует
- **Evidence** `test -d src/internal/api/handler/profile/ && echo exists || echo not found` возвращает "not found"

### AC-004 Profile entity и value objects удалены
- **Given** файлы `entity/profile.go`, `value/profile_id.go`, `value/profile_slug.go`
- **When** проверяется наличие файлов
- **Then** файлы не существуют
- **Evidence** `ls src/internal/domain/shield/entity/profile.go value/profile_id.go value/profile_slug.go` возвращает error

### AC-005 DictionaryCache не реализует ProfileRepository
- **Given** `src/internal/adapters/repository/dictionary/cached.go`
- **When** проверяется отсутствие `var _ shield.ProfileRepository`
- **Then** assertion отсутствует
- **Evidence** `grep 'ProfileRepository' src/internal/adapters/repository/dictionary/cached.go` пуст

### AC-006 ListByProfile удалён
- **Given** интерфейс `IncidentRepository` и реализация
- **When** проверяется наличие `ListByProfile`
- **Then** метод отсутствует
- **Evidence** `grep -r 'ListByProfile' src/` пуст

### AC-007 ScanPipelineFactory.Build(ctx, profile) удалён
- **Given** `src/internal/app/usecase/shield/pipeline_factory.go`
- **When** проверяется отсутствие метода `Build(ctx context.Context, profile`
- **Then** метод удалён
- **Evidence** `grep 'func.*Build(ctx' src/internal/app/usecase/shield/pipeline_factory.go` показывает только `BuildFromRules`

### AC-008 RegisterProfileHandler удалён
- **Given** `server.go` и `admin.go`
- **When** проверяется наличие `RegisterProfileHandler`
- **Then** функция не найдена
- **Evidence** `grep -r 'RegisterProfileHandler' src/internal/api/` пуст

### AC-009 Компиляция и тесты
- **Given** код после cleanup
- **When** `go build ./... && go vet ./... && go test ./...`
- **Then** все три команды возвращают exit code 0
- **Evidence** вывод команд не содержит ошибок

### AC-010 Миграция cleanup создана
- **Given** `deployments/migrations/`
- **When** проверяется наличие файла `006_cleanup.up.sql` (или аналогичного)
- **Then** файл существует и содержит `DROP TABLE IF EXISTS profiles, dictionary_entries`
- **Evidence** `cat deployments/migrations/006_cleanup.up.sql`

### AC-011 profileSlug удалён из Dictionary
- **Given** `src/internal/domain/shield/dictionary/dictionary.go`
- **When** проверяется отсутствие поля `profileSlug`
- **Then** поле и метод `ProfileSlug()` удалены
- **Evidence** `grep 'profileSlug\|ProfileSlug' src/internal/domain/shield/dictionary/dictionary.go` пуст

### AC-012 ProfileID удалён из MaskEntry
- **Given** `src/internal/domain/shield/mask/entity.go`
- **When** проверяется отсутствие поля `ProfileID`
- **Then** поле удалено
- **Evidence** `grep 'ProfileID' src/internal/domain/shield/mask/entity.go` пуст

### AC-013 DictionaryCacheWarmer удалён
- **Given** файл `src/internal/adapters/repository/dictionary/warm.go`
- **When** проверяется наличие файла
- **Then** файл не существует
- **Evidence** `test -f src/internal/adapters/repository/dictionary/warm.go && echo exists || echo not found` возвращает "not found"

### AC-014 Incident.ProfileSlug удалён
- **Given** `src/internal/domain/shield/entity/incident.go`
- **When** проверяется отсутствие поля `profileSlug` и метода `ProfileSlug()`
- **Then** поле и метод удалены
- **Evidence** `grep 'profileSlug\|ProfileSlug' src/internal/domain/shield/entity/incident.go | grep -v '@sk-'` пуст (trace-маркеры не считаются)

## Допущения

- Tenant-profile-sync spec полностью реализована: tenant dictionaries работают, ProfileRepository не нужен.
- `DictionaryCache` после удаления profile-методов всё ещё может быть востребован для tenant dictionary cache в будущем.
- Миграция `006_cleanup` идёт после `005_tenants` в порядке миграций.

## Краевые случаи

- Если `IncidentFilterParams.ProfileSlug` используется в UI для фильтрации — решение: проверить при плане, заменить на tenant_slug если нужно.

## Открытые вопросы

- **IncidentFilterParams.ProfileSlug**: Используется ли UI для фильтрации по profile_slug? Нужно проверить при плане. Если не используется — удалить. Если используется — заменить на tenant_slug фильтрацию.
- **incident.profile_slug колонка в БД**: колонка не удаляется этой спекой (только таблицы profiles/dictionary_entries). Если нужно — отдельная spec.
