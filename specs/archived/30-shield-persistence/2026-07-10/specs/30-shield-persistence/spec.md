# Persistence Layer: PostgreSQL Repositories for Profiles, Dictionaries, and Incidents

## Scope Snapshot

- In scope: полноценная PostgreSQL persistence для профилей, словарей и инцидентов Content Shield: миграции, CRUD-репозитории, connection pool, транзакционная поддержка, unit-мок-тесты и integration-тесты.
- Out of scope: кэширование (Valkey), REST API endpoints, UI, бизнес-логика сканирования/детекции, Envoy data plane.

## Цель

Разработчик/администратор Content Shield получает надёжное хранилище профилей справочников со словарями и препроцессорами (JSONB), а также лог инцидентов. Успех фичи измеряется прохождением CRUD-тестов (unit + integration) и возможностью запуска gateway с PostgreSQL-хранилищем без потери данных.

## Основной сценарий

1. Gateway стартует, применяет миграции к PostgreSQL через goose/golang-migrate.
2. Оператор создаёт профиль: ProfileRepo сохраняет запись в `profiles`, препроцессоры — в `profiles.preprocessors` (JSONB).
3. Оператор добавляет словарные записи профиля: DictionaryRepo сохраняет их в `dictionary_entries` с привязкой по `profile_slug`.
4. Shield Engine детектирует инцидент: IncidentRepo сохраняет запись в `incidents`.
5. При запросе профиля ProfileRepo загружает профиль, словари (через DictionaryRepo) и препроцессоры одной транзакцией.
6. Ошибка БД (сетевой сбой, конфликт) — транзакция откатывается, вызывающий получает ошибку.

## User Stories

- P1 Story: Оператор может создать профиль, добавить к нему словари, и прочитать профиль со словарями и препроцессорами.
- P2 Story: Оператор может просмотреть лог инцидентов по профилю и по тенанту.
- P3 Story: Оператор может удалить профиль каскадно (словари + инциденты).

## MVP Slice

P1 (создание/чтение профиля со словарями и препроцессорами) + базовый connection pool + миграции. AC-001–AC-005.

## First Deployable Outcome

После первого implementation pass можно запустить Gateway локально (Docker Compose), применить миграции, создать профиль через прямой вызов ProfileRepo и прочитать его со всеми связанными данными. Проверяется через integration-тест (testcontainers).

## Scope

- Миграции (goose/golang-migrate) для таблиц: `profiles`, `dictionary_entries`, `incidents`
- `profiles` — id (UUIDv7/PK), slug (UNIQUE), name, tenant_id, preprocessors (JSONB), status, version, created_at, updated_at
- `dictionary_entries` — id (PK), profile_slug (FK → profiles.slug), entry_value (TEXT), match_mode (VARCHAR), created_at
- `incidents` — id (PK), profile_slug (FK), request_id, detector_type, entry_value, severity, action, raw_snippet (TEXT), timestamp
- `src/internal/adapters/repository/postgres/` — ProfileRepo, DictionaryRepo, IncidentRepo
- ProfileRepo с полной загрузкой словарей + препроцессоров (транзакционно)
- DictionaryRepo — CRUD для словарных записей профиля
- IncidentRepo — Save + ListByProfile + ListByTenant
- Connection pool config (pgxpool) в `src/internal/infra/config/`
- Транзакционный wrapper/helper для репозиториев
- Unit-тесты (mock repository interfaces) + integration-тесты (testcontainers)
- Каскадное удаление: удаление профиля → удаление связанных dictionary_entries и incidents

## Контекст

- Репозитории реализуют существующие port-интерфейсы: `shield.ProfileRepository`, `shield.IncidentRepository`, `dictionary.DictionaryRepository`
- Существующие stubs: `PostgresProfileRepo` (пустые методы), `PostgresDictionaryRepo` (частичная реализация)
- Существующая миграция `002_dictionary_entries.sql` с другой схемой (profile_slug PK, entries JSONB) — требует замены
- Модуль `src/internal/infra/config/` уже содержит `DatabaseConfig{DSN}` — требуется расширение (pool config)
- go.mod: pgx/v5 уже подключён; testcontainers-go требуется добавить для integration-тестов
- Profile slug — первичный идентификатор для связей (FK от dictionary_entries и incidents)

