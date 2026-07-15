# Sessions — Session Tracking & Statistics for Shield

## Scope Snapshot

- In scope: domain entity, value objects, store port, Postgres+Valkey repositories, REST API, and cleanup worker for tracking dialogs (sessions) in Content Shield.
- Out of scope: UI for session management, Envoy data plane handling, cross-tenant session sharing, batch export, ReplacementMap/unmarshal (уже покрыто MaskEntry).

## Цель

Оператору (tenant owner) нужно понимать сколько сессий (диалогов) было, каким тенантом, какой моделью, сколько сообщений и токенов потрачено, а также сколько маскировок каждого типа сработало: словарных, PII, через препроцессоры. Shield middleware создаёт сессию при первом запросе с `X-Session-ID` и обновляет счётчики при каждом последующем. Tenant может закрыть сессию явно или дождаться TTL. CleanupWorker удаляет истекшие сессии. Это даёт базовую статистику использования без аналитического пайплайна.

## Основной сценарий

1. Клиент отправляет запрос с `X-Session-ID` (или новый диалог без него). Shield middleware создаёт сессию при первом обращении, привязывая `TenantID`, `Model`, устанавливая TTL.
2. При каждом последующем запросе с тем же `X-Session-ID` middleware обновляет `TokenCount`, `MessageCount`, а также инкрементирует счётчики маскировок по типам: `DictMaskCount`, `PIIMaskCount`, `PreprocessorCount`.
3. Оператор через REST API смотрит список активных сессий, метаданные конкретной сессии, продлевает TTL или закрывает.
4. По истечении TTL сессия переходит в `expired`. CleanupWorker в фоне удаляет expired-записи.
5. Данные сессий используются для ответа на вопросы: «сколько сессий у tenant-alpha за сегодня?», «какие модели используются чаще всего?».

## User Stories

- P1 Story: Оператор видит список сессий с tenant, моделью, статусом, количеством сообщений, токенов и маскировок по типам (словари/PII/препроцессоры). Может закрыть сессию или продлить TTL.
- P2 Story: CleanupWorker автоматически удаляет истекшие сессии. Middleware корректно обновляет счётчики для существующей сессии.

## MVP Slice

P1: Session entity + SessionStore port + PostgresSessionStore + ValkeySessionCache + REST API create/list/get/extend/close. CleanupWorker — P2.

## First Deployable Outcome

`POST /api/v1/sessions` возвращает `session_id`. Последующие запросы через shield middleware с тем же `X-Session-ID` увеличивают `message_count` и `token_count`. `GET /api/v1/sessions/:id` показывает актуальную статистику.

## Scope

- `Session` entity: `SessionID`, `TenantID`, `Model`, `TokenCount`, `MessageCount`, `TotalMasks`, `DictMaskCount`, `PIIMaskCount`, `PreprocessorCount`, `Status` (active/expired/closed), `TTL`, `CreatedAt`, `ExpiresAt` в `src/internal/domain/session/`
- `SessionID` value object (UUIDv7)
- `SessionStore` port interface: `Save`, `Get`, `IncrementCounts`, `ExtendTTL`, `Close`, `DeleteExpired`, `ListByTenant`
- `SessionUseCase` — create, extend TTL, close, increment counts, list, delete expired
- `PostgresSessionStore` — CRUD + атомарный `UPDATE token_count += $1, message_count += $1, total_masks += $1, dict_mask_count += $1, pii_mask_count += $1, preprocessor_count += $1`
- `ValkeySessionCache` — TTL-based кэш с read-through/write-through (fail-open)
- `CleanupWorker` — фоновый interval-based garbage collector для expired сессий
- REST API: `POST/GET /api/v1/sessions`, `GET /api/v1/sessions/:id`, `PATCH .../extend`, `DELETE .../id`
- Tenant-scoped: middleware извлекает `TenantID` из контекста и привязывает сессию к тенанту
- Миграция: `CREATE TABLE sessions` + индексы по `tenant_id`, `status`, `expires_at`; колонки: `id UUID`, `tenant_id TEXT`, `model TEXT`, `token_count BIGINT`, `message_count INT`, `total_masks INT`, `dict_mask_count INT`, `pii_mask_count INT`, `preprocessor_count INT`, `status TEXT`, `ttl INTERVAL`, `created_at TIMESTAMPTZ`, `expires_at TIMESTAMPTZ`
- Graceful degradation: Valkey недоступен — работа через PG (fail-open, log warning)
- Middleware: чтение `X-Session-ID` из запроса, создание/обновление сессии, проброс `X-Session-ID` в ответ

## Контекст

