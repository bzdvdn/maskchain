# 13 — Content Shield: PII per-tenant + Dict Unmask

## Scope Snapshot

- In scope: PII-правила прямо на Tenant `PIIConfig` + unmask словарных placeholders в ответе LLM.
- Out of scope: ProfileRepository, Presidio pipeline, routing engine, UI профилей.

## Цель

Content Shield читает PII-правила **из Tenant**, а не из ProfileMapping конфига. Администратор настраивает правила per-tenant (какие PII-типы блокировать, какие разрешать), словари уже на tenant. Middleware сканирует запрос по правилам tenant и блокирует PII 403, а словарные placeholders восстанавливает в ответе LLM.

## Основной сценарий

1. Gateway получает запрос с tenant из API key (auth middleware).
2. Tenant уже загружен в контекст с `Dictionaries` и `PIIConfig`.
3. Middleware читает `tenant.PIIConfig()`:
   - если `Enabled == false` → только dict-сканирование.
   - если `Enabled == true` → сканирует Rules через engine.
4. Для каждого правила: если детектор сработал и `action == "block"` → HTTP 403; если `action == "allow"` → пропускает.
5. Параллельно: dict-детекция (из `tenant.Dictionaries()`) → маскировка placeholders в request → прокси к LLM.
6. LLM возвращает ответ — middleware заменяет `{{dict.*}}` на оригиналы (non-streaming + SSE).

## User Stories

- P1 Story: Оператор настраивает PII-правила на tenant (email=allow, ssn=block). Middleware применяет их. Tenant B может иметь другие правила.
- P2 Story: Tenant отключает PII (`enabled=false`) — сканирование не мешает latency, dict-маскировка продолжает работать.

## MVP Slice

Tenant c `PIIConfig.Enabled=true` и правилами → PII блокируется 403. Словарные placeholders восстанавливаются в non-streaming ответе.
AC-001–AC-005.

## First Deployable Outcome

Интеграционный тест: tenant с правилом `{label: "email", action: "block"}` → PII-промпт с email → 403.
Интеграционный тест: тот же tenant с email=allow → запрос проходит, dict-значения восстанавливаются.

## Scope

- **PIIConfig** на Tenant: `{enabled, default_action, rules[]}`.
- Поле `Rules` — массив `{label, type, pattern, action}`.
- Middleware: чтение `tenant.PIIConfig()`, передача правил в `engine.Scan()`.
- `ScanRequest`: замена `ProfileSlug` на `Rules`.
- `ScanUseCase`: построение pipeline напрямую из правил (без FindBySlug).
- Удаление `ProfileMapping`/`DefaultAction` из `ShieldConfig`.
- Удаление `PostgresProfileRepo → ScanPipelineFactory → ScanUseCase` из main.go.
- Graceful degradation при ошибке engine.Scan.
- Unmask словарных placeholders (non-streaming + streaming).
- Интеграционные тесты: PII → 403, dict-восстановление.

## Контекст

- Tenant уже в контексте (`TenantFromContext`). У tenant есть `Dictionaries()`. Добавляем `PIIConfig()`.
- Middleware уже сканирует словари и маскирует request body.
- `ScanUseCase` сейчас ищет профиль через `FindBySlug` — меняем на прямой приём правил.
- `ShieldConfig.ProfileMapping`/`DefaultAction` убираются — логика переезжает на tenant.
- ProfileRepository, DictionaryCache, ScanPipelineFactory больше не нужны в middleware-цепочке.

## Зависимости

- Tenant entity (`entity/tenant.go`) — расширяется полем `piiConfig`.
- ShieldEngine/ScanUseCase — API меняется (Rule → pipeline без профиля).
- Внешние: Microsoft Presidio (через DetectorRegistry), dictionaries уже на tenant.

## Требования

- RQ-001 Tenant ДОЛЖЕН содержать `PIIConfig` с полями `enabled`, `default_action`, `rules`.
- RQ-002 Middleware ДОЛЖНА читать PII-правила из `tenant.PIIConfig()`, а не из конфига gateway.
- RQ-003 Каждое правило ДОЛЖНО содержать `label`, `type`, `pattern`, `action`.
- RQ-004 Если `action == "block"` и детектор сработал — middleware ДОЛЖНА вернуть 403.
- RQ-005 Если `engine.Scan` вернул ошибку — middleware ДОЛЖНА применить `default_action` (graceful degradation).
- RQ-006 Система ДОЛЖНА восстанавливать `{{dict.*}}` placeholders в ответе LLM (non-streaming).
- RQ-007 Система ДОЛЖНА восстанавливать `{{dict.*}}` placeholders в SSE-чанках (streaming).

