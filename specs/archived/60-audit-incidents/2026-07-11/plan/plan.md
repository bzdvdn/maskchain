# Audit Incidents План

## Phase Contract

Inputs: specs/active/60-audit-incidents/spec.md, inspect.md, repo context (entity, repository, API routing, UI patterns).
Outputs: plan.md, data-model.md.
Stop if: not applicable — spec and inspect are pass.

## Цель

Расширить существующую модель Incident и PostgresIncidentRepo для read-доступа: list с фильтрацией, детальный просмотр, экспорт CSV/JSON. Добавить UI-страницы списка и деталей по шаблону Profiles. AlertReaction и write path не ломать.

## MVP Slice

Backend-only: миграция + entity + repo.List + handler + routing. AC-001, AC-002, AC-006, AC-007. Проверяется curl без UI.

## First Validation Path

```bash
# после миграции и seed 3-х инцидентов (через AlertReaction или SQL)
curl -s 'http://localhost:8080/api/v1/incidents?page=1&page_size=2' | jq '.data | length'
# → 2, total: 3
curl -s 'http://localhost:8080/api/v1/incidents/nonexistent' | jq '.code'
# → "not_found"
```

## Scope

- Migration `002_incidents.sql`: CREATE TABLE IF NOT EXISTS + ALTER для новых колонок (tenant, response_snippet)
- Domain entity: добавить `tenant`, `responseSnippet`, rename `rawSnippet` → `promptSnippetRedacted`
- `IncidentRepository`: добавить `List(ctx, IncidentFilter) ([]*Incident, total, error)`
- Postgres-реализация `List` с WHERE-фильтрацией и COUNT-пагинацией
- Handler `api/handler/incident/`: три эндпоинта (List, Get, Export)
- DTO: `IncidentResponse`, `IncidentFilterParams`, `ExportQuery`
- Routing: группа `/api/v1/incidents` в server.go
- Wiring: main.go — создать incident handler, зарегистрировать
- UI: страницы `/incidents`, `/incidents/:id`, API client
- AlertReaction: обновить вызов field name после rename

## Performance Budget

- `GET /api/v1/incidents` c фильтром: <200ms p95 при 10k строк (SC-001)
- Export CSV 1000 строк: <2s (SC-002)
- Экспорт в JSON в потоке (не building full body in memory)

## Implementation Surfaces

| Surface | Why | New/Existing |
|---|---|---|
| `src/internal/domain/shield/entity/incident.go` | add fields, rename field | existing — change |
| `src/internal/domain/shield/repository.go` | add `List(ctx, filter)` method | existing — change |
| `src/internal/domain/shield/value/` | filter value object or use raw types | existing — minor |
| `src/internal/adapters/repository/postgres/incident.go` | implement `List` with WHERE/COUNT | existing — change |
| `src/internal/infra/migrations/002_incidents.sql` | new migration | new |
| `src/internal/api/handler/incident/` | handler package (list, get, export) | new |
| `src/internal/api/dto/incident.go` | incident DTOs | new |
| `src/internal/api/server.go` | register incident routes | existing — change |
| `src/cmd/gateway/main.go` | wire incident handler | existing — change |
| `src/internal/domain/shield/reaction/alert.go` | update field name | existing — change |
| `ui/src/api/incidents.ts` | API client | new |
| `ui/src/pages/Incidents/` | list + detail pages | new |
| `ui/src/App.tsx` | add incident routes | existing — change |

## Bootstrapping Surfaces

- `src/internal/api/handler/incident/` — директория handler
- `ui/src/api/incidents.ts` — API client stub
- `ui/src/pages/Incidents/` — pages directory

## Влияние на архитектуру

- AlertReaction пишет через `IncidentRepository.Save` — rename поля требует синхронизации: AlertReaction должен писать `promptSnippetRedacted` вместо `rawSnippet`
- Денормализация tenant в таблицу incidents — небольшое дублирование, но устраняет JOIN при фильтрации
- Export endpoint — отдельный handler, не затрагивает list endpoint
- UI-страницы следуют существующему шаблону Profiles (никаких новых зависимостей)

## Acceptance Approach

- AC-001: handler test с seed данными, проверка PaginatedResponse + фильтр severity
- AC-002: handler test: GET /:id → 200 + все поля; GET /nonexistent → 404
- AC-003: handler test: export?format=csv → Content-Type + тело с заголовками
- AC-004: handler test: export?format=json → Content-Type + массив
- AC-005: manual UI check + component test (table renders columns)
- AC-006: handler test: пустая БД → data:[], total:0
- AC-007: handler test: nonexistent id → 404 error response
- AC-008: handler test: format=xml → 400 error response

