# Sessions План

## Phase Contract

Inputs: spec, inspect (pass), data-model, minimal repo контекст.
Outputs: plan, data-model.
Stop if: нет — spec чёткая, inspect pass.

## Цель

Session tracking как отдельный domain-пакет `src/internal/domain/session/` с портом `SessionStore`, двумя адаптерами (Postgres, Valkey) и декоратором `CachedSessionStore`. REST API на AdminServer (gateway через middleware). CleanupWorker — опциональный фоновый процесс.

## MVP Slice

Domain + PostgresSessionStore + базовый REST API (create/list/get/close/extend). CleanupWorker — P2. Middleware-интеграция — отдельный шаг после API.

AC покрытия MVP: AC-001, AC-003, AC-004, AC-005, AC-006, AC-008, AC-009.

## First Validation Path

1. Запустить `docker-compose up` (PG + Valkey).
2. Выполнить миграцию `010_sessions.up.sql`.
3. `curl -X POST localhost:8081/api/v1/sessions -d '{"model":"gpt-4"}' -H "X-Tenant-ID: alpha"` → 201 + session_id.
4. `curl localhost:8081/api/v1/sessions/<id> -H "X-Tenant-ID: alpha"` → 200 + метаданные.
5. `curl -X PATCH localhost:8081/api/v1/sessions/<id>/extend -d '{"ttl_seconds":3600}' -H "X-Tenant-ID: alpha"` → 200 + новый expires_at.
6. `curl -X DELETE localhost:8081/api/v1/sessions/<id> -H "X-Tenant-ID: alpha"` → 200 + status=closed.

## Scope

- Domain: `src/internal/domain/session/` — entity, value objects, errors, port, use case (новая директория).
- Adapters: `src/internal/adapters/repository/session/` — PostgresSessionStore, ValkeySessionCache, CachedSessionStore (новая директория).
- API: `src/internal/api/session_handler.go` — Handler + регистрация на AdminServer (новый файл).
- Middleware: `src/internal/api/middleware/session.go` — SessionMiddleware (новый файл).
- Config: `SessionConfig` в `src/internal/infra/config/config.go` (расширение).
- Migrations: `src/internal/adapters/repository/postgres/migrations/010_sessions.up/down.sql` (новый файл).
- CleanupWorker: `src/internal/app/worker/session_cleanup.go` (новый файл).
- Shield middleware: интеграция инкремента счётчиков в `middleware/shield.go` (изменение).

Явно не входит: UI, Envoy data plane, аналитика/агрегация.

## Performance Budget

- POST /api/v1/sessions (PG only): P99 < 50ms.
- GET /api/v1/sessions (с Valkey, hit): P99 < 10ms.
- GET /api/v1/sessions (PG only): P99 < 50ms.
- IncrementCounts (PG): P99 < 50ms.
- CleanupWorker: < 5s для 10k expired записей.
- Память: ValkeySessionCache — не более 1MB на сессию (сериализованный JSON).

## Implementation Surfaces

- `src/internal/domain/session/` — новая. Domain слой не существует — нужен новый пакет.
- `src/internal/domain/session/entity.go` — Session struct.
- `src/internal/domain/session/errors.go` — ErrSessionNotFound, ErrSessionExpired, ErrSessionClosed.
- `src/internal/domain/session/storage.go` — SessionStore port.
- `src/internal/domain/session/usecase.go` — SessionUseCase.
- `src/internal/adapters/repository/session/` — новая. Существующие `mask/` адаптеры — шаблон, но новый пакет для session.
- `src/internal/adapters/repository/session/postgres.go` — PostgresSessionStore.
- `src/internal/adapters/repository/session/valkey.go` — ValkeySessionCache.
- `src/internal/adapters/repository/session/cached.go` — CachedSessionStore (декоратор).
- `src/internal/api/session_handler.go` — новый. REST handler.
- `src/internal/api/middleware/session.go` — новый. Session middleware.
- `src/internal/api/server.go` — RegisterSessionHandler.
- `src/internal/api/admin.go` — RegisterSessionHandler (admin).
- `src/internal/app/worker/session_cleanup.go` — новый. CleanupWorker.
- `src/internal/infra/config/config.go` — SessionConfig.
- `src/internal/api/middleware/shield.go` — интеграция инкремента.

## Bootstrapping Surfaces

