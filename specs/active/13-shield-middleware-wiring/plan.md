# 13 — Content Shield: PII per-tenant + Dict Unmask. План

## Phase Contract

Inputs: spec.md, repo context (tenant.go, shield.go, scan_usecase.go, shield_engine.go, config.go, main.go).
Outputs: plan.md, data-model.md.
Stop if: — нет.

## Цель

Перенести PII-конфигурацию с внешнего ProfileMapping на Tenant (PIIConfig). Убрать цепочку ProfileRepository → FindBySlug. Middleware читает правила напрямую из tenant контекста и передаёт их в engine.Scan. Словарная маскировка и unmask остаются без изменений.

## MVP Slice

Tenant с PIIConfig + rules → PII блокируется 403 + dict unmask non-streaming.
AC-001 (PII per-tenant rules), AC-005 (dict unmask).

## First Validation Path

Интеграционный тест: tenant c правилом `email: block` → PII-промпт → 403.
Интеграционный тест: dict-значения → LLM echo → оригиналы в ответе.

## Scope

- `src/internal/domain/shield/entity/tenant.go` — добавить `PIIConfig`
- `src/internal/app/usecase/shield/types.go` — `ScanRequest.Rules` вместо `ProfileSlug`
- `src/internal/app/usecase/shield/scan_usecase.go` — строить pipeline из Rules (без FindBySlug)
- `src/internal/app/usecase/shield/pipeline_factory.go` — метод `BuildFromRules()` или адаптация `Build()`
- `src/internal/api/middleware/shield.go` — читать `tenant.PIIConfig()`, убрать `cfg.ProfileMapping`
- `src/internal/infra/config/config.go` — удалить `ProfileMapping`, `DefaultAction`
- `src/cmd/gateway/main.go` — убрать Wire PostgresProfileRepo → ScanUseCase
- Тесты: все PII-related переписать под tenant.PIIConfig; dict unmask тесты остаются

## Performance Budget

Уменьшение latency: убран ProfileRepository lookup (PG/Valkey). Правила читаются из tenant в памяти.
SC-002: <20ms добавка (было <50ms).

## Implementation Surfaces

| # | Surface | Почему | Статус |
|---|---------|--------|--------|
| 1 | `entity/tenant.go` | Добавить `PIIConfig` + геттер + `WithPIIConfig` | существующий, расширение |
| 2 | `shield/types.go` | `ScanRequest.ProfileSlug` → `Rules` | существующий, изменение |
| 3 | `shield/scan_usecase.go` | Убрать `FindBySlug`, строить pipeline из Rules | существующий, изменение |
| 4 | `shield/pipeline_factory.go` | `BuildFromRules()` | существующий, расширение |
| 5 | `middleware/shield.go` | Читать `tenant.PIIConfig()`, убрать profile lookup | существующий, изменение |
| 6 | `config/config.go` | Удалить `ProfileMapping`/`DefaultAction` | существующий, изменение |
| 7 | `cmd/gateway/main.go` | Убрать DI ProfileRepository → ScanUseCase | существующий, изменение |

## Влияние на архитектуру

- **Локальное**: Tenant становится единым источником PII-правил и словарей. Middleware упрощается (нет cfg.ProfileMapping lookup).
- **Интеграции**: ScanRequest API меняется (ProfileSlug → Rules). ScanUseCase больше не зависит от ProfileRepository (меньше зависимостей).
- **Rollout**: при наличии tenant с PIIConfig — PII работает. Tenant без PIIConfig → enabled=false → PII не сканируется (только dict). Полностью backward compatible (dict работал и работает).

## Acceptance Approach

| AC | Подход | Surfaces |
|----|--------|----------|
| AC-001 | Tenant с правилами → engine.Scan получает Rules | `shield.go`, `scan_usecase.go` |
| AC-002 | Правило action=allow → детекция не блокирует | `shield.go` switch status |
| AC-003 | Enabled=false → engine не вызван | `shield.go` guard |
| AC-004 | engine.Scan error → default_action | `shield.go` err branch |
| AC-005 | Dict mask → dictUnmaskWriter flush | `shield.go` writer |
| AC-006 | Dict mask → streamDictUnmaskWriter.Write | `shield.go` stream writer |
| AC-007 | Rules=[] → engine не вызван | `shield.go` guard |

## Данные и контракты

