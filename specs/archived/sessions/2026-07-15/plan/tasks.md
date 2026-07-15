# Sessions Задачи

## Phase Contract

Inputs: plan, data-model, spec.
Outputs: tasks с фазами, Touches, Surface Map, AC coverage.
Stop if: нет — plan чёткий, data-model определён.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/domain/session/` (new) | T1.1, T1.2, T2.4, T3.3, T6.2 |
| `src/internal/domain/session/entity.go` | T1.1 |
| `src/internal/domain/session/errors.go` | T1.1 |
| `src/internal/domain/session/storage.go` | T1.1 |
| `src/internal/domain/session/usecase.go` | T1.2 |
| `src/internal/infra/config/config.go` | T1.3 |
| `src/internal/adapters/repository/postgres/migrations/` | T1.3 |
| `src/internal/adapters/repository/session/postgres.go` | T2.1 |
| `src/internal/adapters/repository/session/valkey.go` | T3.1 |
| `src/internal/adapters/repository/session/cached.go` | T3.2 |
| `src/internal/adapters/repository/session/` (new) | T2.1, T3.1, T3.2, T3.3, T6.2 |
| `src/internal/api/session_handler.go` | T2.2 |
| `src/internal/api/admin.go` | T2.3 |
| `src/internal/api/middleware/session.go` | T4.1 |
| `src/internal/api/middleware/shield.go` | T4.2 |
| `src/internal/api/server.go` | T4.1 |
| `src/internal/app/worker/session_cleanup.go` | T5.1 |
| `specs/active/sessions/openapi.yaml` (new) | T6.1 |

## Implementation Context

- **Цель MVP:** Domain (entity + port + use case) + PostgresSessionStore + REST API (create/list/get/close/extend) на AdminServer.
- **Инварианты/семантика:**
  - SessionID — UUIDv7 (RFC 9562), генерируется middleware, передаётся в use case.
  - `Status` ∈ {`active`, `expired`, `closed`}. После `expired`/`closed` — update запрещён (кроме DeleteExpired).
  - IncrementCounts — атомарный `UPDATE col = col + $1` в PG, `INCRBY` в Valkey.
  - Tenant-scoped: все запросы фильтруются по `TenantID` из контекста.
  - Valkey fail-open: при ошибке Valkey — WARN + работа через PG.
- **Ошибки/коды:**
  - `ErrSessionNotFound` → 404
  - `ErrSessionExpired` → 410 Gone
  - `ErrSessionClosed` → 409 Conflict (update на closed)
  - `ErrSessionConflict` → 409 Conflict (create с существующим ID)
- **Контракты/протокол:**
  - `POST /api/v1/sessions` → body `{"model":"string"}` → 201 `{session_id, tenant_id, model, status, created_at, expires_at}`
  - `GET /api/v1/sessions/:id` → 200 (full entity)
  - `GET /api/v1/sessions?page=&limit=` → 200 `{items[], total, page, limit}`
  - `PATCH /api/v1/sessions/:id/extend` → body `{"ttl_seconds":int}` → 200 `{session_id, expires_at, status}`
  - `DELETE /api/v1/sessions/:id` → 200 `{status:"closed"}`
- **Proof signals:** HTTP response codes + body; direct PG query; log-level WARN для Valkey fail-open; regex UUIDv7 check.
- **Границы scope:** Нет UI. Нет Envoy. Нет аналитики/агрегации. Нет unmarshal (покрыто MaskEntry). ReplacementMap — не входит.
- **References:** DEC-001 (CachedSessionStore), DEC-002 (SessionID generation), DEC-003 (CleanupWorker opt-in), DEC-004 (Shield integration), DM-001 (Session entity).

## Фаза 1: Основа

Цель: domain-слой, конфиг, миграция — база, от которой зависят все остальные фазы.

- [x] T1.1 Создать domain-пакет `src/internal/domain/session/`: Session entity, SessionID value object (UUIDv7 via `mask.NewUUIDv7()`), ошибки (`ErrSessionNotFound`, `ErrSessionExpired`, `ErrSessionClosed`, `ErrSessionConflict`), SessionStore port interface (`Save`, `Get`, `IncrementCounts`, `ExtendTTL`, `Close`, `DeleteExpired`, `ListByTenant`).
  Touches: `src/internal/domain/session/entity.go`, `src/internal/domain/session/errors.go`, `src/internal/domain/session/storage.go`
  AC: 001, 009
  DEC: 002

- [x] T1.2 Реализовать SessionUseCase: `Create(ctx, sessionID, tenantID, model, ttl)`, `Get(ctx, tenantID, sessionID)`, `IncrementCounts(ctx, tenantID, sessionID, tokens, messages, totalMasks, dictMaskCount, piiMaskCount, preprocessorCount)`, `ExtendTTL(ctx, tenantID, sessionID, ttlSeconds)`, `Close(ctx, tenantID, sessionID)`, `ListByTenant(ctx, tenantID, page, limit)`, `DeleteExpired(ctx)`. Все методы проверяют tenant-scope, статус, и возвращают доменные ошибки.
  Touches: `src/internal/domain/session/usecase.go`
  AC: 001, 002, 003, 004, 005, 006, 007
  DEC: 002

- [x] T1.3 Добавить `SessionConfig` в `src/internal/infra/config/config.go`: `DefaultTTL` (default 30m), `MaxTTL` (24h), `CleanupInterval` (5m), `CleanupEnabled` (false), `CacheTTL` (5m). Создать миграцию `010_sessions.up.sql` (CREATE TABLE sessions) и `010_sessions.down.sql` (DROP TABLE).
  Touches: `src/internal/infra/config/config.go`, `src/internal/adapters/repository/postgres/migrations/010_sessions.up.sql`, `src/internal/adapters/repository/postgres/migrations/010_sessions.down.sql`
  AC: 001, 009

## Фаза 2: MVP

Цель: PostgresSessionStore + REST API на AdminServer — минимальная end-to-end ценность.

- [x] T2.1 Реализовать `PostgresSessionStore` — имплементация SessionStore на pgxpool. `Save` → INSERT ON CONFLICT DO NOTHING → ErrSessionConflict. `Get` → SELECT. `IncrementCounts` → UPDATE SET col = col + $1 ... RETURNING. `ExtendTTL` → UPDATE expires_at, status check. `Close` → UPDATE status = 'closed'. `ListByTenant` → SELECT + COUNT + pagination (LIMIT/OFFSET). `DeleteExpired` → DELETE WHERE status='expired' OR expires_at < NOW().
  Touches: `src/internal/adapters/repository/session/postgres.go`
  AC: 001, 002, 003, 004, 005, 006, 007, 008
  DEC: 001

- [x] T2.2 Реализовать `SessionHandler` в `session_handler.go`: хендлеры для POST, GET:id, GET list, PATCH:extend, DELETE:id. Все получают `TenantID` из gin context. Tenant-scoped фильтрация в use case. Response в JSON.
  Touches: `src/internal/api/session_handler.go`
  AC: 001, 003, 004, 005, 006

- [x] T2.3 Зарегистрировать SessionHandler на AdminServer: добавить `RegisterSessionHandler(h)` (или передать через конструктор). SessionHandler использует SessionUseCase + SessionConfig. Пробросить зависимости в `cmd/admin/main.go`.
  Touches: `src/internal/api/admin.go`, `src/cmd/admin/main.go`
  AC: 001

- [x] T2.4 Написать unit-тесты для entity, use case, port. Integration test для PostgresSessionStore. Handler test через httptest с mocked use case. Проверить MVP curl-ами.
  Touches: `src/internal/domain/session/*_test.go`, `src/internal/adapters/repository/session/postgres_test.go`, `src/internal/api/session_handler_test.go`
  AC: 001, 003, 004, 005, 006, 009

## Фаза 3: Кэш

Цель: Valkey-кэш + декоратор CachedSessionStore с graceful degradation.

- [x] T3.1 Реализовать `ValkeySessionCache` — имплементация SessionStore для Valkey. `Save` → SET EX. `Get` → GET (valkey.Nil → ErrSessionNotFound). `DeleteExpired` → SCAN + DEL (best-effort). Остальные методы — возвращают ErrSessionNotFound (read-through заполнит из PG).
  Touches: `src/internal/adapters/repository/session/valkey.go`
  AC: 008
  DEC: 001

- [x] T3.2 Реализовать `CachedSessionStore` — декоратор над primary (PG) и secondary (Valkey). `Save` → sync PG + best-effort Valkey. `Get` → cache-first (Valkey), miss → PG → backfill Valkey. `IncrementCounts` → sync PG + best-effort Valkey (SET обновлённого объекта). Graceful degradation: ошибка Valkey → log.WARN + работа через PG.
  Touches: `src/internal/adapters/repository/session/cached.go`
  AC: 008
  DEC: 001

- [x] T3.3 Написать integration test для ValkeySessionCache. Test graceful degradation: nil Valkey client → CachedSessionStore не падает, пишет в PG, WARN в логе.
  Touches: `src/internal/adapters/repository/session/valkey_test.go`, `src/internal/adapters/repository/session/cached_test.go`
  AC: 008

## Фаза 4: Middleware + Shield Integration

Цель: автоматическое создание сессии из `X-Session-ID` в gateway и инкремент счётчиков после shield scan.

- [x] T4.1 Реализовать `SessionMiddleware` — читает `X-Session-ID` из заголовка запроса, вызывает SessionUseCase.Get (если сессия существует) или SessionUseCase.Create (если нет). Кладёт Session в gin context. Пробрасывает `X-Session-ID` в response. Зарегистрировать на gateway Server.
  Touches: `src/internal/api/middleware/session.go`, `src/internal/api/server.go`, `src/cmd/gateway/main.go`
  AC: 010
  DEC: 002

- [x] T4.2 Интегрировать инкремент счётчиков в ShieldMiddleware: после shield scan (но до unmask), если session есть в gin context, подсчитать количество масок по типам (dictMaskCount = len(dictMaskMapping), piiMaskCount = количество PII-replacements), общее totalMasks = сумма. Вызвать SessionUseCase.IncrementCounts. Fail-open: ошибка store → log.WARN, не блокировать запрос.
  Touches: `src/internal/api/middleware/shield.go`
  AC: 002
  DEC: 004

- [x] T4.3 Написать unit test для SessionMiddleware (httptest, mocked use case). Integration test для ShieldMiddleware + Session (сквозной сценарий).
  Touches: `src/internal/api/middleware/session_test.go`, `src/internal/api/middleware/shield_test.go`
  AC: 002, 010

## Фаза 5: CleanupWorker

Цель: фоновый garbage collector для expired сессий.

- [x] T5.1 Реализовать `CleanupWorker` — запускается в отдельной goroutine, по таймеру вызывает `SessionUseCase.DeleteExpired()`. Логирует количество удалённых записей. Graceful shutdown через context cancellation. Не стартует если конфиг `session.cleanup.enabled=false`.
  Touches: `src/internal/app/worker/session_cleanup.go`
  AC: 007
  DEC: 003

- [x] T5.2 Подключить CleanupWorker в `cmd/admin/main.go` (и опционально в `cmd/gateway/main.go`). Проверить что не стартует при `cleanup.enabled=false`.
  Touches: `src/cmd/admin/main.go`, `src/cmd/gateway/main.go`
  AC: 007
  DEC: 003

- [x] T5.3 Написать unit test для CleanupWorker: mocked store + time control. Проверить что DeleteExpired вызывается с правильным интервалом и логирует результат.
  Touches: `src/internal/app/worker/session_cleanup_test.go`
  AC: 007

- [x] T6.1 Добавить OpenAPI spec `specs/active/sessions/openapi.yaml` со всеми endpoints, request/response схемами, описание tenant-scoped headers. Привязать к Swagger UI на AdminServer.
  Touches: `specs/active/sessions/openapi.yaml`, `src/internal/api/admin.go`
  AC: 001, 003, 004, 005, 006

- [x] T6.2 Выполнить `@sk-task` trace-маркеры над каждой owning declaration в новом коде. Проверить full-cycle: миграция → создание сессии → список → extend → close → cleanup. Проверить AC coverage — все 10 AC замкнуты задачами.
  Touches: все поверхности
  AC: все

## Покрытие критериев приемки

- AC-001 -> T1.1, T1.3, T2.1, T2.2, T2.3, T2.4, T6.1
- AC-002 -> T1.2, T2.1, T4.2, T4.3
- AC-003 -> T1.2, T2.1, T2.2, T2.4, T6.1
- AC-004 -> T1.2, T2.1, T2.2, T2.4, T6.1
- AC-005 -> T1.2, T2.1, T2.2, T2.4, T6.1
- AC-006 -> T1.2, T2.1, T2.2, T2.4, T6.1
- AC-007 -> T1.2, T2.1, T5.1, T5.2, T5.3
- AC-008 -> T2.1, T3.1, T3.2, T3.3
- AC-009 -> T1.1, T2.4
- AC-010 -> T1.2, T4.1, T4.3