- Tenant isolation уже работает через middleware (`middleware.TenantFromContext`). Сессии наследуют tenant-scoping.
- UUIDv7 уже используется в `mask/uuid.go` — переиспользовать генерацию для SessionID.
- Репозитории Postgres+Valkey уже есть в `src/internal/adapters/repository/mask/` — следовать тем же паттернам.
- MaskEntry уже хранит карту замен и unmarshal — сессии не дублируют эту функцию.
- Shield middleware уже имеет точку входа для интеграции (до/после сканирования).

## Зависимости

- 20-shield-domain — существующий MaskEntry, MaskUseCase
- 80-tenant-isolation — TenantID, tenant middleware, tenant-scoped доступ
- 30-shield-persistence — существующие Postgres/Valkey connection pool, миграции
- 01-config-bootstrap — конфигурация TTL, cleanup interval, Valkey addr

## Требования

- RQ-001 Система ДОЛЖНА создавать сессию с уникальным SessionID (UUIDv7), TenantID, и TTL при первом запросе с `X-Session-ID`.
- RQ-002 Система ДОЛЖНА атомарно увеличивать счётчики `TokenCount`, `MessageCount`, `TotalMasks`, `DictMaskCount`, `PIIMaskCount`, `PreprocessorCount` при каждом запросе в рамках сессии.
- RQ-003 Tenant ДОЛЖЕН получать сессии только своего тенанта (tenant-scoped).
- RQ-004 Session ID ДОЛЖЕН пробрасываться в ответ как `X-Session-ID` header.
- RQ-005 CleanupWorker ДОЛЖЕН удалять сессии со статусом `expired` и `expires_at < NOW()` с настраиваемым интервалом.
- RQ-006 При недоступности Valkey система ДОЛЖНА продолжать работу через PostgreSQL (fail-open) с логированием предупреждения.
- RQ-007 API ДОЛЖЕН поддерживать пагинацию для `GET /api/v1/sessions` (page/limit).

## Вне scope

- UI-страницы для просмотра/управления сессиями (только REST API для automation/admin)
- Envoy data plane — сессии обрабатываются только на native Go data plane
- ReplacementMap и unmarshal по SessionID — уже покрыто MaskEntry и unmarshal по mask_id
- Cross-tenant sharing сессий
- Batch-экспорт/импорт сессий
- Rate limiting на session API (существующий ratelimit middleware покрывает)
- Аналитика/агрегация (Group 14: Analytics)

## Критерии приемки

### AC-001 Создание сессии с UUIDv7 и TenantID

- Почему это важно: гарантирует уникальность и tenant-изоляцию сессии с первой операции.
- **Given** пустая БД и tenant context с `TenantID = "tenant-alpha"`
- **When** POST /api/v1/sessions выполнен с валидным телом (model = "gpt-4")
- **Then** ответ содержит `session_id` (UUIDv7 format), `tenant_id: "tenant-alpha"`, статус `active`
- Evidence: HTTP 201, JSON body с `session_id`, `tenant_id`, `status`, `created_at`, `expires_at`

### AC-002 Middleware атомарно обновляет все счётчики сессии

- Почему это важно: каждый запрос в диалоге должен точно учитываться со всеми типами маскировок.
- **Given** сессия с `session_id = "0190f3a6-7b8c-7d4e-9f01-23456789abcd"`, `message_count = 1`, `token_count = 150`, `total_masks = 3`, `dict_mask_count = 2`, `pii_mask_count = 1`, `preprocessor_count = 0`
- **When** middleware обрабатывает запрос с тем же `X-Session-ID` и инкрементирует счётчики (`tokens = 50, messages = 1, total_masks = 2, dict_mask_count = 1, pii_mask_count = 1, preprocessor_count = 0`)
- **Then** GET /api/v1/sessions/0190f3a6-7b8c-7d4e-9f01-23456789abcd возвращает `message_count = 2`, `token_count = 200`, `total_masks = 5`, `dict_mask_count = 3`, `pii_mask_count = 2`, `preprocessor_count = 0`
- Evidence: HTTP 200, JSON со всеми обновлёнными счётчиками

### AC-003 Tenant-scoped: тенант не видит чужие сессии

- Почему это важно: изоляция данных между тенантами — non-negotiable.
- **Given** tenant-alpha имеет сессию `abc`, tenant-beta имеет сессию `xyz`
- **When** tenant-beta выполняет GET /api/v1/sessions/abc
- **Then** ответ 404 Not Found (сессия не принадлежит тенанту)
- Evidence: HTTP 404 с ошибкой

### AC-004 Список сессий с пагинацией

- Почему это важно: tenant с тысячами сессий должен иметь возможность листать их.
- **Given** tenant-alpha имеет 25 сессий
- **When** GET /api/v1/sessions?page=1&limit=10
- **Then** ответ содержит 10 сессий, `total: 25`, `page: 1`, `limit: 10`
- Evidence: HTTP 200, JSON с `items`, `total`, `page`, `limit`

### AC-005 Продление TTL сессии

