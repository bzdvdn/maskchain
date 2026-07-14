# Rate Limit Wiring — План

## Phase Contract

Inputs: spec, inspect (pass), minimal repo-контекст.
Outputs: plan, data-model.md.
Stop if: нет — spec стабильна, AC чёткие.

## Цель

Подключить существующие rate limit компоненты в gateway request lifecycle. Работа идёт в двух поверхностях:
- `src/cmd/gateway/main.go` — инициализация Valkey репозиториев и регистрация middleware
- `src/internal/api/middleware/ratelimit.go` — добавление Retry-After header в 429-путь

Безопасность подхода: spec не вводит новой доменной логики — только проводка готовых компонентов + один header.

## MVP Slice

Инициализация `ValkeyRateLimitRepo` и регистрация `RateLimit` middleware (без token budget). Закрывает AC-001, AC-003, AC-004, AC-006, AC-007, AC-008.

## First Validation Path

1. Собрать gateway: `go build ./src/cmd/gateway/`
2. Запустить с конфигом, содержащим `ratelimit.default_rate_per_window: 5`
3. Отправить 6 запросов с tenant-контекстом — 6-й должен получить 429 + Retry-After header
4. Проверить `/metrics` на наличие `maskchain_rate_limit_exceeded_total`

## Scope

- `src/cmd/gateway/main.go` — добавление импорта budgetrepo, инициализация репозиториев, регистрация middleware
- `src/internal/api/middleware/ratelimit.go` — добавление `c.Header("Retry-After", ...)` в 429-путь
- Конфигурация не меняется (`RateLimitConfig` уже существует)
- Репозитории не меняются (nil-client guard уже есть)
- Метрики не меняются (уже зарегистрированы)

## Performance Budget

- `none` — существующая ветка middleware не добавляет новых операций в горячий путь (Retry-After — вычисление epoch diff, O(1))

## Implementation Surfaces

| Surface | Change | Reason |
|---------|--------|--------|
| `src/cmd/gateway/main.go` | wire code (new) | DI: init repos, call `srv.RegisterRateLimit()` |
| `src/internal/api/middleware/ratelimit.go` | modify (minor) | Add `Retry-After` header in 429 path |
| `src/internal/adapters/repository/budget/` | no change | Already exists, stable |
| `src/internal/api/server.go` | no change | `RegisterRateLimit()` already exists |

## Bootstrapping Surfaces

- `none` — все необходимые файлы и типы существуют

## Влияние на архитектуру

- Локальное: добавление 5-10 строк в gateway main.go
- Интеграции: нет — репозитории уже зависят от valkey-go, middleware уже зависит от budget и config
- Rollout: обратно совместимо — cfg.RateLimit == nil → middleware не регистрируется

## Acceptance Approach

- **AC-001**: main.go init → integration test / manual: 6 последовательных tenant-запросов → 6-й 429
- **AC-002**: main.go init with tenant_overrides → same test с двумя tenant'ами
- **AC-003**: Retry-After в middleware → headers check на 200 и 429
- **AC-004**: sliding window в репозитории (не меняется) → E2E: ждать 61s → 200
- **AC-005**: main.go init token budget repo → integration test: 5×100 токенов → 6-й 429
- **AC-006**: nil-client guard в репозиториях (не меняется) → тест без Valkey: 200 + warning
- **AC-007**: metrics middleware (не меняется) → проверка `/metrics`
- **AC-008**: conditional init при cfg.RateLimit == nil → gateway без ratelimit: 200, no rate limit logic

## Данные и контракты

- Data model: не меняется (`data-model.md: no-change`)
- API контракты: добавляется HTTP header `Retry-After` (стандарт RFC 7231) на 429 — обратно совместимо
- Открытые вопросы из spec решаются в implement: warn при cfg.RateLimit != nil && cfg.Valkey == nil

## Стратегия реализации

### DEC-001 Retry-After вычислять из rl.ResetTime

**Why:** `rl.ResetTime` уже содержит epoch ms следующего доступного слота — конвертация в `max(1, (ResetTime/1000)-now)` даёт корректный Retry-After без дополнительного запроса к Valkey.
**Tradeoff:** Retry-After аппроксимирует конец sliding window (старейший элемент + window), что достаточно для клиентской адаптации.
**Affects:** `src/internal/api/middleware/ratelimit.go` (setRateLimitHeaders или 429-path).
**Validation:** AC-003.

### DEC-002 Rate limit middleware регистрировать условно при cfg.RateLimit != nil

**Why:** обратная совместимость — существующие конфиги без ratelimit не должны требовать изменений. Nil-конфиг = rate limit disabled.
**Tradeoff:** при переключении конфига с nil → не-nil нужен рестарт gateway (динамическая перерегистрация middleware не поддерживается).
**Affects:** `src/cmd/gateway/main.go`.
**Validation:** AC-008.

### DEC-003 Token budget репозиторий инициализировать всегда при cfg.RateLimit != nil, middleware получает nil если DefaultTokenBudget пуст и нет override.TokenBudget

**Why:** упрощение main.go — одна init-ветка. Middleware сама проверяет `tokenBudgetRepo != nil` и пропускает бюджетную логику если репозиторий nil или модель без лимита.
**Tradeoff:** лишняя аллокация репозитория если token budget не используется — стоимость nil.
**Validation:** AC-005.

## Incremental Delivery

### MVP (Первая ценность)

1. Retry-After header в middleware (один import `"time"`, 2 строки)
2. Wire rate limit repos и middleware в main.go
3. Wire token budget опционально
4. Тесты: AC-001, AC-003, AC-004, AC-008

### Итеративное расширение

- Token budget wiring: AC-005
- Per-tenant override wiring: AC-002 (покрывается той же init-веткой)
- Fail-open / metrics: AC-006, AC-007 (покрываются без изменений)

## Порядок реализации

1. **Retry-After header** — независим, можно делать первым.
2. **Wire rate limit repos + middleware** — основная работа.
3. **Wire token budget** — опционально, можно параллельно с шагом 2.
4. **Тесты** — после wiring.

Шаги 1 и 2 можно делать последовательно в одной задаче.

## Риски

- **Риск:** `ValkeyRateLimitRepo.Allow` использует `EVAL` — если Lua-скрипт не закеширован, первый запрос медленнее (SLOAD). **Mitigation:** однократная задержка при первом вызове, < 1ms на последующих.
- **Риск:** конфиг без Valkey, но с ratelimit — gateway запускается, но rate limit не работает (nil client guard пропускает всё). **Mitigation:** warn при cfg.RateLimit != nil && vkClient == nil (открытый вопрос из spec).

## Rollout и compatibility

- Специальных rollout-действий не требуется. Gateway без `ratelimit` секции работает как раньше.
- Добавление `ratelimit` в конфиг включает rate limiting после рестарта.
- Новый header `Retry-After` — обратно совместим, клиенты его игнорируют если не понимают.

## Проверка

- Добавить/обновить unit-тесты для Retry-After header в `ratelimit_test.go`
- Добавить integration test: запуск gateway с Valkey, проверка rate limit E2E
- Manual: собрать, запустить с тестовым конфигом, проверить curl-запросами
- AC-001..AC-008 покрываются тестами и manual check

## Соответствие конституции

- нет конфликтов
