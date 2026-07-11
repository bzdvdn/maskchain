# Observability: OpenTelemetry, Prometheus, Structured Logging, Distributed Tracing — Задачи

## Phase Contract

Inputs: plan.md (DEC-001–DEC-005, surfaces, sequencing).
Outputs: упорядоченные исполнимые задачи с покрытием критериев приемки.
Stop if: задачи расплывчаты или AC не покрываются — stop conditions не выполнены.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `go.mod` | T1.1 |
| `src/internal/infra/telemetry/` (новый) | T1.2, T4.1 |
| `src/internal/infra/metrics/` (новый) | T1.3, T4.1 |
| `src/internal/infra/logging/` (новый) | T1.4, T4.1 |
| `src/internal/infra/config/config.go` | T1.2 |
| `src/internal/api/server.go` | T2.1 |
| `src/cmd/gateway/main.go` | T2.2 |
| `src/internal/api/middleware/shield.go` | T3.1, T4.1 |
| `examples/docker-compose.yml` | T3.2 |

## Implementation Context

- **Цель MVP**: OTel SDK init + Prometheus `/metrics` + trace propagation + request duration histogram (AC-001, AC-002, AC-003, AC-005, AC-006)
- **Инварианты/семантика**:
  - Все метрики с префиксом `maskchain_` (snake_case) — DEC-001
  - `/metrics` на том же порту (8080) — DEC-002
  - Always-on sampling по умолчанию, configurable ratio — DEC-003
  - slog adapter не заменяет zap (дополнительный канал) — DEC-004
  - metrics middleware отдельный handler, не встроенный в Logger — DEC-005
- **Ошибки/коды**: OTLP endpoint недоступен → warning + noop провайдер (AC-007); пустой endpoint → работа без трейсинга
- **Контракты/протокол**:
  - OTLP gRPC экспорт на endpoint из config (по умолчанию `localhost:4317`)
  - `/metrics` HTTP GET возвращает Prometheus text format
  - `/metrics` excluded from OTel tracing (skip-путь)
- **Границы scope**:
  - НЕ заменяем zap, НЕ делаем Grafana provisioning, НЕ делаем OTel Collector sidecar
- **Proof signals**:
  - `otelgin` создаёт root span для каждого HTTP запроса (AC-001)
  - `curl /metrics | grep maskchain_` не пуст (AC-002)
  - `maskchain_http_request_duration_ms` имеет ненулевой count после запроса (AC-003)
  - slog entry содержит trace_id/span_id (AC-005)
  - initTelemetry возвращает shutdown func, вызываемая в main (AC-006)
- **References**: DEC-001–DEC-005, data-model.md (no-change)

## Фаза 1: Bootstrapping — зависимости и новые пакеты

Цель: подготовить все новые пакеты и зависимости до интеграции в server/main.

- [x] T1.1 Добавить OTel и Prometheus зависимости в go.mod
  `go get go.opentelemetry.io/otel go.opentelemetry.io/otel/sdk go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin github.com/prometheus/client_golang/prometheus github.com/prometheus/client_golang/prometheus/promhttp`
  Touches: `go.mod`, `go.sum`

- [x] T1.2 Создать пакет telemetry/ — OTel SDK init, graceful shutdown, OtelConfig
  - Создать `src/internal/infra/telemetry/telemetry.go`:
    - `OtelConfig` struct (endpoint, service_name, environment, sampling_ratio) с tags mapstructure/yaml
    - `InitProvider(ctx, endpoint, serviceName, environment string, samplingRatio float64, log *slog.Logger) (func(context.Context) error, error)` — инициализирует TracerProvider + MeterProvider с OTLP gRPC exporter; при недоступном endpoint возвращает noop провайдер + warning (AC-007); shutdown func для graceful shutdown
    - `DefaultOtelConfig()` с defaults (endpoint=localhost:4317, service_name=maskchain-gateway, environment=development, sampling_ratio=1.0)
  - Добавить `OtelConfig` в `Config` struct в `src/internal/infra/config/config.go`
  Touches: `src/internal/infra/telemetry/telemetry.go`, `src/internal/infra/config/config.go`

- [x] T1.3 Создать пакет metrics/ — Prometheus metric definitions
  - Создать `src/internal/infra/metrics/metrics.go`:
    - `NewRegistry()` возвращает `*prometheus.Registry` (не глобальный registry)
    - `RegisterMetrics(reg *prometheus.Registry)` — регистрирует все метрики с namespace `maskchain`
    - `HTTPRequestDuration` — `prometheus.NewHistogramVec` с labels method/path/status_code
    - `ShieldScanDuration` — `prometheus.NewHistogramVec` с labels profile/status
    - `ShieldIncidentsBySeverity` — `prometheus.NewCounterVec` с label status
    - `ShieldProfilesEvaluated` — `prometheus.NewCounterVec` с label profile
    - `Handler(reg *prometheus.Registry) gin.HandlerFunc` — возвращает gin handler для `/metrics` (использует promhttp)
  Touches: `src/internal/infra/metrics/metrics.go`

- [x] T1.4 Создать пакет logging/ — slog adapter с OTel enrichment
  - Создать `src/internal/infra/logging/logging.go`:
    - `NewLogger(w io.Writer, level slog.Level) *slog.Logger` — создаёт slog.Logger с JSON handler
    - `NewOTelHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler` — returns handler, который читает span.SpanContext из context и добавляет trace_id/span_id как slog.Attr; если span нет или не sampled — поля опускаются
  Touches: `src/internal/infra/logging/logging.go`

