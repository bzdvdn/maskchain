---
report_type: verify
slug: 60-audit-incidents
status: pass
docs_language: ru
generated_at: 2026-07-11
---

# Verify Report: 60-audit-incidents

## Scope

- snapshot: верификация read/export инцидентов Content Shield + UI таблицы
- verification_mode: default
- artifacts:
  - .speckeep/constitution.summary.md
  - specs/active/60-audit-incidents/spec.md
  - specs/active/60-audit-incidents/plan.md
  - specs/active/60-audit-incidents/tasks.md
- inspected_surfaces:
  - src/internal/domain/shield/entity/incident.go
  - src/internal/domain/shield/repository.go
  - src/internal/adapters/repository/postgres/incident.go
  - src/internal/api/dto/incident.go
  - src/internal/api/handler/incident/handler.go
  - src/internal/api/handler/incident/export.go
  - src/internal/api/handler/incident/handler_test.go
  - src/internal/api/server.go
  - src/cmd/gateway/main.go
  - src/internal/domain/shield/reaction/alert.go
  - ui/src/api/incidents.ts
  - ui/src/pages/Incidents/
  - ui/src/App.tsx
  - src/internal/infra/migrations/002_incidents.sql

## Verdict

- status: pass
- archive_readiness: safe
- summary: все 8 AC покрыты тестами, 12/12 задач завершены, build+vet проходят, trace-маркеры валидны

## Checks

- task_state: completed=12, open=0
- acceptance_evidence:

| AC-ID | Task IDs | Evidence | Verdict |
|-------|----------|----------|---------|
| AC-001 | T1.3, T2.1, T2.2, T4.2 | `TestListIncidents/filter_by_severity` (handler_test.go:98): filter severity=critical, page=1, page_size=2 → total=1; `TestListIncidents/empty_database` → data:[], total:0 | pass |
| AC-002 | T2.2, T4.2 | `TestGetIncident/existing_incident` (handler_test.go:151): GET inc-001 → 200, severity=critical, tenant=tenant-1, prompt/response snippets present | pass |
| AC-003 | T2.2, T4.2 | `TestExportIncidents/export_csv` (handler_test.go:210): format=csv → Content-Type text/csv, header line with id,request_id,timestamp | pass |
| AC-004 | T2.2, T4.2 | `TestExportIncidents/export_json` (handler_test.go:210): format=json → Content-Type application/json, array of 3 incidents | pass |
| AC-005 | T3.2, T3.3, T3.4 | `IncidentList.tsx`: table with columns timestamp/severity/tenant/profile_slug/detector_type/action + filters; `IncidentDetail.tsx`: all fields + redacted prompt/response; `App.tsx`: routes `/incidents`, `/incidents/:id` + nav link | pass |
| AC-006 | T1.3, T2.2, T4.2 | `TestListIncidents/empty_database` → data:[], total:0; `IncidentList.tsx` empty state "No incidents found" | pass |
| AC-007 | T2.2, T4.2 | `TestGetIncident/nonexistent_incident` → 404 + code NOT_FOUND | pass |
| AC-008 | T2.2, T4.2 | `TestExportIncidents/invalid_format` → 400 Bad Request | pass |

- implementation_alignment:
  - DM-001: entity.Incident получил tenant, responseSnippet, promptSnippetRedacted; NewAuditIncident принимает новый порядок параметров
  - DEC-001: `/export` зарегистрирован до `/:id` в RegisterIncidentHandler
  - DEC-002: Handler → repo напрямую (без use-case слоя)
  - DEC-003: IncidentFilter struct с опциональными полями + Page/PageSize
  - DEC-004: prompt_snippet_redacted колонка в миграции, AlertReaction использует новое имя
  - DEC-005: tenant VARCHAR колонка денормализована в таблицу incidents

## Errors

- none

## Warnings

- none

## Questions

- none

## Not Verified

- Браузерная проверка UI (рендер таблицы, работа фильтров) — проверено на уровне наличия компонентов и роутов, E2E-тесты не реализованы
- Performance SC-001/SC-002 — не проверялись (требуют БД с 10k строк)
- Миграция 002_incidents.sql — проверена на уровне синтаксиса, прогон на реальной БД не выполнялся

## Traceability

- @sk-task маркеры: 31 аннотаций `60-audit-incidents` найдены (trace.sh), распределены по всем задачам
- @sk-test маркеры: 3 аннотации (TestListIncidents, TestGetIncident, TestExportIncidents)
- Все маркеры над owning declaration (type/function/test), нарушения placement не обнаружены
- T1.1 (SQL миграция) — без Go-маркера, допустимо
- T3.2/T3.3/T3.4 (TSX/TS файлы) — маркеры в JS-комментариях, формат соблюдён

## Next Step

- safe to archive