- `src/internal/domain/session/` — создать первой, т.к. всё остальное зависит от неё.
- `src/internal/adapters/repository/session/` — сразу после domain.
- Миграция `010_sessions.up.sql` — перед первым запуском адаптера.

## Влияние на архитектуру

- Локальное: новый domain-пакет, новые адаптеры, новый handler, новый middleware.
- Интеграции: SessionMiddleware читает `X-Session-ID` и кладёт Session в gin context. ShieldMiddleware использует session из контекста для инкремента.
- Config: новый блок `session` в YAML.
- No breaking changes: существующие маршруты и middleware не меняются (кроме добавления счётчиков в ShieldMiddleware).
- CleanupWorker не стартует по умолчанию — opt-in через конфиг.

## Acceptance Approach

- AC-001: POST /api/v1/sessions → 201. Surfaces: session_handler + usecase + postgres. Наблюдается через HTTP response.
- AC-002: middleware → IncrementCounts → GET проверка. Surfaces: shield.go (middleware) + usecase + postgres. Наблюдается через GET после обработки запроса.
- AC-003: tenant-beta → GET чужой сессии → 404. Surfaces: session_handler (tenant-scoped filter). Наблюдается через HTTP 404.
- AC-004: GET /api/v1/sessions?page=1&limit=10 → пагинированный список. Surfaces: session_handler + usecase + postgres (ListByTenant). Наблюдается через JSON body.
- AC-005: PATCH .../extend → expires_at сдвинут. Surfaces: session_handler + usecase + postgres. Наблюдается через GET после extend.
- AC-006: DELETE .../id → status=closed. Surfaces: session_handler + usecase + postgres. Наблюдается через GET после close.
- AC-007: CleanupWorker → удаление expired. Surfaces: session_cleanup.go worker. Наблюдается через PG direct query после запуска worker.
- AC-008: Valkey недоступен → POST → 201 + WARN в логе. Surfaces: cached_store + postgres. Наблюдается через HTTP + log.
- AC-009: session_id format → regex. Surfaces: entity (NewSessionID). Наблюдается через любой response с session_id.
- AC-010: request с X-Session-ID → сессия создана. Surfaces: session middleware + usecase. Наблюдается через GET после запроса.

## Данные и контракты

- AC: все 10 AC покрывают entity, store, API, middleware, worker.
- Data model: `sessions` table (см. `data-model.md`).
- API contracts:
  - `POST /api/v1/sessions` — body `{"model":"string"}`, response 201 `{"session_id","tenant_id","model","status","created_at","expires_at"}`.
  - `GET /api/v1/sessions/:id` — response 200 `{"session_id","tenant_id","model","token_count","message_count","total_masks","dict_mask_count","pii_mask_count","preprocessor_count","status","created_at","expires_at"}`.
  - `GET /api/v1/sessions?page=&limit=` — response 200 `{"items":[],"total","page","limit"}`.
  - `PATCH /api/v1/sessions/:id/extend` — body `{"ttl_seconds":int}`, response 200 `{"session_id","expires_at","status"}`.
  - `DELETE /api/v1/sessions/:id` — response 200 `{"status":"closed"}`.
  - Все endpoints tenant-scoped: `X-Tenant-ID` header (через middleware).
  - OpenAPI spec — в плане tasks (добавить после имплементации).
- Event contracts: не меняются.

## Стратегия реализации

### DEC-001 CachedSessionStore (декоратор, как CachedMaskRepo)

Why: единый паттерн в проекте — CachedMaskRepo уже реализует write-through + cache-aside. Переиспользование той же идиомы снижает когнитивную нагрузку.
Tradeoff: два storage backend вместо одного — больше кода, но Valkey опционален (fail-open).
Affects: PostgresSessionStore (primary), ValkeySessionCache (secondary), CachedSessionStore (decorator).
Validation: AC-008 (Valkey недоступен → WARN + PG).

### DEC-002 SessionID генерируется middleware, не use case

Why: SessionID должен быть известен до вызова use case, чтобы middleware могла установить заголовок `X-Session-ID` в response до асинхронного вызова. Middleware вызывает `NewUUIDv7()` и передаёт готовый ID в use case.
Tradeoff: use case не контролирует генерацию ID — это ок, т.к. ID не содержит доменной логики.
Affects: session middleware → use case → postgres.
Validation: AC-009 (UUIDv7 format).

### DEC-003 CleanupWorker opt-in, не стартует по умолчанию

