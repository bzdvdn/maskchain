# Observability: OpenTelemetry, Prometheus, Structured Logging, Distributed Tracing — План

## Phase Contract

Inputs: spec, inspect (pass), repo-контекст (config, server, middleware, docker-compose, go.mod).
Outputs: plan, data-model (no-change).
Stop if: spec не позволяет безопасно спланировать — stop conditions не выполнены.

## Цель

Добавить в gateway три новых инфраструктурных пакета (`telemetry`, `metrics`, `logging`) без рефакторинга существующего кода; обогатить существующий middleware OTel-спанами и Prometheus-метриками; добавить docker-compose сервисы для сбора метрик и трейсов.

## MVP Slice

OTel SDK init + Prometheus `/metrics` + Gin-мидлварь trace propagation + request duration histogram. Закрывает AC-001, AC-002, AC-003, AC-005, AC-006.

## First Validation Path

```bash
go run ./src/cmd/gateway &
sleep 1
curl -s localhost:8080/health
curl -s localhost:8080/metrics | grep maskchain_
kill %1
# Проверить: метрики с префиксом maskchain_, лог с trace_id
```

## Scope

- `src/internal/infra/telemetry/` — новый пакет: OTel SDK init, TracerProvider, MeterProvider, OTLP exporter, graceful shutdown
- `src/internal/infra/metrics/` — новый пакет: Prometheus counters/histograms, registry, `/metrics` handler
- `src/internal/infra/logging/` — новый пакет: slog adapter с OTel span enrichment
- `src/internal/api/server.go` — добавить otelgin middleware, подключить `/metrics` route
- `src/internal/api/middleware/shield.go` — добавить span attributes, shield-specific метрики
- `src/internal/infra/config/config.go` — добавить `OtelConfig`
- `src/cmd/gateway/main.go` — вызвать initTelemetry, shutdown при SIGTERM
- `examples/docker-compose.yml` — добавить Prometheus + Grafana
- `go.mod` — новые зависимости OTel SDK, Prometheus client, otelgin

## Performance Budget

- SC-001: overhead OTel SDK < 5ms p99 на запрос
- SC-002: `/metrics` ответ < 100ms при 100 метриках
- `none` для memory budget — метрики и трейсы не аллоцируют значимых объёмов (регистрация единожды)

## Implementation Surfaces

| Surface | Статус | Роль |
|---------|--------|------|
| `src/internal/infra/telemetry/` | новый | Init OTel SDK, TracerProvider, MeterProvider, OTLP exporter, shutdown |
| `src/internal/infra/metrics/` | новый | Prometheus metric definitions, registry, HTTP handler |
| `src/internal/infra/logging/` | новый | slog adapter с OTel fields |
| `src/internal/infra/config/config.go` | существующий | добавить OtelConfig (endpoint, service_name, environment, sampling_ratio) |
| `src/internal/api/server.go` | существующий | подключить otelgin middleware, `/metrics` handler |
| `src/internal/api/middleware/shield.go` | существующий | instrument span + shield metrics |
| `src/cmd/gateway/main.go` | существующий | wire initTelemetry, graceful shutdown |
| `examples/docker-compose.yml` | существующий | добавить Prometheus + Grafana |

## Bootstrapping Surfaces

- `src/internal/infra/telemetry/`, `src/internal/infra/metrics/`, `src/internal/infra/logging/` — создать первыми, они независимы от остального кода

## Влияние на архитектуру

- Config: новая секция `otel` (endpoint, service_name, environment, sampling_ratio)
- Server: два новых middleware (otelgin + metrics) до существующих middleware
- Main: новый вызов `initTelemetry()` + `shutdown` в defer
- Shield middleware: не изменяет логику, только добавляет span attributes и метрики
- docker-compose: новые сервисы, не влияющие на существующие

## Acceptance Approach

- **AC-001**: `telemetry/` init с OTLP gRPC exporter; unit test через `otlptracetest`
- **AC-002**: `metrics/` пакет регистрирует метрики с namespace `maskchain`; handler на `/metrics`
- **AC-003**: metrics middleware оборачивает gin ctx, записывает duration после c.Next()
- **AC-004**: shield middleware после shield scan записывает метрики через metrics пакет
- **AC-005**: `logging/` adapter читает span из context, добавляет trace_id/span_id в slog record
- **AC-006**: shutdown func из initTelemetry вызывается в main.Shutdown
- **AC-007**: initTelemetry логирует warning при недоступном endpoint, возвращает noop-провайдер
- **AC-008**: docker-compose сервисы с корректным scrape config

## Данные и контракты

Data model не меняется. Все изменения — инфраструктурные, без новых entity/value objects.
См. `data-model.md`.

## Стратегия реализации

### DEC-001 Snake_case для имён метрик

