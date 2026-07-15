# Remove: удаление audit incident инфраструктуры

## Scope Snapshot

- **In scope:** удаление `Incident` entity, `IncidentRepository` interface, `PostgresIncidentRepo`, `IncidentHandler`, incident DTO, incident-зависимостей в middleware/reactions/scan/metrics, incident UI страниц и API клиента. Миграция cleanup (DROP TABLE `incidents`) — в scope.
- **Out of scope:** session-based audit как замена (phase 120+). Tenant-сущности, политики, scan engine, dictionary cache — не трогаются.

## Цель

Разработчик больше не видит `Incident` entity, `IncidentRepository` и связанные типы, которые станут мёртвым грузом после перехода на session-based аудит. Alert/Block/Mask/Redact реакции перестают требовать `IncidentRepository` и пишут структурированный лог вместо персистентного Incident. Shield middleware перестаёт создавать `Incident` объекты, заменяя их structured log + metrics. ScanResult теряет поле `[]Incident`, severity выводится из статуса. После cleanup код компилируется, `go vet` чист, все тесты проходят.

## Основной сценарий

1. `Incident` entity удалён из `src/internal/domain/shield/entity/`, `ScanResult.Incidents()` больше не существует.
2. `IncidentRepository` интерфейс отсутствует в `src/internal/domain/shield/repository.go`.
3. `PostgresIncidentRepo` и весь файл `src/internal/adapters/repository/postgres/incident.go` удалён.
4. `AlertReaction`, `BlockReaction`, `MaskReaction`, `RedactReaction` не принимают `IncidentRepository` и не возвращают `*entity.Incident`.
5. Shield middleware (`src/internal/api/middleware/shield.go`) не создаёт `Incident` — логирует structured info и эмитит метрики.
6. `IncidentHandler` и весь пакет `src/internal/api/handler/incident/` удалён.
7. `ShieldIncidentsBySeverity` метрика удалена.
8. UI страницы Incidents (List, Detail) и API клиент удалены.
9. Миграция cleanup удаляет таблицу `incidents`.
10. `go build ./...` и `go test ./...` проходят без ошибок.

## MVP Slice

Весь scope — MVP. Чистый cleanup без новой функциональности.

## First Deployable Outcome

После первого implementation pass:
- `src/internal/domain/shield/entity/incident.go` удалён
- `src/internal/domain/shield/repository.go` не содержит `IncidentRepository` и `IncidentFilter`
- `src/internal/adapters/repository/postgres/incident.go` удалён
- `src/internal/api/handler/incident/` удалён
- `src/internal/api/dto/incident.go` удалён
- `src/internal/api/server.go` и `admin.go` не содержат `RegisterIncidentHandler`
- Реакции (alert/block/mask/redact) не принимают `IncidentRepository`
- Shield middleware не создаёт `Incident`
- `ShieldIncidentsBySeverity` удалён из метрик
- `ui/src/pages/Incidents/` и `ui/src/api/incidents.ts` удалены
- Миграция cleanup добавлена в `deployments/migrations/`
- `go build ./...` и `go test ./...` проходят

## Scope

