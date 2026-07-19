---
report_type: inspect
slug: prompt-injection-shield
status: pass
docs_language: ru
generated_at: 2026-07-19
---

# Inspect Report: prompt-injection-shield

## Scope

- snapshot: проверка spec нового типа детектора `prompt_injection` в Content Shield
- artifacts:
  - CONSTITUTION.md (summary)
  - specs/active/prompt-injection-shield/spec.md

## Verdict

- status: pass

## Errors

- none

## Warnings

- AC-004, AC-005: fixed in spec (Evidence скорректирован).

## Questions

- none

## Suggestions

- Рассмотреть добавление категории "payload splitting" в AC-004 как явный pattern для built-in coverage (сейчас только SC-002, не проверяемый в AC).

## Traceability

- AC-001 ← RQ-001 (регистрация в DetectorRegistry)
- AC-002 ← RQ-002, RQ-003 (детекция injection-фраз)
- AC-003 ← RQ-002 (no false positives)
- AC-004 ← RQ-005 (built-in patterns)
- AC-005 ← RQ-004 (интеграция с ScanPipeline)
- AC-006 ← RQ-006 (tenant-level override)

Покрытие: все RQ имеют ≥ 1 AC. Все AC уникальны.

## Next Step

- safe to continue to plan
