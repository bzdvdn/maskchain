# Observability: OpenTelemetry, Prometheus, Structured Logging, Distributed Tracing

## Scope Snapshot

- In scope: внедрение OpenTelemetry SDK (traces + metrics), Prometheus-метрик для HTTP-запросов и Content Shield, адаптера slog для структурированного логирования, Gin-мидлвари для trace propagation и инструментирования, docker-compose с Prometheus + Grafana.
- Out of scope: миграция существующего zap-логирования на slog (замена библиотеки), дашборды Grafana (конфигурация provisioning), алертинг, OpenTelemetry Collector sidecar, distributed tracing через внешний коллектор (Jaeger/Tempo) — только экспорт OTLP gRPC наружу.

## Цель

Разработчик и оператор получают наблюдаемость системы: unified-trace через OTel, Prometheus-метрики для SRE-мониторинга (request latency, error rate, shield events), и структурированные логи (slog) с корреляцией trace_id/span_id. Успех фичи измеряется возможностью запустить `docker-compose up`, отправить несколько запросов и увидеть трейсы в Jaeger/Tempo, метрики в Prometheus, а в логах — trace_id=... в каждом entry.

## Основной сценарий

1. Gateway стартует: OTel SDK инициализирует TracerProvider и MeterProvider с OTLP gRPC exporter (адрес из конфига).
2. Каждый HTTP-запрос проходит через Gin-мидлварь, которая создаёт root span, injects propagation context, и записывает request duration + status code как Prometheus histogram.
3. Shield-мидлварь дополняет current span атрибутами `shield.profile`, `shield.status`, `shield.incident_id` и записывает shielded-specific метрики (scan_duration_ms, incidents_by_severity, profiles_evaluated).
4. Логи через адаптер `slog.Logger` обогащаются `trace_id` и `span_id` из OTel context, если span активен.
5. Метрики экспортируются на `/metrics` (Prometheus HTTP handler), доступный в docker-compose сети.
6. Ошибка: если OTLP endpoint недоступен — SDK падает на `ShutdownOnError=false`, продолжает работу без трейсинга (graceful degradation).

## User Stories

- P1 Story: Оператор запускает `docker-compose up`, делает curl-запрос к gateway, и видит метрики в Prometheus + трейсы в Tempo.
- P2 Story: Разработчик при отладке shield-инцидента читает лог-запись с `trace_id` и переходит в трейс для полной картины.

## MVP Slice

Наименьший срез: OTel SDK init, Prometheus `/metrics` endpoint, request duration + status код гистограмма, базовая Gin-мидлварь trace propagation. Закрывает AC-001, AC-003, AC-005, AC-006.

## First Deployable Outcome

После первого implementation pass можно запустить `go run ./src/cmd/gateway --config config.yaml`, отправить `curl localhost:8080/health` и проверить:
- в stdout-логах присутствуют `trace_id=...` и `span_id=...`
- `curl localhost:8080/metrics` возвращает Prometheus-метрики с префиксом `maskchain_`

## Scope

- `src/internal/infra/telemetry/` — OTel SDK init (TracerProvider, MeterProvider, OTLP gRPC exporter, graceful shutdown)
- `src/internal/infra/metrics/` — Prometheus metric definitions (counter, histogram, gauge заводятся в одном пакете)
- `src/internal/infra/logging/` — slog adapter: хелпер для создания `slog.Logger` с OTel-полями
- Gin middleware: OTel trace propagation (OpenTelemetry Gin middleware integration), request duration histogram
- Shield middleware: instrument existing shield middleware with span attributes and shield-specific metrics
- `docker-compose` update: add Prometheus (scrape `gateway:8080/metrics`) + Grafana (provisioning-ready)
- Config section: `otel:` блок с endpoint, service_name, environment
- `go.mod` dependency: `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/sdk`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace`, Prometheus `promhttp`

## Контекст

- Существующее логирование на `go.uber.org/zap` остаётся; адаптер slog добавляется параллельно как дополнительный канал (с OTel-обогащением). В main будет два логгера: zap (для существующего кода) и slog (для новых observability-компонент).
- OTel SDK должен корректно завершать работу (graceful shutdown) при SIGTERM/SIGINT.
- Функция инициализации OTel SDK (`initTelemetry`) возвращает `shutdown func(context.Context) error`, которую main вызывает при graceful shutdown.
- Prometheus `/metrics` endpoint должен быть доступен только внутри docker-compose сети (не публичный), но в MVP открыт на `0.0.0.0:8080/metrics` без auth.
- Gateway работает в enterprise-сетях с outbound proxy — OTLP gRPC должен поддерживать proxy настройки (env `HTTP_PROXY`/`HTTPS_PROXY`)
- Профиль `info` / `debug` определяет частоту сбора и уровень трассировки (sampling ratio из конфига)

## Зависимости

- `go.opentelemetry.io/otel` — core API
- `go.opentelemetry.io/otel/sdk` — SDK (TracerProvider, MeterProvider)
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace` — OTLP trace exporter
- `go.opentelemetry.io/otel/exporters/otlp/otlpmetric` — OTLP metric exporter
- `go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin` — Gin middleware
- `github.com/prometheus/client_golang/prometheus` + `promhttp` — Prometheus metrics
- `github.com/prometheus/client_model` — metric model types (test support)
- None внешних сервисов — все компоненты in-process