- `src/internal/domain/shield/entity/incident.go` — удалить весь файл (Incident, IncidentType, Severity, NewAuditIncident, IncidentOption)
- `src/internal/domain/shield/entity/scan_result.go` — удалить поле `incidents []Incident`, метод `Incidents()`, конструктор `NewScanResult` без incidents
- `src/internal/domain/shield/repository.go` — удалить `IncidentRepository` interface, `IncidentFilter` struct, `ListIncidentsFilter`
- `src/internal/domain/shield/service/scan.go` — заменить создание Incident на status-only ScanResult
- `src/internal/domain/shield/service/evaluate.go` — удалить incident-based severity logic
- `src/internal/app/usecase/shield/scan_usecase.go` — удалить incident array из response
- `src/internal/api/middleware/shield.go` — перестать создавать Incident; логировать structured info + метрики
- `src/internal/adapters/repository/postgres/incident.go` — удалить весь файл
- `src/internal/domain/shield/reaction/alert.go` — удалить IncidentRepository dependency, заменить на structured log
- `src/internal/domain/shield/reaction/block.go` — удалить возврат `*entity.Incident` из react
- `src/internal/domain/shield/reaction/mask.go` — удалить возврат `*entity.Incident` из react
- `src/internal/domain/shield/reaction/redact.go` — удалить возврат `*entity.Incident` из react
- `src/internal/domain/shield/reaction/reaction.go` — удалить `*entity.Incident` из ReactionResult
- `src/internal/api/dto/incident.go` — удалить весь файл
- `src/internal/api/handler/incident/` — удалить весь пакет (handler.go, handler_test.go, dto.go)
- `src/internal/api/server.go` — удалить `RegisterIncidentHandler`, import incident handler
- `src/internal/api/admin.go` — удалить `RegisterIncidentHandler`, import incident handler
- `src/cmd/gateway/main.go` — удалить `RegisterIncidentHandler` вызов
- `src/cmd/admin/main.go` — удалить `RegisterIncidentHandler` вызов
- `src/internal/infra/metrics/metrics.go` — удалить `ShieldIncidentsBySeverity` метрику и регистрацию
- `src/internal/infra/metrics/shield.go` — удалить `IncidentsBySeverityGauge`, `IncidentIDForRequest`
- `src/internal/api/middleware/shield_metrics.go` — удалить `X-Shield-Incident-ID` header
- `src/internal/api/router.go` — удалить incident routes (GET /incidents, GET /incidents/:id)
- `ui/src/api/incidents.ts` — удалить весь файл
- `ui/src/pages/Incidents/` — удалить весь пакет (List, Detail, компоненты)
- `ui/src/App.tsx` — удалить incident route
- `deployments/migrations/XXX_cleanup_incidents.sql` — создать миграцию DROP TABLE incidents

## Контекст

- Session-based аудит запланирован на phase 120+, который заменит персистентные Incident объекты на session logs.
- Текущая `Incident` сущность создаётся в трёх местах: shield middleware (dictionary hits), scan_usecase (pattern hits), и PostgresIncidentRepo (audit incidents через `NewAuditIncident`).
- Alert/Block/Mask/Redact реакции принимают `IncidentRepository` только для сохранения Incident, после удаления они будут логировать structured info.
- `ScanResult` содержит `[]Incident`, который фактически не используется downstream (только для передачи в middleware).
- `IncidentFilter` используется только incident list handler и будет удалён вместе с ним.
- Исходная миграция `003_incidents` (создание таблицы) сохраняется как есть — она уже применена на всех окружениях.

## Зависимости

- Нет внешних зависимостей. Фича чистит мёртвый код, который будет заменён в phase 120+.

## Требования

### RQ-001 Удаление Incident entity
Система ДОЛЖНА удалить `entity.Incident`, `IncidentType`, `Severity`, `NewAuditIncident`, `IncidentOption` из `src/internal/domain/shield/entity/incident.go`.

### RQ-002 Удаление IncidentRepository
Система ДОЛЖНА удалить `IncidentRepository` interface и `IncidentFilter` из `src/internal/domain/shield/repository.go`.

### RQ-003 Удаление PostgresIncidentRepo
Система ДОЛЖНА удалить `PostgresIncidentRepo` и весь файл `src/internal/adapters/repository/postgres/incident.go`.

### RQ-004 Удаление IncidentHandler
Система ДОЛЖНА удалить пакет `src/internal/api/handler/incident/` (handler + handler_test).

### RQ-005 Удаление incident DTO
Система ДОЛЖНА удалить `src/internal/api/dto/incident.go`.

### RQ-006 Очистка ScanResult
Система ДОЛЖНА удалить поле `incidents []Incident`, метод `Incidents()`, и адаптировать конструктор `NewScanResult` в `entity/scan_result.go`.

