---
report_type: inspect
slug: 24-shield-dictionaries
status: pass
docs_language: ru
generated_at: 2026-07-10
---

# Inspect Report: 24-shield-dictionaries

## Scope

- snapshot: проверка спецификации словарей Content Shield — Dictionary ValueObject, MatchMode (exact/contains/regex/fuzzy), DictionaryRepository, DictionaryDetector, WordlistMatcher, интеграция с профилем
- artifacts:
  - CONSTITUTION.md (via .speckeek/constitution.summary.md)
  - specs/active/24-shield-dictionaries/spec.md

## Verdict

- status: pass

## Errors

- none

## Warnings

- none

## Questions

- **AC-006 vs MVP Slice**: AC-006 (profile integration) не включён в MVP Slice, но без него MVP не приносит ценности — словари существуют в БД, но не загружаются с профилем. Рекомендуется добавить AC-006 в MVP Slice.

## Suggestions

- **AC-006 следует в MVP Slice**: без загрузки словарей через ProfileRepository детектор не получит данные. Предлагается расширить MVP Slice до AC-001, AC-002, AC-003, AC-006, AC-007.
- **DictionaryRepository — Upsert vs Create**: spec не уточняет поведение при повторном Save для того же ProfileSlug. Рекомендуется в plan заложить Upsert-семантику, т.к. профиль имеет максимум один словарь.
- **Кэширование Aho-Corasick автомата**: открытый вопрос в spec — стоит явно указать в plan, что автомат строится при загрузке профиля (lazy, при первом Scan), а не на каждое сканирование.

## Traceability

- 8 RQ → 8 AC: полное покрытие 1:1
- AC-004 и AC-008 тестируют разные уровни (DictionaryDetector vs WordlistMatcher) — дублирования нет
- AC-003/AC-004/AC-005 покрывают три режима матчинга (exact/contains/regex)
- AC-001 покрывает создание Dictionary, AC-002 — repository, AC-006 — profile integration
- К каждому AC привязан Evidence с указанием типа теста

## Next Step

- safe to continue to plan
