---
report_type: inspect
slug: 25-shield-preprocessors
status: pass
docs_language: ru
generated_at: 2026-07-10
---

# Inspect Report: 25-shield-preprocessors

## Scope

- snapshot: препроцессоры CSV/JSON для Shield Engine — маскировка колонок/полей по имени/пути до детекторов, хранение в JSONB-поле профиля
- artifacts:
  - CONSTITUTION.md
  - specs/active/25-shield-preprocessors/spec.md

## Verdict

- status: pass

## Errors

- none

## Warnings

- **W001 — SC-002 формулировка**: «нулевой overhead на неструктурированных данных (proof: benchmark)» — понятие «нулевой» строго недостижимо (любой вызов Process имеет хотя бы allocation). Рекомендация: переформулировать в «overhead < 1% на неструктурированных данных (proof: benchmark)» на фазе plan, если критерий останется.

- **W002 — Краевые случаи: вложенные wildcard**: строка «Вложенные JSONPath с несколькими wildcard (...) — поддержка не входит в MVP» не имеет соотв. RQ или AC, фиксирующего границу парсера. Рекомендуется добавить RQ-007-bis или явную строку в Вне scope, что multi-wildcard JSONPath не поддерживается.

## Questions

- none

## Suggestions

- **S001 — AC-007 (factory unknown type)**: тест проверяет `Type: "xml"` → error. Это корректно, но стоит убедиться на фазе plan, что ошибка содержит название неизвестного типа для диагностики (`unknown preprocessor type: xml`).

- **S002 — AC-008 логирование**: единственный AC, проверяющий pipeline order. Evidence указывает «логирование порядка вызова или unit-тест с mock». Рекомендуется на фазе plan явно выбрать один вариант (mock-детектор надёжнее и не зависит от лог-конфигурации).

## Traceability

- 8 AC (AC-001–AC-008) покрывают 10 RQ (RQ-001–RQ-010): каждый RQ имеет ≥1 подтверждающего AC
- AC-001 → RQ-004, RQ-006
- AC-002 → RQ-006
- AC-003 → RQ-005, RQ-007
- AC-004 → RQ-005
- AC-005 → RQ-007
- AC-006 → RQ-008
- AC-007 → RQ-003, RQ-009
- AC-008 → RQ-010
- MVP slice (AC-001, AC-002, AC-003, AC-005, AC-007, AC-008) покрывает core functionality

## Next Step

- safe to continue to plan

Готово к: `/spk.plan 25-shield-preprocessors`
