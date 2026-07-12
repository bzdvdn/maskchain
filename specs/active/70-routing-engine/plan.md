# Routing Engine — План

## Phase Contract

Inputs: spec, inspect (pass), minimal repo-контекст (domain entity patterns, config, API surfaces).
Outputs: plan.md, data-model.md.
Stop if: spec неоднозначна — нет, inspect pass.

## Цель

Реализовать domain-слой роутинга (Provider, Route, RoutingRule, ProviderRegistry, RouteSelector, FallbackHandler), YAML-конфигурацию провайдеров и routing rules, health-aware routing и замену proxy stub на routing resolution в `POST /v1/chat/completions`.

Подход: новый domain-пакет `routing/` по DDD-шаблону проекта; in-memory registry без БД; stub/mock адаптеры провайдеров для тестов.

## MVP Slice

- Domain entities + ProviderRegistry + RouteSelector + FallbackHandler
- YAML routing config (`routing.providers` + `routing.rules`) в существующем `Config`
- HealthChecker с периодическим опросом
- Proxy handler с routing resolution
- AC-001, AC-002, AC-003, AC-004, AC-006, AC-007

## First Validation Path

Один `go test` в пакете `domain/routing/service`: конфигурация с 2 провайдерами (primary + fallback), RouteSelector возвращает primary, FallbackHandler при ошибке primary переключается на fallback. Проверка: возвращён правильный Provider, заголовок X-Provider.

## Scope

- `src/internal/domain/routing/` — новая domain-пакет (entities + services)
- `src/internal/ports/provider.go` — новый port interface (outbound)
- `src/internal/api/provider_handler.go` — замена stub на routing handler
- `src/internal/api/server.go` — DI wiring routing -> handler (via config)
- `src/internal/infra/config/config.go` — добавление `RoutingConfig`
- `src/internal/adapters/provider/` — mock/stub для тестов (не production-клиенты)

Не затрагивается: shield middleware, существующие domain-пакеты (shield/), UI, БД.

## Performance Budget

- Routing resolution + health lookup: < 1ms p99 (in-memory map access)
- Health checker full cycle: < 1s p95 (SC-002 spec)
- `none` особых memory/alloc лимитов для MVP

## Implementation Surfaces

| Surface | Тип | Зачем |
|---|---|---|
| `src/internal/domain/routing/` | NEW | Provider, Route, RoutingRule entities |
| `src/internal/domain/routing/service/` | NEW | ProviderRegistry, RouteSelector, FallbackHandler, HealthChecker |
| `src/internal/ports/provider.go` | NEW | Interface ProviderClient (stub-ready) |
| `src/internal/infra/config/config.go` | MODIFY | +RoutingConfig, +ProviderConfig, +RuleConfig |
| `src/internal/api/provider_handler.go` | MODIFY | Замена stub на routing handler |
| `src/internal/api/server.go` | MODIFY | Прокидывание routing dependencies в handler |
| `src/internal/adapters/provider/stub.go` | NEW | Stub provider client для тестов |

## Bootstrapping Surfaces

- `src/internal/domain/routing/` (entities)
- `src/internal/domain/routing/service/` (services)
- `src/internal/ports/provider.go`

Без них нельзя начать реализацию поведения.

## Влияние на архитектуру

- Локально: новый domain-пакет с внутренними зависимостями (Registry → Provider, Selector → Registry, Fallback → Selector).
- Интеграции: proxy handler теперь зависит от routing services (через DI/конструктор).
- Config: новый блок `routing` в корневой конфиг, парсится стандартным viper-пайплайном.
- Обратная совместимость: config опционален; если routing не настроен — proxy handler возвращает 400 на любой запрос.

## Acceptance Approach

| AC | Подход | Surfaces |
|---|---|---|
| AC-001 | Unit: ProviderRegistry из конфига, RouteSelector.Select(model) → Provider | domain/routing/service, domain/routing/entities |
| AC-002 | Integration: mock HTTP servers, FallbackHandler.Call → X-Provider header | domain/routing/service, adapters/provider/stub |
| AC-003 | Integration: все провайдеры unhealthy → HTTP 503 через proxy handler | api/provider_handler, domain/routing/service |
| AC-004 | Integration: неизвестная модель → HTTP 400 | api/provider_handler |
| AC-005 | Integration: tenant header → разный выбор провайдера | api/provider_handler + domain/routing/service (tenant context propagation) |
| AC-006 | Unit: HealthChecker.Check меняет статус провайдера | domain/routing/service, domain/routing/entities |
| AC-007 | Unit: FallbackHandler вызывает всех провайдеров по порядку | domain/routing/service |

## Данные и контракты

- Data model changes: см. `data-model.md`.
- API контракты: `POST /v1/chat/completions` сохраняет существующий request/response формат; добавляется header `X-Provider` в response; ошибки 400/503 — новые статусы на этом пути.
- Config контракты: новый блок `routing` в YAML; если блок отсутствует — роутинг неактивен (400 для любого запроса).
- `data-model.md` прилагается.

