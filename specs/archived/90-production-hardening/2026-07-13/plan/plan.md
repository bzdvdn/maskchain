# Production Hardening План

## Phase Contract

Inputs: spec (90-production-hardening), inspect (pass), minimal repo context.
Outputs: plan.md, data-model.md.
Stop if: нет — spec и inspect в состоянии pass.

## Цель

Форма реализации фичи без пересказа спеки. Изменяемое: pprof endpoints за admin auth на основном порту; параметры connection pool (PG + HTTP) с логированием и метриками; load-test скрипт (Python); security CI-шаги (gitleaks, TLS lint, config audit); production docker-compose profile; runbook. Все изменения локальны — ни одна не требует миграций БД, новых domain entity или rollback-процедур.

## MVP Slice

Connection pool tuning + pprof endpoints за admin auth + startup логирование параметров пулов.
Покрываемые AC: AC-001 (pprof auth), AC-002 (логи пулов).
MVP можно валидировать без docker-compose и CI — достаточно `go run` или `go test`.

## First Validation Path

1. `go run ./src/cmd/gateway/ --config testdata/config.yaml` с `debug.enabled: true` и `admin_token: test`.
2. `curl -H "X-Admin-Token: test" http://localhost:8080/debug/pprof/` → `200 OK`.
3. `curl http://localhost:8080/debug/pprof/` → `401 Unauthorized`.
4. Проверить лог старта на наличие строк `max_open_conns`, `max_idle_conns_per_host`.

## Scope

- `src/internal/infra/config/`: добавить `DebugConfig` с `Enabled` и `AdminToken`; добавить `MaxIdleConnsPerHost` в `EgressConfig` или отдельный `HTTPPoolConfig`
- `src/internal/api/middleware/`: добавить `AdminAuth` middleware (shared-secret check)
- `src/internal/api/server.go`: register pprof routes + admin auth middleware
- `src/internal/infra/metrics/`: добавить PG pool gauge metrics
- `src/cmd/gateway/main.go`: логировать pool configs при старте, wire metrics
- `deployments/docker-compose/docker-compose.yml`: создать с production profile
- `deployments/loadtest/chat_completion.py`: новый Python-скрипт load-теста
- `deployments/runbook.md`: новый runbook
- `Makefile`: добавить targets `security-check`, `load-test`
- Явная граница: CI-провайдер не конфигурируется — Makefile targets достаточно для любого CI

## Performance Budget

- SC-002: p99 latency < 500ms при 50 RPS на mock-провайдере (затрагивает routing proxy — сам proxy и egress transport)
- SC-003: graceful shutdown < 30s при 10 активных streaming-соединениях
- Pprof endpoints: 0 overhead при `debug.enabled: false`
- PG pool metrics: overhead < 1ms на сбор метрик

## Implementation Surfaces

- `src/internal/infra/config/config.go` — новая секция `DebugConfig`, расширение `EgressConfig` (или новый `HTTPPoolConfig`)
- `src/internal/api/middleware/adminauth.go` — новый файл (shared-secret admin auth, отдельно от tenant auth)
- `src/internal/api/server.go` — RegisterDebugRoutes метод
- `src/internal/infra/metrics/` — новый файл `pool_metrics.go` или расширение существующего
- `src/cmd/gateway/main.go` — DI wiring для debug routes, pool metrics, pool config logging
- `deployments/docker-compose/docker-compose.yml` — новый файл
- `deployments/loadtest/chat-completion.js` — новый файл
- `deployments/runbook.md` — новый файл
- `Makefile` — расширение

## Bootstrapping Surfaces

- `deployments/docker-compose/` — директория существует (.gitkeep). Никаких новых директорий не требуется.
- `deployments/loadtest/` — новая директория.
- `src/internal/api/middleware/` — существует. Новый файл вливается в существующий пакет.

## Влияние на архитектуру

- Локальное: admin auth middleware не пересекается с tenant auth (разные header/секреты)
- pprof endpoints за admin auth на основном порту — не создают отдельный admin-сервер
- PG pool metrics — новый collector, не затрагивает существующие метрики
- Graceful shutdown таймаут уже есть в `ServerConfig.ShutdownTimeout` — нужно только проверить wire

## Acceptance Approach

- AC-001: AdminAuth middleware + pprof routes. Surfaces: `adminauth.go`, `server.go`. Валидация: curl +/- token.
- AC-002: Startup log pool params. Surfaces: `main.go`, config. Валидация: grep лога.
- AC-003: PG pool metrics. Surfaces: `pool_metrics.go`, `main.go`. Валидация: `/metrics` содержит pgx pool stats.
- AC-004: Makefile targets. Surfaces: `Makefile`. Валидация: `make security-check` запускает gitleaks/TLS lint/config audit.
- AC-005: Python script. Surfaces: `chat_completion.py`. Валидация: `python3 chat_completion.py` exit 0.
- AC-006: docker-compose profile. Surfaces: `docker-compose.yml`. Валидация: `docker compose --profile production up -d`.
- AC-007: runbook. Surfaces: `runbook.md`. Валидация: файл с требуемыми секциями.

