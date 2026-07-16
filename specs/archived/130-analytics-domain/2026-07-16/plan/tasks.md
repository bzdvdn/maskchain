# Analytics Domain Layer Задачи

## Phase Contract

Inputs: plan (`plan.md`), spec (`spec.md`), конституция.
Outputs: исполнимые задачи с покрытием AC.
Stop if: задачи расплывчаты — нет, scope чёткий.

## Surface Map

| Surface | Tasks |
|---|---|
| `src/internal/domain/analytics/cost_rate.go` | T1.1, T3.1 |
| `src/internal/domain/analytics/token_usage.go` | T1.2, T3.1 |
| `src/internal/domain/analytics/usage_record.go` | T2.1, T3.1 |
| `src/internal/domain/analytics/aggregation.go` | T2.1, T3.1 |
| `src/internal/domain/analytics/usage_store.go` | T2.2, T3.1 |
| `src/internal/domain/analytics/analytics_test.go` | T3.1 |

## Implementation Context

- Цель MVP: реализовать пакет `src/internal/domain/analytics/` с типами `TokenUsage`, `UsageRecord`, `CostRate`, `Aggregation` и port-интерфейсом `UsageStore`
- Границы приемки: AC-001, AC-002, AC-003, AC-004, AC-005
- Инварианты/семантика:
  - `InputTokens >= 0`, `OutputTokens >= 0` — конструктор возвращает ошибку при отрицательных
  - `TenantID` непустой — валидация в конструкторе `TokenUsage`
  - `CostRate.Cost` считает: `(inputTokens/1000)*inputPrice + (outputTokens/1000)*outputPrice`
  - Все monetary значения — `float64`; `AvgLatency` — `time.Duration`
- Ошибки/коды: конструкторы возвращают `(*T, error)` — валидация на входе, без panic
- Контракты/протокол: нет wire-форматов; `UsageStore` — Go interface в том же пакете
- Границы scope: не делаем адаптеры, use case, API, UI, интеграцию с gateway
- Proof signals: `go vet ./src/internal/domain/analytics/...` + `go test ./src/internal/domain/analytics/...` pass
- References: DEC-001 (one package), DEC-002 (panic-free), DEC-003 (exported fields), DEC-004 (float64 cost)

## Фаза 1: Основа

Цель: создать пакет и независимые доменные типы без внешних зависимостей внутри пакета.

- [x] T1.1 Реализовать `CostRate` value object — конструктор `NewCostRate(model string, inputPricePer1K, outputPricePer1K float64) (*CostRate, error)`, метод `Cost(inputTokens, outputTokens int64) float64`. Touches: `src/internal/domain/analytics/cost_rate.go`
- [x] T1.2 Реализовать `TokenUsage` entity — конструктор `NewTokenUsage(tenantID value.TenantID, model string, inputTokens, outputTokens int64, cost float64, timestamp time.Time) (*TokenUsage, error)` с валидацией неотрицательных токенов и непустого tenantID. Touches: `src/internal/domain/analytics/token_usage.go`

## Фаза 2: MVP Slice

Цель: реализовать оставшиеся типы и port interface — полный набор domain-контрактов.

- [x] T2.1 Реализовать `UsageRecord` VO (конструктор `NewUsageRecord(…)`) и `Aggregation` entity (конструктор `NewAggregation(…)`). Touches: `src/internal/domain/analytics/usage_record.go`, `src/internal/domain/analytics/aggregation.go`
- [x] T2.2 Реализовать `UsageStore` port interface с методами `Record(ctx, TokenUsage) error`, `QueryByTenant(ctx, tenantID, from, to time.Time) ([]UsageRecord, error)`, `QueryByModel(ctx, model string, from, to time.Time) ([]UsageRecord, error)`, `AggregateByDay(ctx, tenantID, from, to time.Time) ([]Aggregation, error)`. Touches: `src/internal/domain/analytics/usage_store.go`

## Фаза 3: Проверка

Цель: unit-тесты на все AC, включая краевые случаи и валидацию.

- [x] T3.1 Написать unit-тесты покрывающие AC-001–005:
  - AC-001: создание `TokenUsage` через конструктор, проверка всех полей; ошибка при отрицательных токенах; ошибка при пустом tenantID
  - AC-002: `UsageStore` — присвоение переменной типа интерфейса, компиляция метода
  - AC-003: создание `Aggregation`, проверка полей
  - AC-004: `CostRate` с фиксированными ценами, `Cost(500, 200)` == 0.011; нулевая цена; нулевые токены
  - AC-005: создание `UsageRecord`, проверка агрегированных полей
  Touches: `src/internal/domain/analytics/analytics_test.go`

## Покрытие критериев приемки

- AC-001 -> T1.2, T3.1
- AC-002 -> T2.2, T3.1
- AC-003 -> T2.1, T3.1
- AC-004 -> T1.1, T3.1
- AC-005 -> T2.1, T3.1

## Заметки

- Фаза 3 (Проверка) template mapping: Фаза 4 в шаблоне, но т.к. основная реализация (Фаза 3) вся в MVP — тесты выделены отдельно.
- T1.1 и T1.2 безопасно параллелить — нет взаимных зависимостей.
- T2.1 и T2.2 безопасно параллелить — `UsageStore` импортирует `TokenUsage`/`UsageRecord`/`Aggregation`, но не блокирует написание типов.

Готово к: /spk.implement 130-analytics-domain