## Стратегия реализации

- DEC-001 In-memory ProviderRegistry без БД
  Why: Конфиг-based routing — единственное требование spec; БД добавит миграции, sync и сложность без пользы для MVP.
  Tradeoff: Конфигурация live только до перезапуска; hot-reload отложен (вне scope).
  Affects: domain/routing/service, infra/config
  Validation: ProviderRegistry загружается из Config и возвращает провайдеров без внешних хранилищ.

- DEC-002 RouteSelector — stateless (чистая функция от Registry и model)
  Why: Не хранит состояние, легко тестировать; вся мутабельность — в HealthStatus провайдера.
  Tradeoff: Каждый запрос делает lookup в map — O(1), незначительно.
  Affects: domain/routing/service
  Validation: Select(&registry, "gpt-4") возвращает Provider без side-эффектов.

- DEC-003 FallbackHandler получает список провайдеров от Selector и перебирает
  Why: Selector отвечает только за первый healthy; Fallback итерирует по списку, вызывая каждого.
  Tradeoff: Дублирование логики "skip unhealthy" в Selector и Fallback (первый healthy ≠ первый в списке).
  Affects: domain/routing/service
  Validation: Fallback при ошибке первого пробует второго, третьего.

- DEC-004 HealthStatus — mutex-protected поле на Provider entity
  Why: Provider — естественный владелец статуса; HealthChecker обновляет его через метод SetHealth().
  Tradeoff: Provider становится mutable; конкурентный доступ требует sync.RWMutex.
  Affects: domain/routing/entities
  Validation: HealthChecker.Check() → provider.Health() отражает новый статус.

- DEC-005 proxy handler принимает routing services через functional options
  Why: Сохраняет существующий паттерн регистрации роутов; routing опционален.
  Tradeoff: Handler не может быть полностью построен без routing config.
  Affects: api/provider_handler.go, api/server.go
  Validation: Server без routing config — proxy возвращает 400.

## Incremental Delivery

### MVP (Первая ценность)

1. Domain entities + ProviderRegistry (config-driven) + RouteSelector
2. FallbackHandler (перебор провайдеров по списку)
3. HealthChecker (периодический опрос, обновление статуса)
4. Proxy handler с routing resolution
5. AC-001, AC-002, AC-003, AC-004, AC-006, AC-007

Критерий: интеграционный тест с двумя mock-провайдерами, fallback при недоступности primary, возврат X-Provider.

### Итеративное расширение

- Tenant-scoped routing (AC-005): добавление tenant контекста в RouteSelector и правила. Может быть отложено до второй итерации, если MVP без tenant-разделения.
- Completion endpoint (не chat): `POST /v1/completions` — по той же схеме.

## Порядок реализации

1. **Domain entities** — Provider, Route, RoutingRule (типы данных, без логики)
2. **ProviderRegistry** — загрузка из Config, хранение в map
3. **RouteSelector** — выбор по модели, учёт HealthStatus
4. **FallbackHandler** — перебор, проброс ошибок
5. **HealthChecker** — ticker, HTTP health check, обновление статуса
6. **Config** — RoutingConfig, мост в существующий Config
7. **Proxy handler** — замена stub, DI wiring
8. **Тесты** — unit + integration (покрытие всех AC)

Шаги 1–5 можно параллелить с шагом 6 (config), так как типы независимы.
Шаги 7–8 последовательны — handler зависит от готовых сервисов.

## Риски

- Риск: HealthChecker с реальными HTTP вызовами замедлит тесты
  Mitigation: HealthChecker принимает `http.Client` interface; тесты используют mock-серверы.

- Риск: Tenant-scoped routing усложняет RouteSelector
  Mitigation: Отложить AC-005 до второй итерации, если MVP без tenant-разделения достаточен.

- Риск: FallbackHandler не отличает 4xx (ошибка клиента) от 5xx (ошибка провайдера)
  Mitigation: Spec явно specifies — fallback только на 5xx, timeout, connection refused; 4xx не вызывают fallback.

## Rollout и compatibility

- Config опционален: без `routing` блока proxy возвращает 400 — поведение совместимо с отсутствием фичи.
- Новый header `X-Provider` не breaking change.
- Существующий shield middleware не меняется.
- Rollout: достаточно добавить routing block в config.yaml.

## Проверка

- Unit-тесты: ProviderRegistry, RouteSelector (mock HealthStatus), FallbackHandler (mock provider client), HealthChecker (mock HTTP)
- Integration-тесты: proxy handler с mock-серверами провайдеров, симуляция 5xx/timeout
- AC-001..AC-007 покрыты тестами
- DEC-001..DEC-005 валидированы через unit-тесты соответствующих компонентов

## Соответствие конституции

- нет конфликтов.

Готово к: /spk.tasks 70-routing-engine
