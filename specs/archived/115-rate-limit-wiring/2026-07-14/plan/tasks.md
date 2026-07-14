# Rate Limit Wiring — Задачи

## Phase Contract

Inputs: plan + data-model (no-change) + spec + inspect (pass).
Outputs: tasks.md с покрытием AC.
Stop if: нет — plan конкретный, surfaces известны.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/api/middleware/ratelimit.go` | T2.1 |
| `src/cmd/gateway/main.go` | T2.2 |
| `src/internal/api/middleware/ratelimit_test.go` | T4.1 |
| `src/cmd/gateway/main_test.go` (new) | T4.2 |

## Implementation Context

- Цель MVP: gateway с секцией `ratelimit` в конфиге ограничивает запросы per-tenant, возвращает 429 + Retry-After при превышении
- Инварианты/семантика:
  - `cfg.RateLimit == nil` → middleware не регистрируется (AC-008)
  - Retry-After = `max(1, (rl.ResetTime/1000) - time.Now().Unix())` (DEC-001)
  - Rate limit repo инициализируется всегда при `cfg.RateLimit != nil`; token budget repo — опционально, middleware проверяет `tokenBudgetRepo != nil` (DEC-003)
  - nil-client guard в репозиториях уже реализован (fail-open, AC-006)
- Ошибки/коды: `RATE_LIMIT_EXCEEDED` / `TOKEN_BUDGET_EXCEEDED` (429)
- Контракты/протокол: headers `X-RateLimit-*` + `Retry-After` на 429
- Границы scope:
  - Admin server (`api/admin.go`) без rate limiting
  - In-memory rate limit репозитории не поставляются
- Proof signals:
  - 6-й запрос tenant'а — 429 + Retry-After
  - `/metrics` показывает `maskchain_rate_limit_exceeded_total`
  - Gateway без `ratelimit` секции работает без ошибок
- References: DEC-001, DEC-002, DEC-003; AC-001 — AC-008

## Фаза 1: Основа

Пропущена — все необходимые файлы и типы (`RateLimitConfig`, `budget.RateLimitRepository`, `middleware.RateLimit()`, `srv.RegisterRateLimit()`) уже существуют.

## Фаза 2: MVP — core rate limit wiring

Цель: Retry-After header + проводка rate limit middleware в gateway.

- [x] T2.1 Добавить Retry-After header в 429-путь middleware
  - В `src/internal/api/middleware/ratelimit.go`, в блоке `if !rl.Allowed` (строка 53), после `AbortWithError` или до него: `c.Header("Retry-After", strconv.FormatInt(max(1, (rl.ResetTime/1000)-time.Now().Unix()), 10))`. Нужно добавить `"time"` в import.
  - AC-003, DEC-001
  - Touches: `src/internal/api/middleware/ratelimit.go`

- [x] T2.2 Инициализировать Valkey rate limit репозитории и зарегистрировать middleware в gateway main.go
  - После `vkClient, err := initValkey(...)` (строка 135) и health probe регистрации (строка 157), перед `srv := api.New(...)` (строка 168):
    ```go
    var rlRepo budgetrepo.ValkeyRateLimitRepo
    var tbRepo budgetrepo.ValkeyTokenBudgetRepo
    if cfg.RateLimit != nil {
        rlRepo = budgetrepo.NewValkeyRateLimitRepo(vkClient)
        tbRepo = budgetrepo.NewValkeyTokenBudgetRepo(vkClient)
        logger.Info("rate limit repositories initialized")
    }
    ```
  - После `srv := api.New(...)` (строка 168), после регистрации auth (строка 204), перед debug routes (строка 209):
    ```go
    if cfg.RateLimit != nil {
        rateLimitMw := middleware.RateLimit(rlRepo, cfg.RateLimit, tbRepo)
        srv.RegisterRateLimit(rateLimitMw)
        logger.Info("rate limit middleware registered")
    } else {
        logger.Info("rate limit disabled — no ratelimit config section")
    }
    ```
  - Добавить импорт: `"github.com/bzdvdn/maskchain/src/internal/adapters/repository/budget"` (алиас `budgetrepo`).
  - AC-001, AC-002, AC-004, AC-006, AC-007, AC-008; DEC-002, DEC-003
  - Touches: `src/cmd/gateway/main.go`

## Фаза 3: Token budget + warn

Цель: опциональный token budget wiring, warn при cfg.RateLimit != nil && cfg.Valkey == nil.

_Содержимое фазы 3 может быть выполнено в T2.2 при желании (те же поверхности)._

- [x] T3.1 Добавить warn при cfg.RateLimit != nil && cfg.Valkey == nil
  - В main.go, перед или после инициализации репозиториев: если `vkClient == nil && cfg.RateLimit != nil` — `logger.Warn("rate limit configured but Valkey unavailable — rate limiting disabled, requests will pass through")`.
  - AC-006
  - Touches: `src/cmd/gateway/main.go`

## Фаза 4: Проверка

Цель: automated тесты на Retry-After и wiring.

- [x] T4.1 Добавить тесты на Retry-After header в ratelimit_test.go
  - Добавить проверку `Retry-After` header в существующие тесты `TestRateLimitBlocksWhenExceeded` (строка 88) и/или `TestRateLimitHeadersOn429` (строка 243).
  - AC-003, DEC-001
  - Touches: `src/internal/api/middleware/ratelimit_test.go`

- [x] T4.2 Добавить integration-тест для wiring
  - Создать `src/cmd/gateway/main_test.go` или дополнить существующий тест, проверяющий:
    - Gateway с `ratelimit` секцией: 6 запросов → 6-й 429 + Retry-After
    - Gateway без `ratelimit` секции: все запросы 200
  - AC-001, AC-002, AC-004, AC-005, AC-008
  - Touches: `src/cmd/gateway/main_test.go` (new)

## Покрытие критериев приемки

- AC-001 -> T2.2, T4.2
- AC-002 -> T2.2, T4.2
- AC-003 -> T2.1, T4.1
- AC-004 -> T2.2, T4.2
- AC-005 -> T2.2, T4.2
- AC-006 -> T2.2, T3.1, T4.2
- AC-007 -> T2.2, T4.2
- AC-008 -> T2.2, T4.2

## Заметки

- T2.1 можно выполнять до или параллельно с T2.2
- T2.2 включает обе репозитория (rate limit + token budget) в одной ветке — это упрощает код (одно условие `if cfg.RateLimit != nil`)
- T4.2 — новый файл, но для простоты можно разместить тесты рядом с main.go
