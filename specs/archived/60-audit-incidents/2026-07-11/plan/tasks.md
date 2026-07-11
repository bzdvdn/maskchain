# Audit Incidents Задачи

## Phase Contract

Inputs: plan.md, data-model.md, spec.md, inspect.md, repo context.
Outputs: tasks.md.
Stop if: not applicable — plan, spec, inspect are pass.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/infra/migrations/002_incidents.sql` | T1.1 |
| `src/internal/domain/shield/entity/incident.go` | T1.2 |
| `src/internal/domain/shield/repository.go` | T1.3 |
| `src/internal/adapters/repository/postgres/incident.go` | T1.3, T4.2 |
| `src/internal/api/dto/incident.go` | T2.1 |
| `src/internal/api/handler/incident/handler.go` | T2.2 |
| `src/internal/api/handler/incident/export.go` | T2.2 |
| `src/internal/api/server.go` | T2.3 |
| `src/cmd/gateway/main.go` | T2.3 |
| `src/internal/domain/shield/reaction/alert.go` | T4.1 |
| `ui/src/api/incidents.ts` | T3.1 |
| `ui/src/pages/Incidents/IncidentList.tsx` | T3.2 |
| `ui/src/pages/Incidents/IncidentDetail.tsx` | T3.3 |
| `ui/src/pages/Incidents/index.ts` | T3.2, T3.3 |
| `ui/src/App.tsx` | T3.4 |
| `src/internal/api/handler/incident/handler_test.go` | T4.2 |

## Implementation Context

- Цель MVP: backend-only read endpoints для инцидентов (list, detail, export) — проверка curl
- Инварианты/семантика:
  - Incident immutable после создания (только read в этой фиче)
  - `tenant` денормализован в таблицу (DM-001, DEC-005)
  - `promptSnippetRedacted` — rename из `rawSnippet` (DEC-004); AlertReaction и repo.Save обновляются синхронно
  - `responseSnippet` — новое опциональное поле (nil по умолчанию)
  - Пагинация page/page_size offset = (page-1)*page_size (как в profiles)
- Ошибки/коды:
  - 404: инцидент не найден (code: `not_found`)
  - 400: неверный формат экспорта (code: `validation_error`)
  - 500: внутренняя ошибка (code: `internal`)
- Контракты/протокол:
  - `GET /api/v1/incidents?severity=&tenant=&profile_slug=&page=1&page_size=20` → PaginatedResponse
  - `GET /api/v1/incidents/:id` → IncidentResponse
  - `GET /api/v1/incidents/export?format=csv|json&severity=&tenant=&profile_slug=` → file download
- Proof signals:
  - curl на list endpoint возвращает пагинированный JSON
  - curl на export возвращает CSV/JSON с правильным Content-Type
  - AlertReaction компилируется и пишет в новое имя поля
- Вне scope: acknowledge/resolve/assign, realtime, dashboard, push-SIEM

## Фаза 1: Data foundation

Цель: подготовить миграцию, entity и репозиторий для чтения инцидентов.

- [x] T1.1 Создать `002_incidents.sql` — идемпотентная миграция: CREATE TABLE IF NOT EXISTS incidents с полями (id SERIAL PRIMARY KEY или UUID, profile_slug, request_id, detector_type, entry_value, severity, action, raw_snippet → prompt_snippet_redacted, timestamp, tenant VARCHAR, response_snippet TEXT). Если таблица уже существует — ALTER TABLE ADD COLUMN IF NOT EXISTS для tenant и response_snippet. Touches: `src/internal/infra/migrations/002_incidents.sql`

- [x] T1.2 Обновить `entity.Incident`: добавить поля `tenant string` и `responseSnippet *string`, переименовать `rawSnippet` → `promptSnippetRedacted`. Обновить `NewAuditIncident` — добавить параметр tenant (string). Добавить/обновить геттеры `Tenant()`, `ResponseSnippet()`, `PromptSnippetRedacted()`. Удалить старый геттер `RawSnippet()`. Touches: `src/internal/domain/shield/entity/incident.go`

- [x] T1.3 Добавить метод `List(ctx, IncidentFilter) ([]*Incident, total int, error)` в `IncidentRepository`. Определить `IncidentFilter` struct с полями Severity, Tenant, ProfileSlug (все *string/optional), Page, PageSize (int). Реализовать в `PostgresIncidentRepo`: динамический WHERE + COUNT + LIMIT/OFFSET. Touches: `src/internal/domain/shield/repository.go`, `src/internal/adapters/repository/postgres/incident.go`

## Фаза 2: API endpoints

Цель: handler + routing — list, detail, export работают через curl.

- [x] T2.1 Создать `dto/incident.go` — `IncidentResponse` (все поля из DM-001 для API), `IncidentListResponse` (reuse PaginatedResponse), `IncidentFilterParams` (query-параметры), `ExportQuery` (format + фильтры). Touches: `src/internal/api/dto/incident.go`

- [x] T2.2 Создать `handler/incident/handler.go`:
  - `ListIncidents(c)` — парсит `IncidentFilterParams` из query, вызывает `repo.List(ctx, filter)`, возвращает PaginatedResponse
  - `GetIncident(c)` — парсит `:id`, вызывает `repo.FindByID`, 404 если nil, иначе IncidentResponse
  Создать `handler/incident/export.go`:
  - `ExportIncidents(c)` — парсит ExportQuery, вызывает `repo.List` с фильтрами (page_size=0 для всех), сериализует в CSV или JSON по формату, устанавливает Content-Type, пишет response. CSV: encoding/csv с заголовками. JSON: json.Marshal в response. Touches: `src/internal/api/handler/incident/handler.go`, `src/internal/api/handler/incident/export.go`

- [x] T2.3 Зарегистрировать incident handler:
  - В `server.go`: `RegisterIncidentHandler(h *incident.Handler)` — группа `/api/v1/incidents` с порядком: GET `/export` (до `/:id`), GET `/` (list), GET `/:id` (detail)
  - В `main.go`: создать `postgres.NewPostgresIncidentRepo(pgPool)`, создать `incident.NewHandler(repo)`, вызвать `srv.RegisterIncidentHandler(h)` (после блока `if pgPool != nil`)
  Touches: `src/internal/api/server.go`, `src/cmd/gateway/main.go`

## Фаза 3: UI

Цель: страницы списка, деталей и экспорта инцидентов.

- [x] T3.1 Создать `ui/src/api/incidents.ts` — функции `listIncidents(params)`, `getIncident(id)`, `exportIncidents(params, format)` с типами `IncidentResponse`, `IncidentListItem`, `IncidentFilterParams`. Touches: `ui/src/api/incidents.ts`

- [x] T3.2 Создать `ui/src/pages/Incidents/IncidentList.tsx` — таблица с колонками timestamp, severity, tenant, profile_slug, detector_type, action. Фильтры: severity (select), tenant (input/text), profile_slug (input/text). Пагинация (page/page_size). Пустое состояние: "No incidents found". Touches: `ui/src/pages/Incidents/IncidentList.tsx`, `ui/src/pages/Incidents/index.ts`

- [x] T3.3 Создать `ui/src/pages/Incidents/IncidentDetail.tsx` — все поля инцидента: request_id, timestamp, tenant, profile_slug, detector_type, entry_value, severity, action, prompt_snippet_redacted (redacted text), response_snippet. Ссылка "Back to list". Touches: `ui/src/pages/Incidents/IncidentDetail.tsx`, `ui/src/pages/Incidents/index.ts`

- [x] T3.4 Добавить роуты в `ui/src/App.tsx`: `/incidents` → IncidentList, `/incidents/:id` → IncidentDetail. Навигация: пункт "Incidents" в header nav. Touches: `ui/src/App.tsx`

## Фаза 4: Sync + Verification

Цель: синхронизировать AlertReaction, покрыть тестами, верифицировать.

- [x] T4.1 Обновить `AlertReaction.Execute`: вызов `NewAuditIncident` с новым параметром `tenant` (пока пустая строка или `"default"`), использовать `PromptSnippetRedacted()` вместо `RawSnippet()`. Touches: `src/internal/domain/shield/reaction/alert.go`

- [x] T4.2 Написать handler tests:
  - List: пустая БД → data:[], total:0; с 3 critical инцидентами → фильтр по critical возвращает 3
  - Detail: существующий id → 200 + все поля; nonexistent → 404
  - Export: format=csv → text/csv + заголовки; format=json → application/json + массив; format=xml → 400
  Touches: `src/internal/api/handler/incident/handler_test.go`, `src/internal/adapters/repository/postgres/incident.go` (если нужен test helper)

## Покрытие критериев приемки

- AC-001 -> T1.3, T2.1, T2.2, T4.2
- AC-002 -> T2.2, T4.2
- AC-003 -> T2.2, T4.2
- AC-004 -> T2.2, T4.2
- AC-005 -> T3.2, T3.4
- AC-006 -> T1.3, T2.2, T4.2
- AC-007 -> T2.2, T4.2
- AC-008 -> T2.2, T4.2
