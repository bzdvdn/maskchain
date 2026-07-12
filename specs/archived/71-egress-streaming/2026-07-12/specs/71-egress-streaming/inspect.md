---
report_type: inspect
slug: 71-egress-streaming
status: pass
docs_language: ru
generated_at: 2026-07-12
---

# Inspect Report: 71-egress-streaming

## Scope

- snapshot: Проверка spec egress HTTP/HTTPS-клиента с proxy dialer, SSE streaming, retry, cancellation, connection pooling
- artifacts:
  - CONSTITUTION.md
  - specs/active/71-egress-streaming/spec.md

## Verdict

- status: pass

## Errors

- none

## Warnings

- ~~W001 — Vague adjectives~~ ✓ исправлено
- ~~W002 — MVP implementation-приоритизация~~ ✓ исправлено

## Questions

- ~~Q001 — Streaming API~~ ✓ решено: отдельный `Stream(ctx, req) (<-chan ProviderChunk, error)`
- ~~Q002 — Retry idempotency~~ ✓ решено: 5xx opt-in per-provider, сетевые ошибки — всегда

## Suggestions

- **S001 — Обратная связь AC-006 с Proxy**: AC-001 (proxy) и AC-006 (jitter) комбинируются — при retry через proxy jitter особенно важен, т.к. proxy может быть общим для многих клиентов. Хорошо бы добавить edge-case в spec об этом.
- **S002 — Proxy credential handling**: spec не упоминает обработку credentials в proxy URL (`http://user:pass@proxy:3128`). Рекомендуется добавить в Краевые случаи (или подтвердить, что `net/http` обрабатывает прозрачно).

## Traceability

- spec: 6 RQ, 7 AC, 3 SC
- AC-001 ↔ RQ-001 (proxy)
- AC-002 ↔ RQ-006 (pool)
- AC-003 ↔ RQ-002 (SSE)
- AC-004 ↔ RQ-005 (timeout)
- AC-005 ↔ RQ-004 (cancellation)
- AC-006 ↔ RQ-003 (jitter)
- AC-007 ↔ RQ-003 (retry exhaustion)
- Все AC покрыты RQ, обратное тоже верно. SC не привязаны к конкретным AC — ожидаемо для performance-критериев.

## Next Step

- safe to continue to plan
