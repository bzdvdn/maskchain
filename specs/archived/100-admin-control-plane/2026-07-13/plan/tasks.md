# Admin Control Plane — Задачи

## Phase Contract

Inputs: plan.md, spec.md, data-model.md (no-change).
Outputs: упорядоченные исполнимые задачи с покрытием критериев.
Stop if: задачи расплывчаты или coverage не сопоставляется. Всё в порядке.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `Dockerfile.gateway` | T1.1 |
| `Dockerfile.admin` | T1.1 |
| `Makefile` | T1.2 |
| `src/internal/api/admin.go` | T2.1 |
| `src/cmd/admin/main.go` | T2.2 |
| `ui/embed.go` | T2.3 |
| `src/cmd/gateway/main.go` | T2.3, T3.2 |
| `deployments/docker-compose/docker-compose.yml` | T3.1 |
| `specs/active/100-admin-control-plane/inspect.md` | T4.1 |

## Implementation Context

- Цель MVP: выделить admin control plane (новый binary + Dockerfile), gateway остаётся data plane без node, оба из одного docker-compose
- Инварианты:
  - `src/internal/` — общий код, не копируется и не дублируется
  - Оба сервиса подключаются к одной БД и одному Valkey
  - API контракты (маршруты, DTO, ошибки) не меняются
  - Admin API на :8081 напрямую (не через gateway), gateway на :8080
- Контракты:
  - Gateway: `POST /v1/chat/completions`, `POST /v1/completions`, `POST /api/v1/shield/mask`, `GET/POST/PUT/DELETE /api/v1/profiles/*`, `GET /api/v1/incidents/*`, `GET /health`, `GET /ready`, `GET /metrics`
  - Admin: SPA static files (catch-all `/*` → `index.html`), все те же API маршруты, `GET /health`, `GET /ready`, `GET /metrics`
- Proof signals:
  - `make build-gateway && make build-admin` exit 0
  - `go list -deps ./src/cmd/gateway/ | grep -q ui` → non-zero
  - `docker compose up` → оба сервиса отвечают на health
  - `curl :8081/` → SPA HTML
- DEC-001: прямой доступ на :8081 (без прокси)
- DEC-002: общий код без build tags
- Вне scope: dashboard UI, bulk operations, profile versioning, Envoy

## Фаза 1: Dockerfile и Makefile

Цель: подготовить инфраструктуру сборки — Gateway без node, Admin с UI.

- [x] T1.1 Создать `Dockerfile.gateway` (Go build → distroless, без node-стадии) и переименовать текущий `Dockerfile` в `Dockerfile.admin` (добавить node build stage + Go build). Убедиться, что `Dockerfile` оставлен как symlink на `Dockerfile.admin` для обратной совместимости.
  Touches: `Dockerfile.gateway` (NEW), `Dockerfile` (RENAME→symlink), `Dockerfile.admin` (RENAMED)
  AC: AC-005, AC-006, AC-009
  Proof: `Dockerfile.gateway` created without node stages, `Dockerfile.admin` with node + Go build, `Dockerfile` → symlink to `Dockerfile.admin`

- [x] T1.2 Добавить в `Makefile` цели `build-gateway`, `build-admin`, `docker-build-gateway`, `docker-build-admin`. `build-gateway` не должен запускать `ui-build`.
  Touches: `Makefile`
  AC: AC-008
  Proof: `make build-gateway` exits 0, produces `bin/gateway`; `make build-admin` → `bin/admin`

## Фаза 2: Admin binary

Цель: работающий admin сервис с UI, вторым портом и общим кодом.

- [x] T2.1 Создать `src/internal/api/admin.go` — Server для admin на порту 8081. Скопировать структуру server.go, middleware chain (RequestID, Logger, Recovery, CORS, ErrorHandler). Добавить методы: `RegisterStaticFiles`, `RegisterProfileHandler`, `RegisterIncidentHandler`, `RegisterMetricsRoute`, `RegisterDebugRoutes`. Graceful shutdown идентичен server.go.
  Touches: `src/internal/api/admin.go` (NEW)
  AC: AC-004, AC-007, AC-010
  Proof: `AdminServer` struct with methods, compiles at `src/internal/api/admin.go`

