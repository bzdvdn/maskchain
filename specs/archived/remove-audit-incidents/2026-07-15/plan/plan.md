# Remove: удаление audit incident инфраструктуры — План

## Phase Contract

Inputs: spec (remove-audit-incidents), inspect (pass), минимальный repo-контекст.
Outputs: plan.md, data-model.md.
Stop if: spec неопределённа или inspect не pass.

## Цель

Удалить все следы `Incident` entity, `IncidentRepository`, реакций, middleware-логики, API, UI и метрик, связанных с audit incidents. Alert/Block/Mask/Redact реакции переходят на structured logging. Shield middleware логирует dictionary hits без создания Incident. ScanResult теряет поле incidents. Миграция drop table incidents. Код остаётся compiling и tests passing на каждом шаге.

## MVP Slice

Весь scope — MVP. Это чистый cleanup без новой функциональности. Все 16 AC должны быть закрыты.

## First Validation Path

После реализации первого потока (domain + repository layer):
1. `grep -r 'Incident' src/internal/domain/` — пусто (кроме trace-маркеров)
2. `grep -r 'IncidentRepository' src/` — пусто
3. `go build ./...` — exit 0
4. `go test ./...` — exit 0

## Scope

- Удаление entity: `incident.go`, чистка `scan_result.go`
- Удаление repository interface: `IncidentRepository`, `IncidentFilter` из `repository.go`
- Удаление Postgres реализации: `postgres/incident.go`
- Удаление incident из реакций: alert/block/mask/redact — убрать `IncidentRepository` dependency и возврат `*entity.Incident`
- Удаление scan service incident creation: `service/scan.go`, `service/evaluate.go`
- Удаление scan use case incident: `app/usecase/shield/scan_usecase.go`
- Удаление shield middleware incident creation: `api/middleware/shield.go`, `shield_metrics.go`
- Удаление API слоя: `api/handler/incident/`, `api/dto/incident.go`, `api/server.go`, `api/admin.go`, `api/router.go`
- Удаление wiring: `cmd/gateway/main.go`, `cmd/admin/main.go`
- Удаление метрик: `infra/metrics/metrics.go`, `infra/metrics/shield.go`
- Удаление UI: `ui/src/api/incidents.ts`, `ui/src/pages/Incidents/`, `ui/src/App.tsx`
- Добавление миграции: `deployments/migrations/*_cleanup_incidents.up.sql`
- Обновление тестов: все файлы, ссылающиеся на удаляемые типы

## Performance Budget

- `none`: cleanup не меняет горячие пути (только убирает код). После удаления Incident создания в middleware путь запроса укорачивается.

## Implementation Surfaces

### Deleted (целиком):
- `src/internal/domain/shield/entity/incident.go` — entity + value types
- `src/internal/adapters/repository/postgres/incident.go` — PostgresIncidentRepo
- `src/internal/api/handler/incident/` — весь пакет
- `src/internal/api/dto/incident.go` — DTO
- `ui/src/api/incidents.ts` — API клиент
- `ui/src/pages/Incidents/` — весь пакет

### Edited:
- `src/internal/domain/shield/entity/scan_result.go` — убрать `incidents []Incident`, `Incidents()`
- `src/internal/domain/shield/repository.go` — убрать `IncidentRepository`, `IncidentFilter`
- `src/internal/domain/shield/service/scan.go` — убрать создание Incident
- `src/internal/domain/shield/service/evaluate.go` — убрать incident-based severity
- `src/internal/app/usecase/shield/scan_usecase.go` — убрать incidents из response
- `src/internal/api/middleware/shield.go` — убрать создание Incident, header
- `src/internal/api/middleware/shield_metrics.go` — убрать IncidentIDForRequest
- `src/internal/domain/shield/reaction/alert.go` — убрать IncidentRepository dependency
- `src/internal/domain/shield/reaction/block.go` — убрать возврат Incident
- `src/internal/domain/shield/reaction/mask.go` — убрать возврат Incident
- `src/internal/domain/shield/reaction/redact.go` — убрать возврат Incident
- `src/internal/domain/shield/reaction/reaction.go` — убрать Incident из ReactionResult
- `src/internal/api/server.go` — убрать RegisterIncidentHandler
- `src/internal/api/admin.go` — убрать RegisterIncidentHandler
- `src/internal/api/router.go` — убрать incident routes
- `src/cmd/gateway/main.go` — убрать wiring
- `src/cmd/admin/main.go` — убрать wiring
- `src/internal/infra/metrics/metrics.go` — убрать ShieldIncidentsBySeverity
- `src/internal/infra/metrics/shield.go` — убрать IncidentsBySeverityGauge
- `ui/src/App.tsx` — убрать incident routes

