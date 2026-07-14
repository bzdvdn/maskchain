# Rate Limit Wiring — подключение rate limiting в gateway request lifecycle

## Scope Snapshot

- In scope: инициализация Valkey RateLimitRepository/TokenBudgetRepository, регистрация RateLimit middleware в gateway main.go, добавление Retry-After header в 429-путь middleware
- Out of scope: имплементация логики rate limiting, репозиториев, метрик — всё уже существует из фазы rate-limiting-budgets

## Цель

Оператор gateway получает работающий rate limiting «из коробки» после настройки секции `ratelimit` в конфиге: валидные запросы проходят, превышения лимита возвращают 429 с Retry-After, per-tenant и per-model переопределения учитываются, недоступность Valkey не ломает gateway (fail-open). Успех измеряется тестами: после конфигурации tenant с лимитом 5rps 6-й запрос получает 429.

## Основной сценарий

1. Gateway стартует, загружает конфиг, инициализирует Valkey client
2. Если секция `ratelimit` присутствует в конфиге — создаются `ValkeyRateLimitRepo` и (опционально) `ValkeyTokenBudgetRepo`
3. RateLimit middleware регистрируется глобально через `srv.RegisterRateLimit()`
4. Входящий запрос с tenant-контекстом: middleware проверяет лимит, устанавливает headers, при превышении — 429
5. Если Valkey недоступен — репозитории возвращают passthrough, middleware пропускает запрос без ошибки
6. Метрики `rate_limit_exceeded_total` и `budget_exceeded_total` инкрементируются

## User Stories

- P1: Оператор настраивает `ratelimit.default_rate_per_window: 100` — gateway начинает ограничивать запросы до 100/window на tenant
- P2: Оператор добавляет `tenant_overrides` с `rate_per_window: 1000` — конкретный tenant получает повышенный лимит
- P3: Оператор настраивает `default_token_budget: { "gpt-4": 100000 }` — токен-бюджеты учитываются наряду с rate limit

## MVP Slice

Инициализация RateLimitRepository и регистрация middleware для tenant-based rate limiting (без token budget). Закрывает AC-001, AC-003, AC-004.

## First Deployable Outcome

После apply конфига с `ratelimit.default_rate_per_window: 5` gateway отклоняет 6-й запрос от одного tenant в окне. Ответ содержит 429 + Retry-After header. VictoriaMetrics (или /metrics) показывает `maskchain_rate_limit_exceeded_total`.

## Scope

- Инициализация `ValkeyRateLimitRepo` и `ValkeyTokenBudgetRepo` в gateway main.go
- Вызов `srv.RegisterRateLimit(middleware.RateLimit(repo, cfg.RateLimit, tokenBudgetRepo))`
- Graceful degradation: nil-client guard в репозиториях уже реализован, middleware обрабатывает ошибки Allow() через fail-open
- Настройка `ratelimit` секции конфига (defaults, tenant_overrides, default_token_budget)
- Метрики уже зарегистрированы — их инициализация не меняется
- Error code `RATE_LIMIT_EXCEEDED` / `TOKEN_BUDGET_EXCEEDED` уже возвращаются
- Добавление `Retry-After` header в 429-путь middleware middleware/ratelimit.go

## Контекст

- Tenant-контекст устанавливается Auth middleware — rate limit middleware читает tenant из контекста
- `RegisterRateLimit` использует `s.engine.Use()` — middleware применяется глобально ко всем маршрутам
- Slug feature/rate-limiting-budgets — уже реализована и смержена; её артефакты (middleware, репозитории, тесты) — входной контракт
- Статусы health probes (Valkey) уже реализованы в 114-real-health-probes — падение Valkey детектируется
- Admin server (`api/admin.go`) не имеет `RegisterRateLimit` — не входит в scope

## Зависимости

- `rate-limiting-budgets` — предоставляет интерфейсы `budget.RateLimitRepository`, `budget.TokenBudgetRepository`, реализации ValkeyRepo, middleware и метрики
- `114-real-health-probes` — предоставляет health probes для Valkey (не блокер)

## Требования

- RQ-001 Gateway ДОЛЖЕН инициализировать `ValkeyRateLimitRepo` при наличии секции `ratelimit` в конфиге
- RQ-002 Gateway ДОЛЖЕН регистрировать RateLimit middleware глобально через `srv.RegisterRateLimit()`
- RQ-003 Система ДОЛЖНА учитывать `ratelimit.tenant_overrides` для per-tenant rate limit и window
- RQ-004 Система ДОЛЖНА учитывать `ratelimit.default_token_budget` для per-model token budgets (опционально, если настроено)
- RQ-005 При недоступности Valkey middleware ДОЛЖЕН пропускать запрос (fail-open) с логированием warning
- RQ-006 Метрики `rate_limit_exceeded_total` и `budget_exceeded_total` ДОЛЖНЫ инкрементироваться при соответствующих отказах
- RQ-007 Ответ 429 ДОЛЖЕН содержать Retry-After header (секунды до конца окна)

## Вне scope

- Rate limiting для admin server — нет `RegisterRateLimit` на `AdminServer`, не добавляется
- Rate limiting без Valkey (in-memory standalone) — репозитории in-memory не поставляются
- Token budget deduction — уже реализована в middleware, проводка не требует изменений
- Конфигурация defaults в `DefaultConfig()` — `RateLimit` может быть nil, middleware проверяет cfg.RateLimit через наличие секции
- Любые изменения логики rate limiting, репозиториев или метрик за пределами Retry-After header

## Критерии приемки

### AC-001 Rate limit применяется к запросам с tenant-контекстом

