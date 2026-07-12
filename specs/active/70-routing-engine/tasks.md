# Routing Engine — Задачи

## Phase Contract

Inputs: plan.md, data-model.md, spec.md.
Outputs: упорядоченные исполнимые задачи с покрытием всех AC.
Stop if: задачи расплывчаты — нет.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/domain/routing/` | T1.1 |
| `src/internal/domain/routing/service/` | T2.1, T2.2, T2.3, T2.4 |
| `src/internal/ports/provider.go` | T1.3 |
| `src/internal/infra/config/config.go` | T1.2 |
| `src/internal/api/provider_handler.go` | T3.1, T3.2, T3.3 |
| `src/internal/api/server.go` | T3.2 |
| `src/internal/adapters/provider/stub.go` | T1.3, T2.3 |
| `src/internal/domain/routing/*_test.go` | T4.1 |
| `src/internal/api/provider_handler_test.go` | T4.2 |

## Implementation Context

- Цель MVP: in-memory routing engine с provider registry, route selector, fallback handler, health checker и заменой proxy stub на routing resolution для `POST /v1/chat/completions`
- Инварианты:
  - ProviderRegistry загружается из Config при старте, immutable до перезапуска (DEC-001)
  - RouteSelector — stateless, возвращает первого healthy провайдера из упорядоченного списка (DEC-002)
  - FallbackHandler перебирает провайдеров по списку, реагирует только на 5xx/timeout/connection refused (4xx не вызывают fallback)
  - HealthStatus — mutex-protected поле на Provider, HealthChecker периодически обновляет (DEC-004)
  - Config опционален: без `routing` блока все запросы к proxy → 400
- Ошибки/коды:
  - `ErrNoRoute` → HTTP 400 `{"error": "no route for model <model>"}`
  - `ErrNoHealthyProvider` → HTTP 503 `{"error": "no healthy provider for model <model>"}`
- Контракты:
  - `POST /v1/chat/completions` — существующий request/response формат + header `X-Provider` в response
  - YAML config: `routing.providers[].{name,base_url,health_endpoint,timeout,priority}`, `routing.rules[].{tenant,routes[].{model,providers[]}}`
  - `github.com/bzdvdn/maskchain/src/internal/domain/routing/service.ProviderRegistry` → `RouteSelector` → `FallbackHandler` → внешний HTTP вызов
- Границы scope:
  - Не реализуем production клиенты провайдеров (OpenAI, Anthropic и т.д.) — только stub/mock
  - Не делаем tenant-scoped routing (AC-005) — scaffolding в T3.3, полная реализация в следующей итерации
- Proof signals:
  - Unit-тест: RouteSelector возвращает правильного провайдера по модели
  - Unit-тест: HealthChecker обновляет статус
  - Integration-тест: fallback при недоступности primary, header X-Provider
- References: DEC-001..DEC-005, DM (Provider/Route/RoutingRule/HealthStatus), RQ-001..RQ-010

## Фаза 1: Foundation

Цель: domain entities, config structs, port interface — bootstrapping surfaces для всех последующих задач.

- [x] T1.1 Создать domain entities и value types в `src/internal/domain/routing/`
  - Provider (Name, BaseURL, HealthEndpoint, Timeout, Priority, HealthStatus c sync.RWMutex), Route (Model, Providers []string), RoutingRule (TenantID, Routes)
  - HealthStatus enum (Unknown/Healthy/Unhealthy) как typed string или int
  - Constructor-функции и accessor-методы (паттерн как в domain/shield/entity)
  - Touches: `src/internal/domain/routing/health_status.go`, `src/internal/domain/routing/route.go`, `src/internal/domain/routing/routing_rule.go`

- [x] T1.2 Добавить RoutingConfig в `src/internal/infra/config/`
  - ProviderConfig: Name, BaseURL, HealthEndpoint, Timeout (time.Duration), Priority
  - RuleConfig: Tenant (string), Routes ([]RouteConfig)
  - RouteConfig: Model (string), Providers ([]string)
  - RoutingConfig: Providers ([]ProviderConfig), Rules ([]RuleConfig)
  - Поле `Routing *RoutingConfig` с тегами `mapstructure:"routing" yaml:"routing"` на Config
  - Touches: `src/internal/infra/config/config.go`

- [x] T1.3 Создать port interface ProviderClient и stub адаптер
  - Interface в `src/internal/ports/provider.go`: `Call(ctx, request) (response, error)` — минимальный контракт для прокси
  - Stub в `src/internal/adapters/provider/stub.go`: возвращает фиксированный ответ или ошибку по конфигурации (для тестов)
  - Touches: `src/internal/ports/provider.go`, `src/internal/adapters/provider/stub.go`

## Фаза 2: Core Services

Цель: ProviderRegistry, RouteSelector, FallbackHandler, HealthChecker — основная domain-логика роутинга.

- [x] T2.1 Реализовать ProviderRegistry
  - NewProviderRegistry(routingConfig) загружает провайдеров из конфига в `map[string]*Provider`
  - Get(name string) *Provider — lookup по имени
  - List() []*Provider — все провайдеры (для HealthChecker)
  - GetHealthy(model string, routeSelector) *Provider — делегирует RouteSelector (или объединить с ним)
  - Touches: `src/internal/domain/routing/service/registry.go`

- [x] T2.2 Реализовать RouteSelector
  - Select(registry, model string, tenantID string) *Provider — возвращает первого healthy провайдера по model
  - Ищет RoutingRule по tenantID (или "default"), затем Route по model
  - Пропускает провайдеров с HealthStatus != Healthy
  - Возвращает nil + ErrNoRoute / ErrNoHealthyProvider
  - Touches: `src/internal/domain/routing/service/selector.go`

- [x] T2.3 Реализовать FallbackHandler
  - Call(ctx, providers []string, request) (response, providerName, error) — вызывает каждого провайдера по порядку
  - Классифицирует ошибки: 5xx/timeout/connection refused → fallback; 4xx → немедленный возврат
  - ProviderName в ответе для X-Provider header
  - Использует ProviderClient interface (из ports)
  - Touches: `src/internal/domain/routing/service/fallback.go`

- [x] T2.4 Реализовать HealthChecker
  - Start(ctx, interval) запускает ticker с goroutine
  - check() обходит всех провайдеров из Registry, вызывает health_endpoint, обновляет Provider.HealthStatus
  - Провайдеры без health_endpoint пропускаются (считаются Healthy)
  - Graceful shutdown через ctx cancellation
  - Touches: `src/internal/domain/routing/service/health.go`

## Фаза 3: Интеграция

Цель: замена proxy stub на routing resolution, DI wiring в server.go, scaffolding для tenant-scoped routing.

- [x] T3.1 Заменить ProxyChatCompletionHandler stub на routing handler
  - Handler принимает зависимость: RouteSelector + FallbackHandler + ProviderRegistry
  - Из gin.Context извлекает model из body, tenant ID из header `X-Tenant-ID`
  - RouteSelector.Select → FallbackHandler.Call → response с X-Provider
  - ErrNoRoute → 400, ErrNoHealthyProvider → 503
  - Touches: `src/internal/api/provider_handler.go`

- [x] T3.2 Выполнить DI wiring в server.go и main.go
  - Server.RegisterProxyRoute принимает routing dependencies (или handler-фабрику)
  - В main.go/wiring: создать Config → load RoutingConfig → ProviderRegistry → RouteSelector → FallbackHandler → HealthChecker → handler
  - Touches: `src/internal/api/server.go`, `src/cmd/gateway/main.go`

- [x] T3.3 Добавить scaffolding для tenant-scoped routing
  - RouteSelector принимает tenantID string (уже в сигнатуре из T2.2)
  - RoutingRule поддерживает TenantID (уже в entity из T1.1)
  - Config rules секция содержит tenant поле (уже в T1.2)
  - Полная реализация tenant-диспетчеризации для разных tenant'ов отложена; scaffolding обеспечивает, что tenant ID пробрасывается и может быть использован
  - Touches: `src/internal/api/provider_handler.go`

## Фаза 4: Проверка

Цель: automated tests подтверждают все AC, фича в reviewable состоянии.

- [x] T4.1 Добавить unit-тесты для domain сервисов
  - ProviderRegistry: загрузка из конфига, Get, List
  - RouteSelector: выбор по модели, пропуск unhealthy, ErrNoRoute, ErrNoHealthyProvider
  - FallbackHandler: перебор провайдеров, fallback на 5xx, не-fallback на 4xx
  - HealthChecker: обновление статуса через mock HTTP server
  - Touches: `src/internal/domain/routing/service/service_test.go`

- [x] T4.2 Добавить integration-тест для proxy handler
  - Mock HTTP servers для primary и fallback провайдеров
  - Симуляция 5xx/timeout на primary → fallback → X-Provider header
  - Симуляция неизвестной модели → 400
  - Симуляция всех unhealthy → 503
  - Touches: `src/internal/api/provider_handler_test.go`

## Покрытие критериев приемки

- AC-001 → T2.1, T2.2, T4.1
- AC-002 → T2.3, T4.2
- AC-003 → T3.1, T4.2
- AC-004 → T3.1, T4.2
- AC-005 → T3.3 (scaffolding; полная реализация deferred)
- AC-006 → T2.4, T4.1
- AC-007 → T2.3, T4.1

Готово к: /spk.implement 70-routing-engine
