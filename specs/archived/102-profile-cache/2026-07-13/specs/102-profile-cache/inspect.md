---
report_type: inspect
slug: 102-profile-cache
status: concerns
docs_language: ru
generated_at: 2026-07-13
---

# Inspect Report: 102-profile-cache

## Scope

- snapshot: Read-through/write-through двухуровневый кэш профилей (Valkey + LRU) с PubSub-инвалидацией
- artifacts:
  - CONSTITUTION.md
  - specs/active/102-profile-cache/spec.md

## Verdict

- status: pass

## Errors

- ~~E-01 **Cache key collision для разных tenant'ов**: Ключ не включает `tenant_id`.~~ **Fixed**: RQ-002, AC-001, AC-002, AC-003, AC-007 обновлены.

## Warnings

- ~~W-01 **Delete-инвалидация не покрыта AC**.~~ **Fixed**: RQ-011 + AC-009 добавлены.
- ~~W-02 **Transient write-through failure не рассмотрен**.~~ **Fixed**: RQ-005 дополнен, краевой случай добавлен.

## Questions

- ~~Q-01 PubSub pattern `profile.invalidate:*` — PSUBSCRIBE?~~ **Fixed**: AC-005 + RQ-012.

## Suggestions

- ~~S-01 AC-007 неоднозначность.~~ **Fixed**: однозначное описание.
- S-02 RQ-008: лейблы метрик детализированы до уровня implementation (конкретные значения operation/level). Это ок для acceptance criteria, но в spec можно ослабить до «метрики с лейблами операции и уровня кэша».

## Architecture Changes After User Review

- **DEC-001 переработан**: read order изменён с `LRU → Valkey → PG` на `Valkey → LRU+PG(fallback)`. LRU хранит только ProfileMetadata (~300B), не содержит dictionary entries. Valkey — full profile (включая словарь) для 1 round trip на hot path `/mask`.
- **Cache warming**: добавлен RQ-013 + AC-010 (async warm на старте gateway). DEC-006.

## Traceability

- 10 AC (AC-001–AC-010) покрывают ключевые сценарии: read-through, write-through, Valkey hit, LRU fallback, PubSub-инвалидация, graceful degradation (hot + cold), version bump, delete, метрики, cache warming.
- 13 RQ (RQ-001–RQ-013) маппятся на AC, кроме RQ-002 (key format — проверяется косвенно) и RQ-009/RQ-010 (конфигурация — проверяется интеграционно).
- План отсутствует — задача покрытия AC задачами на фазе plan.

## Next Step

- E-01 исправлен в spec. Warnings не блокируют планирование — учесть в plan.
- Готово к: /spk.plan 102-profile-cache
