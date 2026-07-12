# Rate Limiting & Token Budgets План

## Phase Contract

Inputs: spec, inspect (pass), repo map.
Outputs: plan.md, data-model.md.
Stop if: —.

## Цель

Добавить per-tenant rate limiting (sliding window) и per-tenant, per-model token budget enforcement. Вся stateful логика — на существующем Valkey, без новых зависимостей. Rate limit middleware встраивается в существующий Gin middleware pipeline.

## MVP Slice

AC-001, AC-004 — базовое per-tenant sliding window rate limiting с блокировкой при превышении и восстановлением после окна. Без token budget, без per-model, без метрик.

## First Validation Path

`go test ./...` с интеграционным тестом: 11 последовательных запросов от одного tenant в 60s window → 10×200 + 1×429, затем wait + 1 запрос → 200.

## Scope

- `src/internal/domain/budget/` — новые DDD-сущности TokenBudget, RateLimit, порт-интерфейсы BudgetRepository/RateLimitRepository
- `src/internal/adapters/repository/budget/` — Valkey-реализации репозиториев
- `src/internal/api/middleware/ratelimit.go` — Gin middleware для rate limit проверки и token budget дедукции
- `src/internal/infra/config/config.go` — новая секция RateLimitConfig
- `src/internal/infra/metrics/` — новые Prometheus-счётчики
- `src/internal/api/server.go` — регистрация middleware
- Web UI, billing, runtime config reload — не меняются

## Performance Budget

- P99 overhead rate limit check: < 5ms (Valkey localhost)
- P99 token budget check: < 5ms
- No additional allocs on hot path (reuse buffers)
- Memory: ~0 per-request (state in Valkey)

## Implementation Surfaces

| Surface | Роль | Статус |
|---|---|---|
| `src/internal/domain/budget/` | TokenBudget, RateLimit value objects + repository interfaces | new |
| `src/internal/adapters/repository/budget/` | ValkeyRateLimitRepo, ValkeyTokenBudgetRepo | new |
| `src/internal/api/middleware/ratelimit.go` | RateLimitMiddleware (Gin HandlerFunc) | new |
| `src/internal/infra/config/config.go` | RateLimitConfig struct | extended |
| `src/internal/infra/metrics/` | Prometheus counters for rate-limited requests | extended |
| `src/internal/api/server.go` | регистрация rate limit middleware | extended |

## Bootstrapping Surfaces

- `src/internal/domain/budget/` — создать первой: entity types + repository interfaces, от них зависят все остальные поверхности
- Valkey key prefix constants в domain layer

## Влияние на архитектуру

- Новая domain-подсистема `budget` — ортогональна существующим (shield, routing).
- Rate limit middleware встраивается в цепочку Gin middleware и исполняется **до** routing proxy handler.
- Token budget middleware исполняется **после** LLM ответа (как wrapper вокруг handler).
- Config: `RateLimitConfig` — nullable, при nil middleware не регистрируется.
- Data model: не меняется (все данные — временные, в Valkey с TTL).

## Acceptance Approach

- **AC-001**: unit test для ValkeyRateLimitRepo (ZADD+ZCOUNT) + integration test 11 requests → asserts 11th is 429. Surfaces: domain/budget, repo/budget, middleware, server.
- **AC-002**: integration test с mock provider, возвращающим известное token usage. Surfaces: domain/budget, repo/budget, middleware.
- **AC-003**: integration test — проверить наличие заголовков после успешного запроса. Surfaces: middleware.
- **AC-004**: integration test — дождаться окончания sliding window, проверить 200. Surfaces: repo/budget, middleware.
- **AC-005**: integration test — два разных model key в Valkey, разный budget. Surfaces: repo/budget, middleware.
- **AC-006**: integration test — два tenant с разными конфигами. Surfaces: config, repo/budget, middleware.
- **AC-007**: unit test — проверить инкремент Prometheus counter. Surfaces: metrics, middleware.

## Данные и контракты

- no-change для PostgreSQL и API contracts.
- Rate-limit заголовки HTTP — новый, но стандартизированный контракт (X-RateLimit-*), не ломает обратную совместимость.
- 429 response body — новый формат ошибки, расширяет существующий error handler.
- Valkey key schema (non-persistent, TTL-based): `ratelimit:{tenantID}:{window}` (sorted set), `tokenbudget:{tenantID}:{model}:{window}` (string counter).
- `data-model.md` — stub no-change.

## Стратегия реализации

### DEC-001 Sliding window через Valkey Sorted Set

- **Why**: Sorted set (score=ms timestamp) даёт точный скользящий window — каждый запрос добавляется с ZADD, старые удаляются ZREMRANGEBYSCORE, количество в окне — ZCOUNT. Атомарность через Lua-скрипт.
- **Tradeoff**: Храним каждый запрос (N entry в окне) → больше памяти Valkey, чем INCR-based fixed window. Но точность важнее памяти.
- **Affects**: repo/budget, domain/budget
- **Validation**: AC-001, AC-004

