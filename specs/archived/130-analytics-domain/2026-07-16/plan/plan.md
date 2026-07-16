# Analytics Domain Layer План

## Phase Contract

Inputs: spec (`specs/active/130-analytics-domain/spec.md`), конституция, repo-контекст.
Outputs: plan, data-model (no-change).
Stop if: spec неоднозначна — spec достаточна.

## Цель

Реализовать доменный пакет `src/internal/domain/analytics/` с типами `TokenUsage`, `UsageRecord`, `CostRate`, `Aggregation` и port-интерфейсом `UsageStore`. Работа ограничена domain-слоем — ни адаптеров, ни use case, ни API. Подход безопасен: новый пакет не затрагивает существующий код, зависимости — только `shield/value` для `TenantID`.

## MVP Slice

Один инкремент: все типы + интерфейс + unit-тесты в одном package. Закрывает AC-001, AC-002, AC-003, AC-004, AC-005.

## First Validation Path

```bash
go vet ./src/internal/domain/analytics/...
go test ./src/internal/domain/analytics/...
```

## Scope

- Новый пакет `src/internal/domain/analytics/`
- entity: `TokenUsage`, `Aggregation`
- value object: `UsageRecord`, `CostRate`
- port interface: `UsageStore`
- unit-тесты на все конструкторы и методы
- Существующие пакеты, адаптеры, use cases, API, UI — не затрагиваются

## Performance Budget

`none` — domain-типы без горячего пути; latency и allocs определяются адаптером.

## Implementation Surfaces

| Surface | Тип | Почему |
|---|---|---|
| `src/internal/domain/analytics/` | новый пакет | domain-слой аналитики, изолирован от остальных domain-пакетов |

## Bootstrapping Surfaces

- Директория `src/internal/domain/analytics/` — не существует, создать.

## Влияние на архитектуру

- Локальное: новый domain-пакет, импортирующий `shield/value` для `TenantID`.
- Интеграции/границы: `UsageStore` — новый port interface; реализация адаптера в следующей фазе.
- Миграций/rollout: нет — фича добавления, не замены.

## Acceptance Approach

| AC | Подход | Surfaces | Наблюдение |
|---|---|---|---|
| AC-001 | Конструктор `NewTokenUsage` + exported fields | `analytics/token_usage.go` | unit-тест проверяет поля после создания |
| AC-002 | Интерфейс `UsageStore` с 4 методами | `analytics/usage_store.go` | компиляция; тест с переменной типа интерфейса |
| AC-003 | Конструктор `NewAggregation` | `analytics/aggregation.go` | unit-тест проверяет поля |
| AC-004 | `CostRate` struct + метод `Cost(inputTokens, outputTokens float64) float64` | `analytics/cost_rate.go` | unit-тест с фиксированными ценами, сверка результата |
| AC-005 | Конструктор `NewUsageRecord` | `analytics/usage_record.go` | unit-тест проверяет агрегированные поля |

## Данные и контракты

- Data model: не меняется — сущности живут только в domain-слое (in-memory), persistence не требуется. `data-model.md` — no-change stub.
- API/event contracts: не меняются — порт не подразумевает wire format.
- Зависимость: импорт `shield/value` для `TenantID`.

## Стратегия реализации

### DEC-001 Один package вместо subpackages

- Why: все типы аналитики тесно связаны (5 структур, 1 интерфейс); разделение на entity/vo/port будет овер-инжинирингом для текущего размера. Существующие домены (shield) используют subpackages из-за объёма (детекторы, маски, реакции) — здесь объём на порядок меньше.
- Tradeoff: при разрастании (10+ типов) потребуется рефакторинг, но spec явно ограничивает текущий scope.
- Affects: `src/internal/domain/analytics/`
- Validation: `go build ./src/internal/domain/analytics/...`

### DEC-002 Panic-free конструкторы с возвратом ошибки

- Why: `TokenUsage`, `CostRate` имеют инварианты (неотрицательные токены, непустой TenantID). Конструкторы возвращают `(*T, error)`, а не panic. Следует общепринятой Go-практике и конвенции проекта (`shield/value/tenant_id.go`).
- Tradeoff: caller всегда обязан проверять ошибку; альтернатива (panic) сдвигает проверку на runtime без явного handling.
- Affects: все файлы с конструкторами в `analytics/`
- Validation: unit-тесты с краевыми случаями (отрицательные токены, пустой tenantID)

### DEC-003 Exported fields (не getter-методы)

- Why: entity — простая запись без инвариантов после создания (конструктор уже провалидировал). Exported fields проще для тестов, агрегации, моков. Конвенция в shield/entity использует getter'ы для Tenant (сложный aggregate), но session/entity использует exported fields — здесь ближе к session.
- Tradeoff: нельзя добавить lazy validation/триггеры при чтении, но для value-подобных сущностей это не нужно.
- Affects: все entity/value object файлы

### DEC-004 CostRate.Cost принимает float64, возвращает float64

- Why: стоимость токена — дробное число. float64 — естественный выбор для cost calculations в Go, используется во всём проекте (см. допущения spec: монитарные значения — float64).
- Tradeoff: ошибки округления примитивны; если потребуется точный расчёт — перейти на `shopspring/decimal` в адаптере.
- Affects: `analytics/cost_rate.go`

## Incremental Delivery

### MVP (Первая ценность)

1. Создать пакет `src/internal/domain/analytics/`
2. Реализовать `TokenUsage` + конструктор
3. Реализовать `CostRate` + конструктор + метод `Cost`
4. Реализовать `UsageRecord` + конструктор
5. Реализовать `Aggregation` + конструктор
6. Реализовать `UsageStore` interface
7. Unit-тесты для всех AC
Критерий: `go test ./src/internal/domain/analytics/...` проходит.

### Итеративное расширение

Не требуется — все AC в MVP.

## Порядок реализации

1. `token_usage.go` (зависимость для UsageRecord, UsageStore)
2. `cost_rate.go` (самостоятельный VO)
3. `usage_record.go` (зависит от TokenUsage)
4. `aggregation.go` (самостоятельный, зависит от TenantID)
5. `usage_store.go` (зависит от TokenUsage, UsageRecord, Aggregation)
6. `analytics_test.go` (unit-тесты)
Параллельно: безопасно писать `cost_rate.go` и `token_usage.go`.

## Риски

- **Риск:** Неверная типизация `AvgLatency` (spec допускает `time.Duration`).
  - Mitigation: фиксируем `time.Duration` в Aggregation; адаптер конвертирует при записи.

## Rollout и compatibility

Специальных rollout-действий не требуется — пакет новый, не влияет на существующий код.

## Проверка

- Unit-тесты: `src/internal/domain/analytics/analytics_test.go` — покрывает все AC-001–005.
- `go vet ./src/internal/domain/analytics/...` — убедиться в отсутствии статических ошибок.
- `go build ./...` — убедиться в отсутствии cyclic imports.

## Соответствие конституции

нет конфликтов — pure domain layer, Go, DDD, без UI, без persistence.