### RQ-007 Удаление Incident из реакций
Система ДОЛЖНА удалить `IncidentRepository` dependency и возврат `*entity.Incident` из AlertReaction, BlockReaction, MaskReaction, RedactReaction.

### RQ-008 Очистка shield middleware
Система ДОЛЖНА удалить создание Incident в `src/internal/api/middleware/shield.go`, заменив на structured log + метрики. `X-Shield-Incident-ID` header удаляется.

### RQ-009 Удаление Incident-маршрутов
Система ДОЛЖНА удалить `RegisterIncidentHandler` из `server.go`, `admin.go`, `cmd/gateway/main.go`, `cmd/admin/main.go`.

### RQ-010 Удаление метрик Incident
Система ДОЛЖНА удалить `ShieldIncidentsBySeverity` метрику и `IncidentIDForRequest` из `src/internal/infra/metrics/`.

### RQ-011 Удаление UI incident страниц
Система ДОЛЖНА удалить `ui/src/api/incidents.ts`, `ui/src/pages/Incidents/`, и incident routes из `ui/src/App.tsx`.

### RQ-012 Миграция cleanup
Система ДОЛЖНА создать миграцию, удаляющую таблицу `incidents`.

### RQ-013 Компиляция и тесты
После всех изменений `go build ./...` и `go test ./...` ДОЛЖНЫ проходить без ошибок.

## Вне scope

- Session-based аудит (phase 120+) — не реализуется.
- Tenant-сущности, политики, scan engine, dictionary cache — не трогаются.
- Исходная миграция `003_incidents` — не изменяется (уже применена).
- Изменение схемы БД для других таблиц — не в scope.

## Критерии приемки

### AC-001 Incident entity удалён
- Почему это важно: entity больше не мозолит глаза
- **Given** файл `src/internal/domain/shield/entity/incident.go`
- **When** проверяется наличие файла
- **Then** файл не существует
- **Evidence** `test -f src/internal/domain/shield/entity/incident.go && echo exists || echo not found` возвращает "not found"

### AC-002 IncidentRepository удалён
- **Given** `src/internal/domain/shield/repository.go`
- **When** разработчик ищет `IncidentRepository` в коде
- **Then** ни одного вхождения не найдено
- **Evidence** `grep -r 'IncidentRepository' src/` возвращает пустой результат

### AC-003 PostgresIncidentRepo удалён
- **Given** файл `src/internal/adapters/repository/postgres/incident.go`
- **When** проверяется наличие файла
- **Then** файл не существует
- **Evidence** `test -f src/internal/adapters/repository/postgres/incident.go && echo exists || echo not found` возвращает "not found"

### AC-004 IncidentHandler удалён
- **Given** пакет `src/internal/api/handler/incident/`
- **When** проверяется наличие пакета
- **Then** пакет не существует
- **Evidence** `test -d src/internal/api/handler/incident/ && echo exists || echo not found` возвращает "not found"

### AC-005 Incident DTO удалён
- **Given** файл `src/internal/api/dto/incident.go`
- **When** проверяется наличие файла
- **Then** файл не существует
- **Evidence** `test -f src/internal/api/dto/incident.go && echo exists || echo not found` возвращает "not found"

### AC-006 ScanResult не содержит incidents
- **Given** `src/internal/domain/shield/entity/scan_result.go`
- **When** проверяется отсутствие поля `incidents` и метода `Incidents()`
- **Then** поле и метод удалены
- **Evidence** `grep 'incidents\|Incidents' src/internal/domain/shield/entity/scan_result.go` не показывает поле или метод (кроме `ScanStatus`)

### AC-007 Реакции не принимают IncidentRepository
- **Given** файлы реакций: `alert.go`, `block.go`, `mask.go`, `redact.go` в `src/internal/domain/shield/reaction/`
- **When** проверяется наличие `IncidentRepository` в конструкторах
- **Then** ни одна реакция не принимает `IncidentRepository`
- **Evidence** `grep -r 'IncidentRepository' src/internal/domain/shield/reaction/` пуст

