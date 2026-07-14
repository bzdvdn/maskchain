---
report_type: inspect
slug: 115-rate-limit-wiring
status: pass
docs_language: ru
generated_at: 2026-07-14
---

# Inspect Report: 115-rate-limit-wiring

## Scope

- snapshot: Проверка spec на подключение rate limit middleware в gateway main.go
- artifacts:
  - CONSTITUTION.md (через constitution.summary.md)
  - specs/active/115-rate-limit-wiring/spec.md
  - src/internal/api/middleware/ratelimit.go
  - src/internal/domain/budget/ratelimit.go
  - src/internal/domain/budget/tokenbudget.go

## Verdict

- status: pass

## Errors

- none

## Warnings

### ~~W-001 AC-005: Сценарий токен-бюджета не соответствует порядку проверки в middleware~~

**Исправлено:** AC-005 переписан — 5 запросов по 100 токенов, 6-й получает 429.

### ~~W-002 RQ-007 / AC-003: Retry-After header не реализован в middleware~~

**Исправлено:** scope расширен — добавление Retry-After header в 429-путь middleware включено в scope. Spec обновлён.

## Questions

- none

## Suggestions

### S-001 SC-001: Формулировка критерия успеха

SC-001: "Rate limit middleware добавляет < 1ms p99 к latency" — это корректно, но стоит уточнить, что это относится именно к фазе wiring (никаких новых операций не добавляется). Оставить как есть — консервативный критерий.

### S-002 AC-003: Evidence можно конкретизировать

Evidence "проверить headers на первом (200) и шестом (429) запросах" — хорошо, но можно явно перечислить ожидаемые значения:
- 200: `X-RateLimit-Limit: 5`, `X-RateLimit-Remaining: 4`, `X-RateLimit-Reset: <epoch+60>`
- 429: `X-RateLimit-Limit: 5`, `X-RateLimit-Remaining: 0`, `X-RateLimit-Reset: <epoch+60>`[, `Retry-After: <sec>`]

Не блокер.

## Traceability

- 8 AC (AC-001 — AC-008), все с Given/When/Then
- 7 RQ (RQ-001 — RQ-007), каждый имеет observable outcome
- AC-001..AC-004, AC-006..AC-008 — корректны, traceable к коду middleware
- AC-005 — требует исправления сценария (W-001)
- Нет плана или задач — проверка plan→tasks coverage не применяется

## Next Step

- safe to continue to plan
