# Persistence Layer Plan

## Phase Contract

Inputs: spec (`specs/active/30-shield-persistence/spec.md`), inspect (`pass`), repo surfaces (existing stubs, domain entities, config).
Outputs: `plan.md`, `data-model.md`.
Stop if: spec расплывчата — нет, spec specific.

## Цель

Реализовать полноценные PostgreSQL репозитории (Profile, Dictionary, Incident) с миграциями через golang-migrate, connection pool config, транзакционным helper и тестами. Заменить существующие stubs в `src/internal/adapters/repository/{profile,dictionary}/` на единый пакет `postgres`.

## MVP Slice

Migration setup + ProfileRepo (CRUD + полная загрузка словарей/препроцессоров) + базовый connection pool. Закрывает AC-001, AC-005.

## First Validation Path

`go test -tags=integration -count=1 ./src/internal/adapters/repository/postgres/...` с testcontainers PostgreSQL — проверяет создание профиля, запись словарей, чтение со всеми связанными данными.

## Scope

- `src/internal/adapters/repository/postgres/` — единый пакет для всех postgres-репозиториев
- `src/internal/adapters/repository/postgres/migrations/` — SQL-миграции (golang-migrate)
- `src/internal/infra/config/config.go` — расширение DatabaseConfig (pool params)
- `src/internal/infra/config/config_test.go` — тест конфига (опционально)
- `go.mod` — добавление golang-migrate + testcontainers-go
- Удалить: `src/internal/adapters/repository/profile/postgres.go`, `src/internal/adapters/repository/dictionary/postgres.go`, `src/internal/infra/migrations/002_dictionary_entries.sql`
- Нетронуто: domain-сущности, port-интерфейсы, REST API, Valkey, UI

## Performance Budget

- SC-001: ProfileRepo.FindBySlug (1000 словарных записей) < 100ms p95 (local dev)
- SC-002: IncidentRepo.ListByProfile (10k записей) < 200ms p95
- `none` для memory budget (данные небольшие на MVP)

## Implementation Surfaces

| Surface | Change | Reason |
|---------|--------|--------|
| `src/internal/adapters/repository/postgres/migrations/` | NEW | Миграции golang-migrate |
| `src/internal/adapters/repository/postgres/profile.go` | NEW | ProfileRepo (полная загрузка) |
| `src/internal/adapters/repository/postgres/dictionary.go` | NEW | DictionaryRepo (новая схема) |
| `src/internal/adapters/repository/postgres/incident.go` | NEW | IncidentRepo |
| `src/internal/adapters/repository/postgres/transaction.go` | NEW | TransactionManager interface + impl |
| `src/internal/adapters/repository/postgres/pool.go` | NEW | pool initialization helper |
| `src/internal/adapters/repository/postgres/postgres_test.go` | NEW | Unit + integration тесты |
| `src/internal/infra/config/config.go` | MODIFY | DatabaseConfig pool params |
| `src/internal/adapters/repository/profile/postgres.go` | DELETE | Replaced |
| `src/internal/adapters/repository/dictionary/postgres.go` | DELETE | Replaced |
| `src/internal/infra/migrations/002_dictionary_entries.sql` | DELETE | Replaced |
| `go.mod` | MODIFY | Добавить deps |

## Bootstrapping Surfaces

`src/internal/adapters/repository/postgres/` — новая директория, требуется создать.

## Влияние на архитектуру

- Переход от раздельных пакетов (`profile`, `dictionary`) к единому пакету `postgres` — упрощает транзакционную координацию
- `TransactionManager` — новая абстракция в adapter layer, не затрагивает domain/ports
- Существующие port-интерфейсы (`shield.ProfileRepository` и др.) не меняются
- Старые stubs удаляются, перестают компилироваться — все клиенты должны перейти на новый пакет (в данной фиче прямых клиентов, кроме тестов, нет)

## Acceptance Approach

- **AC-001**: integration-тест: Save профиля → FindBySlug → assert по словарям и препроцессорам
- **AC-002**: integration-тест: Save с инцидентами → Delete → SELECT COUNT = 0
- **AC-003**: unit-тест с моком tx, симулирующим ошибку на втором запросе
- **AC-004**: integration-тест: Save 3 инцидента → ListByProfile → count + data assertion
- **AC-005**: config unit-тест: LoadConfig с pool params → assert на структуре; integration: pool init + SELECT 1
- **AC-006**: unit-тесты с go stdlib `testing` + ручным моком pgxpool через интерфейс (не mockgen)
- **AC-007**: testcontainers: запуск контейнера → все CRUD-сценарии

## Данные и контракты

- Data model меняется: см. `data-model.md`
- API/event контракты не затрагиваются
- Dictionary schema меняется: entries JSONB → отдельные строки entry_value + match_mode

## Стратегия реализации

### DEC-001 Миграции через golang-migrate с pgx v5 driver

Why: pgx/v5 уже в проекте; `github.com/golang-migrate/migrate/v4/database/pgx/v5` работает с pgxpool напрямую, без `database/sql`. Goose требует обёртки.
Tradeoff: формат .up.sql/.down.sql (2 файла на миграцию) вместо одного с аннотациями.
Affects: `postgres/migrations/`, файлы миграций
Validation: миграции применяются, таблицы создаются — verified in integration test