## Требования

- RQ-001 Gateway ДОЛЖЕН инициализировать OTel SDK (TracerProvider + MeterProvider) с конфигурируемым OTLP gRPC endpoint при старте.
- RQ-002 Gateway ДОЛЖЕН экспортировать Prometheus-метрики на `/metrics` endpoint с префиксом `maskchain_`.
- RQ-003 Каждый HTTP-запрос ДОЛЖЕН иметь root span с атрибутами `http.method`, `http.url`, `http.status_code`, `http.duration_ms`.
- RQ-004 Shield scan event ДОЛЖЕН записывать метрики: `maskchain_shield_scan_duration_ms` (histogram), `maskchain_shield_incidents_by_severity` (counter by status label), `maskchain_shield_profiles_evaluated` (counter by profile label).
- RQ-005 Logger (slog) ДОЛЖЕН включать `trace_id` и `span_id` из активного OTel span, если span присутствует в контексте.
- RQ-006 Gateway ДОЛЖЕН graceful shutdown OTel SDK (flush remaining spans/metrics) при SIGTERM/SIGINT перед выходом.
- RQ-007 docker-compose ДОЛЖЕН включать сервис Prometheus с конфигурацией scrape для gateway (target `gateway:8080`) и сервис Grafana.
- RQ-008 Если OTLP endpoint недоступен при старте, gateway ДОЛЖЕН продолжить работу (graceful degradation) без трейсинга, логируя предупреждение.

## Вне scope

- Замена `go.uber.org/zap` на `log/slog` — zap остаётся основным логгером в существующем коде.
- Grafana dashboard provisioning — конфигурация дашбордов вынесена в follow-up.
- OpenTelemetry Collector sidecar deployment — OTLP экспорт идёт напрямую в endpoint.
- Алертинг (Prometheus Alertmanager) — только сбор метрик.
- Метрики бизнес-уровня (количество масок, unmask-запросов) — только инфраструктурные + shield-специфичные.
- Tracing для фоновых задач (миграции, maintenance jobs).

## Критерии приемки

### AC-001 OTel SDK initializes and exports a root span

- Почему это важно: гарантирует, что OTel SDK подключен и трейсы доходят до collector/Tempo.
- **Given** конфигурация с валидным OTLP endpoint
- **When** gateway запускается и обрабатывает GET `/health`
- **Then** на OTLP endpoint приходит span с именем `/health`, атрибутами `http.method=GET`, `http.status_code=200`
- Evidence: mock OTLP receiver (например, `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracetest`) подтверждает получение экспортированного span.

### AC-002 Prometheus /metrics returns maskchain_ prefix metrics

- Почему это важно: оператор и Prometheus scrape target должны получать метрики с ожидаемым namespace.
- **Given** запущенный gateway
- **When** выполняется HTTP GET `/metrics`
- **Then** response body содержит строки, начинающиеся с `maskchain_`
- Evidence: `curl -s http://localhost:8080/metrics | grep ^maskchain_` возвращает непустой вывод.

### AC-003 HTTP request duration histogram recorded

- Почему это важно: SRE-метрика для SLI latency.
- **Given** запущенный gateway
- **When** любой HTTP-запрос обработан
- **Then** метрика `maskchain_http_request_duration_ms` (histogram) содержит наблюдение с labels `method`, `path`, `status_code`
- Evidence: GET `/metrics` показывает ненулевой `_count` и `_sum` для `maskchain_http_request_duration_ms`.

