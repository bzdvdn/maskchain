# Routing Engine — Провайдеры, модели, роутинг запросов

## Scope Snapshot

- In scope: domain-сущности Provider/Model/Route/RoutingRule, provider registry, RouteSelector (определение провайдера по модели с fallback), FallbackHandler, конфигурация роутинга в YAML, health-aware routing, прокси-хендлер с routing resolution на `POST /v1/chat/completions`.
- Out of scope: реализация клиентов конкретных LLM-провайдеров (OpenAI, Anthropic, Azure и т.д.), балансировка нагрузки между несколькими инстансами одного провайдера, кэширование ответов, rate limiting, mTLS и сертификаты для провайдеров.

## Цель

Разработчик интеграции настраивает YAML-конфигурацию, в которой модели (например, `gpt-4`, `claude-3-opus`) мапятся на одного или нескольких провайдеров с указанием fallback-порядка. При запросе `POST /v1/chat/completions` система определяет целевого провайдера по модели из запроса, проверяет его health, и при недоступности переключается на fallback-провайдера (если настроен). Успех фичи измеряется тем, что интеграционный тест с конфигурацией `gpt-4 → openai, fallback: azure-openai` при симулированной недоступности OpenAI успешно выполняет запрос через Azure OpenAI.

## Основной сценарий

1. Оператор конфигурирует routing rules в YAML: модель → список провайдеров с приоритетом и fallback.
2. Клиент отправляет `POST /v1/chat/completions` с полем `model: gpt-4`.
3. Shield middleware выполняет проверку контента (существующая логика).
4. Routing resolution: `RouteSelector` по модели `gpt-4` находит первичного провайдера (например, `openai`).
5. Health check: если провайдер здоров, запрос проксируется к нему.
6. Fallback: если провайдер недоступен (timeout, 5xx) или нездоров, `FallbackHandler` пробует следующего провайдера из списка.
7. Ответ возвращается клиенту с указанием провайдера, который фактически обработал запрос (header `X-Provider`).

## User Stories

- P1 Story: Как оператор, я хочу задать в YAML, что `gpt-4` идёт через OpenAI, а при недоступности — через Azure OpenAI, чтобы обеспечить отказоустойчивость без изменения кода клиента.
- P2 Story: Как оператор, я хочу указать для разных моделей разных провайдеров (например, `gpt-4 → openai`, `claude-3 → anthropic`) в единой конфигурации, чтобы избежать hardcoded маршрутов.

## MVP Slice

- Domain-сущности `Provider` (id, name, base_url, health_status), `Model`, `Route` (model + ordered provider list), `RoutingRule` (tenant_id + routes).
- `ProviderRegistry` — in-memory реестр провайдеров из конфига.
- `RouteSelector` — по модели возвращает первого здорового провайдера.
- `FallbackHandler` — при ошибке вызова пробует следующий провайдер.
- YAML-конфигурация провайдеров и routing rules.
- Health-aware routing: периодическая проверка health endpoint'ов провайдеров.
- Прокси-хендлер `POST /v1/chat/completions` с routing resolution (замена существующего stub).

## First Deployable Outcome

Один интеграционный тест (Go test), который конфигурирует два провайдера (primary + fallback), отправляет запрос `POST /v1/chat/completions` с моделью `gpt-4`, симулирует недоступность первичного провайдера и проверяет, что запрос выполнен через fallback-провайдера, а ответ содержит `X-Provider: azure-openai`.

## Scope

- `src/internal/domain/routing/` — новые entity: `Provider`, `Model`, `Route`, `RoutingRule`, `HealthStatus`.
- `src/internal/domain/routing/service/` — `ProviderRegistry`, `RouteSelector`, `FallbackHandler`.
- `src/internal/adapters/provider/` — stub/mock адаптеры для тестов (реальные клиенты провайдеров — вне scope).
- Конфигурация: новый блок `routing` в YAML (`providers`, `rules`).
- Прокси-хендлер: замена stub в `provider_handler.go` на реальную routing resolution.
- Health checker: периодическая проверка `/health` endpoint'ов провайдеров.
- Response header `X-Provider` с именем фактически использованного провайдера.

## Контекст

