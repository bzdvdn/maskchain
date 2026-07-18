# Tenant & Profile Config-DB Sync

## Scope Snapshot

- **In scope:** Tenants — единая сущность, содержащая auth config и dictionaries (JSONB). Profile как отдельная сущность удалён. Tenants управляются через admin API (DB — source of truth) и seed YAML-файлы (GitOps). Startup sync blocking: Config → DB (не перезатирает). Dictionaries endpoint для CI/CD job.
- **Out of scope:** K8s operator/controller как отдельный бинарник — только spec-формат и механизм sync, на котором оператор сможет строиться потом.

## Цель

Администратор управляет tenant'ами через admin API. В каждом tenant — auth (api_keys) и dictionaries (JSONB). CI/CD job обновляет dictionaries через отдельный endpoint, не рискуя затереть auth. Profile как сущность удалён, всё в одном JSONB-поле tenant.

## Основной сценарий

1. Tenant определён в config.yaml (дефолтный auth + dictionaries для локальной разработки).
2. При старте gateway/admin: blocking sync config tenant → БД (upsert, не перезатирает существующие записи с тем же slug).
3. Администратор создаёт tenant через `POST /api/v1/admin/tenants` с `slug`, `name`, `auth.api_keys`, `dictionaries[]`.
4. CI/CD job обновляет dictionaries через `PUT /api/v1/admin/tenants/:slug/dictionaries`, не трогая auth.
5. Gateway shield scan: auth middleware резолвит tenant из API key → читает tenant.dictionaries напрямую (один запрос JSONB).
6. Profile как сущность отсутствует: таблицы `profiles` + `dictionary_entries` удалены, `incidents.tenant_slug FK → tenants(slug)`.

## MVP Slice

Весь scope — MVP. Tenant + dictionaries, удаление Profile, dictionaries endpoint.

## First Deployable Outcome

После первого implementation pass:
- Таблица `tenants` с JSONB-полем `dictionaries` создана миграцией 005
- Миграции `001_profiles`, `002_dictionary_entries` удалены (или оставлены пустыми для обратной совместимости)
- Admin CRUD: `GET/POST/PUT/DELETE /api/v1/admin/tenants`
- Dictionary endpoint: `GET/PUT /api/v1/admin/tenants/:slug/dictionaries`
- Auth gateway middleware резолвит tenant из БД (с fallback на config)
- Shield scan читает dictionaries из tenant.dictionaries JSONB
- Startup sync (blocking): config → БД, не перезатирает существующие
- DictionaryCache (взамен ProfileCache) — кеш dictionaries по tenant_slug

## Scope

- Миграция `005_tenants` — таблица `tenants` с `dictionaries JSONB NOT NULL DEFAULT '[]'`, `auth_api_keys JSONB`, `auth_header TEXT`
- Миграция `006_cleanup` — удаление таблиц `profiles`, `dictionary_entries`, обновление FK `incidents.profile_slug → incidents.tenant_slug`
- Tenant repository: `List`, `Get(slug)`, `Create`, `Update`, `Delete`, `UpdateDictionaries(slug)`
- Admin API: `GET/POST/PUT/DELETE /api/v1/admin/tenants`
- Dictionaries API: `GET /api/v1/admin/tenants/:slug/dictionaries`, `PUT /api/v1/admin/tenants/:slug/dictionaries`
- Tenant resolver: объединяет tenants из DB + config, DB приоритет
- Auth middleware gateway/admin переключена на resolver
- Startup sync (blocking): config → БД, не перезатирает
- DictionaryCache (кеш по tenant_slug, warm on startup)
- Удаление Profile usecase, repository, handler, routes
- Удаление `X-Shield-Profile-Slug` (profile_slug из tenant)

## Контекст

- Сейчас tenants живут только в config.yaml (загружаются при старте, не мигрируют)
- Профили живут в БД (profiles + dictionary_entries), создаются через admin API
- Profile — прослойка, которая не нужна: tenant → profile_slug → dictionaries. Проще tenant → dictionaries напрямую
- ProfileCache уже есть, переименовывается в DictionaryCache (та же логика, другой ключ)
- Embed-миграции уже работают (golang-migrate + iofs)
- `incidents` таблица ссылается на `profiles(slug)` — нужно обновить FK

## Зависимости

- Миграция `005_tenants` — после `004_mask_entries`
- Миграция `006_cleanup` — после `005_tenants`
- DictionaryCache заменяет ProfileCache (тот же интерфейс кеша, другой ключ: tenant_slug вместо profile_slug)
- Shield scan middleware переключается с profile_slug → tenant.dictionaries

## Требования