## Зависимости

- Зависит от интерфейсов: `shield.ProfileRepository`, `shield.IncidentRepository`, `dictionary.DictionaryRepository` (существуют)
- Зависит от сущностей: `entity.Profile`, `entity.Incident`, `dictionary.Dictionary`, `preprocessor.PreprocessorDef` (существуют)
- Внешние библиотеки: pgx/v5 (уже есть), goose или golang-migrate (требуется добавить), testcontainers-go (требуется добавить для integration-тестов)
- `none` меж-спековых dependencies

## Требования

- RQ-001 Система ДОЛЖНА применять миграции БД при старте (goose/golang-migrate) в порядке нумерации.
- RQ-002 ProfileRepo ДОЛЖЕН сохранять и загружать профиль с полным набором словарей и препроцессоров в одной транзакции.
- RQ-003 Profile slug ДОЛЖЕН быть уникальным идентификатором профиля, генерироваться автоматически при создании и использоваться как внешний ключ для dictionary_entries и incidents.
- RQ-004 DictionaryRepo ДОЛЖЕН поддерживать CRUD для словарных записей, привязанных к profile_slug.
- RQ-005 IncidentRepo ДОЛЖЕН сохранять инцидент и поддерживать выборку по profile_slug и tenant_id.
- RQ-006 При удалении профиля ДОЛЖНЫ каскадно удаляться связанные dictionary_entries и incidents.
- RQ-007 Connection pool ДОЛЖЕН конфигурироваться через config (max_conns, min_conns, max_conn_lifetime).
- RQ-008 Репозитории ДОЛЖНЫ поддерживать транзакционное выполнение (возможность передать pgx.Tx или использовать транзакционный helper).
- RQ-009 Таблица `incidents` ДОЛЖНА иметь индекс по `timestamp` для эффективной сортировки лога.

## Вне scope

- REST API/HTTP handlers для профилей, словарей и инцидентов — будут в следующей фиче
- React UI для управления профилями
- Valkey-кэширование профилей или словарей
- Версионирование профилей (history/changelog)
- Импорт/экспорт профилей
- Пагинация для списков (ListByTenant, ListByProfile)
- Envoy-режим

## Критерии приемки

### AC-001 Создание и чтение профиля со словарями и препроцессорами

- Почему это важно: профиль — центральная сущность; без чтения всех связанных данных Shield Engine не может работать.
- **Given** пустая БД PostgreSQL
- **When** ProfileRepo.Save() создаёт профиль с двумя словарными записями и одним препроцессором, затем ProfileRepo.FindBySlug() загружает его
- **Then** загруженный профиль содержит те же словари (entry_value, match_mode), те же препроцессоры (тип, правила), корректные slug, name, tenant_id, status, version
- Evidence: data equality assertion в integration-тесте; миграции применились (таблицы существуют)

### AC-002 Каскадное удаление профиля

- Почему это важно: защита от orphan-записей и утечки данных.
- **Given** профиль со словарными записями и инцидентами в БД
- **When** ProfileRepo.Delete() удаляет профиль
- **Then** связанные записи в dictionary_entries и incidents удалены; профиль не находится через FindBySlug
- Evidence: SELECT COUNT после удаления возвращает 0; integration-тест

### AC-003 Транзакционная загрузка профиля с partial failure

- Почему это важно: консистентность данных при сбоях.
- **Given** профиль в БД с 5 словарными записями
- **When** ProfileRepo.FindBySlug() выполняется в транзакции, и на середине загрузки словарей происходит разрыв соединения
- **Then** транзакция откатывается; профиль не загружается (ошибка или nil); БД остаётся в консистентном состоянии
- Evidence: mock/симуляция ошибки в integration-тесте; assert на rollback