### DEC-002 Token budget через Valkey INCR + TTL

- **Why**: Token budget — мягкий лимит, не требуется sliding window точность. INCR + EXPIRE — O(1), минимальная память.
- **Tradeoff**: Допускает burst на границе окна (reset). Приемлемо для token budget (overage в одном запросе уже возможен).
- **Affects**: repo/budget, domain/budget
- **Validation**: AC-002, AC-005

### DEC-003 Fail-open при недоступности Valkey

- **Why**: Менее разрушительно — существующий трафик не блокируется. Ошибка логируется.
- **Tradeoff**: Rate limit временно не работает → оператор должен мониторить `gateway_rate_limited_total` и алерты на Valkey.
- **Affects**: middleware
- **Validation**: SC-002

### DEC-004 Отдельная секция RateLimitConfig

- **Why**: Nullable — при отсутствии секции rate limit отключён. Не смешивается с ValkeyConfig (где только подключение). Можно в будущем добавить runtime reload.
- **Tradeoff**: Дополнительная config-секция, но явная и самодокументированная.
- **Affects**: config
- **Validation**: AC-006 (per-tenant override), AC-007 (middleware не регистрируется без секции)

### DEC-005 Middleware на tenantID из auth context

- **Why**: Tenant ID уже в gin.Context после AuthMiddleware — не требуется парсить запрос повторно.
- **Tradeoff**: Зависимость от формата tenantID в контексте; если auth middleware меняет ключ — rate limit сломается.
- **Affects**: middleware
- **Validation**: AC-001

### DEC-006 Post-response token budget deduction

- **Why**: Точное количество токенов известно только после ответа LLM. Pre-flight estimation — приблизительный.
- **Tradeoff**: Один запрос может превысить budget (overshoot). Приемлемо — overshoot в одном запросе меньше, чем abuse без лимитов.
- **Affects**: middleware (wrapper вокруг upstream handler)
- **Validation**: AC-002

## Incremental Delivery

### MVP (AC-001, AC-004)

- RateLimit domain entity + порт
- ValkeyRateLimitRepo (sorted set + Lua атомарность)
- Config: дефолтные лимиты
- Middleware: проверка rate limit + 429
- Server: регистрация middleware
- Tests: AC-001, AC-004
- Проверка: `go test` + manual 11 запросов

### Итерация 2 (AC-003, AC-006, AC-007)

- Rate-limit заголовки в middleware (compute remaining + reset)
- Per-tenant override в конфиге
- Prometheus метрики
- Tests: AC-003, AC-006, AC-007

### Итерация 3 (AC-002, AC-005)

- TokenBudget domain entity + порт
- ValkeyTokenBudgetRepo (INCR + TTL per-model)
- Middleware: post-response token deduction
- Per-model конфиг
- Tests: AC-002, AC-005

## Порядок реализации

1. Config (RateLimitConfig) — без неё нельзя ничего.
2. Domain entity + repository interface.
3. Valkey repository implementation.
4. Middleware (rate limit check + 429).
5. Middleware registration в server.go.
6. Rate-limit headers + metrics.
7. Token budget (entity, repo, middleware).
8. Tests.

## Риски

- **Valkey недоступен**: fail-open — rate limit не работает. Mitigation: алерт на недоступность Valkey, метрика ошибок репозитория.
- **Clock skew**: sliding window использует серверное время. Mitigation: все ноды в одной time-zone; ms timestamp с `time.Now()`.
- **Token budget overshoot**: один большой запрос может превысить budget. Mitigation: задокументировано как tradeoff в DEC-006; budget ставится с запасом.
- **Key collision с mask-кэшом**: общий Valkey. Mitigation: префикс `ratelimit:` и `tokenbudget:` для rate-limit ключей.

## Rollout и compatibility

- Feature флаг не требуется — секция `RateLimit` в config включается явно.
- При добавлении конфига — старые конфиги без `RateLimit` продолжают работать (nil → middleware не регистрируется).
- Специальных rollout-действий не требуется.

## Проверка

- `go test ./...` — unit + integration тесты для каждого AC.
- `go test -run TestRateLimit` — выделенный запуск rate limit тестов.
- Manual: sent 11 curl-запросов, проверить 429 на 11-м.
- `go vet ./...`, `golangci-lint run` — без новых предупреждений.
- Проверка Valkey key space после тестов: `KEYS ratelimit:*` — ключи истекают корректно.

## Соответствие конституции

- **нет конфликтов**: DDD (domain/budget), Go + Gin + Valkey, lang policy соблюдён.
