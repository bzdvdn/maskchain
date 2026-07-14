# Dependency-aware health/readiness probes — Задачи

## Phase Contract

Inputs: plan.md, spec.md, data-model.md.
Outputs: tasks.md с фазами, Touches, Surface Map, покрытие AC.
Stop if: нет.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `internal/api/health/probe.go` | T1.1 |
| `internal/api/health/service.go` | T1.1, T3.1, T4.1 |
| `internal/infra/config/config.go` | T1.2 |
| `internal/api/health/handler.go` | T2.1 |
| `internal/api/server.go` | T2.2 |
| `internal/api/admin.go` | T2.2 |
| `cmd/gateway/main.go` | T2.3, T3.2 |
| `cmd/admin/main.go` | T2.3, T3.2 |
| `internal/api/health/probes.go` | T3.1 |
| `internal/api/health/health_test.go` | T4.1 |

## Implementation Context

- Цель MVP: Probe interface + Service + конфиг + handler + wiring — /health и /live работают через health.Service, /ready — через конкретные probes.
- Инварианты/семантика:
  - `Probe.Check()` принимает `context.Context`, возвращает `Result{Status, LatencyMs, Error}`.
  - `Service.CheckAll(ctx)` последовательно вызывает все probes; `latency_ms` — duration per probe в ms.
  - `critical_deps` из конфига: если dep в списке и её probe вернул `Status != "ok"` → общий статус `down` (503); если не в списке → `degraded` (200); все ok → `ok` (200).
  - `/live` возвращает `{status:"ok"}` после конструктора (зависимости уже в DI).
  - `/health` всегда `{status:"ok"}`.
- Ошибки/коды:
  - `/ready` — 200 при ok/degraded, 503 при down.
  - `/health`, `/live` — всегда 200.
- Контракты/протокол:
  - `GET /health` → `{"status":"ok"}`
  - `GET /live` → `{"status":"ok"}`
  - `GET /ready` → `{"status":"ok|degraded|down","checks":{"<name>":{"status":"ok|down","latency_ms":<int>,"error":"..."}}}`
- Границы scope: не трогаем provider HealthChecker (`domain/routing/service/health.go`); не добавляем Prometheus метрики для health; probes синхронные, без кэша.
- Proof signals (DEC-*): DEC-001 (Probe interface), DEC-003 (sync probes), DEC-004 (egress TCP dial).
- References: DEC-001, DEC-003, DEC-004, AC-001 — AC-008.

## Фаза 1: Основа

Цель: Probe interface, Result type, HealthService (регистрация + агрегация), HealthCheckConfig.

- [x] T1.1 Создать `internal/api/health/probe.go` + `service.go` — интерфейс `Probe` с `Name() string` и `Check(ctx) Result`, тип `Result` (Status, LatencyMs, Error). `HealthService` хранит `[]Probe`, `criticalDeps map[string]bool`. Метод `Register(p Probe)`, `CheckAll(ctx) *AggregatedResult`. Агрегация: если все ok → ok; если хоть один critical не ok → down; иначе degraded. Touches: `internal/api/health/probe.go`, `internal/api/health/service.go`
- [x] T1.2 Добавить `HealthCheckConfig` в `ServerConfig`: `CriticalDeps []string` с mapstructure `"critical_deps"`. В `DefaultConfig()` инициализировать `CriticalDeps: []string{"database"}`. Touches: `internal/infra/config/config.go`

## Фаза 2: MVP Slice

Цель: Gin handler, Server/AdminServer wiring. /health и /live через `HealthService`.

- [x] T2.1 Создать `internal/api/health/handler.go` — `NewHandler(svc *HealthService) gin.HandlerFunc` для каждого эндпоинта: `LivenessHandler` (всегда `{status:"ok"}`), `StartupHandler` (всегда `{status:"ok"}`), `ReadinessHandler` (вызывает `svc.CheckAll`, маппит статус + HTTP code). Touches: `internal/api/health/handler.go`
- [x] T2.2 В `server.go` и `admin.go`: конструкторы `New` / `NewAdminServer` принимают `*health.HealthService`. Заменить `healthHandler("ok")` / `healthHandler("alive")` на `h.LivenessHandler`, `h.ReadinessHandler`, `h.StartupHandler`. Touches: `internal/api/server.go`, `internal/api/admin.go`
- [x] T2.3 В `cmd/gateway/main.go` и `cmd/admin/main.go`: создать `healthSvc := health.NewService(cfg.Server.HealthCheck.CriticalDeps)`, передать в `api.New(cfg.Server, logger, serviceName, healthSvc)` и `api.NewAdminServer(...)`. Touches: `cmd/gateway/main.go`, `cmd/admin/main.go`

## Фаза 3: Основная реализация

Цель: конкретные probe-реализации (PG, Valkey, egress). /ready возвращает динамический статус.

- [x] T3.1 Создать `internal/api/health/probes.go` — три реализации:
  - `PGProbe`: принимает `*pgxpool.Pool`, Name="database". Check: `pool.Ping(ctx)` → latency. Если pool nil → status ok (not configured).
  - `ValkeyProbe`: принимает `*valkey.Client`, Name="valkey". Check: `client.Do(ctx, "PING")`. Если client nil → status ok.
  - `EgressProbe`: принимает `[]string` (provider targets host:port), Name="egress". Check: TCP dial с `net.Dialer{Timeout: 5s}`. ok если хотя бы один dial успешен.
  Touches: `internal/api/health/probes.go`
- [x] T3.2 Дополнить `main.go` (gateway + admin): создать и зарегистрировать probes. PGProbe(pgPool), ValkeyProbe(vkClient), EgressProbe(providerTargets). Provider targets извлекаются из `cfg.Routing.Providers[*].BaseURL`. Touches: `cmd/gateway/main.go`, `cmd/admin/main.go`

## Фаза 4: Проверка

Цель: unit-тесты с mock probes.

- [x] T4.1 Создать `internal/api/health/health_test.go` — тесты:
  - mock Probe с контролируемым Result → проверка агрегации ok/degraded/down.
  - Проверка critical_deps порога (AC-006).
  - Проверка latency_ms в ответе.
  - Проверка nil-dependency probe (not configured → ok).
  - Проверка HTTP кодов для каждого эндпоинта.
  Touches: `internal/api/health/health_test.go`

## Покрытие критериев приемки

- AC-001 → T2.1, T2.2, T4.1
- AC-002 → T3.1, T3.2, T4.1
- AC-003 → T3.1, T3.2, T4.1
- AC-004 → T3.1, T3.2, T4.1
- AC-005 → T2.1, T2.2, T4.1
- AC-006 → T1.2, T3.1, T4.1
- AC-007 → T1.1, T2.1, T4.1
- AC-008 → T2.2 (auth bypass уже существует)

## Заметки

- T2.2 меняет сигнатуру `New()` и `NewAdminServer()` — синхронизировать оба `cmd/`.
- `EgressProbe` экстракция host:port из `*url.URL` баз провайдеров.
- Фаза 1 и 2 не зависят от реальных probes — можно тестировать с mock.