- Существующий `ShieldConfig.TenantModelMapping` предвосхищает tenant→model→provider маппинг, но не используется — routing engine будет его преемником.
- Прокси-хендлеры (`ProxyChatCompletionHandler`, `ProxyCompletionHandler`) сейчас — stub, возвращающие `"ok"`.
- Shield middleware уже перехватывает `/v1/chat/completions` и выполняет content scan перед проксированием.
- Tenant ID (`X-Tenant-ID`) уже резолвится в middleware — routing rules могут быть tenant-специфичными.
- Реальные HTTP-вызовы к провайдерам (OpenAI client, Anthropic client) не входят в scope — адаптеры провайдеров будут заглушками для тестов.

## Зависимости

- Зависит от существующего shield middleware (не изменяется, но сохраняется порядок: shield scan → routing → proxy).
- Использует `X-Tenant-ID` из существующей middleware для tenant-scoped routing.
- `none` внешних библиотек сверх Go stdlib + gin для прокси-запросов.

## Требования

- RQ-001 Система ДОЛЖНА поддерживать конфигурацию провайдеров через YAML-блок `routing.providers` с полями: `name`, `base_url`, `health_endpoint`, `timeout`, `priority`.
- RQ-002 Система ДОЛЖНА поддерживать routing rules в YAML-блоке `routing.rules`, где каждой модели (или tenant+model) сопоставлен упорядоченный список провайдеров.
- RQ-003 `RouteSelector` ДОЛЖЕН по модели возвращать первого здорового провайдера из списка, учитывая health status.
- RQ-004 `FallbackHandler` ДОЛЖЕН при ошибке запроса (timeout, 5xx, connection refused) к текущему провайдеру пробовать следующего из списка, пока не исчерпаны все.
- RQ-005 Если ни один провайдер для модели не здоров, система ДОЛЖНА возвращать HTTP 503 с телом `{"error": "no healthy provider for model <model>"}`.
- RQ-006 Если модель не найдена ни в одном routing rule, система ДОЛЖНА возвращать HTTP 400 с телом `{"error": "no route for model <model>"}`.
- RQ-007 Health checker ДОЛЖЕН периодически (по умолчанию каждые 30s) опрашивать `health_endpoint` каждого провайдера и обновлять его `HealthStatus`.
- RQ-008 Прокси-хендлер ДОЛЖЕН добавлять в HTTP-ответ header `X-Provider` с именем провайдера, фактически обработавшего запрос.
- RQ-009 Routing rules ДОЛЖНЫ поддерживать tenant-scoped маппинг: для разных `X-Tenant-ID` могут быть разные маршруты для одной и той же модели.
- RQ-010 Одна и та же модель ДОЛЖНА иметь возможность мапиться на разных провайдеров в разных tenant-контекстах или routing rules.

## Вне scope

- Реализация HTTP-клиентов для конкретных LLM-провайдеров (OpenAI, Anthropic, Azure, etc.) — только stub/mock адаптеры.
- Балансировка нагрузки (round-robin, weighted) между инстансами одного провайдера.
- Кэширование ответов LLM.
- Rate limiting и quota management.
- mTLS и кастомные TLS-сертификаты для провайдеров.
- UI для управления routing rules.
- Webhook-уведомления о смене health status провайдера.
- Динамическое обновление конфигурации без перезапуска (hot-reload).

## Критерии приемки

### AC-001 Провайдер определён по модели из routing rule

- Почему это важно: гарантирует, что RouteSelector корректно находит провайдера для заданной модели.
- **Given** конфигурация: провайдер `openai` (base_url: `https://api.openai.com`), routing rule: `gpt-4 → [openai]`
- **When** `RouteSelector.Select(ctx, "gpt-4")` вызван
- **Then** возвращается `Provider{Name: "openai", BaseURL: "https://api.openai.com"}`
- Evidence: unit-тест проверяет имя и base_url возвращённого провайдера

### AC-002 Fallback при недоступности первичного провайдера

- Почему это важно: обеспечивает отказоустойчивость при сбоях провайдера.
- **Given** конфигурация: провайдеры `openai` (недоступен), `azure-openai` (доступен); routing rule: `gpt-4 → [openai, azure-openai]`
- **When** `FallbackHandler.Call(ctx, "gpt-4", request)` вызван
- **Then** запрос выполнен через `azure-openai`, ответ содержит `X-Provider: azure-openai`
- Evidence: интеграционный тест с mock-серверами, симулирующими недоступность `openai`; проверка header `X-Provider`

### AC-003 HTTP 503 когда ни один провайдер для модели не здоров