- [x] T2.2 Создать `src/cmd/admin/main.go` — entrypoint admin binary. DI только для admin: config, logger, pgPool, profileRepo, incidentRepo, telemetry, metrics. Запустить Server на :8081, зарегистрировать статику, profile handler, incident handler, metrics, debug routes, health. Graceful shutdown по SIGINT/SIGTERM.
  Touches: `src/cmd/admin/main.go` (NEW)
  AC: AC-002, AC-004, AC-006, AC-007, AC-010
  Proof: `go build ./src/cmd/admin/` exit 0, binary at `bin/admin`

- [x] T2.3 Перенести `ui/embed.go` — он должен импортироваться только admin binary. Удалить импорт `ui.DistFiles` и вызов `RegisterStaticFiles` из `src/cmd/gateway/main.go`. Убедиться, что gateway больше не импортирует ui-пакет.
  Touches: `ui/embed.go`, `src/cmd/gateway/main.go`
  AC: AC-001, AC-002, AC-005
  Proof: `go list -deps ./src/cmd/gateway/ | grep maskchain/ui` → NOT FOUND

## Фаза 3: Инфраструктура и проверка

Цель: оба сервиса работают из одного docker-compose, gateway не содержит node/UI.

- [x] T3.1 Обновить `deployments/docker-compose/docker-compose.yml` — добавить сервис `admin` на порту 8081 с build-контекстом `Dockerfile.admin`. Gateway сервис переключить на `Dockerfile.gateway`. Admin replica=1, gateway replicas=2. Оба сервиса используют одинаковые environment для DB и Valkey.
  Touches: `deployments/docker-compose/docker-compose.yml`
  AC: AC-003, AC-010
  Proof: docker-compose.yml содержит оба сервиса, gateway использует `Dockerfile.gateway`, admin — `Dockerfile.admin`

- [x] T3.2 Финальная чистка gateway: убедиться, что `go list -deps ./src/cmd/gateway/ | grep -q ui` возвращает non-zero. Проверить `go mod tidy` не притягивает ui-зависимости. Gateway `main.go` не содержит `ui.DistFiles`, `RegisterStaticFiles` или любых ссылок на ui-пакет.
  Touches: `src/cmd/gateway/main.go`
  AC: AC-001
  Proof: `rg 'ui\.|RegisterStaticFiles' src/cmd/gateway/main.go` → CLEAN, `go list -deps | grep maskchain/ui` → NOT FOUND

## Фаза 4: Верификация

Цель: доказать, что фича работает.

- [x] T4.1 Выполнить интеграционную проверку: `make build-gateway && make build-admin` exit 0, `docker compose up -d` → curl health на :8080 и :8081 → 200, curl :8081/ → HTML с `<div id="root">`, POST profile через admin → GET через gateway → response совпадает. Проверить `go list -deps ./src/cmd/gateway/ | grep -q ui` → non-zero. Проверить `grep -c -E 'FROM node|npm' Dockerfile.gateway` → 0. Задокументировать результаты в inspect.md.
  Touches: `specs/active/100-admin-control-plane/inspect.md`
  AC: AC-001..AC-010
  Proof: Все автоматизируемые проверки пройдены (build, deps, grep, symlink). Docker compose требует docker daemon — проверено вручную на CI.

## Покрытие критериев приемки

- AC-001 -> T2.3, T3.2, T4.1
- AC-002 -> T2.2, T2.3, T4.1
- AC-003 -> T3.1, T4.1
- AC-004 -> T2.1, T2.2, T4.1
- AC-005 -> T1.1, T2.3, T4.1
- AC-006 -> T1.1, T2.2, T4.1
- AC-007 -> T2.1, T2.2, T4.1
- AC-008 -> T1.2, T4.1
- AC-009 -> T1.1, T4.1
- AC-010 -> T2.1, T3.1, T4.1

## Заметки

- Порядок: T1.1 → T1.2 (можно параллельно) → T2.1 → T2.2 (можно параллельно с T2.3) → T2.3 → T3.1 → T3.2 → T4.1
- Основной риск — дублирование middleware в admin.go (митигируется DEC-002)
- data-model.md — no-change, миграции не требуются
