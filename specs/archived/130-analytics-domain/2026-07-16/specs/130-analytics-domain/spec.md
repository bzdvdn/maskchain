# Analytics Domain Layer

## Scope Snapshot

- In scope: Domain-слой аналитики — entities, value objects и port interface для учёта потребления токенов и агрегации по тенантам/моделям/дням.
- Out of scope: Реализация адаптеров (БД, кэш), HTTP/gRPC API, UI, интеграция с существующим pipeline (gateway/admin).

## Цель

Разработчик получает доменные типы и порт для записи/чтения метрик использования токенов. Успех фичи — наличие полного набора domain-типов (`TokenUsage`, `UsageRecord`, `CostRate`, `Aggregation`) и port-интерфейса `UsageStore`, достаточных для реализации use case учета потребления без привязки к конкретному storage.

## Основной сценарий

1. Система генерирует событие завершения LLM-запроса с данными о потреблённых токенах.
2. Создаётся `TokenUsage` entity с tenant/model/токенами/стоимостью/временем.
3. `TokenUsage` записывается через `UsageStore.Record()`.
4. По запросу аналитики извлекаются агрегированные данные через `UsageStore.AggregateByDay()` или выборки по tenant/model через `QueryByTenant`/`QueryByModel`.
5. `CostRate` предоставляет стоимость токена для расчёта `Cost` в `TokenUsage`.

## User Stories

- P1: Как разработчик, я хочу создать `TokenUsage` entity и записать её через `UsageStore`, чтобы фиксировать каждый завершённый LLM-запрос.
- P2: Как разработчик, я хочу получить агрегированные данные за день по тенанту, чтобы видеть суммарное потребление.

## MVP Slice

Определение всех domain-типов (`TokenUsage`, `UsageRecord`, `CostRate`, `Aggregation`) и port-интерфейса `UsageStore` в пакете `src/internal/domain/analytics/`. MVP закрывает AC-001, AC-002, AC-003, AC-004.

## First Deployable Outcome

Пакет `src/internal/domain/analytics/` с экспортируемыми типами и интерфейсом. Результат проверяется через `go build` и unit-тесты на создание типов через конструкторы.

## Scope

- `TokenUsage` entity: `TenantID`, `Model`, `InputTokens`, `OutputTokens`, `Cost`, `Timestamp`
- `UsageRecord` value object: агрегированная запись за период (суммарные токены, cost, количество запросов)
- `CostRate` value object: стоимость токена по модели (config-driven источник)
- `UsageStore` port interface: `Record`, `QueryByTenant`, `QueryByModel`, `AggregateByDay`
- `Aggregation` entity: агрегаты по tenant/model/day: `total_tokens`, `total_cost`, `request_count`, `avg_latency`
- Пакет `src/internal/domain/analytics/` — все перечисленные типы в одном Go-пакете

## Контекст

- Фича закладывает только domain-слой; use case оркестрация и адаптеры — в следующих фазах.
- `TenantID` переиспользуется из существующего `shield/value/tenant_id.go`.
- `CostRate` не выполняет валютных конверсий — стоимость в единой условной единице.
- `Timestamp` — `time.Time`.

## Зависимости

- Пакет `src/internal/domain/shield/value` (для `TenantID`).
- Внешних сервисов нет.

## Требования

- RQ-001 Система ДОЛЖНА предоставлять `TokenUsage` entity с полями `TenantID`, `Model`, `InputTokens`, `OutputTokens`, `Cost`, `Timestamp`.
- RQ-002 Система ДОЛЖНА предоставлять `UsageStore` port-интерфейс с методами `Record(ctx, TokenUsage) error`, `QueryByTenant(ctx, tenantID, from, to) ([]UsageRecord, error)`, `QueryByModel(ctx, model, from, to) ([]UsageRecord, error)`, `AggregateByDay(ctx, tenantID, from, to) ([]Aggregation, error)`.
- RQ-003 Система ДОЛЖНА предоставлять `Aggregation` entity с полями `TenantID`, `Model`, `Date`, `TotalTokens`, `TotalCost`, `RequestCount`, `AvgLatency`.
- RQ-004 Система ДОЛЖНА предоставлять `CostRate` value object, хранящий стоимость 1K input- и output-токенов для модели, с конструктором, принимающим конфигурационные значения.
- RQ-005 Система ДОЛЖНА предоставлять `UsageRecord` value object как снимок потребления за период: `TenantID`, `Model`, `PeriodStart`, `PeriodEnd`, `TotalInputTokens`, `TotalOutputTokens`, `TotalCost`, `RequestCount`.