### DEC-002 TransactionManager interface

Why: чистая Dependency Inversion — репозитории не зависят от pgx.Tx. Позволяет мокать транзакции в unit-тестах (AC-003, AC-006).
Tradeoff: дополнительный слой абстракции (~15 строк интерфейса + ~20 строк impl). Незначительно.
Affects: `postgres/transaction.go`, все репозитории в `postgres/`
Validation: unit-тест с моком TransactionManager, симулирующим rollback

### DEC-003 Auto-generated UUIDv7 slug

Why: переиспользует существующий `mask.NewUUIDv7()` в `src/internal/domain/shield/mask/uuid.go`. Устраняет необходимость в дополнительном запросе на проверку уникальности — UNIQUE constraint БД защищает от коллизий.
Tradeoff: slug нечеловекочитаем. Если потребуются user-friendly slug — добавить позже с отдельным полем или опцией.
Affects: application-слой генерации slug (будет определён в tasks)
Validation: UNIQUE constraint violation test

### DEC-004 Единый пакет postgres вместо раздельных profile/dictionary

Why: ProfileRepo.DictionaryRepo — композиция; DictionaryRepo нужен внутри ProfileRepo для загрузки словарей. В одном пакете нет циклических dependences. Упрощает транзакции между таблицами.
Tradeoff: более крупный пакет. Не проблема для adapter layer — это impl detail.
Affects: удаление старых пакетов `profile/` и `dictionary/`
Validation: `go build ./...` — отсутствие ошибок импорта

### DEC-005 Cascade delete на уровне приложения (не ON DELETE CASCADE)

Why: больше контроля — можно добавить логирование, аудит, проверки перед удалением. FK-constraint остаётся для защиты ссылочной целостности.
Tradeoff: дополнительный запрос на удаление словарей и инцидентов; риск неконсистентности при сбое между DELETE. Mitigation: всё в одной транзакции.
Affects: ProfileRepo.Delete()
Validation: integration-тест AC-002

## Incremental Delivery

### MVP (AC-001, AC-005)

- Миграции + pool init + config + ProfileRepo (Save/FindBySlug)
- DictionaryRepo (базовый, только Save/FindByProfileSlug для поддержки ProfileRepo)
- Integration-тест

### Итеративное расширение (AC-002, AC-004)

- IncidentRepo + каскадное удаление
- Unit-тесты (AC-006: моки TransactionManager, моки pgx-запросов)

### Финальные AC (AC-003, AC-007)

- TransactionManager + транзакционные сценарии
- Полный integration-тест всех репозиториев

## Порядок реализации

1. Bootstrapping: директория `postgres/`, template-файлы
2. Migration files: `001_profiles.sql`, `002_dictionary_entries.sql`, `003_incidents.sql`
3. Config: pool params в DatabaseConfig
4. Pool init helper
5. TransactionManager
6. DictionaryRepo (базовый CRUD)
7. ProfileRepo (Save/FindBySlug с полной загрузкой)
8. IncidentRepo (Save/ListByProfile)
9. Обновление main.go: инициализация pool, применение миграций
10. Unit-тесты (моки)
11. Integration-тесты (testcontainers)

Параллельно: удаление старых stub-файлов.

## Риски

- **Риск 1**: golang-migrate embedded driver не поддерживает pgx v5 в той версии, что совместима с текущим pgx. Mitigation: проверить совместимость `github.com/golang-migrate/migrate/v4/database/pgx/v5` с pgx v5.10.0 из go.mod; при конфликте — использовать `database/sql` driver с `pgx/stdlib`.
- **Риск 2**: Замена schema dictionary_entries (JSONB → строки) сломает существующие данные, если они есть. Mitigation: на текущем этапе проекта production данных нет; при появлении — добавить миграцию-конвертор.
- **Риск 3**: testcontainers не работает в CI без Docker. Mitigation: integration-тесты под `//go:build integration`; CI запускает только unit-тесты; integration — опционально с Docker.

## Rollout и compatibility

- Специальных rollout-действий не требуется: старые stubs не используются production-кодом (все методы возвращают nil)
- После реализации — удалить старые файлы и проверить `go build ./...`

## Проверка

| Этап | Проверка | Связь |
|------|----------|-------|
| Unit | `go test -count=1 -short ./src/internal/adapters/repository/postgres/...` | AC-006, DEC-002 |
| Integration | `go test -tags=integration -count=1 ./src/internal/adapters/repository/postgres/...` | AC-001, AC-002, AC-004, AC-007 |
| Build | `go build ./...` | DEC-004 |
| Config | `go test ./src/internal/infra/config/...` | AC-005 |
| Lint | `make lint` | code quality |

## Соответствие конституции

- нет конфликтов: PostgreSQL persistence, DDD/Clean Architecture, Content Shield core domain — все принципы соблюдены
- Language policy: doc=ru, comments=en — spec соблюдает