Why: стандарт Prometheus, совместимость с best practices и стандартными дашбордами.
Tradeoff: несовместимо с potential OpenTelemetry semantic conventions (но OTel semconv рекомендует underscore).
Affects: `metrics/` пакет — все метрики регистрируются с префиксом `maskchain_`.
Validation: `curl /metrics | grep maskchain_` возвращает snake_case имена.

### DEC-002 Один порт для API и метрик

Why: не требуется отдельный port-binding, упрощает конфигурацию и docker-compose.
Tradeoff: `/metrics` endpoint не изолирован (нет firewall-правила на отдельный порт). Принято для MVP; отдельный порт — follow-up.
Affects: `server.go` — `/metrics` на том же Gin engine.
Validation: `curl localhost:8080/metrics` возвращает метрики.

### DEC-003 Always-on sampling для MVP, configurable ratio в конфиге

Why: MVP нужно видеть все трейсы для отладки; ratio настраивается позже.
Tradeoff: лишний трафик на OTLP endpoint при high load. Mitigation: sampling_ratio=1.0 по умолчанию, оператор уменьшает через конфиг.
Affects: `telemetry/` init — ParentBased(AlwaysOnSampler) по умолчанию, конфигурируемый.
Validation: при sampling_ratio=0.5 примерно половина запросов имеет Sampled=true.

### DEC-004 slog adapter как отдельный пакет, не замена zap

Why: zap остаётся основным логгером; slog adapter используется только для логов, которым нужна OTel-корреляция (shield events, critical errors). Избегаем глобальной замены логгера.
Tradeoff: два логгера в коде. Mitigation: чёткая граница — zap для существующего, slog для observability-логов.
Affects: `logging/`, `main.go`.
Validation: unit test на slog.Handler с trace_id/span_id.

### DEC-005 Метрики middleware — отдельный gin.HandlerFunc, а не встроенный в Logger

Why: разделение ответственности; Logger middleware остаётся для access log, metrics middleware — для Prometheus-инструментирования.
Tradeoff: дополнительный middleware в цепочке (микро overhead).
Affects: `server.go` — metrics middleware добавлен после RequestID и Logger.
Validation: оба middleware работают независимо.

## Incremental Delivery

### MVP (Первая ценность)

- Задачи: T1 (go.mod), T2 (telemetry/), T3 (metrics/), T4 (logging/), T5 (server middleware), T6 (main wire)
- AC: AC-001, AC-002, AC-003, AC-005, AC-006
- Проверка: `curl /health && curl /metrics | grep maskchain_ && grep trace_id` в логе

### Итеративное расширение

- Шаг 2: instrument shield middleware + otel config — AC-004, AC-007
- Шаг 3: docker-compose Prometheus + Grafana — AC-008

## Порядок реализации

1. T1 — go get (все зависимости, блокирующий для остальных)
2. T2 — telemetry/ пакет (базовый, не зависит от других)
3. T3 — metrics/ пакет (базовый, зависит от telemetry/ только через registry)
4. T4 — logging/ пакет (зависит от telemetry/ concepts)
5. T5-T6 — wiring в server + main (интеграция всех компонентов)
6. T7 — shield middleware instrument (зависит от T3)
7. T8 — docker-compose (зависит от T5 для работающего /metrics)
8. T9 — tests (сквозные для всех AC)

T1-T4 можно параллелить (независимые пакеты). T5-T6 — последовательно после T1-T4.

## Риски

- OTLP gRPC endpoint недоступен при старте — graceful degradation через noop провайдер
  Mitigation: initTelemetry логирует warning, возвращает noopTracerProvider/noopMeterProvider
- Конфликт имён метрик при повторной регистрации в тестах
  Mitigation: в тестах использовать отдельный prometheus.NewRegistry(), не глобальный registry
- otelgin middleware может захватить span для /metrics запросов (нежелательный noise)
  Mitigation: metrics middleware добавляет route `/metrics` в список skip-путей для otelgin

## Rollout and compatibility

- Все изменения обратно совместимы: новые пакеты, новые middleware, новые config поля — всё опционально
- Если otel endpoint пустой — gateway работает без трейсинга (AC-007)
- Если зависимости не установлены — go build не скомпилируется до go get (стандартное поведение)
- docker-compose изменения — только для локальной разработки, не влияют на production

## Проверка

- Unit tests: telemetry init (mock exporter), metrics registry, slog adapter, middleware duration recording
- Integration tests: `/metrics` HTTP response, shield metrics with mock, graceful shutdown
- Manual: `curl /health` + `curl /metrics` + `docker-compose up` Prometheus target UP
- AC coverage: AC-001–AC-008 все покрыты automated tests

## Соответствие конституции

- нет конфликтов: фича следует Clean Architecture (новые пакеты в infra), не нарушает workflow (spec→plan→tasks), language policy соблюдён