### RQ-001 Tenants table migration
Система ДОЛЖНА создать таблицу `tenants` с полями: `slug TEXT PK`, `name TEXT NOT NULL`, `auth_header TEXT NOT NULL DEFAULT 'Authorization'`, `api_keys JSONB NOT NULL DEFAULT '[]'`, `dictionaries JSONB NOT NULL DEFAULT '[]'`, `created_at TIMESTAMPTZ`, `updated_at TIMESTAMPTZ`.
Формат `dictionaries`: массив объектов `{"name":"...", "match_mode":"exact|contains|regex|fuzzy", "entries": [...]}`. Каждый entry — string (плоское значение) или object (структурированное, `AllValues()` извлекает все string-значения).

### RQ-002 Tenant repository
Система ДОЛЖНА предоставить postgres-репозиторий с методами: `List`, `Get(slug)`, `Create`, `Update`, `Delete`, `GetDictionaries(slug)`, `UpdateDictionaries(slug, dictionaries)`. `Create` возвращает ошибку при дубликате slug. `Delete` возвращает `ErrNotFound` если tenant не существует.

### RQ-003 Tenant admin CRUD API
Система ДОЛЖНА предоставить REST API `/api/v1/admin/tenants` с методами `GET` (list all), `POST` (create), `PUT /:slug` (update), `DELETE /:slug` (delete). Все методы аутентифицированы admin-токеном.
При `POST`/`PUT` поле `dictionaries` валидируется: каждый entry — string или object. При object — разрешены любые ключи (не валидируется схема).

### RQ-004 Tenant dictionaries API
Система ДОЛЖНА предоставить:
- `GET /api/v1/admin/tenants/:slug/dictionaries` — возвращает текущий JSON dictionaries (без auth-полей)
- `PUT /api/v1/admin/tenants/:slug/dictionaries` — заменяет dictionaries (тело — JSON, массив как в RQ-001). Не трогает auth-поля tenant'а.
Оба метода аутентифицированы admin-токеном.

### RQ-005 Tenant resolver (DB + config, DB priority)
Система ДОЛЖНА предоставить resolver, который объединяет tenants из БД и config. При конфликте slug побеждает запись из БД. Resolver используется auth middleware вместо прямого `cfg.Tenants`.

### RQ-005a Remove X-Shield-Profile-Slug header
Система ДОЛЖНА определять dictionaries tenant'а без заголовка `X-Shield-Profile-Slug`. Auth middleware после валидации API-Key находит tenant, читает `dictionaries` из tenant.Dictionaries и прокидывает в контекст запроса. Shield middleware использует dictionaries напрямую (без profile).

### RQ-006 Auth middleware uses resolver
Gateway и admin auth middleware ДОЛЖНЫ получать tenants через resolver, а не из `cfg.Tenents`. Валидация API-ключа не меняется.

### RQ-007 Startup sync: config → DB
При старте gateway/admin ДОЛЖНЫ upsert tenants из config в БД, НО НЕ перезатирать существующие записи с тем же slug. Это позволяет config.yaml задавать "базовые" tenants, не удаляя созданные через API.

### RQ-008 Cleanup migration: drop profiles + dictionary_entries
Система ДОЛЖНА удалить таблицы `profiles` и `dictionary_entries` (миграция 006), обновить FK `incidents.profile_slug → incidents.tenant_slug`. Profile usecase, repository, handler, routes — удалить из кода. ProfileCache → DictionaryCache (тот же интерфейс, ключ tenant_slug).

## Вне scope

- K8s operator/controller как отдельный бинарник (будет отдельная spec)
- RBAC/роли для tenant CRUD (только admin-токен, без fine-grained permissions)
- UI для tenants (React) — только backend API
- Soft-delete для tenants (физический DELETE)
- Bulk-import/export tenants (только single-resource CRUD)
- Rate limiting per tenant
- Audit log для изменений tenants
- Webhook-уведомления при изменениях tenant
- Версионирование dictionaries (перезапись целиком, без diff/history)

## Критерии приемки

### AC-001 Tenant created via admin API with dictionaries
- **Given** postgres доступен и миграция выполнена, admin API запущен
- **When** отправлен `POST /api/v1/admin/tenants` с JSON-телом `{"slug":"acme","name":"Acme Corp","auth_header":"Authorization","api_keys":["sk-acme"],"dictionaries":[{"name":"users","match_mode":"exact","entries":[{"name":"John","email":"john@acme.com"}]}]}`
- **Then** возвращается `201 Created`; `GET /api/v1/admin/tenants` возвращает tenant с dictionaries
- Evidence: HTTP 201 + dictionaries сохранены в JSONB, entries доступны через shield scan

### AC-002 Tenant from config.yaml accessible via resolver
- **Given** config.yaml содержит `tenants.default` с api_key `sk-test-default` и dictionaries
- **When** вызывается `tenantResolver.List(ctx)`
- **Then** в результате присутствует tenant `default` с полями из config
- Evidence: resolver возвращает tenant slug=default с корректными api_keys и dictionaries

### AC-003 DB tenant overrides config tenant with same slug
- **Given** config.yaml содержит `tenants.default.dictionaries[0].name: "from-config"`, а в БД есть `tenants.default.dictionaries[0].name: "from-db"`
- **When** вызывается `tenantResolver.Get(ctx, "default")`
- **Then** возвращается tenant из БД (dictionaries.name = "from-db")
- Evidence: resolver возвращает DB-значение

