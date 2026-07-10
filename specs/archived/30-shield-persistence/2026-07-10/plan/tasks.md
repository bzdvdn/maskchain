# Persistence Layer Tasks

## Phase Contract

Inputs: `plan.md`, `spec.md`, `data-model.md`.
Outputs: исполнимые задачи с Touches: и покрытием AC.
Stop if: любой AC нельзя привязать к задачам — нет, все покрыты.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `go.mod` | T1.1 |
| `src/internal/adapters/repository/postgres/migrations/001_profiles.up.sql` | T1.2 |
| `src/internal/adapters/repository/postgres/migrations/001_profiles.down.sql` | T1.2 |
| `src/internal/adapters/repository/postgres/migrations/002_dictionary_entries.up.sql` | T1.2 |
| `src/internal/adapters/repository/postgres/migrations/002_dictionary_entries.down.sql` | T1.2 |
| `src/internal/adapters/repository/postgres/migrations/003_incidents.up.sql` | T1.2 |
| `src/internal/adapters/repository/postgres/migrations/003_incidents.down.sql` | T1.2 |
| `src/internal/infra/config/config.go` | T1.3 |
| `src/internal/infra/config/config_test.go` | T1.3 |
| `src/internal/adapters/repository/profile/postgres.go` | T1.4 |
| `src/internal/adapters/repository/dictionary/postgres.go` | T1.4 |
| `src/internal/infra/migrations/002_dictionary_entries.sql` | T1.4 |
| `src/internal/adapters/repository/postgres/pool.go` | T2.1 |
| `src/internal/adapters/repository/postgres/transaction.go` | T2.1 |
| `src/internal/adapters/repository/postgres/dictionary.go` | T2.2 |
| `src/internal/adapters/repository/postgres/profile.go` | T2.3, T3.2 |
| `src/cmd/gateway/main.go` | T2.4 |
| `src/internal/adapters/repository/postgres/incident.go` | T3.1 |
| `src/internal/adapters/repository/postgres/postgres_unit_test.go` | T3.3 |
| `src/internal/adapters/repository/postgres/postgres_integration_test.go` | T4.1 |

## Implementation Context

- Цель MVP: ProfileRepo (Save/FindBySlug с полной загрузкой словарей + препроцессоров) + миграции + connection pool. AC-001, AC-005.
- Инварианты/семантика:
  - Profile slug = UUIDv7 (через `mask.NewUUIDv7()`), уникальность — UNIQUE constraint
  - Dictionaries: replace all (DELETE + INSERT per profile_slug), не in-place update
  - Preprocessors: JSONB, read/write целиком (no partial update)
  - Preprocessors NULL = SQL NULL, not empty array (см. unmarshalPreprocessors)
  - cascade delete на уровне приложения, в одной транзакции (DEC-005)
  - version increment на каждый Save
- Ошибки/коды:
  - not found -> return (nil, nil) — nil error (существующий convention)
  - FK violation -> pgx error, пробрасывается caller-у
  - duplicate slug -> pgx unique violation, пробрасывается
- Контракты/протокол:
  - Репозитории реализуют существующие port-интерфейсы из `domain/shield/` и `domain/shield/dictionary/`
  - TransactionManager: `RunInTx(ctx, func(context.Context) error) error`
  - Connection pool: pgxpool, инициализируется через конфиг + pool helper
- Proof signals:
  - `go build ./...` — без ошибок импорта
  - `go test -short ./...` — unit-тесты проходят
  - `go test -tags=integration ./...` — integration-тесты проходят с testcontainers
- Вне scope: REST API, Valkey cache, пагинация, версионирование, история изменений
- References: DEC-001..DEC-005, DM-001..DM-003

## Фаза 1: Основа

Цель: подготовить зависимости, миграции, конфиг и очистить старые stubs.

- [x] T1.1 Добавить golang-migrate и testcontainers-go в go.mod. Touches: `go.mod`.

