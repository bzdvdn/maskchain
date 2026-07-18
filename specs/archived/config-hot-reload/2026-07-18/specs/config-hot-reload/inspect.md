---
report_type: inspect
slug: config-hot-reload
status: pass
docs_language: ru
generated_at: 2026-07-18
---

# Inspect Report: config-hot-reload

## Scope

- snapshot: проверка spec на hot-reload конфигурации через fsnotify — intent, acceptance criteria, scope boundaries, constitution compliance
- artifacts:
  - CONSTITUTION.md
  - specs/active/config-hot-reload/spec.md

## Verdict

- status: pass

## Errors

- none

## Warnings

- none

## Questions

- `github.com/fsnotify/fsnotify` — не видно явного импорта в коде (транзитивно может быть через viper?). На реализации проверить go.mod/go.sum и при необходимости добавить.

## Suggestions

- В spec не указан механизм graceful swap для router/tenant store. На уровне plan стоит рассмотреть atomic pointer (`atomic.Pointer[Router]`) — это стандартный паттерн, не блокирует запросы.
- SC-001 "reload < 50ms" — реалистично при diff-merge без блокировок. Если окажется дороже, стоит пересмотреть на plan-фазе.

## Traceability

- AC-001 → RQ-001: routing hot-reload, observable via log + curl
- AC-002 → RQ-001: tenants hot-reload
- AC-003 → RQ-003: error rollback
- AC-004 → RQ-001: debounce
- AC-005 → RQ-005: non-blocking reload
- AC-006 → RQ-006: base isolation
Все AC покрыты requirements, все наблюдаемы.

## Next Step

- safe to continue to plan