### AC-004 Сохранение и список инцидентов по профилю

- Почему это важно: observability — оператор должен видеть историю инцидентов.
- **Given** профиль с slug="test-profile"
- **When** IncidentRepo.Save() вызывается 3 раза с разными данными для того же profile_slug, затем ListByProfile()
- **Then** возвращаются ровно 3 инцидента с корректными полями (request_id, detector_type, severity, action, timestamp)
- Evidence: count assertion и data equality в integration-тесте

### AC-005 Connection pool конфигурация

- Почему это важно: управление нагрузкой на БД.
- **Given** config.yaml с database.max_conns=5, database.min_conns=1, database.max_conn_lifetime=30m
- **When** приложение загружает конфиг и инициализирует pgxpool
- **Then** pool создаётся с указанными параметрами; репозитории его используют
- Evidence: конфигурация читается, pool успешно создаётся, тестовый запрос выполняется

### AC-006 Unit-тесты с моками

- Почему это важно: верификация логики репозиториев без БД.
- **Given** моки для pgxpool
- **When** каждый метод репозитория вызывается с корректными и некорректными данными
- **Then** ошибки обрабатываются корректно (not found, constraint violation, connection error)
- Evidence: go test -run ./... с mock-реализациями (min 3 test cases на метод)

### AC-007 Integration-тесты с testcontainers

- Почему это важно: гарантия корректной работы с реальной PostgreSQL.
- **Given** testcontainers PostgreSQL контейнер
- **When** выполняются все CRUD-сценарии (Save, FindBySlug, ListByTenant, Delete) для всех трёх репозиториев
- **Then** все операции завершаются без ошибок; данные сохраняются и читаются корректно
- Evidence: go test -tags=integration -run ./... (tags: integration)

## Допущения

- Миграции выполняются при старте gateway (не отдельным CLI-шагом)
- Slug генерируется на уровне приложения (entity/domain), не в БД
- Словари перешли со схемы "одна строка = один словарь с JSONB entries" на "одна строка = одна entry_value + match_mode" (замена существующей миграции 002)
- Каскадное удаление реализуется на уровне приложения (репозитория) или через ON DELETE CASCADE
- NULL preprocessors = NULL, не пустой массив — обрабатывается аналогично `jsonb 'null'` как в unmarshalPreprocessors

## Критерии успеха

- SC-001 ProfileRepo.FindBySlug с 1000 словарных записей выполняется < 100ms (local dev, без нагрузки)
- SC-002 IncidentRepo.ListByProfile с 10k записей < 200ms (с индексом по profile_slug)
- SC-003 Все тесты проходят: `go test -count=1 ./src/internal/adapters/repository/postgres/...`

## Краевые случаи

- Профиль не найден (FindBySlug возвращает nil, nil)
- Профиль без словарей и препроцессоров (загрузка корректна, пустые списки)
- Дубликат profile slug при создании — violation error
- Попытка создать dictionary_entry для несуществующего profile_slug — FK violation
- Пустой список при ListByTenant (нет профилей для tenant)
- Concurrent create/delete одного профиля (pgx serialization error)
- Очень длинные entry_value (>64KB)
- NULL в опциональных полях (description, raw_snippet)

## Открытые вопросы

~~1. Использовать goose или golang-migrate?~~ **Решено: golang-migrate** — native pgx v5 driver, без лишней абстракции через `database/sql`.
~~4. Нужен ли индекс по `incidents.timestamp` для эффективной сортировки лога?~~ **Да**, добавить.
2. Транзакционный helper: передавать `pgx.Tx` через контекст или использовать отдельный `TransactionManager` interface? Первый — легковеснее, второй — тестируемее.
3. Slug generation strategy: UUID-based slug (например, `prefix-random`) или user-provided slug с валидацией на уникальность? Текущий `ProfileSlug` value object с regex-контролем.