### AC-004 Shield scan metrics are recorded

- Почему это важно: мониторинг производительности и статусов Content Shield.
- **Given** gateway с настроенным shield middleware и in-memory profile repository (без внешней БД)
- **When** POST `/v1/chat/completions` с `X-Shield-Profile-Slug` проходит shield scan
- **Then** метрики `maskchain_shield_scan_duration_ms`, `maskchain_shield_incidents_by_severity`, `maskchain_shield_profiles_evaluated` обновляются
- Evidence: unit/integration test с mock Scanner и mock ProfileRepository подтверждает ненулевые значения всех трёх метрик после shield scan.

### AC-005 slog log entry contains trace_id and span_id

- Почему это важно: корреляция логов с трейсами для отладки.
- **Given** активный span в context
- **When** создаётся slog-запись через адаптер
- **Then** в структурированном выводе присутствуют ключи `trace_id` и `span_id` с корректными hex-значениями
- Evidence: unit test проверяет, что slog.Handler содержит поля trace_id и span_id при наличии span в контексте.

### AC-006 Graceful shutdown flushes OTel SDK

- Почему это важно: избежать потери последних спанов и метрик при рестарте.
- **Given** gateway обрабатывает запросы
- **When** приходит SIGTERM
- **Then** main вызывает shutdown func, полученную от `initTelemetry`, которая дожидается flush TracerProvider и MeterProvider с таймаутом
- Evidence: unit test вызывает shutdown func и проверяет, что TracerProvider.Shutdown был вызван (mock provider); интеграционный тест отправляет SIGTERM и проверяет лог о завершении flush.

### AC-007 Graceful degradation when OTLP endpoint is unreachable

- Почему это важно: gateway не должен падать при отсутствии наблюдаемой инфраструктуры.
- **Given** конфигурация с недоступным OTLP endpoint
- **When** gateway запускается
- **Then** gateway логирует предупреждение и продолжает работу без трейсинга; метрики (/metrics) работают
- Evidence: gateway запускается без ошибок, `GET /health` возвращает 200, `GET /metrics` содержит метрики.

### AC-008 docker-compose up starts Prometheus and Grafana

- Почему это важно: локальный dev-стенд для проверки наблюдаемости.
- **Given** docker-compose.yml с добавленными Prometheus и Grafana
- **When** `docker-compose up -d`
- **Then** Prometheus (port 9090) и Grafana (port 3000) запускаются, Prometheus scrape target `gateway:8080` в статусе UP
- Evidence: `curl localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job == "gateway") | .health'` возвращает `"up"`.

## Допущения

- OTLP gRPC endpoint настраивается через конфиг-файл (`otel.endpoint`) или env `CONFIG_OTEL_ENDPOINT`; по умолчанию `localhost:4317`.
- Prometheus `/metrics` не требует аутентификации (внутренняя сеть).
- Go-версия 1.26.3 и выше поддерживает стандартный пакет `log/slog` (доступен с 1.21).
- Существующий zap-логгер остаётся для обратной совместимости; slog используется только для новых observability-компонентов.
- Grafana запускается без provisioning дашбордов (только explore mode).

## Критерии успеха

- SC-001 HTTP-запросы с latency < 1s: overhead OTel SDK < 5ms на запрос (p99).
- SC-002 Prometheus scrape (`/metrics`) отвечает за < 100ms при ~100 активных метриках.
- SC-003 После SIGTERM все in-flight спаны и метрики экспортированы в течение shutdown_timeout.

## Краевые случаи

- OTLP endpoint не указан (пустая строка) — gateway работает без трейсинга, метрики доступны.
- Prometheus `/metrics` запрашивается до первого HTTP-запроса — возвращает нулевые значения (empty histogram, zero counters).
- Shield-метрики при отключённом shield middleware (без БД) — метрики не записываются (ноль), но не падают.
- Множественные параллельные запросы — гистограммы и counters thread-safe (Prometheus client гарантирует).
- Повторный старт OTel SDK (например, после ошибки инициализации) — не предусмотрен (fail-once).

## Открытые вопросы

- Должен ли OTel SDK поддерживать sampling (parent-based всегда, или конфигурируемый ratio)?
- Нужен ли отдельный порт для `/metrics` или достаточно существующего (8080)?
- Какая стратегия именования метрик предпочтительна: snake_case (`maskchain_http_requests_total`) или colon-separated (`maskchain:http:requests:total`)?
