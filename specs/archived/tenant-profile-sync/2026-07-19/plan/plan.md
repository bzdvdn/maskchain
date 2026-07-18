# Tenant & Profile Config-DB Sync — План

## Phase Contract

Inputs: spec `specs/active/tenant-profile-sync/spec.md`, inspect report (pass), repo surfaces.
Outputs: plan.md, data-model.md.
Stop if: нет — spec ясна.

## Цель

Удалить Profile как сущность. Dictionaries перенести в `tenants.dictionaries JSONB`. Добавить tenant CRUD admin API + dictionaries endpoint. Обновить shield scan (без X-Shield-Profile-Slug) и auth middleware (tenant resolver). ProfileCache → DictionaryCache.

## MVP Slice

Весь scope — MVP. Последовательность:
1. Миграция 005 (tenants table) + tenant entity + repository
2. Tenant admin CRUD + dictionaries endpoint
3. Auth middleware → tenant resolver
4. Shield scan → читает dictionaries из tenant
5. Удаление Profile entity, repository, handler, API
6. Cleanup migration 006 (drop old tables)
7. ProfileCache → DictionaryCache
8. Examples

## First Validation Path

1. `docker compose up -d --build`
2. `POST /api/v1/admin/tenants` → 201 с dictionaries в JSONB
3. `PUT /api/v1/admin/tenants/:slug/dictionaries` → 200, auth не задет
4. `POST /v1/chat/completions` c Authorization без X-Shield-Profile-Slug → 403
5. `DELETE /api/v1/admin/tenants/test` → 204, auth ключ tenant не работает
6. `go test ./... -count=1` — все тесты проходят

## Scope

- **Новое:** Tenant entity, TenantRepository, TenantSlug VO, TenantHandler, TenantDTO
- **Новое:** `POST/PUT/GET/DELETE /api/v1/admin/tenants`, `GET/PUT /api/v1/admin/tenants/:slug/dictionaries`
- **Новое:** TenantResolver (DB + config, DB priority)
- **Изменение:** Auth middleware → TenantResolver вместо cfg.Tenants
- **Изменение:** Shield middleware → читает dictionaries из tenant, без X-Shield-Profile-Slug
- **Изменение:** ProfileCache → DictionaryCache (rename + key tenant_slug)
- **Изменение:** cfg.ProfileCache → cfg.DictionaryCache
- **Удаление:** Profile entity, ProfileSlug, ProfileID, ProfileRepository, profile handler, profile DTO
- **Удаление:** `src/internal/adapters/repository/postgres/profile.go`
- **Удаление:** `src/internal/api/handler/profile/`, `src/internal/api/dto/profile.go`
- **Миграция 005:** CREATE TABLE tenants
- **Миграция 006:** DROP TABLE profiles, dictionary_entries; ALTER incidents
- **Examples:** config.yaml, seed-профиль, test-prompt.md

## Performance Budget

- Tenant GET (full): <10ms p99
- Tenant dictionaries PUT 10k entries: <200ms p95
- Shield scan dictionary load: <5ms p99 (из DictionaryCache или JSONB)

## Implementation Surfaces

| Surface | Изменение |
|---------|-----------|
| `entity/tenant.go` | NEW — Tenant entity с slug, name, auth, dictionaries |
| `value/tenant_slug.go` | NEW — TenantSlug VO |
| `repository.go` | Изменить — TenantRepository вместо ProfileRepository |
| `postgres/tenant.go` | NEW — Tenant postgres repo |
| `postgres/profile.go` | DELETE |
| `admin/tenant_handler.go` | NEW — Tenant CRUD handler |
| `dto/tenant.go` | NEW — Tenant DTO |
| `middleware/auth.go` | Изменить — TenantResolver вместо cfg.Tenants |
| `middleware/shield.go` | Изменить — убрать X-Shield-Profile-Slug, читать dictionaries из tenant |
| `handler/profile/` | DELETE — весь пакет |
| `dto/profile.go` | DELETE |
| `entity/profile.go` | DELETE |
| `value/profile_slug.go`, `value/profile_id.go` | DELETE |
| `repository/profile/` | RENAME → repository/dictionary/ |
| `config.go` | Изменить — ProfileCache → DictionaryCache |
| `metrics.go` | Изменить — метрики ProfileCache → DictionaryCache |
| `cmd/admin/main.go` | Изменить — tenant resolver, tenant routes |
| `cmd/gateway/main.go` | Изменить — tenant resolver, dictionary cache |
| `migrations/005_tenants.up.sql` | NEW |
| `migrations/005_tenants.down.sql` | NEW |
| `migrations/006_cleanup.up.sql` | NEW — DROP profiles, dictionary_entries; ALTER incidents |
| `migrations/006_cleanup.down.sql` | NEW |
| `examples/config.yaml` | Изменить — tenants с dictionaries |
| `examples/seed-profile.sh` | Изменить — POST /api/v1/admin/tenants |
| `examples/test-prompt.md` | Изменить — без X-Shield-Profile-Slug |

## Bootstrapping Surfaces

- `entity/tenant.go` — новая entity, от которой всё зависит
- `value/tenant_slug.go` — новый VO

## Влияние на архитектуру

- **Удаляется** Profile как слой (entity → VO → repository → handler → DTO). DictionaryEntries table уходит.
- **Добавляется** Tenant как единая точка: auth + dictionaries.
- **ProfileCache** → DictionaryCache, ключ tenant_slug. Та же архитектура (LRU + valkey + pubsub + warm).
- **Shield** больше не читает X-Shield-Profile-Slug. ProfileSlug VO удаляется.
- **Incidents** FK → tenants(slug).
- **Constitution:** Content Shield core domain не затронут. Profile в БД → Tenant в БД.