## Вне scope

- Presidio pipeline — не меняется, только правила → pipeline.
- Dictionaries — уже на tenant, не меняется.
- Routing engine, provider handler.
- Метрики shield-блокировок (будущая фича).
- Unmask PII-маски (только словарный unmask).

## Критерии приемки

### AC-001 PII блокируется по правилам tenant

- Почему это важно: tenant сам решает, какие PII типы блокировать.
- **Given** tenant с PIIConfig: `{rules: [{label: "email", type: "presidio", pattern: "EMAIL", action: "block"}]}`
- **When** запрос с email-адресом приходит
- **Then** middleware возвращает 403 Forbidden
- Evidence: интеграционный тест: tenant с правилами → PII-промпт → 403

### AC-002 Per-pattern action (allow vs block)

- Почему это важно: одним tenant нужен email в LLM, другим нет.
- **Given** tenant с PIIConfig: `{rules: [{label: "email", action: "allow"}, {label: "ssn", action: "block"}]}`
- **When** запрос содержит email и SSN
- **Then** email пропускается, SSN блокируется 403
- Evidence: интеграционный тест: один tenant блокирует ssn, пропускает email

### AC-003 PIIConfig disabled — PII не сканируется

- Почему это важно: если tenant не настроил PII, не должны падать лишние ошибки.
- **Given** tenant с `PIIConfig.Enabled=false`
- **When** запрос с PII приходит
- **Then** middleware не вызывает engine.Scan, словарная маскировка работает
- Evidence: unit-тест: disabled → engine не вызван, dict-маскировка выполняется

### AC-004 Graceful degradation

- Почему это важно: ошибка Presidio не роняет gateway.
- **Given** tenant с правилами, но engine.Scan возвращает ошибку
- **When** запрос проходит middleware
- **Then** middleware логирует ошибку и применяет `PIIConfig.default_action`
- Evidence: unit-тест: mock Scanner → error → default_action

### AC-005 Dict unmask в non-streaming

- Почему это важно: пользователь получает имена сотрудников в ответе LLM, а не {{dict.*}}.
- **Given** tenant со словарём, middleware маскирует request
- **When** LLM возвращает non-streaming JSON с `{{dict.X.0}}`
- **Then** middleware заменяет `{{dict.X.0}}` на оригинал
- Evidence: интеграционный тест: dict-запрос → LLM echo → в ответе оригиналы

### AC-006 Dict unmask в SSE (streaming)

- **Given** tenant со словарём, запрос с `stream:true`
- **When** LLM шлёт SSE-чанки с `{{dict.X.0}}`
- **Then** каждый чанк проходит unmask перед клиентом
- Evidence: интеграционный тест: streaming → чанки без placeholders

### AC-007 Graceful no-op при пустых правилах

- **Given** tenant с `PIIConfig.Enabled=true` и пустым массивом rules
- **When** запрос проходит middleware
- **Then** middleware не вызывает engine.Scan (нет правил), dict-маскировка работает
- Evidence: unit-тест: enabled=true, rules=[] → engine не вызван

## Допущения

- Tenant уже резолвится auth middleware, загружается с словарями.
- PIIConfig загружается вместе с Tenant (из tenant-repo или синхронизации).
- default_action проверяется только при ошибке engine.Scan — не при отсутствии правил.
- Словарная маскировка и unmask не зависят от PII-сканирования.

## Краевые случаи

- PIIConfig.Enabled=false → engine не вызывается, dict работает.
- Rules пуст → engine не вызывается (нет правил для сканирования).
- engine.Scan ошибка → default_action.
- LLM ответ без placeholders → unmask no-op.
- Placeholder с некорректным индексом → не заменяется.
- Частичный placeholder в конце SSE chunk → не заменяется (no-op).

## Открытые вопросы

- Как tenant получает PIIConfig? Через tenant-repo (уже существует) или через sync-механизм. Решается в реализации.