Why: на dev-окружениях и тестах cleanup не нужен. Опциональность снижает риск случайного удаления данных при настройке.
Tradeoff: на production надо не забыть включить. Решается default = true в production-конфиге (документация).
Affects: config (`session.cleanup.enabled`), worker.
Validation: AC-007 (CleanupWorker удаляет expired).

### DEC-004 Middleware инкрементирует счётчики по типам маскировок после shield scan

Why: shield middleware уже знает результаты dict scan и PII scan. SessionMiddleware получает session из контекста и вызывает IncrementCounts с готовыми значениями.
Tradeoff: session middleware не универсальна — завязана на shield scan pipeline.
Affects: shield.go (вычисление кол-ва масок по типам), session middleware (IncrementCounts).
Validation: AC-002 (счётчики после middleware).

## Incremental Delivery

### MVP (Первая ценность)

Domain + PostgresSessionStore + REST API (create/list/get/close/extend).
AC: 001, 003, 004, 005, 006, 008, 009.
Валидация: ручные curl-ы к AdminServer.

### Итеративное расширение 1

ValkeySessionCache + CachedSessionStore.
AC: 008 (graceful degradation).
Валидация: отключить Valkey → проверить WARN + PG работает.

### Итеративное расширение 2

SessionMiddleware + интеграция в ShieldMiddleware (инкремент счётчиков).
AC: 002, 010.
Валидация: curl через gateway с X-Session-ID → проверить message_count/total_masks.

### Итеративное расширение 3

CleanupWorker.
AC: 007.
Валидация: создать expired сессию через PG → запустить worker → проверить удаление.

## Порядок реализации

1. **Domain** (`src/internal/domain/session/`) — entity, errors, port, use case. Не зависит ни от чего в проекте.
2. **Migraton** `010_sessions.up/down.sql` — CREATE TABLE sessions.
3. **PostgresSessionStore** — реализация порта на PG.
4. **REST API** — SessionHandler + регистрация на AdminServer + базовый UseCase wire-up.
5. **Test MVP** — curl до AdminServer (AC-001, 003, 004, 005, 006).
6. **ValkeySessionCache + CachedSessionStore** — кэш с fail-open.
7. **SessionMiddleware** — чтение X-Session-ID, создание/получение сессии.
8. **Shield middleware интеграция** — инкремент счётчиков после scan.
9. **CleanupWorker** — фоновый garbage collector.
10. **OpenAPI spec** — docs.

Параллельно: domain + tests (unit).

## Риски

- **Риск: race condition на increment (одновременные запросы с одним SessionID)**
  Mitigation: атомарный `UPDATE ... SET col = col + $1` в PG — serializable. Valkey использует `INCRBY`. Разница в 1-2 токена/сообщения допустима.
- **Риск: рост таблицы sessions при забытом CleanupWorker**
  Mitigation: hard cap через конфиг `session.max_lifetime` (макс. TTL). Индекс по `expires_at` для эффективного cleanup.
- **Риск: X-Session-ID не UUIDv7 от клиента**
  Mitigation: middleware логирует WARN при невалидном UUIDv7, генерирует новый. AC-010 обновлён — клиент обязан слать UUIDv7.

## Rollout и compatibility

- Миграция `010_sessions.up.sql` — обратно совместима, новая таблица.
- CleanupWorker opt-in: `session.cleanup.enabled: false` по умолчанию.
- SessionMiddleware не блокирует запрос при ошибке store (fail-open — логирует WARN, пропускает).
- Мониторинг: метрики `session_active_total`, `session_created_total`, `session_increment_errors`.
- После релиза: проверить логи на WARN от Valkey и session store.

## Проверка

- Unit tests: entity, value object, use case, errors.
- PostgresSessionStore test: integration (testcontainers или docker-compose).
- ValkeySessionCache test: integration.
- SessionHandler test: httptest + mocked use case.
- SessionMiddleware test: httptest + mocked use case.
- ShieldMiddleware integration test: проверка инкремента счётчиков.
- CleanupWorker test: mocked store + time control.
- AC-003 tenant-scoped: test с tenant-alpha и tenant-beta.
- AC-008 graceful degradation: test с nil Valkey client.

## Соответствие конституции

- нет конфликтов. Content Shield остаётся core domain — sessions не дублируют маскирование, а добавляют статистику поверх. Go + Gin + PostgreSQL + Valkey — все технологии в стеке. DDD + Clean Architecture соблюдены (domain/ports/adapters).