### AC-004 Startup sync does not overwrite DB tenants
- **Given** в БД есть tenant `manual` (создан через API), в config.yaml его нет
- **When** gateway стартует с sync-процедурой
- **Then** tenant `manual` остаётся в БД без изменений
- Evidence: GET /api/v1/admin/tenants показывает `manual` после рестарта

### AC-005 Tenant dictionaries endpoint replaces only dictionaries
- **Given** tenant `acme` c `api_keys: ["sk-acme"]` и dictionaries `[{"name":"old","entries":["a"]}]`
- **When** отправлен `PUT /api/v1/admin/tenants/acme/dictionaries` с телом `[{"name":"new","entries":["b"]}]`
- **Then** `GET /api/v1/admin/tenants/acme/dictionaries` возвращает новые dictionaries; `api_keys` tenant'а не изменились (`sk-acme`)
- Evidence: HTTP 200 + dictionaries обновлены, auth работает с тем же ключом

### AC-006 Shield scan reads dictionaries from tenant
- **Given** tenant `acme` имеет dictionaries с entry "John"
- **When** отправлен `POST /v1/chat/completions` c `Authorization: Bearer <acme-api-key>` без `X-Shield-Profile-Slug` и с текстом "hello John"
- **Then** shield обнаруживает совпадение и блокирует (403)
- Evidence: HTTP 403 с X-Shield-Status: blocked, incident создан

### AC-007 Shield scan without dictionaries = no dictionary match
- **Given** tenant `empty` с пустыми dictionaries `[]`
- **When** отправлен `POST /v1/chat/completions` c `Authorization: Bearer <empty-api-key>`
- **Then** shield НЕ блокирует по dictionary (502/continue — провайдера нет, но это не ошибка shield)
- Evidence: X-Shield-Status без dictionary-детектов (только regex, если есть)

### AC-008 Tenant delete via admin API removes from resolver
- **Given** tenant `test` существует в БД
- **When** отправлен `DELETE /api/v1/admin/tenants/test`
- **Then** возвращается `204 No Content`; tenant не виден в resolver; auth с api_key этого tenant возвращает 401
- Evidence: HTTP 204 + GET /api/v1/admin/tenants не содержит `test`

### AC-009 Cleanup: profiles + dictionary_entries tables removed
- **Given** миграция 006 выполнена
- **When** проверяется схема БД
- **Then** таблицы `profiles` и `dictionary_entries` отсутствуют; `incidents` имеет FK на `tenants(slug)`
- Evidence: SQL-запрос к information_schema подтверждает

## Допущения

- Все gateway-инстансы имеют доступ к той же БД postgres (единая БД)
- Валидация api_key tenants не меняется — только источник данных (БД вместо config)
- Config-файл при старте читается viper'ом, tenants из config загружаются в viper до sync
- DictionaryCache работает как ProfileCache: in-memory LRU + valkey TTL
- Миграция 006 (cleanup) должна быть идемпотентной (IF EXISTS)
- Gateway НЕ обрабатывает tenant CRUD — это только admin API

## Критерии успеха

- SC-001 Tenant CRUD: создание через API доступно сразу после миграции, без перезапуска
- SC-002 Обновление tenants не требует перезапуска gateway/admin (runtime CRUD)
- SC-003 Tenant dictionaries update: PUT 10k entries → БД за <1s

## Краевые случаи

- Config.yaml не содержит tenants (nil) — resolver работает только по БД; при пустой БД auth возвращает 401
- БД недоступна — resolver падает, auth middleware возвращает 500
- Slug tenant превышает лимит длины — HTTP 400
- Dictionaries entry не string и не object — HTTP 400
- Tenant delete с активными сессиями — не блокируется (следующий запрос получит 401)
- Concurrency: POST /api/v1/admin/tenants с тем же slug — второй 409
- PUT /api/v1/admin/tenants/:slug/dictionaries с невалидным JSON — HTTP 400
- Tenant не существует, PUT /api/v1/admin/tenants/:slug/dictionaries — HTTP 404

## Открытые вопросы

1. DictionaryCache TTL и размер LRU — те же дефолты, что в ProfileCache (300s, 10k)? Или менять?

### Решено

- **Profile удалён.** Dictionaries живут в `tenants.dictionaries JSONB`.
- **Dictionaries endpoint:** `GET/PUT /api/v1/admin/tenants/:slug/dictionaries` для CI/CD job.
- **X-Shield-Profile-Slug удалён.** Shield читает dictionaries из tenant напрямую.
- **Startup sync:** Blocking. Config → БД, не перезатирает существующие.
- **Gateway не управляет tenant'ами.** Profile CRUD только admin API.
- **base64 не нужен.** JSONB хранит как есть, YAML-манифесты с inline JSON.
