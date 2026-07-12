# Rate Limiting & Token Budgets Задачи

## Phase Contract

Inputs: plan.md, data-model.md, spec.md.
Outputs: исполнимые задачи с покрытием всех 7 AC.
Stop if: —.

## Surface Map

| Surface | Tasks |
|---|---|
| `src/internal/infra/config/config.go` | T1.1 |
| `src/internal/domain/budget/` (new) | T1.2, T3.5 |
| `src/internal/adapters/repository/budget/` (new) | T2.1, T3.5 |
| `src/internal/api/middleware/ratelimit.go` (new) | T2.2, T3.1 |
| `src/internal/api/server.go` | T2.2 |
| `src/internal/infra/metrics/` | T3.2 |
| `src/internal/api/middleware/` (tests) | T2.3, T3.4, T4.1 |

## Implementation Context

- **Цель MVP**: per-tenant sliding window rate limit на Valkey Sorted Set + 429 блокировка (AC-001, AC-004)
- **Инварианты/семантика**: rate limit check до LLM вызова; token budget deduction после ответа; fail-open при недоступности Valkey; ключи с префиксами `ratelimit:` и `tokenbudget:`
- **Ошибки/коды**: 429 + `{"error":"rate_limit_exceeded"}` / `{"error":"token_budget_exceeded"}`
- **Контракты/протокол**: заголовки `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `X-RateLimit-Budget-Remaining`; tenantID из gin.Context (auth middleware)
- **Границы scope**: не делаем UI, runtime reload, per-user limiting, billing, глобальный rate limit, concurrent limit
- **Proof signals**: go test pass; 11 curl-запросов → 10×200 + 1×429; headers присутствуют
- **References**: DEC-001 (sorted set), DEC-002 (INCR+TTL), DEC-003 (fail-open), DEC-005 (tenantID), DEC-006 (post-response deduction)

## Фаза 1: Основа

Цель: подготовить конфиг и domain-слой — без этого нельзя начать реализацию.

- [x] T1.1 **Добавить RateLimitConfig в конфиг**
  Добавить структуру `RateLimitConfig` в `src/internal/infra/config/config.go` с полями: `Enabled` (bool), `DefaultRatePerWindow` (int), `DefaultWindowSec` (int), `DefaultTokenBudget` (map[string]int64 — model→budget), `TenantOverrides` (map[string]*RateLimitConfig). Добавить дефолты в `DefaultConfig()`. Секция nullable — при nil middleware не регистрируется.
  Touches: `src/internal/infra/config/config.go`

- [x] T1.2 **Создать domain entities и repository interfaces**
  Создать `src/internal/domain/budget/` с типами: `RateLimit` (value object: Limit, Window, Remaining, ResetTime), `TokenBudget` (value object: Model, Budget, Remaining), интерфейсы `RateLimitRepository` (Allow/Incr/Reset) и `TokenBudgetRepository` (GetRemaining/Deduct/Reset). Добавить константы Valkey key prefixes: `KeyPrefixRateLimit = "ratelimit:"`, `KeyPrefixTokenBudget = "tokenbudget:"`.
  Touches: `src/internal/domain/budget/`

## Фаза 2: MVP Slice

Цель: per-tenant sliding window rate limit работает end-to-end (AC-001, AC-004).

- [x] T2.1 **Реализовать ValkeyRateLimitRepo**
  Реализовать `ValkeyRateLimitRepo` в `src/internal/adapters/repository/budget/`. Использовать Sorted Set (ZADD score=ms timestamp, ZREMRANGEBYSCORE, ZCOUNT). Атомарность через Lua-скрипт (EVALSHA). Поддержка fail-open — при ошибке Valkey возвращает allow=true + лог.
  Touches: `src/internal/adapters/repository/budget/`

- [x] T2.2 **Реализовать middleware и server registration**
  Создать `src/internal/api/middleware/ratelimit.go`: Gin HandlerFunc, извлекает tenantID из контекста, вызывает RateLimitRepository.Allow(), при превышении возвращает 429 + `{"error":"rate_limit_exceeded"}`, иначе `c.Next()`. В `server.go`: если `cfg.RateLimit != nil` — зарегистрировать middleware через `engine.Use()`.
  Touches: `src/internal/api/middleware/ratelimit.go`, `src/internal/api/server.go`

- [x] T2.3 **MVP тесты (AC-001, AC-004)**
  Unit-тест для ValkeyRateLimitRepo (ZADD + ZCOUNT с mocked valkey). Integration-тест: 11 запросов от одного tenant → 10×200 + 1×429; wait + 1 запрос → 200. Покрытие AC-001, AC-004.
  Touches: `src/internal/adapters/repository/budget/`, `src/internal/api/middleware/ratelimit.go`

## Фаза 3: Основная реализация

Цель: rate-limit заголовки, per-tenant конфиг, метрики, token budget (AC-002, AC-003, AC-005, AC-006, AC-007).

- [x] T3.1 **Добавить rate-limit заголовки**
- [x] T3.2 **Добавить per-tenant override в конфиг**
- [x] T3.3 **Добавить Prometheus метрики**
- [x] T3.4 **Тесты (AC-003, AC-006, AC-007)**
- [x] T3.5 **Реализовать TokenBudget (entity + repo + middleware)**
- [x] T3.6 **Тесты (AC-002, AC-005)**
  Integration-тесты: mock provider возвращает известное token usage → проверка 429 при превышении бюджета (AC-002); два разных model budget — один исчерпан, другой нет (AC-005).
  Touches: `src/internal/api/middleware/`, `src/internal/adapters/repository/budget/`

## Фаза 4: Проверка

Цель: полный verify pass — все AC подтверждены, код чист.

- [x] T4.1 **End-to-end integration suite**
  Написать сквозной тест: запуск gateway с test config, последовательность запросов от tenant A и B, проверка всех rate-limit заголовков, проверка 429 с обеими причинами, проверка восстановления после окна, проверка Prometheus метрик. Один тестовый файл покрывает все 7 AC.
  Touches: `src/internal/api/middleware/`, `src/internal/cmd/gateway/` (test helper)

- [x] T4.2 **Verify + cleanup**
  `go vet ./...`, `golangci-lint run`, `go test ./...`. Проверить отсутствие регрессий. Убедиться, что Valkey ключи с TTL корректно истекают после тестов.
  Touches: (проверка без изменений кода)

## Покрытие критериев приемки

- AC-001 -> T2.1, T2.2, T2.3, T4.1
- AC-002 -> T3.5, T3.6, T4.1
- AC-003 -> T3.1, T3.4, T4.1
- AC-004 -> T2.1, T2.2, T2.3, T4.1
- AC-005 -> T3.5, T3.6, T4.1
- AC-006 -> T3.2, T3.4, T4.1
- AC-007 -> T3.3, T3.4, T4.1

## Заметки

- trace-маркеры `@sk-task` ставить над owning declaration (функция/метод), не на package/import/file-header
- Lua-скрипт для атомарности sorted set — реализовать как строковую константу в репозитории с EVALSHA
- Middleware регистрируется в `server.go` до routing proxy handler, но после auth middleware
- Token budget middleware — обёртка вокруг handler (post-response), а не inline middleware