- Почему это важно: длительные диалоги не должны терять контекст до завершения.
- **Given** сессия со статусом `active` и `expires_at = "2026-07-15T12:00:00Z"`
- **When** PATCH /api/v1/sessions/abc/extend с телом `{"ttl_seconds": 3600}`
- **Then** `expires_at` сдвинут на 3600 секунд вперёд, статус остался `active`
- Evidence: HTTP 200, JSON с обновлённым `expires_at`

### AC-006 Закрытие сессии (soft delete)

- Почему это важно: tenant может явно завершить диалог, что помечает сессию как closed до истечения TTL.
- **Given** активная сессия `abc`
- **When** DELETE /api/v1/sessions/abc
- **Then** статус сессии меняется на `closed`, данные сохранены
- Evidence: HTTP 200, GET /api/v1/sessions/abc возвращает `status: "closed"`

### AC-007 CleanupWorker удаляет expired сессии

- Почему это важно: предотвращает рост таблицы sessions неограниченным числом истекших записей.
- **Given** сессия со статусом `expired` и `expires_at < NOW() - 1h`
- **When** CleanupWorker запускается (интервал истёк)
- **Then** сессия удалена из БД
- Evidence: прямой запрос к PG показывает отсутствие записи; GET /api/v1/sessions/abc возвращает 404

### AC-008 Graceful degradation при недоступности Valkey

- Почему это важно: Valkey — кэш, его отсутствие не должно блокировать работу.
- **Given** Valkey недоступен, PostgreSQL доступен
- **When** POST /api/v1/sessions выполнен
- **Then** сессия создана в PostgreSQL (запись читается через PG), запрос в логе содержит warning о недоступности Valkey
- Evidence: HTTP 201 success, лог содержит `level=WARN` с сообщением о недоступности Valkey

### AC-009 SessionID генерируется как UUIDv7

- Почему это важно: UUIDv7 обеспечивает временну́ю упорядоченность и уникальность без централизованного генератора.
- **Given** новый запрос на создание сессии
- **When** сессия создана
- **Then** `session_id` соответствует формату UUIDv7 (RFC 9562)
- Evidence: проверка формата через regex `^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`

### AC-010 Middleware создаёт сессию из X-Session-ID

- Почему это важно: автоматическое создание сессии при первом запросе без ручного вызова API.
- **Given** запрос к gateway с заголовком `X-Session-ID: 0190f3a6-7b8c-7d4e-9f01-23456789abcd` (тенант `tenant-alpha`, модель `gpt-4`)
- **When** middleware обрабатывает запрос
- **Then** создана сессия с `session_id = "0190f3a6-7b8c-7d4e-9f01-23456789abcd"`, `tenant_id = "tenant-alpha"`, `status = "active"`
- Evidence: GET /api/v1/sessions/0190f3a6-7b8c-7d4e-9f01-23456789abcd возвращает сессию с корректными tenant_id и status

## Допущения

- Заголовок `X-Session-ID` передаётся клиентом; если заголовок отсутствует — middleware не создаёт сессию (пропускает).
- TTL по умолчанию — 30 минут, конфигурируется через `session.default_ttl`.
- CleanupInterval по умолчанию — 5 минут.
- PostgreSQL — source of truth; Valkey — ускоряющий кэш, его потеря не критична.
- UUIDv7 генератор из `mask/uuid.go` переиспользуется или выделяется общий `pkg/uuid`.
- TokenCount считается на уровне middleware (из запроса/ответа), не в domain-слое.

## Критерии успеха

- SC-001 Создание сессии (POST) — latency P99 < 50ms (PG only) / < 10ms (с Valkey)
- SC-002 IncrementCounts (все 6 счётчиков) — latency P99 < 50ms
- SC-003 ListSessions с пагинацией (1000 записей) — latency P99 < 100ms
- SC-004 CleanupWorker удаляет > 10k expired записей за один проход без блокировок
- SC-005 Error rate session API < 0.1% после rollout

## Краевые случаи

- Несуществующий SessionID: 404.
- Сессия expired: 410 Gone, create с тем же ID — 409 Conflict.
- Одновременный increment (race condition): атомарный UPDATE в PG; последняя операция побеждает.
- TenantID из middleware отсутствует: 401 Unauthorized.
- Превышение TTL при продлении: capped до `max_ttl` из конфига.
- Пагинация с offset за пределами данных: пустой `items`, корректные `total/page/limit`.
- X-Session-ID без TenantID: 401 (мидлвара аутентификации должна отработать раньше).

## Открытые вопросы

- Должен ли CleanupWorker быть включён по умолчанию или opt-in? По умолчанию выключен, включается через конфиг `session.cleanup.enabled`.
- Нужен ли `PATCH /api/v1/sessions/:id/increment` как публичный endpoint или только внутренний вызов из middleware? Решение: внутренний метод на use case, без публичного endpoint (increment — только из middleware).