## Acceptance Approach

| AC | Approach | Validation |
|----|----------|------------|
| AC-001 | POST /api/v1/admin/tenants с dictionaries → JSONB | HTTP 201 + БД |
| AC-002 | TenantResolver.List читает config + БД | unit test |
| AC-003 | DB tenant переопределяет config | unit test |
| AC-004 | Startup sync не трогает DB tenant | integration |
| AC-005 | PUT dictionaries не меняет api_keys | HTTP 200 + auth |
| AC-006 | Shield блокирует по dictionary из tenant | HTTP 403 |
| AC-007 | Пустые dictionaries = нет dictionary match | HTTP 502/continue |
| AC-008 | DELETE tenant → 401 для его api_key | HTTP 204 + 401 |
| AC-009 | Таблицы profiles/dictionary_entries удалены | SQL check |

## Данные и контракты

- См. `data-model.md`.
- **REST API:** POST/PUT/GET/DELETE /api/v1/admin/tenants, GET/PUT /api/v1/admin/tenants/:slug/dictionaries
- **Удаляется:** GET/POST/PUT/DELETE /api/v1/profiles, GET /api/v1/profiles/:slug
- **Удаляется:** заголовок X-Shield-Profile-Slug
- **Incidents:** profile_slug → tenant_slug, UPDATE при миграции

## Стратегия реализации

### DEC-001 TenantResolver — DB first, config fallback
- Why: config.yaml — дефолт для разработки, БД — source of truth в production. Startup: config upsert в БД. Runtime: читает БД. Если БД недоступна — 500.
- Tradeoff: Единая точка отказа — БД. Config как fallback при недоступности БД не используется (безопасность).
- Affects: auth middleware, startup sync, resolver impl
- Validation: AC-002, AC-003, AC-004

### DEC-002 DictionaryCache — rename ProfileCache, same architecture
- Why: ProfileCache (LRU, valkey, pubsub, warm) работает и покрыт тестами. Только rename + key profile_slug → tenant_slug.
- Tradeoff: rename требует обновления импортов, метрик, конфига.
- Affects: repository/profile/ → repository/dictionary/, config.go, metrics.go, gateway/main.go, admin/main.go
- Validation: go build + go test

### DEC-003 Миграция 006 — данные сохраняются
- Why: incidents.profile_slug FK ломается при удалении profiles. UPDATE incidents SET tenant_slug = profile_slug, DROP FK, DROP tables.
- Tradeoff: Откат восстанавливает таблицы, но данные profiles не сохраняются.
- Affects: migrations/006_cleanup.up.sql, incidents таблица
- Validation: AC-009

### DEC-004 Tenant entity — data holder
- Why: Tenant — простая data holder с геттерами. Dictionaries как []*dictionary.Dictionary (существующий domain-тип).
- Tradeoff: Нет сложной логики, только CRUD.
- Affects: entity/tenant.go, repository/tenant.go, tenant_handler.go
- Validation: AC-001

## Incremental Delivery

### MVP (первые 8 шагов)

1. Tenant entity + TenantSlug VO + миграция 005 + TenantRepository + TenantResolver
2. Auth middleware → TenantResolver
3. Tenant admin CRUD handler + routes
4. Shield middleware → tenant dictionaries, убрать X-Shield-Profile-Slug
5. ProfileCache → DictionaryCache
6. Миграция 006 (cleanup)
7. Удаление Profile entity, handler, repo, API
8. Dictionaries endpoint PUT /api/v1/admin/tenants/:slug/dictionaries

### Итеративное расширение

9. Examples: config.yaml, seed-profile.sh, test-prompt.md

## Порядок реализации

1. **Шаг 1 (фундамент):** Tenant entity + VO + миграция 005 + TenantRepository + TenantResolver
2. **Шаг 2 (auth):** Auth middleware → TenantResolver
3. **Шаг 3 (crud):** Tenant admin CRUD handler + routes (включая dictionaries endpoint)
4. **Шаг 4 (shield):** Shield middleware → tenant dictionaries, убрать X-Shield-Profile-Slug
5. **Шаг 5 (cache):** ProfileCache → DictionaryCache
6. **Шаг 6 (cleanup):** Миграция 006 + удаление Profile из кода
7. **Шаг 7 (examples):** config.yaml, seed-profile.sh, test-prompt.md

Шаги 5 и 6 можно параллелить.

## Риски

- **Риск:** TenantResolver кеширует tenant'ов в памяти, CRUD через admin API не виден gateway до перезапуска.
  - Mitigation: DictionaryCache уже имеет pubsub-инвалидацию. TenantResolver должен тоже триггерить инвалидацию при PUT/DELETE.
- **Риск:** Миграция 006 удаляет таблицы, к которым может быть привязан старый код.
  - Mitigation: Порядок: сперва код переключается на Tenant, потом миграция 006. Если код ещё не обновлён — миграция 006 не выполняется.

## Rollout и compatibility

- **Миграция 005** (CREATE TABLE tenants) — безопасна на любой стадии.
- **Код переключается на TenantResolver** — cfg.Tenants всё ещё работает как fallback.
- **Shield middleware** — старый header X-Shield-Profile-Slug игнорируется (backward compat).
- **Миграция 006** (DROP tables) — последний шаг, только после удаления всего кода, который их использует.
- Специальных rollout-действий не требуется.

## Проверка

- `go test ./... -count=1` — юнит + интеграционные тесты
- `go build ./...` — компиляция
- `docker compose up -d --build` + mask/unmask + shield scan — acceptance проверка
- AC-009 проверяется SQL-запросом к information_schema

## Соответствие конституции

- Нет конфликтов.
