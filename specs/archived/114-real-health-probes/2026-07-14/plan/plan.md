# Dependency-aware health/readiness probes — План

## Phase Contract

Inputs: spec (pass), inspect (pass).
Outputs: plan.md, data-model.md.
Stop if: неоднозначность по egress probe не блокирует — закрывается в DEC-001.

## Цель

Замена статического `healthHandler` на probe-based систему: новый пакет `internal/api/health` с интерфейсом `Probe`, конкретными реализациями для PG, Valkey и egress, и агрегирующим `Service`, который подключается в `Server`/`AdminServer` через DI.

## MVP Slice

- Шаг 1: Конфиг `server.health_check.critical_deps` + Probe interface + `health.Service` + mock-тесты (AC-006, AC-007).
- Шаг 2: Статические /health, /live через `health.Service` без проверок (AC-001, AC-005, AC-008).
- Шаг 3: Конкретные probes: PG (SELECT 1), Valkey (PING), egress (TCP dial) — (AC-002, AC-003, AC-004).

## First Validation Path

```bash
# mock: все probes ok
curl -s http://localhost:8080/ready | jq .status  # "ok"
# mock: valkey down
curl -s http://localhost:8080/ready | jq .status  # "degraded"
# mock: pg down
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/ready  # 503
```

## Scope

- `internal/api/health/` — новый пакет: Probe interface, конкретные реализации, Service (aggregator), handler bridge.
- `internal/infra/config/config.go` — `HealthCheckConfig` + `CriticalDeps []string` в `ServerConfig`.
- `internal/api/server.go` + `internal/api/admin.go` — замена `healthHandler` на вызов `health.Service`.
- `cmd/gateway/main.go` + `cmd/admin/main.go` — регистрация probes и передача в конструктор сервера.
- Не трогается: middleware/auth, domain-модели PG/Valkey, provider HealthChecker в routing.

## Performance Budget

- `none` — пробы вызываются синхронно, latency включается в ответ и ожидаемо равна времени ping. SC-001 (100ms) — deploy-time, не код-тайм бюджет.

## Implementation Surfaces

| Surface | Роль | Статус |
|---|---|---|
| `internal/api/health/probe.go` | Интерфейс Probe, тип Result | новый |
| `internal/api/health/service.go` | HealthService: регистрация + CheckAll (status агрегация) | новый |
| `internal/api/health/handler.go` | Gin handler, читающий service | новый |
| `internal/api/health/probes.go` | Конкретные probe-реализации (PG, Valkey, egress) | новый |
| `internal/infra/config/config.go` | `HealthCheckConfig` + `ServerConfig.HealthCheck` | изменение |
| `internal/api/server.go` | Принять `*health.Service` через New | изменение |
| `internal/api/admin.go` | Принять `*health.Service` через NewAdminServer | изменение |
| `cmd/gateway/main.go` | Создать probes, зарегистрировать в health.Service, передать в New | изменение |
| `cmd/admin/main.go` | Аналогично gateway | изменение |

## Bootstrapping Surfaces

- `internal/api/health/` — новая директория, до реализации handler-ов.

## Влияние на архитектуру

- Локальное: новый пакет `health` под `api/`. Не нарушает DDD/Clean Architecture — это инфра-слой для API.
- Интеграции: `Server` и `AdminServer` получают новый аргумент конструктора (`*health.Service`). Обратная совместимость: не требуется (private конструктор, меняется только `cmd/`).
- No migration, no feature flag — замена статики на динамику без rollout-риска (старые ответы сохраняют структуру `{"status":"ok"}` для /health и /live).

## Acceptance Approach

| AC | Подход | Surfaces | Наблюдение |
|---|---|---|---|
| AC-001 | health handler возвращает `{status:ok}` без вызова probes | `handler.go`, `service.go` | curl /health |
| AC-002 | Все три probes зарегистрированы, CheckAll вызывает каждый, агрегация ok | `service.go`, `probes.go` | curl /ready, jq .status |
| AC-003 | Valkey probe возвращает down, PG ok → degraded | `service.go`, `probes.go` | curl /ready, jq .status |
| AC-004 | PG probe возвращает down → 503, status down | `service.go`, `probes.go` | curl -w "%{http_code}" /ready |
| AC-005 | /live handler возвращает 200 после New() (deps уже в DI) | `handler.go` | curl /live |
| AC-006 | CriticalDeps из конфига влияет на down/degraded порог | `service.go`, `config.go` | два curl с разным конфигом |
| AC-007 | Единая JSON-структура {status, checks: {name: {status, latency_ms}}} | `service.go`, `handler.go` | jq eval |
| AC-008 | Auth bypass сохранён (publicPaths уже содержит пути) | middleware не трогается | curl без headers |

## Данные и контракты

