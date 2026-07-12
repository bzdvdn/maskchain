# Rate Limiting & Token Budgets

## Scope Snapshot

- In scope: per-tenant request rate limiting (sliding window) and per-tenant, per-model token budget enforcement, backed by Valkey counters.
- Out of scope: UI for rate limit configuration, dynamic runtime limit reload, per-user (sub-tenant) limiting, billing/quotas aggregation, global rate limit across all tenants.

## Цель

Tenant-операторы AI gateway получают гарантию, что ни один tenant не превысит согласованный лимит запросов или токенов за окно, предотвращая abuse и обеспечивая fair sharing. Успех фичи измеряется блокировкой превышающих лимит запросов до вызова LLM и возвратом корректных rate-limit-заголовков.

## Основной сценарий

1. Запрос поступает в gateway с tenant-идентификатором (из auth-контекста).
2. Middleware проверяет rate limit (request count) через Valkey sorted set для данного tenant.
3. Если rate limit превышен — возвращается `429 Too Many Requests`.
4. Если rate limit в норме — запрос проходит к LLM provider.
5. После ответа от LLM (или streaming-завершения) middleware списывает потраченные токены из token budget tenant+model.
6. При превышении token budget — запрос блокируется до следующего окна с `429` и указанием причины `token_budget_exceeded`.

## User Stories

- P1 Story: Как tenant-оператор, я хочу, чтобы превышение лимита запросов блокировалось с понятной ошибкой, чтобы избежать неожиданных счетов от LLM провайдера.
- P2 Story: Как tenant-оператор, я хочу видеть rate-limit заголовки в ответах, чтобы понимать текущую загрузку и планировать использование.

## MVP Slice

AC-001, AC-004 — базовое per-tenant rate limiting с блокировкой и сбросом окна. Без token budget и per-model различения.

## First Deployable Outcome

После implementation pass можно запустить gateway, отправить `N+1` запросов от одного tenant за окно и получить `429` на последний запрос с корректными `X-RateLimit-*` заголовками. Всё работает на существующей Valkey-инфраструктуре, без новых зависимостей.

## Scope

- Sliding window rate limiter на Valkey Sorted Set (score = timestamp)
- Token budget tracking per-tenant, per-model (INCR + TTL)
- Budget enforcement middleware для HTTP API
- Valkey repository для rate limit counters и token budget counters
- Конфигурация: default limits в config, override через tenant config
- Метрики Prometheus для rate-limited запросов и превышений бюджета
- Конфигурация nullable — если RateLimit секция отсутствует, middleware не регистрируется

## Контекст

- Rate limit проверяется **до** вызова LLM provider — экономит стоимость.
- Token budget списывается **после** успешного ответа LLM — только за реально потраченные токены.
- Tenant идентифицируется из auth middleware контекста (`tenantID`).
- Valkey client уже инициализирован в DI и может быть переиспользован.
- При недоступности Valkey rate limiter должен fail-open или fail-closed — решение в `Открытые вопросы`.
- Конфигурация секции RateLimit может отсутствовать полностью — тогда лимитирование отключено.

## Зависимости

- `src/internal/domain/routing/` — для tenant extraction из auth контекста
- `src/internal/infra/config/` — существующий ValkeyConfig (Addr, Password)
- `github.com/valkey-io/valkey-go` — уже в go.mod, новая зависимость не требуется
- `specs/active/50-shield-engine/` — опционально, если budget применяется после detector pipeline (зависимость только по порядку запуска middleware)

## Требования

- RQ-001 Система ДОЛЖНА ограничивать количество запросов от одного tenant в скользящем окне, используя Valkey Sorted Set.
- RQ-002 Система ДОЛЖНА отслеживать и ограничивать суммарное количество токенов (input + output) от одного tenant для одной модели в окне.
- RQ-003 Система ДОЛЖНА возвращать `429 Too Many Requests` при превышении rate limit или token budget с телом ошибки, содержащим причину (`rate_limit_exceeded` или `token_budget_exceeded`).
- RQ-004 Система ДОЛЖНА возвращать заголовки `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset` и `X-RateLimit-Budget-Remaining` в каждом ответе, где применимо.
- RQ-005 Rate limit и token budget ДОЛЖНЫ быть конфигурируемы через `RateLimitConfig` (default limits + per-tenant overrides).
- RQ-006 Система ДОЛЖНА экспортировать Prometheus-метрики: `gateway_rate_limited_total{tenant,reason}`, `gateway_token_budget_remaining{tenant,model}`.
- RQ-007 Система ДОЛЖНА использовать существующий Valkey-клиент; при отсутствии конфигурации секции RateLimit — middleware не регистрируется.

## Вне scope

- Per-user (sub-tenant) rate limiting
- UI для просмотра/изменения лимитов
- Динамическая перезагрузка лимитов без restart
- Поддержка очереди запросов при превышении лимита (backpressure)
- Billing/quotas aggregation (учёт потребления для выставления счетов)
- Concurrent request limit (только rate over window)
- Глобальный rate limit (sum across all tenants)

## Критерии приемки

### AC-001 Блокировка превышения rate limit

- Почему это важно: tenant не может потребить больше разрешённых запросов за окно.
- **Given** tenant с лимитом 10 requests per 60 seconds
- **When** tenant отправляет 11-й запрос в том же окне
- **Then** 11-й запрос получает `429 Too Many Requests` с `{"error":"rate_limit_exceeded"}`
- Evidence: тест отправляет 11 последовательных запросов и проверяет status = 429 на последнем