- Почему это важно: клиент должен получить детерминированный ответ, а не бесконечный retry.
- **Given** все провайдеры для модели `gpt-4` помечены как unhealthy
- **When** `POST /v1/chat/completions` с `{"model": "gpt-4"}` отправлен
- **Then** ответ HTTP 503 с `{"error": "no healthy provider for model gpt-4"}`
- Evidence: тест проверяет status code 503 и тело ответа

### AC-004 HTTP 400 для модели без routing rule

- Почему это важно: клиент должен сразу узнать, что модель не поддерживается.
- **Given** конфигурация не содержит routing rule для модели `unknown-model`
- **When** `POST /v1/chat/completions` с `{"model": "unknown-model"}` отправлен
- **Then** ответ HTTP 400 с `{"error": "no route for model unknown-model"}`
- Evidence: тест проверяет status code 400 и тело ответа

### AC-005 Tenant-scoped routing: разные провайдеры для одной модели в разных tenant'ах

- Почему это важно: multi-tenant сценарий, где у каждого tenant'а свои провайдеры.
- **Given** конфигурация: tenant `alpha`: `gpt-4 → [openai]`; tenant `beta`: `gpt-4 → [azure-openai]`
- **When** запрос с `X-Tenant-ID: beta` и `{"model": "gpt-4"}` отправлен
- **Then** запрос направлен к `azure-openai`, ответ содержит `X-Provider: azure-openai`
- Evidence: тест с tenant-заголовками проверяет корректный выбор провайдера

### AC-006 Health checker обновляет статус провайдера

- Почему это важно: автоматическое исключение недоступных провайдеров из роутинга.
- **Given** провайдер `openai` с health_endpoint, возвращающим 200
- **When** `HealthChecker.Check()` выполнен
- **Then** провайдер `openai` помечен как healthy
- **When** health_endpoint начинает возвращать 503
- **Then** после следующего `HealthChecker.Check()` провайдер помечен как unhealthy
- Evidence: unit-тест с mock HTTP server, проверка HealthStatus после check

### AC-007 FallbackHandler исчерпывает всех провайдеров перед 503

- Почему это важно: fallback должен перебрать всех провайдеров, а не только первый fallback.
- **Given** три провайдера: `p1` (недоступен), `p2` (недоступен), `p3` (доступен); rule: `gpt-4 → [p1, p2, p3]`
- **When** `FallbackHandler.Call(ctx, "gpt-4", request)` вызван
- **Then** попытки сделаны к p1, p2, p3; успешный ответ от p3
- Evidence: unit-тест проверяет, что все три провайдера были вызваны (call count на mock'ах)

## Допущения

- Конфигурация провайдеров и routing rules загружается при старте из YAML и не меняется до перезапуска (hot-reload не требуется в MVP).
- Health endpoint провайдера соответствует простому контракту: HTTP 200 = healthy, любой другой статус или timeout = unhealthy.
- Tenant ID извлекается из header `X-Tenant-ID` (существующая логика).
- Прокси-запросы к провайдерам выполняются через HTTP/HTTPS (Go `net/http`).
- Реальные клиенты LLM-провайдеров (формирование запросов, обработка ответов, stream) будут реализованы в следующих фазах; на этой фазе — stub/mock, возвращающие фиксированный ответ.

## Критерии успеха

- SC-001 Routing resolution (выбор провайдера, health check) добавляет < 5ms latency к запросу.
- SC-002 Health checker завершает полный цикл опроса всех провайдеров за < 1s.

## Краевые случаи

- Конфигурация без routing rules — система принимает запросы, но любой запрос к модели без правила возвращает 400.
- Провайдер без health_endpoint — считается всегда healthy (skip health check).
- FallbackHandler: все провайдеры недоступны — 503 после исчерпания списка.
- FallbackHandler: часть провайдеров возвращает не-LLM ошибки (4xx) — такие ошибки не вызывают fallback (только 5xx, timeout, connection refused).
- Модель присутствует в правиле, но все провайдеры исключены из конфига (опечатка в имени) — 503.

## Открытые вопросы

- Должен ли FallbackHandler поддерживать частичный успех (например, стриминг начался, но провайдер упал)? Решение отложено до plan — в MVP только non-streaming.
- Должен ли health checker логировать смену статуса? Да, но уровень детализации — на plan.

Готово к: /spk.inspect 70-routing-engine
