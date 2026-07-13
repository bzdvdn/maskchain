---
report_type: inspect
slug: 110-provider-adapters
status: concerns
docs_language: ru
generated_at: 2026-07-14
---

# Inspect Report: 110-provider-adapters

## Scope

- snapshot: инспекция spec адаптеров для OpenAI-compatible и Anthropic с фабрикой по api_type и хранением api_key в конфиге
- artifacts:
  - CONSTITUTION.md
  - .speckeep/constitution.summary.md
  - specs/active/110-provider-adapters/spec.md

## Verdict

- status: concerns

## Errors

- none

## Warnings

- AC-001: формулировка «возвращён экземпляр `*OpenAIClient` (или `ProviderClient` с поведением openai)» размывает проверяемость — `NewProviderClient` возвращает интерфейс `ProviderClient`, конкретный тип известен только через type assertion. Рекомендация: уточнить evidence как «type assertion в тесте подтверждает, что под интерфейсом — `*OpenAIClient`».
- api_key хранится в `ProviderConfig` в открытом виде — это осознанное решение (secrets management вне scope), но стоит явно добавить в Допущения: «api_key хранится в plaintext в YAML/ENV; шифрование не предусмотрено на этой фазе».

## Questions

- Для тестирования Anthropic Stream (AC-006) потребуется замокать SSE-ответ с event: content_block_delta. Это нормально для unit-теста, но стоит убедиться, что тест-хелперы для SSE уже есть или будут созданы.
- Нужна ли общая структура для ошибок провайдера (например, `ProviderError {Code, Message, Type}`), или каждый адаптер возвращает raw body в `ProviderResponse`? Spec говорит «преобразует в стандартный формат» — хорошо бы уточнить формат.

## Suggestions

- AC-001: заменить evidence на «type assertion `client.(*OpenAIClient)` успешен для `api_type: openai` и `client.(*AnthropicClient)` для `api_type: anthropic`».
- Добавить в Допущения: «api_key хранится в конфиге в открытом виде; шифрование/ротация — вне scope данной фазы».
- Рассмотреть добавление AC на неизвестный api_type (описано в Краевые случаи, но нет AC) — фабрика должна вернуть ошибку.

## Traceability

- 8 AC, 6 RQ — все AC имеют Given/When/Then с наблюдаемым evidence.
- AC-001…AC-003, AC-005 покрывают OpenAI-compatible (MVP).
- AC-004, AC-006 покрывают Anthropic.
- AC-007, AC-008 покрывают конфигурацию.
- Нет незакрытых `[NEEDS CLARIFICATION]`, нет плейсхолдеров.

## Next Step

- Warnings устранены: AC-001 уточнён, допущение про plaintext api_key добавлено
- safe to continue to plan