- [x] T1.2 Создать SQL-миграции golang-migrate в `src/internal/adapters/repository/postgres/migrations/`. Схемы — по DM-001, DM-002, DM-003. Touches: `src/internal/adapters/repository/postgres/migrations/001_profiles.up.sql`, `src/internal/adapters/repository/postgres/migrations/001_profiles.down.sql`, `src/internal/adapters/repository/postgres/migrations/002_dictionary_entries.up.sql`, `src/internal/adapters/repository/postgres/migrations/002_dictionary_entries.down.sql`, `src/internal/adapters/repository/postgres/migrations/003_incidents.up.sql`, `src/internal/adapters/repository/postgres/migrations/003_incidents.down.sql`.

- [x] T1.3 Расширить DatabaseConfig в `src/internal/infra/config/config.go`: добавить `MaxConns`, `MinConns`, `MaxConnLifetime`. Обновить тест в `config_test.go`. Touches: `src/internal/infra/config/config.go`, `src/internal/infra/config/config_test.go`.

- [x] T1.4 Удалить старые stub-файлы: `src/internal/adapters/repository/profile/postgres.go`, `src/internal/adapters/repository/dictionary/postgres.go`, `src/internal/infra/migrations/002_dictionary_entries.sql`. Touches: `src/internal/adapters/repository/profile/postgres.go`, `src/internal/adapters/repository/dictionary/postgres.go`, `src/internal/infra/migrations/002_dictionary_entries.sql`.

## Фаза 2: MVP Slice

Цель: реализовать ProfileRepo (Save/FindBySlug) с DictionaryRepo, pool init, TransactionManager. AC-001, AC-005.

- [x] T2.1 Реализовать pool init helper и TransactionManager: `pool.go` — `NewPool(ctx, cfg) (*pgxpool.Pool, error)` с Ping; `transaction.go` — TransactionManager interface + PGXTransactionManager impl. Touches: `src/internal/adapters/repository/postgres/pool.go`, `src/internal/adapters/repository/postgres/transaction.go`.

- [x] T2.2 Реализовать DictionaryRepo (новая схема DM-002): Save (DELETE + INSERT batch), FindByProfileSlug, Delete. Все через TransactionManager. Touches: `src/internal/adapters/repository/postgres/dictionary.go`.

- [x] T2.3 Реализовать ProfileRepo (DM-001), имплементирующий `shield.ProfileRepository`: Save, FindBySlug, FindByID, ListByTenant. Slug через `mask.NewUUIDv7()`, preprocessors JSONB marshal/unmarshal. Полная загрузка словарей. Touches: `src/internal/adapters/repository/postgres/profile.go`.

- [x] T2.4 Обновить `src/cmd/gateway/main.go`: при старте инициализировать pool, применить миграции, создать репозитории. Touches: `src/cmd/gateway/main.go`.

## Фаза 3: Основная реализация

Цель: IncidentRepo, cascade delete, unit-тесты. AC-002, AC-004, AC-006.

- [x] T3.1 Реализовать IncidentRepo (DM-003), имплементирующий `shield.IncidentRepository`: Save, ListByProfile (ORDER BY timestamp DESC), ListByTenant. Touches: `src/internal/adapters/repository/postgres/incident.go`.

- [x] T3.2 Реализовать cascade delete в ProfileRepo.Delete: в одной транзакции DELETE incidents, dictionary_entries, profiles через profile_slug/slug/id. Touches: `src/internal/adapters/repository/postgres/profile.go`.

- [x] T3.3 Добавить unit-тесты с моками: мок TransactionManager, мок pgx.Rows. Тесты: Save успех, conflict, not found, empty list, Delete cascade, транзакционный rollback (AC-003). Без build tag. Touches: `src/internal/adapters/repository/postgres/postgres_unit_test.go`.

## Фаза 4: Проверка

Цель: integration-тесты, финальная верификация. AC-007.

- [x] T4.1 Добавить integration-тесты с testcontainers. `//go:build integration`. Запуск PG контейнера, init pool, миграции. CRUD-сценарии для всех трёх репозиториев (AC-001, AC-002, AC-004, AC-005). Touches: `src/internal/adapters/repository/postgres/postgres_integration_test.go`.

## Покрытие критериев приемки

- AC-001 -> T2.2, T2.3, T4.1
- AC-002 -> T3.2, T4.1
- AC-003 -> T3.3, T4.1
- AC-004 -> T3.1, T4.1
- AC-005 -> T1.3, T2.1, T4.1
- AC-006 -> T3.3
- AC-007 -> T4.1