## Данные и контракты

- **AC-001..AC-007** — ни один не требует изменения data model или API contracts
- Config structs расширяются, но это не breaking change (viper unmarshal defaults)
- API: pprof эндпоинты новые, существующие не меняются
- `data-model.md` прилагается (no-change для DB, config-only)

## Стратегия реализации

### DEC-001: Admin auth через shared-secret middleware

Why: существующий tenant auth не подходит — он привязан к tenant API keys. Для admin-доступа (pprof, debug) нужен отдельный механизм. Shared secret из конфига — минимальная поверхность, достаточная для MVP.
Tradeoff: не RBAC, не scoped. Secret в конфиге — риск утечки. Mitigation: только для debug-режима, выключен в production по умолчанию.
Affects: `src/internal/api/middleware/adminauth.go` (новый), `src/internal/infra/config/config.go` (DebugConfig), `server.go` (регистрация)
Validation: AC-001

### DEC-002: HTTP pool параметры в EgressConfig

Why: `EgressConfig` уже содержит `MaxIdleConns`, `IdleTimeout`. Добавление `MaxIdleConnsPerHost` и `DisableKeepAlives` в ту же структуру — минимальный diff.
Tradeoff: EgressConfig становится и про egress, и про pool tuning. Альтернатива — отдельный `HTTPPoolConfig`. Выбран прагматичный подход: меньше новых типов.
Affects: `src/internal/infra/config/config.go`
Validation: AC-002 (логирование)

### DEC-003: PG pool metrics через Prometheus gauge + pgxpool.Stat()

Why: pgx `pool.Stat()` возвращает живые значения. Prometheus gauge collectors обновляются при scrape — без отдельного горутин-пуллера.
Tradeoff: polling-based, latency зависит от scrape interval. Для pool мониторинга это приемлемо.
Affects: `src/internal/infra/metrics/pool_metrics.go` (новый), `src/cmd/gateway/main.go`
Validation: AC-003

### DEC-004: Graceful shutdown через существующий ShutdownTimeout

Why: `ServerConfig.ShutdownTimeout` уже есть. Нужно проверить, что `http.Server.Shutdown()` вызывается с таймаутом из конфига, и увеличить default с 10s до 30s.
Tradeoff: нет — улучшение существующего механизма.
Affects: `server.go`, `config.go`
Validation: SC-003

## Incremental Delivery

### MVP (Первая ценность)

- Connection pool tuning + логирование (AC-002)
- Pprof endpoints за admin auth (AC-001)
- Default graceful shutdown таймаут 30s (SC-003 частично)

**Критерий готовности MVP:** go test проходит + ручная проверка curl pprof + лог содержит pool params.

### Итеративное расширение

1. **PG pool metrics** (AC-003) — добавляется после MVP, не блокирует deploy
2. **docker-compose production profile + runbook** (AC-006, AC-007) — документация и инфраструктура
3. **Makefile security targets + load test script** (AC-004, AC-005) — CI-готовность

## Порядок реализации

1. Config: `DebugConfig` + pool params — основа для всего
2. Admin auth middleware + pprof routes — независимо
3. Startup logging pool params — 1 строка в main.go
4. PG pool metrics collector
5. docker-compose + runbook — можно параллельно с п.4
6. Makefile targets + Python load-test script — можно параллельно с п.5

Что параллелимо: п.5 и п.6 (документация/инфраструктура не пересекаются с кодом).

## Риски

- Риск 1: Admin token утекёт через логи/env dump. Mitigation: token только в `debug.enabled: true`; warning в документации; не логируется.
- Риск 2: PG pool metrics collector не сработает если pgx pool не экспортирует Stat(). Mitigation: pgxpool.Pool.Stat() — часть публичного API pgx.
- Риск 3: Python-скрипт требует mock-провайдер, который может не быть запущен. Mitigation: скрипт включает health-check перед тестом; fallback сообщение.
- Риск 4: Изменение `ShutdownTimeout` default с 10s на 30s может замаскировать проблемы с зависшими соединениями. Mitigation: значение конфигурируемо; default 30s — это SC-003 requirement.

## Rollout и compatibility

- Все изменения backwards compatible: config defaults не ломают существующие конфиги; новые эндпоинты не трогают существующие.
- docker-compose profile `production` — новый профиль, dev-профиль не меняется.
- security-check в Makefile — opt-in (ручной запуск или явный CI-шаг).
- Специальных rollout-действий не требуется.

## Проверка

- Go unit tests: adminauth middleware, config validation (pool params), metrics collector
- Integration test (optional): `make test` с запущенным docker-compose test profile
- Manual: curl pprof, grep log, `docker compose --profile production up -d`, `python3 ./deployments/loadtest/chat_completion.py`
- AC-001: admin auth unit test + curl
- AC-002: grep log test
- AC-003: /metrics check
- AC-004: `make security-check` exit code
- AC-005: `python3 ./deployments/loadtest/chat_completion.py` exit 0
- AC-006: `docker inspect` проверка limits/healthcheck/restart
- AC-007: file existence + section grep

## Соответствие конституции

- нет конфликтов