### Test files (edited):
- Все `*_test.go`, которые импортируют или используют удаляемые типы

### New:
- `deployments/migrations/XXX_cleanup_incidents.up.sql` — DROP TABLE IF EXISTS incidents

## Bootstrapping Surfaces

- `none`: удаление не требует новой структуры

## Влияние на архитектуру

- DDD entity слой теряет `Incident`, `IncidentType`, `Severity` — core domain чистка
- Repository слой теряет `IncidentRepository` interface — на один repository меньше
- Reactions перестают зависеть от persistence — чище dependency injection
- Middleware перестаёт быть source of truth для Incident — ответственность переходит к structured logs
- API слой теряет целый handler + DTO
- Metrics теряют одну timeseries
- UI теряет две страницы + API client
- Миграция не backward-compatible (данные incidents теряются)
- После удаления `X-Shield-Incident-ID` header перестаёт устанавливаться

## Acceptance Approach

| AC | Подход | Surfaces | Evidence |
|----|--------|----------|----------|
| AC-001 Incident entity удалён | delete entity file | `entity/incident.go` | `test -f` |
| AC-002 IncidentRepository удалён | grep за интерфейсом | `repository.go` | `grep -r` |
| AC-003 PostgresIncidentRepo удалён | delete impl file | `postgres/incident.go` | `test -f` |
| AC-004 IncidentHandler удалён | delete handler dir | `handler/incident/` | `test -d` |
| AC-005 Incident DTO удалён | delete dto file | `dto/incident.go` | `test -f` |
| AC-006 ScanResult без incidents | edit entity | `entity/scan_result.go` | `grep` |
| AC-007 Реакции без IncidentRepository | edit reactions | `reaction/*.go` | `grep` |
| AC-008 Middleware без Incident | edit middleware | `middleware/shield.go` | `grep` |
| AC-009 RegisterIncidentHandler удалён | edit mains + api | `server.go`, `admin.go`, mains | `grep` |
| AC-010 Metrics удалены | edit metrics | `infra/metrics/*.go` | `grep` |
| AC-011 Header удалён | edit middleware | `middleware/shield.go` | `grep` |
| AC-012 UI pages удалены | delete UI dirs | `ui/` | `test -f/d` |
| AC-013 UI routes удалены | edit App.tsx | `App.tsx` | `grep` |
| AC-014 Компиляция + тесты | build | все edited files | `go build/test` |
| AC-015 Миграция cleanup | new file | `deployments/migrations/` | `grep` |
| AC-016 IncidentFilter удалён | edit | `repository.go` | `grep` |

## Данные и контракты

- `data-model.md` — статус `changed`: Incident entity и таблица `incidents` удаляются. ScanResult теряет поле incidents.
- HTTP API теряет: `GET /incidents`, `GET /incidents/:id`, `X-Shield-Incident-ID` header.
- Prometheus теряет: `shield_incidents_by_severity` metric.
- UI теряет: маршруты `/incidents`, `/incidents/:id`.

## Стратегия реализации

### DEC-001 Layer-by-layer bottom-up (domain → repository → API → UI)
- Why: каждая зависимость тестируется сразу. Domain entity удаляется первой — компилятор покажет все использования вверх по стеку. Это самый безопасный порядок для cleanup.
- Tradeoff: временный partial compile error между шагами. Минимизируется параллельной работой в отдельных коммитах/задачах.
- Affects: все surfaces
- Validation: `go build ./...` после каждого слоя

### DEC-002 Structured logging вместо IncidentRepository в реакциях
- Why: реакции (alert/block/mask/redact) принимают `IncidentRepository` только для `Save`. После удаления persistence, structured log (`zap.Info` с severity, action, tenant, pattern) даёт достаточную audit trail для текущей фазы. Session-based аудит (120+) даст полноценную замену.
- Tradeoff: audit trail перестаёт быть queryable через SQL до появления session-based аудита.
- Affects: `reaction/alert.go`, `block.go`, `mask.go`, `redact.go`
- Validation: `grep 'IncidentRepository' src/internal/domain/shield/reaction/` пуст