## Фаза 2: MVP Slice — интеграция в server и main

Цель: OTel + Prometheus + slog работают end-to-end через gateway.

- [x] T2.1 Подключить otelgin middleware, metrics middleware и /metrics роут в server.go
  - В `api.New()` после существующих middleware добавить:
    1. OTel middleware: `otelgin.Middleware(serviceName)` с опцией `WithFilter(func(req *http.Request) bool { return req.URL.Path != "/metrics" })` (DEC-002, mitigate span noise)
    2. Metrics middleware — gin.HandlerFunc, который после c.Next() записывает duration и status_code в `metrics.HTTPRequestDuration` (AC-003)
  - После всех роутов добавить `engine.GET("/metrics", metricsHandler)` (AC-002)
  - `New()` принимает `*metrics.Registry` и `metricsHandler gin.HandlerFunc` как параметры
  Touches: `src/internal/api/server.go`

- [x] T2.2 Wire initTelemetry и shutdown в main.go
  - После `cfg := config.MustLoadConfig()`:
    ```go
    otelShutdown, err := telemetry.InitProvider(context.Background(), cfg.OTel, slogLogger)
    if err != nil {
        slogLogger.Warn("telemetry init", "error", err)
    }
    ```
  - В defer перед `logger.Sync()`:
    ```go
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := otelShutdown(ctx); err != nil {
            slogLogger.Error("otel shutdown", "error", err)
        }
    }()
    ```
  - Создать `slogLogger` через `logging.NewLogger` для observability-логов
  - Передать metrics registry и handler в `api.New()`
  Touches: `src/cmd/gateway/main.go`

## Фаза 3: Итеративное расширение — shield метрики и docker-compose

Цель: shield-specific observability и локальный dev-стенд.

- [x] T3.1 Instrument shield middleware с span attributes и shield метриками
  - В `ShieldMiddleware()` после shield scan:
    - Добавить к текущему span атрибуты: `shield.profile`, `shield.status`, `shield.incident_id`
    - Записать метрики: `metrics.ShieldScanDuration`, `metrics.ShieldIncidentsBySeverity`, `metrics.ShieldProfilesEvaluated`
  - Middleware принимает `*metrics.Registry` (или метрики передаются через closure)
  Touches: `src/internal/api/middleware/shield.go`

- [x] T3.2 Добавить Prometheus + Grafana в docker-compose
  - Добавить сервис `prometheus`:
    - image: `prom/prometheus:latest`
    - port 9090
    - configmap: scrape gateway:8080/metrics каждые 15s
    - depends_on: gateway
  - Добавить сервис `grafana`:
    - image: `grafana/grafana:latest`
    - port 3000
    - environment: GF_AUTH_ANONYMOUS_ENABLED=true
    - depends_on: prometheus
  - Добавить trace-маркер `# @sk-task 61-observability#T3.2: ...` над первым non-comment сервисом
  Touches: `examples/docker-compose.yml`

## Фаза 4: Проверка

Цель: automated доказательство всех AC.

- [x] T4.1 Добавить unit и integration тесты для всех AC
  - `telemetry/telemetry_test.go`:
    - `TestInitProvider_WithMockExporter` — mock OTLP receiver, проверка экспорта span (AC-001)
    - `TestInitProvider_UnreachableEndpoint` — graceful degradation (AC-007)
    - `TestInitProvider_Shutdown` — shutdown func flushes (AC-006)
    - `TestInitProvider_EmptyEndpoint` — empty endpoint → noop (AC-007)
    - `TestInitProvider_EndpointConfig` — valid endpoint accepted (AC-001)
  - `metrics/metrics_test.go`:
    - `TestMetricsPrefix` — все метрики с префиксом maskchain_ (AC-002)
    - `TestHTTPRequestDuration` — histogram записывается после запроса (AC-003)
    - `TestShieldMetrics` — shield метрики записываются через mock (AC-004)
  - `logging/logging_test.go`:
    - `TestOTelHandler_TraceID` — slog entry содержит trace_id/span_id при span в context (AC-005)
    - `TestOTelHandler_NoSpan` — no trace fields when no span in context (AC-005)
  - `middleware/middleware_test.go`:
    - `TestMetricsMiddleware` — metrics обновляются после HTTP запроса
    - `TestShieldMiddleware_Metrics` — shield metrics через mock scanner + mock profileRepo (AC-004)
  Touches: `src/internal/infra/telemetry/telemetry_test.go`, `src/internal/infra/metrics/metrics_test.go`, `src/internal/infra/logging/logging_test.go`, `src/internal/api/middleware/middleware_test.go`

## Покрытие критериев приемки

- AC-001 -> T1.2, T2.1, T2.2, T4.1
- AC-002 -> T1.3, T2.1, T4.1
- AC-003 -> T1.3, T2.1, T4.1
- AC-004 -> T1.3, T3.1, T4.1
- AC-005 -> T1.4, T4.1
- AC-006 -> T1.2, T2.2, T4.1
- AC-007 -> T1.2, T4.1
- AC-008 -> T3.2

## Заметки

- T1.1–T1.4 можно выполнять параллельно
- T2.1–T2.2 последовательно после T1.x
- T3.1 после T2.1 (нужен работающий /metrics)
- T3.2 после T2.1 (нужен /metrics endpoint)
- T4.1 после всех реализационных задач
- Для shield middleware тестов (AC-004) использовать отдельный `prometheus.NewRegistry()` для изоляции