- AC-001–AC-004, AC-007 зависят от PIIConfig на Tenant.
- AC-005, AC-006 (dict unmask) — только от middleware, не зависят от PII.
- `data-model.md`: статус `changed` (новое поле Tenant.PIIConfig).
- ScanRequest.ProfileSlug удаляется, добавляется Rules.

## Стратегия реализации

### DEC-001 PIIConfig на Tenant

- **Why**: PII-правила — атрибут tenant, как и словари. Tenant — единый источник конфигурации сканирования. Не нужно внешнее ProfileMapping.
- **Affects**: `tenant.go`, `shield.go`, `scan_usecase.go`
- **Validation**: AC-001, AC-002, AC-003, AC-007

### DEC-002 Rules → pipeline напрямую (без профиля)

- **Why**: ScanUseCase строит детекторы из правил, а не загружает профиль через FindBySlug. Убирает зависимость от ProfileRepository и БД.
- **Tradeoff**: ScanPipelineFactory нужно уметь строить из правил (новый метод или адаптация).
- **Affects**: `scan_usecase.go`, `pipeline_factory.go`
- **Validation**: AC-001

### DEC-003 default_action на tenant

- **Why**: tenant сам решает что делать при ошибке сканирования (block/allow). Глобальный default_action в конфиге не нужен.
- **Affects**: `tenant.go` (PIIConfig.DefaultAction), `shield.go`
- **Validation**: AC-004

### DEC-004 Dict unmask независим от PII

- **Why**: dict-маскировка и unmask работают всегда, независимо от PIIConfig.Enabled. Это две ортогональные фичи.
- **Affects**: `shield.go` — dict-секция не меняется
- **Validation**: AC-005, AC-006

### DEC-005 Убрать ProfileRepository из middleware-цепочки

- **Why**: после переноса правил на Tenant ProfileRepository больше не нужен в middleware. main.go упрощается.
- **Affects**: `main.go`, `config.go`
- **Validation**: build pass

## Incremental Delivery

### MVP (первая ценность)
- Tenant.PIIConfig + Rules → engine.Scan → 403 (AC-001)
- Dict unmask non-streaming (AC-005)
- Enabled=false guard (AC-003)

### Итеративное расширение
- Per-pattern action allow/block (AC-002)
- Graceful degradation (AC-004)
- Streaming unmask (AC-006)
- Edge cases (AC-007, пустые rules, disabled)

## Порядок реализации

1. PIIConfig на Tenant entity
2. ScanRequest.Rules + ScanUseCase.buildFromRules
3. Middleware: tenant.PIIConfig() вместо ProfileMapping
4. Удалить ProfileMapping/DefaultAction из config
5. Упростить main.go (убрать ProfileRepository DI)
6. Переписать PII-тесты под tenant.PIIConfig
7. Graceful degradation + streaming unmask (уже есть, адаптировать)
8. Edge cases

## Риски

- **Риск 1**: ScanPipelineFactory.Build требует entity.Profile — нужен новый метод BuildFromRules
  - Mitigation: добавить `BuildFromRules(ctx, rules []PIARule) (*Pipeline, error)`, который создаёт DetectorBinding напрямую из правил
- **Риск 2**: Tenant может не иметь PIIConfig при загрузке (старые tenant)
  - Mitigation: Tenant.PIIConfig() возвращает zero-value PIIConfig{Enabled: false} — PII не сканируется, dict работает

## Rollout и compatibility

- Удаление ProfileMapping/DefaultAction — breaking change конфига. При deploy нужно убедиться, что все tenant имеют PIIConfig.
- PIIConfig tenant получает через tenant-repo (уже существует). Если поле пустое — PII disabled.
- Feature flag не требуется — dict незатронут, PII включается наличием правил на tenant.

## Проверка

| Шаг | Тип | Что проверяет | AC |
|-----|-----|---------------|----|
| 1 | Unit | Tenant.PIIConfig → engine.Scan с правилами | AC-001 |
| 2 | Unit | Per-pattern action allow не блокирует | AC-002 |
| 3 | Unit | Enabled=false → engine не вызван | AC-003 |
| 4 | Unit | engine error → default_action | AC-004 |
| 5 | Integration | PII-промпт → 403 | AC-001 |
| 6 | Integration | Dict-запрос → LLM echo → оригиналы | AC-005 |
| 7 | Integration | Streaming → unmask | AC-006 |
| 8 | Unit | Rules=[] → engine не вызван | AC-007 |

## Соответствие конституции

Content Shield становится tenant-центричным — PII-правила и словари живут на tenant. Core domain, не opt-in.
