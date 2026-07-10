---
report_type: inspect
slug: 30-shield-persistence
status: concerns
docs_language: ru
generated_at: 2026-07-10
---

# Inspect Report: 30-shield-persistence

## Scope

- snapshot: проверка спецификации persistence layer для PostgreSQL репозиториев профилей, словарей и инцидентов Content Shield
- artifacts:
  - CONSTITUTION.md
  - specs/active/30-shield-persistence/spec.md

## Verdict

- status: pass

## Errors

- none

## Warnings

- none

## Questions

- **Q-001** (из spec, Open Questions #2): Транзакционный helper — `pgx.Tx` через context vs `TransactionManager` interface. Требуется решение в plan.
- **Q-002** (из spec, Open Questions #3): Slug generation — UUID-based vs user-provided. Требуется решение в plan.

## Suggestions

- **S-001** SC-001/SC-002 (перформанс) — оба указаны как success criteria с измеримыми порогами, что корректно для spec. При реализации убедиться, что в CI окружении те же пороги не будут ложно падать из-за меньших ресурсов.
- **S-002** В AC-001 явно не указан механизм провекри миграций (факт применения). Рекомендуется в план добавить шаг verify: `SELECT 1` после запуска миграций, либо проверку наличия таблиц через information_schema.

## Constitution Alignment

- Content Shield как core domain — spec сохраняет (persistence для профилей/словарей/инцидентов)
- Profile-driven policy management — spec соответствует (хранение профилей)
- PostgreSQL — spec соответствует
- DDD + Clean Architecture — spec соответствует (репозитории реализуют существующие port-интерфейсы)
- Language policy: doc=ru, comments=en — соблюдено
- Traceability: @sk-task/@sk-test placement rules — spec не нарушает; остаётся под контролем plan/implement

## Traceability

- AC-001 → RQ-001, RQ-002, RQ-003, RQ-004 (migrations, transactional load, slug as FK, DictionaryRepo)
- AC-002 → RQ-006 (cascade delete)
- AC-003 → RQ-008 (transactional execution)
- AC-004 → RQ-005 (IncidentRepo)
- AC-005 → RQ-007 (pool config)
- AC-006 → RQ-002, RQ-004, RQ-005, RQ-008 (unit tests for all repos)
- AC-007 → RQ-001 (integration tests with real PG)
- RQ-009 (index on incidents.timestamp) не привязан к отдельному AC — рекомендуется добавить в AC-004 или выделить AC-008

## Next Step

- safe to continue to plan
