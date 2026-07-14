---
report_type: inspect
slug: 116-connection-pool-fixes
status: pass
docs_language: ru
generated_at: 2026-07-14
---

# Inspect Report: 116-connection-pool-fixes

## Scope

- snapshot: проверка spec для исправления бага MaxIdleConnsPerHost, per-provider timeout, TLS, circuit breaker, per-provider connection pool
- artifacts:
  - CONSTITUTION.md (summary)
  - specs/active/116-connection-pool-fixes/spec.md

## Verdict

- status: pass

## Errors

- none

## Warnings

- none

## Questions

- none

## Suggestions

- **S-01**: SC-001 "Все существующие тесты egress проходят без изменений" — изменение `newTransport()` в pool.go (починка MaxIdleConnsPerHost, подключение DisableKeepAlives) может сломать тесты, которые проверяют точные значения полей Transport. Рекомендуется явно проверить существующие тесты на совместимость и при необходимости обновить expected values.
- **S-02**: AC-004 (InsecureSkipVerify) и AC-005 (mTLS) в evidence полагаются на assertion после создания транспорта. Но создание транспорта — внутренняя деталь egress.Client. Рекомендуется уточнить, должны ли эти настройки применяться в `newTransport()` (pool.go) или на уровне фабрики provider. Это снизит риск дублирования логики.

## Traceability

- RQ-001 → AC-001 (MaxIdleConnsPerHost)
- RQ-002 → AC-002 (per-provider timeout)
- RQ-003 → AC-003, AC-004, AC-005 (TLS/custom CA/insecure/mTLS)
- RQ-004 → AC-006, AC-007 (circuit breaker open/close)
- RQ-005 → AC-008 (per-provider transport isolation)
- Все RQ покрыты AC. AC-003–AC-007 помечены как "следующий приоритет" после MVP (AC-001, AC-002, AC-008) — это ок для spec, но на уровне plan потребуется явное разбиение на фазы имплементации.

## Next Step

- safe to continue to plan