### AC-008 Shield middleware не создаёт Incident
- **Given** `src/internal/api/middleware/shield.go`
- **When** проверяется создание `entity.Incident`
- **Then** middleware не создаёт Incident, не устанавливает `X-Shield-Incident-ID`
- **Evidence** `grep 'Incident' src/internal/api/middleware/shield.go` пуст (кроме комментариев или trace-маркеров)

### AC-009 RegisterIncidentHandler удалён
- **Given** `server.go`, `admin.go`, `cmd/gateway/main.go`, `cmd/admin/main.go`
- **When** проверяется наличие `RegisterIncidentHandler`
- **Then** функция не найдена
- **Evidence** `grep -r 'RegisterIncidentHandler' src/` пуст

### AC-010 ShieldIncidentsBySeverity метрика удалена
- **Given** `src/internal/infra/metrics/`
- **When** проверяется наличие `ShieldIncidentsBySeverity`
- **Then** метрика отсутствует
- **Evidence** `grep -r 'ShieldIncidentsBySeverity\|IncidentsBySeverity' src/` пуст

### AC-011 Incident ID header удалён
- **Given** `src/internal/api/middleware/shield.go` и `shield_metrics.go`
- **When** проверяется `X-Shield-Incident-ID`
- **Then** header не устанавливается
- **Evidence** `grep -r 'X-Shield-Incident-ID\|ShieldIncidentID\|IncidentID' src/` пуст (кроме trace-маркеров)

### AC-012 UI incident страницы удалены
- **Given** `ui/src/pages/Incidents/` и `ui/src/api/incidents.ts`
- **When** проверяется наличие файлов
- **Then** файлы не существуют
- **Evidence** `test -f ui/src/api/incidents.ts && echo exists || echo not found` возвращает "not found"; `test -d ui/src/pages/Incidents/ && echo exists || echo not found` возвращает "not found"

### AC-013 Incident routes удалены из UI
- **Given** `ui/src/App.tsx`
- **When** проверяется наличие incident-import и route
- **Then** ни импорта `Incidents*`, ни route `/incidents` нет
- **Evidence** `grep -i 'incident' ui/src/App.tsx` пуст

### AC-014 Компиляция и тесты
- **Given** код после cleanup
- **When** `go build ./... && go vet ./... && go test ./...`
- **Then** все три команды возвращают exit code 0
- **Evidence** вывод команд не содержит ошибок

### AC-015 Миграция cleanup создана
- **Given** `deployments/migrations/`
- **When** проверяется наличие файла миграции cleanup incidents
- **Then** файл существует и содержит `DROP TABLE IF EXISTS incidents`
- **Evidence** `grep -l 'DROP.*TABLE.*incidents' deployments/migrations/*` находит файл

### AC-016 IncidentFilter удалён
- **Given** `src/internal/domain/shield/repository.go`
- **When** проверяется наличие `IncidentFilter`
- **Then** struct отсутствует
- **Evidence** `grep 'IncidentFilter' src/internal/domain/shield/repository.go` пуст

## Допущения

- Session-based аудит (phase 120+) заменит функциональность персистентных Incident объектов.
- Alert/Block/Mask/Redact реакции после удаления IncidentRepository будут использовать structured logging для аудита.
- Shield middleware продолжит эмитить метрики (без Incident-специфичных метрик).
- Исходная миграция `003_incidents` остаётся в истории миграций (уже применена на всех окружениях).

## Краевые случаи

- Если какой-то внешний интегратор полагается на `X-Shield-Incident-ID` header — после удаления header исчезнет. Замена появится в session-based аудите.
- Если UI incident страницы используются операторами — после удаления нужно убедиться, что операторы знают о переходе на session-based UI в будущем.

## Открытые вопросы

- **Внешние интеграторы**: Есть ли известные интеграторы, читающие `X-Shield-Incident-ID`? Если да — нужно согласовать EOL. Пока считаем, что нет.
- **Grafana dashboard**: Есть ли дашборды, использующие `ShieldIncidentsBySeverity` метрику? Если да — нужно удалить или заменить панели.