- Почему это важно: tenant-based rate limiting — ключевой сценарий; без tenant запросы не ограничиваются
- **Given** gateway запущен с `ratelimit.default_rate_per_window: 5, default_window_sec: 60`
- **When** 6 последовательных запросов от одного tenant поступают в течение 60s
- **Then** 6-й запрос получает HTTP 429 с телом `{"error":{"code":"RATE_LIMIT_EXCEEDED","message":"rate_limit_exceeded"}}`
- Evidence: запрос 6 возвращает status 429; headers содержат X-RateLimit-Remaining: 0

### AC-002 Per-tenant overrides применяются

- Почему это важно: разные tenant'ы могут иметь разные контрактные лимиты
- **Given** конфиг содержит `ratelimit.tenant_overrides["tenant-a"].rate_per_window: 10` и `default_rate_per_window: 5`
- **When** tenant-a отправляет 11 запросов в окне
- **Then** 11-й запрос получает 429
- **When** tenant-b (без override) отправляет 6 запросов в окне
- **Then** 6-й запрос tenant-b получает 429
- Evidence: tenant-a блокируется на 11-м, tenant-b на 6-м

### AC-003 Rate limit headers присутствуют на 200 и 429

- Почему это важно: клиенты и прокси полагаются на headers для адаптивного поведения
- **Given** запрос tenant проходит rate limit
- **When** ответ получен
- **Then** ответ содержит заголовки X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset
- **And** при превышении — также Retry-After
- Evidence: проверить headers на первом (200) и шестом (429) запросах

### AC-004 Лимит восстанавливается после окна

- Почему это важно: tenant не должен быть заблокирован навсегда; лимит сбрасывается per-window
- **Given** tenant превысил лимит и получил 429 на 6-м запросе
- **When** после expiry window (60s) приходит новый запрос
- **Then** запрос получает 200 с ненулевым X-RateLimit-Remaining
- Evidence: подождать 61s, отправить запрос — status 200, Remaining > 0

### AC-005 Token budget применяется при настройке default_token_budget

- Почему это важно: per-model квоты — отдельный механизм контроля сверх rate limit
- **Given** конфиг содержит `ratelimit.default_token_budget: {"gpt-4": 500}` и `default_rate_per_window: 1000`
- **When** tenant отправляет 6 запросов с model=gpt-4, каждый с token_usage=100
- **Then** первые 5 запросов получают 200, 6-й запрос получает 429 с `code: TOKEN_BUDGET_EXCEEDED`
- Evidence: первые 5 запросов status 200 с X-RateLimit-Budget-Remaining > 0; 6-й запрос status 429, header X-RateLimit-Budget-Remaining: 0

### AC-006 Fail-open при недоступности Valkey

- Почему это важно: gateway не должен отказывать в обслуживании из-за отказа rate limiting storage
- **Given** Valkey недоступен (контейнер остановлен, сеть разорвана)
- **When** tenant отправляет запрос
- **Then** запрос проходит (не 429) с warning в логах
- Evidence: status 200, лог содержит сообщение о недоступности Valkey/ошибке Allow()

### AC-007 Метрики инкрементируются при rate limit exceeded

- Почему это важно: мониторинг должен видеть, сколько запросов отклонено по rate limit
- **Given** tenant превышает лимит и получает 429
- **When** метрики собраны
- **Then** `maskchain_rate_limit_exceeded_total{tenant="tenant-a", reason="rate_limit_exceeded"}` > 0
- Evidence: запрос /metrics или VictoriaMetrics

### AC-008 Rate limit не применяется при отсутствии секции ratelimit в конфиге

- Почему это важно: обратная совместимость — существующие конфиги без rate limit не должны ломаться
- **Given** конфиг не содержит секцию `ratelimit` (cfg.RateLimit == nil)
- **When** gateway запускается
- **Then** gateway стартует без ошибок, все запросы проходят без rate limit проверки
- Evidence: gateway логируют "rate limit disabled" (info), запросы проходят с 200

## Допущения

- Admin server не требует rate limiting — не wired
- `ValkeyRateLimitRepo.Allow` уже содержит nil-client guard (passthrough) — не требует изменений
- Rate limit middleware не требует on-by-default — middleware регистрируется только при cfg.RateLimit != nil
- Token budget — опциональный компонент; middleware проверяет `tokenBudgetRepo != nil`
- Gateway перечитывает tenants через SyncConfig — rate limit применяется к tenant'ам, загруженным на старте (динамическое обновление лимитов не в scope)

## Критерии успеха

- SC-001 Rate limit middleware добавляет < 1ms p99 к latency для разрешённых запросов (минимальный оверхед локальной проверки)
- SC-002 После rollout rate limit error rate для настроенных лимитов должна быть предсказуемой: 429 только при превышении настроенного порога

## Краевые случаи

- Конфиг без секции `ratelimit` — middleware не регистрируется, gateway работает как раньше
- Valkey timeout — репозиторий возвращает passthrough, middleware пропускает запрос
- Tenant без override — используются default_rate_per_window и default_window_sec
- Token budget не настроен для модели — бюджет не проверяется (X-RateLimit-Budget-Remaining не устанавливается)
- Override с nil RatePerWindow или WindowSec — используется default
- Tenant отсутствует в контексте (no auth) — middleware пропускает запрос без проверки

## Открытые вопросы

- Стоит ли добавить `RegisterRateLimit` на `AdminServer` для symmetry — решается в отдельной задаче если потребуется
- Нужен ли warn/log при старте если cfg.RateLimit != nil && cfg.Valkey == nil — решается в implement