- Новых persisted сущностей нет. Ответ health probes — transient JSON, не хранится.
- Конфиг: новая nested struct `HealthCheckConfig` в `ServerConfig`.
- API response — новый JSON-контракт (документирован в spec AC-007).
- `data-model.md`: no-change stub прилагается.

## Стратегия реализации

### DEC-001 Probe interface вместо callback-функций

Why: единый контракт позволяет регистрировать любую зависимость без модификации `Service`. Альтернатива (слайс функций `func(ctx) Result`) — короче, но теряет именование и возможность добавлять метаданные (critical flag).

Tradeoff: один интерфейс с одним методом — минимальная индирекция.

Affects: `internal/api/health/probe.go`, `service.go`, `probes.go`.

Validation: unit-тест с mock Probe.

### DEC-002 Probes живут рядом с адаптерами (api/health), не в domain

Why: PG pool, Valkey client — infra-объекты, не domain entity. Probe не содержит бизнес-логики. Domain/routing уже имеет свой HealthChecker для провайдеров — не смешивать.

Tradeoff: probes не переиспользуются между разными агрегаторами (но их и нет).

Affects: `internal/api/health/probes.go`.

Validation: пакет не импортирует domain-слои, кроме типов данных для маршрутов (egress).

### DEC-003 Синхронные probes без кэширования

Why: spec явно требует свежее состояние на каждый запрос /ready. Кэширование добавило бы race condition между probe-результатом и реальным состоянием.

Tradeoff: каждый /ready — latency суммы всех probe-запросов. При SC-001 (100ms) приемлемо.

Affects: `service.go`.

Validation: два последовательных /ready с разным состоянием PG (up/down) возвращают разные статусы.

### DEC-004 Egress probe: TCP dial к BaseURL провайдеров

Why: проверка, что исходящие соединения проходят (proxy, FW). Не HTTP health-endpoint — это задача provider HealthChecker в routing.

Tradeoff: не проверяет полный HTTP handshake, только транспорт.

Affects: `probes.go` получает список `[]string` (host:port баз провайдеров).

Validation: egress probe возвращает ok если хотя бы один TCP dial успешен.

## Incremental Delivery

### MVP (Шаг 1 + Шаг 2)

- `Probe` interface, `Result`, `Service` (регистрация + CheckAll + агрегация).
- `HealthCheckConfig` + `critical_deps` в конфиг.
- Gin handler, использующий `Service`.
- mock-probes в тестах.
- /health и /live через service (без регистрации реальных probes).
- AC-001, AC-005, AC-006, AC-007, AC-008.

### Итеративное расширение (Шаг 3)

- Конкретные probe-реализации: `PGProbe`, `ValkeyProbe`, `EgressProbe`.
- Регистрация в `main.go`.
- /ready с dynamic status.
- AC-002, AC-003, AC-004.

## Порядок реализации

1. `internal/api/health/probe.go` + `service.go` — интерфейс и агрегатор (база, от которой всё зависит).
2. `config.go` — `HealthCheckConfig` (нужен для AC-006 на этапе тестов).
3. `handler.go` — Gin handler; временно без реальных probes, только /health, /live.
4. `server.go` + `admin.go` — подключение `*health.Service` через конструктор.
5. `main.go` (gateway + admin) — создание сервиса, передача в конструктор.
6. `probes.go` — три конкретные реализации.
7. Регистрация probes в `main.go`.
8. Unit-тесты с mock probes.

Параллельно: 1-3 → 4-5 → 6-8.

## Риски

- PG pool может быть `nil` (если не настроен) — probe должен корректно возвращать "not configured" ok, не паниковать.
  Mitigation: probe принимает `*pgxpool.Pool`, nil-check в `Check()`.
- `/ready` latency == sum latency всех probes. При трёх probes с таймаутом 5s каждый сумма 15s — k8s probe timeout может сработать.
  Mitigation: timeout per-probe (net.Dialer.Timeout), общий контекст с дедлайном от запроса.
- egress probe требует список провайдеров из RoutingConfig — если routing не настроен, список пуст.
  Mitigation: пустой список → egress probe не регистрируется.

## Rollout and compatibility

- Обратная совместимость: /health и /live возвращают тот же `{"status":"ok"}`. /ready раньше возвращал `{"status":"ok"}`, теперь — структурированный JSON с checks. Потенциальные consumer-ы (k8s probes) читают только HTTP status code, для них изменений нет.
- Специальных rollout-действий не требуется.

## Проверка

- Unit-тесты: mock Probe → проверка агрегации (ok/degraded/down), проверка critical_deps порога, проверка latency_ms.
- Integration-тесты: реальные PG/Valkey (опционально, через `testcontainers` или existing test pool).
- Manual: curl три эндпоинта, валидация JSON-структуры через jq.
- Каждый AC привязан к Evidence в spec — те же curl/jq команды.

## Соответствие конституции

- нет конфликтов. Feature не затрагивает: Content Shield core domain, data model, React UI, Envoy path, DDD layers.
