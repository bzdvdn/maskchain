# Tenant & Profile Config-DB Sync — Задачи

## Phase Contract

Inputs: plan, spec, data-model.
Outputs: упорядоченные исполнимые задачи с покрытием AC.
Stop if: нет.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/domain/shield/entity/tenant.go` | T1.1 |
| `src/internal/domain/shield/value/tenant_slug.go` | T1.1 |
| `src/internal/domain/shield/entity/profile.go` | T4.2 |
| `src/internal/domain/shield/value/profile_slug.go` | T4.2 |
| `src/internal/domain/shield/value/profile_id.go` | T4.2 |
| `src/internal/domain/shield/repository.go` | T1.3, T4.2 |
| `src/internal/adapters/repository/postgres/tenant.go` | T1.3 |
| `src/internal/adapters/repository/postgres/profile.go` | T4.2 |
| `src/internal/adapters/repository/postgres/migrations/005_tenants.up.sql` | T1.2 |
| `src/internal/adapters/repository/postgres/migrations/006_cleanup.up.sql` | T4.1 |
| `src/internal/api/handler/admin/tenant_handler.go` | T2.2 |
| `src/internal/api/handler/profile/` (delete) | T4.2 |
| `src/internal/api/dto/tenant.go` | T2.2 |
| `src/internal/api/dto/profile.go` (delete) | T4.2 |
| `src/internal/api/middleware/auth.go` | T2.1 |
| `src/internal/api/middleware/shield.go` | T3.1 |
| `src/internal/adapters/repository/profile/` (→ dictionary/) | T4.1 |
| `src/internal/infra/config/config.go` | T4.1 |
| `src/internal/infra/metrics/metrics.go` | T4.1 |
| `src/cmd/admin/main.go` | T2.1, T2.2 |
| `src/cmd/gateway/main.go` | T2.1, T3.1, T4.1 |
| `examples/config.yaml` | T5.1 |
| `examples/seed-profile.sh` | T5.1 |
| `examples/test-prompt.md` | T5.1 |

## Implementation Context

- **Цель MVP:** Profile удалён. Dictionaries в `tenants.dictionaries JSONB`. Tenant CRUD + dictionaries endpoint через admin API. Shield scan читает dictionaries из tenant. X-Shield-Profile-Slug удалён.
- **Границы приемки:** AC-001 – AC-009.
- **Инварианты:**
  - slug tenant уникален (PK)
  - api_keys — непустой массив строк
  - dictionaries — массив `{name, match_mode, entries[]}`, каждый entry string или object
  - startup sync: INSERT ON CONFLICT DO NOTHING (не перезатирает)
  - TenantResolver: DB first, config fallback. Если БД недоступна — 500.
- **Контракты:**
  - `POST /api/v1/admin/tenants` — создание tenant с dictionaries
  - `PUT /api/v1/admin/tenants/:slug/dictionaries` — замена dictionaries, не трогает auth
  - `POST /v1/chat/completions` — без `X-Shield-Profile-Slug`, shield читает dictionaries из tenant
- **Proof signals:**
  - `go test ./... -count=1` — pass
  - `go build ./...` — pass
  - mask/unmask + shield scan e2e через docker compose
- **Вне scope:**
  - K8s operator (отдельная spec)
  - Soft-delete, bulk-import, audit log, rate limiting per tenant
  - Profile CRUD через gateway

## Фаза 1: Фундамент (Tenant entity + миграция + репозиторий + резолвер)

Цель: Tenant entity, TenantSlug VO, миграция 005, TenantRepository, TenantResolver.

- [x] T1.1 Создать Tenant entity и TenantSlug VO.
  - Файлы: entity/tenant.go, value/tenant_slug.go
  - Tenant entity: slug, name, auth_header, api_keys, dictionaries []*dictionary.Dictionary, created_at, updated_at. Геттеры как в Profile.
  - TenantSlug VO: обёртка над string, validation.
  - AC: AC-001, AC-002
  - Touches: src/internal/domain/shield/entity/tenant.go, src/internal/domain/shield/value/tenant_slug.go

- [x] T1.2 Создать миграцию 005: CREATE TABLE tenants.
  - Поля: slug TEXT PK, name TEXT NOT NULL, auth_header TEXT NOT NULL DEFAULT 'Authorization', api_keys JSONB NOT NULL DEFAULT '[]', dictionaries JSONB NOT NULL DEFAULT '[]', created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ.
  - Добавить 005_tenants.down.sql (DROP TABLE).
  - AC: AC-001
  - Touches: src/internal/adapters/repository/postgres/migrations/005_tenants.up.sql, src/internal/adapters/repository/postgres/migrations/005_tenants.down.sql

- [x] T1.3 Создать TenantRepository (postgres).
  - Методы: List, Get(slug), Create, Update, Delete, GetDictionaries(slug), UpdateDictionaries(slug, dictionaries).
  - Create: UNIQUE violation → ErrDuplicate. Delete: not found → ErrNotFound.
  - json.Marshal/Unmarshal для api_keys и dictionaries при записи/чтении JSONB.
  - AC: AC-001, AC-005, AC-008
  - Touches: src/internal/adapters/repository/postgres/tenant.go, src/internal/domain/shield/repository.go (добавить TenantRepository interface)

- [x] T1.4 Создать TenantResolver (DB + config, DB priority).
  - Реализует интерфейс TenantResolver { List(ctx), Get(ctx, slug) }.
  - Сначала проверяет БД, потом config. При конфликте slug побеждает БД.
  - При старте: upsert config tenants → БД (INSERT ON CONFLICT DO NOTHING).
  - AC: AC-002, AC-003, AC-004
  - Touches: src/internal/domain/shield/repository.go, src/internal/adapters/repository/postgres/tenant.go

## Фаза 2: Auth + Tenant CRUD API

Цель: Auth middleware использует TenantResolver. Tenant CRUD handler + dictionaries endpoint.

- [x] T2.1 Переключить auth middleware на TenantResolver.
  - Gateway и admin: вместо `cfg.Tenants` читать tenant через resolver.
  - Startup: resolver.SyncConfig(ctx) — upsert tenants из config в БД (blocking, перед стартом HTTP).
  - После валидации API-Key: прокинуть tenant в контекст (c.Set("tenant", tenant)).
  - AC: AC-002, AC-005
  - Touches: src/internal/api/middleware/auth.go, src/cmd/admin/main.go, src/cmd/gateway/main.go

- [x] T2.2 Создать Tenant admin CRUD handler + dictionaries endpoint.
  - POST /api/v1/admin/tenants — 201 Created
  - GET /api/v1/admin/tenants — список (без dictionaries, только мета)
  - GET /api/v1/admin/tenants/:slug — полный tenant
  - PUT /api/v1/admin/tenants/:slug — полное обновление
  - DELETE /api/v1/admin/tenants/:slug — 204, с инвалидацией кеша
  - GET /api/v1/admin/tenants/:slug/dictionaries — только dictionaries
  - PUT /api/v1/admin/tenants/:slug/dictionaries — замена dictionaries (не трогает auth)
  - Валидация: entry должен быть string или object. Иначе 400.
  - Зарегистрировать роуты в admin main.go.
  - Создать TenantDTO.
  - AC: AC-001, AC-005, AC-008
  - Touches: src/internal/api/handler/admin/tenant_handler.go, src/internal/api/dto/tenant.go, src/cmd/admin/main.go

## Фаза 3: Shield scan

Цель: Shield middleware читает dictionaries из tenant, не требует X-Shield-Profile-Slug.

- [x] T3.1 Обновить shield middleware: читать dictionaries из tenant.
  - Убрать чтение X-Shield-Profile-Slug из заголовка.
  - Читать tenant из контекста (c.Get("tenant")) → tenant.Dictionaries.
  - Создать DictionaryDetector из tenant.Dictionaries.
  - AC: AC-006, AC-007
  - Touches: src/internal/api/middleware/shield.go, src/cmd/gateway/main.go

## Фаза 4: Cache rename + cleanup

Цель: ProfileCache → DictionaryCache. Миграция 006. Удаление Profile кода.

- [x] T4.1 Переименовать ProfileCache → DictionaryCache, Config → DictionaryCacheConfig.
  - Пакет: repository/profile/ → repository/dictionary/.
  - Ключ кеша: profile_slug → tenant_slug.
  - Config struct: ProfileCache → DictionaryCache (mapstructure, env).
  - Метрики: ProfileCacheHitsTotal → DictionaryCacheHitsTotal и т.д.
  - Обновить все импорты в admin/main.go, gateway/main.go.
  - AC: AC-006
  - Touches: src/internal/adapters/repository/profile/ (rename to dictionary/), src/internal/infra/config/config.go, src/internal/infra/metrics/metrics.go, src/cmd/admin/main.go, src/cmd/gateway/main.go

- [x] T4.2 Удалить Profile entity, repository, handler, DTO.
  - Удалить: entity/profile.go, value/profile_slug.go, value/profile_id.go, postgres/profile.go.
  - Удалить: handler/profile/ весь пакет, dto/profile.go.
  - Удалить ProfileRepository из repository.go.
  - AC: AC-009
  - Touches: src/internal/domain/shield/entity/profile.go, src/internal/domain/shield/value/profile_slug.go, src/internal/domain/shield/value/profile_id.go, src/internal/domain/shield/repository.go, src/internal/adapters/repository/postgres/profile.go, src/internal/api/handler/profile/, src/internal/api/dto/profile.go

- [x] T4.3 Создать миграцию 006: cleanup → DROP profiles, dictionary_entries, ALTER incidents.
  - UPDATE incidents SET tenant_slug = profile_slug WHERE tenant_slug IS NULL.
  - DROP TABLE dictionary_entries, profiles.
  - incidents: tenant_slug TEXT NOT NULL REFERENCES tenants(slug).
  - 006_cleanup.down.sql: восстановить таблицы (данные не сохраняются).
  - AC: AC-009
  - Touches: src/internal/adapters/repository/postgres/migrations/006_cleanup.up.sql, src/internal/adapters/repository/postgres/migrations/006_cleanup.down.sql

## Фаза 5: Examples

Цель: examples конфиги и скрипты.

- [x] T5.1 Обновить examples.
  - examples/config.yaml: tenants с dictionaries (без profile_slug).
  - examples/seed-profile.sh: POST /api/v1/admin/tenants с dictionaries.
  - examples/test-prompt.md: убрать X-Shield-Profile-Slug, shield scan использует tenant.
  - AC: AC-006
  - Touches: examples/config.yaml, examples/seed-profile.sh, examples/test-prompt.md

## Фаза 6: Проверка

Цель: доказать, что фича работает.

- [x] T6.1 Написать unit-тесты для TenantRepository и TenantResolver.
  - TenantRepository: Create, Get, Update, Delete, GetDictionaries, UpdateDictionaries.
  - TenantResolver: List (config + DB), Get (DB приоритет), SyncConfig (не перезатирает).
  - AC: AC-001, AC-002, AC-003, AC-004, AC-005, AC-008
  - Touches: src/internal/adapters/repository/postgres/tenant_test.go, src/internal/domain/shield/resolver/tenant_resolver_test.go

- [x] T6.2 Написать тесты для shield middleware без X-Shield-Profile-Slug.
  - Shield middleware читает dictionaries из tenant в контексте.
  - Пустые dictionaries → нет dictionary match.
  - AC: AC-006, AC-007
  - Touches: src/internal/api/middleware/shield_test.go

- [x] T6.3 Написать тесты для tenant handler (CRUD + dictionaries endpoint).
  - HTTP-хендлер: 201, 200, 204, 400, 404, 409.
  - dictionaries endpoint: замена не трогает api_keys.
  - AC: AC-001, AC-005, AC-008
  - Touches: src/internal/api/handler/admin/tenant_handler_test.go

- [x] T6.4 Проверить миграцию 006: SQL-запрос к information_schema.
  - Таблицы profiles, dictionary_entries отсутствуют.
  - incidents имеет FK на tenants(slug).
  - AC: AC-009
  - Touches: тест в postgres_test.go или ручная проверка

## Покрытие критериев приемки

- AC-001 → T1.1, T1.2, T1.3, T2.2, T6.1, T6.3
- AC-002 → T1.4, T2.1, T6.1
- AC-003 → T1.4, T6.1
- AC-004 → T1.4, T6.1
- AC-005 → T1.3, T2.2, T6.3
- AC-006 → T3.1, T4.1, T6.2
- AC-007 → T3.1, T6.2
- AC-008 → T1.3, T2.2, T6.3
- AC-009 → T4.2, T4.3, T6.4