## Данные и контракты

- data-model.md прилагается (changed — расширение Incident)
- API контракты: вводятся 3 новых endpoint; существующие (profiles, mask, proxy) не меняются
- PaginatedResponse переиспользуется (уже в dto)
- Event-контракты не меняются

## Стратегия реализации

- DEC-001 Export endpoint — отдельный путь `/export`, зарегистрированный до `/:id` (Gin статический роутинг отрабатывает корректно). Tradeoff: `/export` не может быть id инцидента; это допустимо, т.к. id — UUID
- DEC-002 Handler → Repo напрямую (без use-case слоя) — следует шаблону Profiles. Export-логика выделена в отдельный файл export.go внутри пакета handler. Tradeoff: тоньше, быстрее; при усложнении логики можно добавить use-case слои
- DEC-003 Filter struct — отдельный `IncidentFilter` с опциональными полями + Page/PageSize, передаётся в `repo.List()`. WHERE строится динамически (без ORM)
- DEC-004 Field rename `rawSnippet` → `promptSnippetRedacted` — миграция добавляет колонку `prompt_snippet_redacted` через ALTER TABLE (или CREATE TABLE для новой таблицы). AlertReaction обновляется на новое имя. Старое поле удаляется из кода
- DEC-005 Tenant denormalization — колонка `tenant VARCHAR` в таблице incidents. AlertReaction и другие создатели инцидентов передают tenant (пока default). Tradeoff: дублирование данных vs производительность фильтрации без JOIN

## Incremental Delivery

### MVP (Первая ценность)

Backend endpoints: migration → entity update → repo.List → handler → routing. AC-001, AC-002, AC-006, AC-007. Проверка: curl.

### Итеративное расширение

- Iteration 2: Export endpoint — AC-003, AC-004, AC-008
- Iteration 3: UI страница списка — AC-005 (read-only таблица)
- Iteration 4: UI детальный просмотр — AC-005 + AC-005 detail
- Iteration 5: UI export button — AC-003, AC-004 (UI trigger)

## Порядок реализации

1. Migration `002_incidents.sql` — первым (иначе entity/repo не скомпилируются с новыми полями)
2. Entity `incident.go` — обновить поля (tenant, responseSnippet, rename)
3. Repository interface — добавить `List`
4. Postgres repo — реализовать `List`
5. DTO — `IncidentResponse`, `IncidentFilterParams`
6. Handler — List, Get, Export
7. Server routing — зарегистрировать
8. main.go wiring — wire handler
9. AlertReaction — обновить field name
10. UI API client + pages

Параллельно: UI API client (не зависит от имплементации handler, только от контракта)

## Риски

- **AlertReaction использует nil repo** — в main.go строка `reaction.NewAlertReaction(nil)`. Если инциденты не сохраняются, списки будут пусты. Mitigation: при реализации проверить wiring AlertReaction или отметить как известный пробел (будет исправлено в интеграции ShieldEngine)
- **Rename rawSnippet → promptSnippetRedacted сломает существующие миграции** — если таблица уже существует с колонкой `raw_snippet`. Mitigation: миграция использует ALTER TABLE ADD COLUMN IF NOT EXISTS для новых колонок; старую колонку оставить или удалить отдельным шагом
- **Gin route order** — `/export` должен быть зарегистрирован до `/:id`. Mitigation: порядок регистрации в `RegisterIncidentHandler` явно документирован

## Rollout и compatibility

- Новая миграция идемпотентна (IF NOT EXISTS)
- Старые инциденты без tenant получают default при первом чтении или через backfill
- Обратная совместимость: старые записи без response_snippet возвращают null
- Специальных rollout-действий не требуется; фича read-only, влияния на существующие потоки нет

## Проверка

- Handler unit tests: List (пусто, с фильтрами, без фильтров, пагинация), Get (found, 404), Export (csv, json, bad format)
- Repo integration test: List с seed + различные фильтры
- UI: manual check / component test table renders columns
- AC coverage: AC-001–AC-008 покрыты handler tests, AC-005 дополнительно manual UI

## Соответствие конституции

- нет конфликтов; spec соответствует конституции (Go + Gin + React, DDD, PostgreSQL, Content Shield core domain)