### AC-002 Блокировка превышения token budget

- Почему это важно: tenant не может превысить согласованный лимит токенов за окно.
- **Given** tenant с лимитом 100k токенов в час для модели `gpt-4`
- **When** tenant тратит суммарно 100k+ токенов в одном окне
- **Then** следующий запрос к `gpt-4` получает `429 Too Many Requests` с `{"error":"token_budget_exceeded"}`
- Evidence: интеграционный тест отправляет запросы, суммарно превышающие budget, и проверяет 429

### AC-003 Rate-limit заголовки в ответе

- Почему это важно: клиент может адаптировать поведение, не дожидаясь 429.
- **Given** tenant с лимитом 100 requests per 60 seconds
- **When** tenant отправляет успешный запрос (rate limit не превышен)
- **Then** ответ содержит заголовки `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `X-RateLimit-Budget-Remaining`
- Evidence: проверка наличия и корректности заголовков в HTTP response

### AC-004 Сброс rate limit после окна

- Почему это важно: tenant должен восстанавливать доступ после завершения окна.
- **Given** tenant получил 429 из-за превышения rate limit
- **When** после завершения sliding window tenant отправляет новый запрос
- **Then** запрос обрабатывается успешно (не 429)
- Evidence: тест ждёт истечения окна и проверяет статус 200

### AC-005 Per-model token budget

- Почему это важно: разные модели могут иметь разные лимиты токенов.
- **Given** tenant с разными token budget для `gpt-4` (100k) и `gpt-3.5-turbo` (500k)
- **When** tenant тратит 150k токенов только на `gpt-4`
- **Then** запрос к `gpt-3.5-turbo` проходит, а к `gpt-4` — 429
- Evidence: интеграционный тест проверяет, что разные модели не влияют на бюджет друг друга

### AC-006 Rate limit configurable per-tenant

- Почему это важно: разные tenant могут иметь разные соглашения об уровне обслуживания.
- **Given** tenant A с лимитом 100 req/min и tenant B с лимитом 1000 req/min
- **When** tenant B отправляет 500 запросов (превышает лимит A, но не превышает свой)
- **Then** все запросы tenant B успешны
- Evidence: тест с двумя tenant и разными лимитами проверяет независимость

### AC-007 Prometheus метрики rate-limited запросов

- Почему это важно: оператор должен видеть частоту срабатываний rate limiter в мониторинге.
- **Given** включённый rate limiter и работающий Prometheus
- **When** tenant превышает rate limit
- **Then** счётчик `gateway_rate_limited_total{tenant="<id>",reason="rate_limit_exceeded"}` инкрементируется
- Evidence: тест проверяет значение метрики до и после rate-limited запроса

## Допущения

- Tenant ID доступен в контексте запроса после auth middleware как строковый ключ.
- Valkey доступен на момент старта gateway; при отказе Valkey во время работы rate limiter переходит в fail-open (rate limit не применяется, но ошибка логируется).
- Token usage (input + output tokens) доступен из ответа LLM provider или может быть оценён до запроса. В тестах token count приходит от mock/stub provider.
- Если token budget для tenant/model не сконфигурирован, заголовок `X-RateLimit-Budget-Remaining` не включается в ответ.
- Все таймстемпы для sliding window используют `time.Now().UnixMilli()` — часы системные, не распределённые.
- Первоначальная конфигурация — YAML-файл; env-переменные и runtime reload — PostMVP.
- Sliding window precision — 1 секунда (миллисекундные timestamps, но окно округляется до секунды для читаемости заголовка Reset).

## Критерии успеха

- SC-001 Rate limiter overhead < 5ms на запрос при P99 latency (Valkey на localhost).
- SC-002 Rate limit проверка не добавляет дополнительных точек отказа в data plane при недоступности Valkey (fail-open).

## Краевые случаи

- Tenant без явной конфигурации лимитов — применяется default лимит.
- Token budget не указан для модели — по умолчанию unlimited (не проверяется).
- Запрос без tenant ID (неаутентифицированный) — rate limit не применяется, запрос пропускается.
- На 429-ответах возвращаются все rate-limit заголовки (X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset) с соответствующими значениями.
- Одновременные запросы от одного tenant — корректная атомарность через Valkey (MULTI/EXEC скрипт).
- Большое количество уникальных tenant — память Valkey (очистка старых ключей через TTL).
- Переполнение Valkey памяти — rate limiter fail-open.

## Открытые вопросы

- Какой алгоритм использовать для оценки токенов до отправки запроса, если LLM ещё не ответил? (pre-flight estimation vs post-flight deduction)
- Streaming: pre-flight проверка budget перед началом стрима (хотя бы 1 токен для пропуска); post-stream дедукция фактических токенов после завершения стрима. Блокировка mid-stream не применяется. Если пост-стрим дедукция превышает budget — последующие запросы блокируются.
- Какая стратегия при недоступности Valkey — fail-open (пропустить проверку) или fail-closed (отклонить все запросы)? Выбран fail-open как менее разрушительный для существующего трафика, но это требует обсуждения.
- Нужен ли Redis ACL/key-prefix для rate-limit ключей, чтобы отделить их от mask-кэша?