### DEC-003 Middleware эмитит метрики без создания Incident
- Why: shield middleware создавал Incident чтобы передать ID в `X-Shield-Incident-ID` header и метрики. Header удаляется (будет заменён span attribute). Метрики по severity заменяются на простой counter по action (allow/block/redact/mask).
- Tradeoff: теряется per-request incident ID до session-based аудита.
- Affects: `middleware/shield.go`, `middleware/shield_metrics.go`, `infra/metrics/shield.go`
- Validation: `grep 'Incident' src/internal/api/middleware/shield.go` пуст

### DEC-004 Исходная миграция 003_incidents не меняется
- Why: миграция уже применена на production/staging. Менять её — risk rollback sequence. Новая миграция чистки идёт следующим номером.
- Tradeoff: последовательность миграций удлиняется на один файл.
- Affects: `deployments/migrations/`
- Validation: новая миграция содержит `DROP TABLE IF EXISTS incidents`

## Incremental Delivery

### Поток A: Domain + Repository (AC-001, AC-002, AC-003, AC-006, AC-016)
Удалить entity, repository interface, postgres implementation, очистить ScanResult. `go build ./...` после каждого удаления. Можно проверить: `grep -r 'Incident' src/internal/domain/` пуст.

### Поток B: Reactions + Use Cases (AC-007, AC-008, AC-011)
Убрать IncidentRepository dependency из реакций, заменить на structured log. Убрать создание Incident из scan_usecase и middleware. Убрать header. `go build ./...` + reactions unit tests.

### Поток C: API + Metrics + Wiring (AC-004, AC-005, AC-009, AC-010)
Удалить handler, DTO, routes, метрики. Отвязать wiring в main.go. `go build ./...` + API integration tests.

### Поток D: UI + Migration (AC-012, AC-013, AC-015)
Удалить UI страницы/API/routes. Добавить миграцию cleanup. `go build ./...` + `go test ./...` + `npm run build` (если есть).

### Финальная проверка: Компиляция + тесты (AC-014)
`go build ./... && go vet ./... && go test ./...` — exit 0.

## Порядок реализации

1. **Поток A** — первым. Без него B/C не скомпилируются.
2. **Поток B** — после A. Меняет интерфейсы реакций и middleware.
3. **Поток C** — после B. Удаляет API слой, который использует типы из A и B.
4. **Поток D** — может идти параллельно с C, но после A+B.
5. **Финальная проверка** — в конце.

Потоки A+B можно реализовать одним pass (domain → repository → reactions → middleware), C — вторым, D — третьим.

## Риски

1. **Пропущенные reference**: в коде могут быть неочевидные использования Incident, не покрытые spec/plan.
   - Mitigation: после каждого потока `grep -r 'Incident' src/` для поиска оставшихся reference. `go build` как safety net.

2. **UI сборка сломана**: `npm run build` может не работать в окружении.
   - Mitigation: удаление UI файлов (AC-012, AC-013) проверять через `test -f/d`, а не через сборку. Если нужно — отдельный dev-шаг.

3. **Тесты используют удалённые типы**: mock/fake реализации могут ссылаться на удаляемые интерфейсы.
   - Mitigation: после каждого потока `go test ./...` находит сломанные тесты.

## Rollout и compatibility

- Специальных rollout-действий не требуется. Изменения безопасности: после деплоя новой миграции таблица `incidents` удаляется, данные теряются. Перед деплоем убедиться, что incident данные не нужны для compliance/audit.
- `X-Shield-Incident-ID` header исчезает из ответов middleware — предупредить интеграторов.

## Проверка

- **Automated**: `go build ./...`, `go vet ./...`, `go test ./...`, `grep -r` checks из AC
- **Manual**: `test -f`, `test -d` для удалённых файлов; `grep` для проверки отсутствия reference
- **Operational**: после деплоя проверить, что shield middleware не логирует ошибок о missing IncidentRepository

## Соответствие конституции

- нет конфликтов