## Вне scope

- Реализация адаптера `UsageStore` (БД/кэш) — будет в отдельной фазе.
- Use case / application service для записи и чтения — будет в следующей фазе.
- HTTP/gRPC API для аналитики.
- UI для отображения аналитики.
- Интеграция с существующим gateway pipeline (перехват событий завершения запроса).
- Валютные конверсии и мультивалютность.
- Алгоритмы расчёта `AvgLatency` — вычисляется на стороне адаптера.

## Критерии приемки

### AC-001 TokenUsage entity содержит все требуемые поля

- Почему это важно: единый контракт данных для всех потребителей аналитики.
- **Given** нет существующих ограничений на поля
- **When** создаётся экземпляр `TokenUsage` через конструктор `NewTokenUsage(tenantID, model, inputTokens, outputTokens, cost, timestamp)`
- **Then** все поля заполнены и доступны через экспортированные поля или геттеры
- Evidence: unit-тест создаёт `TokenUsage` и проверяет каждое поле на соответствие переданным значениям.

### AC-002 UsageStore port interface объявляет все методы

- Почему это важно: контракт для всех реализаций storage аналитики.
- **Given** интерфейс `UsageStore` определён в пакете `analytics`
- **When** проверяется набор методов
- **Then** интерфейс содержит `Record`, `QueryByTenant`, `QueryByModel`, `AggregateByDay` с корректными сигнатурами (контекст, параметры, возвращаемые типы)
- Evidence: компиляция пакета проходит; декларация интерфейса проверяется в тесте через присвоение переменной типа интерфейса.

### AC-003 Aggregation entity содержит агрегированные метрики

- Почему это важно: единый формат для дневных срезов по tenant/model.
- **Given** определён тип `Aggregation`
- **When** создаётся экземпляр через конструктор
- **Then** поля `TenantID`, `Model`, `Date`, `TotalTokens`, `TotalCost`, `RequestCount`, `AvgLatency` существуют и типизированы корректно
- Evidence: unit-тест конструирует `Aggregation` и проверяет все поля.

### AC-004 CostRate вычисляет стоимость по модели

- Почему это важно: корректный расчёт стоимости токенов — основа для биллинга.
- **Given** `CostRate` сконфигурирован с ценой input=0.01, output=0.03 за 1K токенов для модели `gpt-4`
- **When** вызывается метод `Cost(inputTokens=500, outputTokens=200)`
- **Then** возвращается `0.5*0.01 + 0.2*0.03 = 0.011`
- Evidence: unit-тест с фиксированными значениями проверяет расчёт.

### AC-005 UsageRecord агрегирует данные за период

- Почему это важно: единый value object для ответов на запросы аналитики.
- **Given** 3 записи `TokenUsage` за период
- **When** создаётся `UsageRecord` через конструктор
- **Then** поля `TotalInputTokens`, `TotalOutputTokens`, `TotalCost`, `RequestCount` содержат сумму соответствующих значений
- Evidence: unit-тест создаёт `UsageRecord` с агрегированными значениями и проверяет каждое поле.

## Допущения

- `TenantID` — тип из пакета `shield/value`, не дублируется.
- `CostRate` загружается из конфигурации на уровне application (не domain layer).
- `AvgLatency` в `Aggregation` хранится как `time.Duration` и заполняется на уровне адаптера.
- `Timestamp` в `TokenUsage` — `time.Time` с точностью до миллисекунды.
- Все monetary значения (`Cost`, `TotalCost`) — `float64`.

## Критерии успеха

- SC-001 Пакет `analytics` компилируется без ошибок: `go build ./src/internal/domain/analytics/...`
- SC-002 Все unit-тесты проходят: `go test ./src/internal/domain/analytics/...`

## Краевые случаи

- Нулевые токены: `InputTokens=0, OutputTokens=0` — допустимая запись (health-check запрос).
- Отрицательные токены: конструктор `TokenUsage` ДОЛЖЕН возвращать ошибку при `InputTokens < 0` или `OutputTokens < 0`.
- `CostRate` с нулевой ценой: допустимо для бесплатных моделей.
- Пустой `TenantID`: конструктор ДОЛЖЕН возвращать ошибку.
- Период `from > to` в `QueryByTenant`/`QueryByModel`/`AggregateByDay`: интерфейс оставляет обработку на адаптер (может вернуть ошибку или пустой слайс).

## Открытые вопросы

- `none` — требования достаточны для domain-слоя без дополнительных уточнений.
