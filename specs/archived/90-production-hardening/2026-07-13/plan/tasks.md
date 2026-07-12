# Production Hardening Задачи

## Phase Contract

Inputs: plan (90-production-hardening), data-model (config-only), spec.
Outputs: упорядоченные исполнимые задачи с покрытием критериев.
Stop if: нет — plan, data-model, spec в состоянии pass.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/infra/config/config.go` | T1.1, T1.2, T1.3 |
| `src/internal/api/middleware/adminauth.go` | T2.1, T4.3 |
| `src/internal/api/server.go` | T2.2 |
| `src/cmd/gateway/main.go` | T2.3, T2.4, T3.2 |
| `src/internal/infra/metrics/pool_metrics.go` | T3.1 |
| `deployments/docker-compose/docker-compose.yml` | T3.3 |
| `deployments/runbook.md` | T3.4 |
| `Makefile` | T4.1 |
 | `deployments/loadtest/chat_completion.py` | T4.2 |
| `src/internal/api/middleware/adminauth_test.go` | T4.3 |
| `src/internal/infra/metrics/pool_metrics_test.go` | T4.3 |

## Implementation Context

- Цель MVP: connection pool tuning + pprof endpoints за admin auth + startup логирование параметров пулов. Покрывает AC-001, AC-002.
- Инварианты: AdminAuth middleware отделён от tenant auth (разные header/ключи); admin token из конфига, не из БД; pprof доступен только при `debug.enabled: true`.
- Ошибки/коды: `401 Unauthorized` при невалидном/отсутствующем `X-Admin-Token`; `404 Not Found` при `debug.enabled: false`; валидация pool params — fallback на defaults + warning в лог.
- Контракты/протокол: header `X-Admin-Token` для pprof доступа; роут `/debug/pprof/*any` за middleware; метрики `/metrics` (существующий).
- Границы scope: не делаем RBAC/scoped admin; не создаём отдельный admin-сервер; не автоматизируем CI-провайдер (только Makefile targets); не добавляем миграций БД.
- Proof signals: `curl -H "X-Admin-Token: test" http://localhost:8080/debug/pprof/` → 200; `curl http://localhost:8080/debug/pprof/` → 401; grep лога `max_open_conns`; `/metrics` содержит `pgx_pool_acquire_count`.
- References: DEC-001 (admin-auth), DEC-002 (pool params в EgressConfig), DEC-003 (PG metrics gauge), DEC-004 (graceful shutdown), DM (config-only).

## Фаза 1: Config foundation

Цель: подготовить config-структуры, чтобы без них не начинать реализацию.

- [x] T1.1 Добавить `DebugConfig` struct с полями `Enabled bool` и `AdminToken string` в `config.go`, добавить поле `Debug *DebugConfig` в `Config`, установить defaults (`Enabled: false`, `AdminToken: ""`). Touches: `src/internal/infra/config/config.go`
- [x] T1.2 Добавить поля `MaxIdleConnsPerHost int` и `DisableKeepAlives bool` в `EgressConfig`, добавить defaults (`MaxIdleConnsPerHost: 2`, `DisableKeepAlives: false`). Touches: `src/internal/infra/config/config.go`
- [x] T1.3 Увеличить default `ShutdownTimeout` с 10 до 30 секунд (SC-003). Touches: `src/internal/infra/config/config.go`

## Фаза 2: MVP slice

Цель: pprof endpoints за admin auth + startup логирование пулов. Покрывает AC-001, AC-002, SC-003.

- [x] T2.1 Реализовать AdminAuth middleware — gin.HandlerFunc, проверяющая header `X-Admin-Token` против `cfg.Debug.AdminToken`. Возвращает 401 при несовпадении; пропускает запрос при совпадении или при `debug.enabled: false` (тогда pprof роуты не регистрируются — 404). Touches: `src/internal/api/middleware/adminauth.go`
- [x] T2.2 Реализовать `RegisterDebugRoutes` на Server — регистрирует `/debug/pprof/*any` через `net/http/pprof` (Index, Cmdline, Profile, Symbol, Trace) за AdminAuth middleware, только если `debug.enabled: true`. Touches: `src/internal/api/server.go`
- [x] T2.3 В `main.go` добавить INFO-логирование актуальных параметров PG pool (`MaxConns`, `MinConns`, `MaxConnLifetime`) и HTTP pool (`MaxIdleConns`, `MaxIdleConnsPerHost`, `IdleTimeout`, `DisableKeepAlives`) при старте. Touches: `src/cmd/gateway/main.go`
- [x] T2.4 В `main.go` обеспечить вызов `server.Shutdown()` с контекстом, ограниченным `cfg.Server.ShutdownTimeout`. Touches: `src/cmd/gateway/main.go`

## Фаза 3: Post-MVP реализация

Цель: PG pool metrics, docker-compose profile, runbook. Покрывает AC-003, AC-006, AC-007.

- [x] T3.1 Реализовать Prometheus gauge collector для PG pool stats (`pgx_pool_acquire_count`, `pgx_pool_idle_conns`, `pgx_pool_in_use_conns`, `pgx_pool_total_conns`). Collector читает `pgxpool.Stat()` при каждом scrape. Touches: `src/internal/infra/metrics/pool_metrics.go`
- [x] T3.2 Зарегистрировать pool metrics collector в Prometheus registry в `main.go`. Touches: `src/cmd/gateway/main.go`, `src/internal/infra/metrics/pool_metrics.go`
- [x] T3.3 Создать `deployments/docker-compose/docker-compose.yml` с profile `production`: сервис gateway с resource limits (CPU/memory), healthcheck (`/health`), restart policy `unless-stopped`. Touches: `deployments/docker-compose/docker-compose.yml`
- [x] T3.4 Создать `deployments/runbook.md` с секциями: startup sequence, health check endpoints, debug procedure (connection pool exhaustion, TLS handshake failure, provider timeout, startup crash), recovery steps. Touches: `deployments/runbook.md`

## Фаза 4: CI/testing readiness

Цель: Makefile targets, k6 script, unit tests. Покрывает AC-004, AC-005; страхует регрессии.

- [x] T4.1 Добавить в Makefile target `security-check`, запускающий: `gitleaks` (secrets scan), TLS config lint (`openssl`/custom), config audit (`check-config`). Добавить target `load-test`, запускающий `k6 run ./deployments/loadtest/chat-completion.js`. Touches: `Makefile`
- [x] T4.2 Создать Python-скрипт `deployments/loadtest/chat_completion.py`: отправляет POST `/v1/chat/completions` на gateway через routing proxy с mock-провайдером; измеряет RPS, p50/p95/p99 latency, error rate. Включает health-check перед запуском. Touches: `deployments/loadtest/chat_completion.py`
- [x] T4.3 Добавить unit-тесты: AdminAuth middleware (200/401 scenarios), pool metrics collector (корректные gauge values при Stat), config validation (invalid pool params → fallback + warning). Touches: `src/internal/api/middleware/adminauth_test.go`, `src/internal/infra/metrics/pool_metrics_test.go`

## Покрытие критериев приемки

- AC-001 -> T2.1, T2.2, T4.3
- AC-002 -> T1.2, T2.3, T4.3
- AC-003 -> T3.1, T3.2, T4.3
- AC-004 -> T4.1
- AC-005 -> T4.2
- AC-006 -> T3.3
- AC-007 -> T3.4

## Заметки

- T1.1, T1.2, T1.3 можно выполнять параллельно (все в одном файле)
- T2.1 и T2.2 логически связаны — T2.2 зависит от существования middleware
- T2.3 и T2.4 — одна строка каждая, можно выполнить вместе
- T3.1 и T3.2 связаны (collector → регистрация)
- T3.3 и T3.4 полностью независимы, параллелимы
- T4.1 и T4.2 независимы, параллелимы
- T4.3 зависит от T2.1, T3.1, T1.2 (тестирует эти компоненты)
