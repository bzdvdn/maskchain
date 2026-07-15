# Remove: удаление audit incident инфраструктуры — Задачи

## Phase Contract

Inputs: plan (4 потока, 4 DEC), data-model (2 DM).
Outputs: задачи с Touches и покрытием AC.
Stop if: coverage не полный.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/domain/shield/entity/incident.go` | T1.1 |
| `src/internal/domain/shield/entity/scan_result.go` | T1.2 |
| `src/internal/domain/shield/repository.go` | T1.3 |
| `src/internal/domain/shield/service/scan.go` | T1.4 |
| `src/internal/domain/shield/service/evaluate.go` | T1.4 |
| `src/internal/domain/shield/reaction/alert.go` | T2.1 |
| `src/internal/domain/shield/reaction/block.go` | T2.1 |
| `src/internal/domain/shield/reaction/mask.go` | T2.1 |
| `src/internal/domain/shield/reaction/redact.go` | T2.1 |
| `src/internal/domain/shield/reaction/reaction.go` | T2.1 |
| `src/internal/app/usecase/shield/scan_usecase.go` | T2.2 |
| `src/internal/api/middleware/shield.go` | T2.3 |
| `src/internal/api/middleware/shield_metrics.go` | T2.3 |
| `src/internal/adapters/repository/postgres/incident.go` | T1.5 |
| `src/internal/api/handler/incident/` | T3.1 |
| `src/internal/api/dto/incident.go` | T3.2 |
| `src/internal/api/server.go` | T3.3 |
| `src/internal/api/admin.go` | T3.3 |
| `src/internal/api/router.go` | T3.3 |
| `src/cmd/gateway/main.go` | T3.4 |
| `src/cmd/admin/main.go` | T3.4 |
| `src/internal/infra/metrics/metrics.go` | T3.5 |
| `src/internal/infra/metrics/shield.go` | T3.5 |
| `ui/src/api/incidents.ts` | T3.6 |
| `ui/src/pages/Incidents/` | T3.6 |
| `ui/src/App.tsx` | T3.6 |
| `deployments/migrations/XXX_cleanup_incidents.up.sql` | T3.7 |
| `*_test.go` (все затронутые пакеты) | T4.1 |

## Implementation Context

- **Цель MVP:** полный cleanup audit incident инфраструктуры — entity, repository, handler, middleware, reactions, API, UI, metrics, migration. Все 16 AC.
- **Инварианты/семантика:**
  - Reactions (alert/block/mask/redact) переходят на `zap.Info()` вместо `IncidentRepository.Save()`
  - Middleware логирует dictionary hit structured info + эмитит counter metric по action
  - `ScanResult` теряет поле `incidents` и метод `Incidents()`; конструктор без incidents
  - `X-Shield-Incident-ID` header не устанавливается
- **Ошибки/коды:** нет новых error paths — только удаление кода
- **Контракты/протокол:**
  - HTTP: `GET /incidents`, `GET /incidents/:id` удаляются
  - Prometheus: `shield_incidents_by_severity` удаляется
  - UI: маршруты `/incidents`, `/incidents/:id` удаляются
- **Границы scope:**
  - Не менять исходную миграцию `003_incidents`
  - Не вводить session-based audit
  - Не менять tenant-сущности и scan engine
- **Proof signals:**
  - `grep -r 'Incident' src/` не находит значимых вхождений (кроме trace-маркеров `@sk-`)
  - `go build ./... && go vet ./... && go test ./...` exit 0
- **References:** DEC-001 (layer-by-layer), DEC-002 (structured logging), DEC-003 (middleware metrics), DEC-004 (new migration); DM-001 (Incident deleted), DM-002 (ScanResult changed)

## Фаза 1: Domain + Repository слой

Цель: удалить entity, repository interface, postgres реализацию, очистить ScanResult и scan service.

- [x] T1.1 Удалить `entity/incident.go` — весь файл (Incident, IncidentType, Severity, NewAuditIncident, IncidentOption). Touches: `src/internal/domain/shield/entity/incident.go`
- [x] T1.2 Очистить `entity/scan_result.go` — удалить поле `incidents []Incident`, метод `Incidents()`, адаптировать `NewScanResult` (убрать incidents parameter). Touches: `src/internal/domain/shield/entity/scan_result.go`
- [x] T1.3 Удалить `IncidentRepository` interface и `IncidentFilter` struct из `repository.go`. Touches: `src/internal/domain/shield/repository.go`
- [x] T1.4 Удалить создание Incident из `service/scan.go` и `service/evaluate.go` — scan возвращает status-only ScanResult. Touches: `src/internal/domain/shield/service/scan.go`, `src/internal/domain/shield/service/evaluate.go`
- [x] T1.5 Удалить `adapters/repository/postgres/incident.go` — весь файл. Touches: `src/internal/adapters/repository/postgres/incident.go`

## Фаза 2: Reactions + Use Cases + Middleware

Цель: убрать IncidentRepository dependency из реакций, убрать создание Incident из use case и middleware.

- [x] T2.1 Удалить `IncidentRepository` dependency из конструкторов `AlertReaction`, `BlockReaction`, `MaskReaction`, `RedactReaction`. Заменить сохранение Incident на `zap.Info()` structured log (severity, action, tenant, pattern). Удалить `*entity.Incident` из возврата и из `ReactionResult`. Touches: `src/internal/domain/shield/reaction/alert.go`, `block.go`, `mask.go`, `redact.go`, `reaction.go`
- [x] T2.2 Удалить incidents array из response в `scan_usecase.go`. Touches: `src/internal/app/usecase/shield/scan_usecase.go`
- [x] T2.3 Удалить создание Incident из shield middleware. Заменить на structured log + counter metric по action. Удалить `X-Shield-Incident-ID` header. Touches: `src/internal/api/middleware/shield.go`, `src/internal/api/middleware/shield_metrics.go`

## Фаза 3: API + Infrastructure + UI + Migration

Цель: удалить handler, DTO, routes, metrics, wiring, UI, добавить миграцию.

- [x] T3.1 Удалить пакет `handler/incident/` — handler.go, handler_test.go. Touches: `src/internal/api/handler/incident/`
- [x] T3.2 Удалить `dto/incident.go`. Touches: `src/internal/api/dto/incident.go`
- [x] T3.3 Удалить `RegisterIncidentHandler` и incident routes из `server.go`, `admin.go`, `router.go`. Touches: `src/internal/api/server.go`, `src/internal/api/admin.go`, `src/internal/api/router.go`
- [x] T3.4 Удалить `RegisterIncidentHandler` вызовы из `cmd/gateway/main.go` и `cmd/admin/main.go`. Touches: `src/cmd/gateway/main.go`, `src/cmd/admin/main.go`
- [x] T3.5 Удалить `ShieldIncidentsBySeverity` метрику, `IncidentsBySeverityGauge`, `IncidentIDForRequest`. Touches: `src/internal/infra/metrics/metrics.go`, `src/internal/infra/metrics/shield.go`
- [x] T3.6 Удалить `ui/src/api/incidents.ts`, `ui/src/pages/Incidents/`, удалить incident routes из `ui/src/App.tsx`. Touches: `ui/src/api/incidents.ts`, `ui/src/pages/Incidents/`, `ui/src/App.tsx`
- [x] T3.7 Создать миграцию cleanup: `deployments/migrations/009_cleanup_incidents.up.sql` с `DROP TABLE IF EXISTS incidents`. Touches: `deployments/migrations/009_cleanup_incidents.up.sql`, `deployments/migrations/009_cleanup_incidents.down.sql`

## Фаза 4: Проверка

Цель: обновить тесты, финальная сборка, верификация AC.

- [x] T4.1 Обновить все `*_test.go` файлы, ссылающиеся на удалённые типы (Incident, IncidentRepository, IncidentFilter, NewAuditIncident, и т.д.). Удалить или адаптировать тесты в: `reaction/*_test.go`, `middleware/shield_test.go`, `handler/incident/*_test.go` (удалён целиком), `postgres/*_test.go`, `service/*_test.go`, `usecase/*_test.go`. Touches: все test файлы затронутых пакетов.
- [x] T4.2 Финальная проверка: `go build ./... && go vet ./... && go test ./...` — exit 0. Параллельно: `grep -r 'IncidentRepository\|IncidentFilter\|RegisterIncidentHandler\|ShieldIncidentsBySeverity\|X-Shield-Incident-ID' src/` — пусто. Touches: весь репозиторий.

## Покрытие критериев приемки

- AC-001 -> T1.1
- AC-002 -> T1.3
- AC-003 -> T1.5
- AC-004 -> T3.1
- AC-005 -> T3.2
- AC-006 -> T1.2
- AC-007 -> T2.1
- AC-008 -> T2.3
- AC-009 -> T3.3, T3.4
- AC-010 -> T3.5
- AC-011 -> T2.3
- AC-012 -> T3.6
- AC-013 -> T3.6
- AC-014 -> T4.1, T4.2
- AC-015 -> T3.7
- AC-016 -> T1.3

## Заметки

- Порядок фаз жёсткий: Фаза 1 → Фаза 2 → Фаза 3 → Фаза 4. Каждая фаза зависит от предыдущей (compile-time dependency).
- T1.2 (ScanResult) может сломать use case/middleware — это нормально, исправляется в Фазе 2.
- T2.1 (reactions) — ключевое решение DEC-002: заменяем persistence на structured log.
- T3.7 (миграция) — номер миграции выбрать следующим по порядку после существующих.
- T4.1 и T4.2 выполняются строго после всех предыдущих фаз.
