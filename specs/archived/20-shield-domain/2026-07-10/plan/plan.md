# Content Shield Domain План

## Phase Contract

Inputs: spec 20-shield-domain, inspect pass, пустая директория `src/internal/domain/`.
Outputs: plan, data-model.
Stop if: нет — spec чёткая, inspect pass.

## Цель

Создать пакет `src/internal/domain/shield/` с entity, value objects, domain services, sentinel errors и repository interfaces — полностью на стандартной библиотеке Go. Домен иммутабельный, DDD-стиль. План безопасен, т.к. domain слой не имеет external зависимостей и не затрагивает существующий код.

## MVP Slice

Entity (Profile, Detector, Pattern, Incident, ScanResult, Reaction) + value objects (ProfileID, ProfileSlug, TenantID, PatternID, Severity, ScanStatus) + sentinel errors — покрывает AC-001, AC-002, AC-006, AC-007, AC-008.

## First Validation Path

```bash
go build ./src/internal/domain/shield/...
go test ./src/internal/domain/shield/...
```

Никаких external зависимостей — только стандартная библиотека. Тесты должны собираться и проходить без доступа к сети.

## Scope

- `src/internal/domain/shield/entity/` — 7 типов (Profile, Detector, DetectorType, Pattern, Reaction, Incident, ScanResult)
- `src/internal/domain/shield/value/` — 6 типов (ProfileID, ProfileSlug, TenantID, PatternID, Severity, ScanStatus)
- `src/internal/domain/shield/service/` — ScanPipeline, PolicyEvaluator
- `src/internal/domain/shield/errors/` — 5 sentinel errors
- `src/internal/domain/shield/repository.go` — ProfileRepository, IncidentRepository
- Все типы иммутабельны: конструкторы с валидацией, геттеры, без сеттеров

## Performance Budget

- `none` — domain слой не содержит горячих путей, IO или аллокаций вне конструкторов

## Implementation Surfaces

| Surface | Статус | Почему участвует |
|---------|--------|-----------------|
| `src/internal/domain/shield/value/*.go` | новая | Value objects — фундамент всех entity |
| `src/internal/domain/shield/entity/*.go` | новая | Entity — ядро домена |
| `src/internal/domain/shield/errors/*.go` | новая | Sentinel errors |
| `src/internal/domain/shield/repository.go` | новая | Port interfaces |
| `src/internal/domain/shield/service/scan.go` | новая | ScanPipeline |
| `src/internal/domain/shield/service/evaluate.go` | новая | PolicyEvaluator |

## Bootstrapping Surfaces

- `src/internal/domain/shield/{entity,value,service,errors}/` — 4 директории

## Влияние на архитектуру

- Новый доменный пакет `shield` под `domain/` — не ломает существующие пакеты
- Repository interfaces — единственная точка соединения с infrastructure слоем в будущем
- Все будущие use case слои будут зависеть от этого пакета

## Acceptance Approach

| AC | Подход | Surfaces | Наблюдение |
|----|--------|----------|------------|
| AC-001 | NewProfile + геттеры | entity/profile.go, value/* | go test |
| AC-002 | NewProfile с невалидным slug | entity/profile.go | go test, errors.Is(ErrInvalidSlug) |
| AC-003 | ScanPipeline.Execute | service/scan.go | go test |
| AC-004 | PolicyEvaluator.Evaluate | service/evaluate.go | go test |
| AC-005 | compile-time mock check | repository.go | go build |
| AC-006 | == сравнение value objects | value/* | go test |
| AC-007 | errors.Is для каждого sentinel | errors/* | go test |
| AC-008 | NewDetector с Pattern/Severity | entity/detector.go | go test |

## Данные и контракты

- Data model — это сам domain слой (entity + value). Описан в `data-model.md`.
- API-контракты: repository interfaces — единственные внешние контракты, определяемые здесь.
- Изменений существующей data model нет.

## Стратегия реализации

- DEC-001 Value objects как plain structs, не интерфейсы
  Why: DDD value objects — иммутабельные structs с value semantic (== сравнение). Интерфейсы добавляют косвенность без пользы.
  Tradeoff: нельзя подменить реализацию в тестах — но value objects не нужно мокать.
  Affects: `value/*.go`
  Validation: AC-006 (сравнение через ==)

- DEC-002 Sentinel errors через var ErrX = errors.New(...)
  Why: идиоматичный Go-способ для domain errors; caller использует errors.Is
  Tradeoff: нельзя добавить контекст (но это feature, не баг — доменная ошибка должна быть чистой)
  Affects: `errors/*.go`
  Validation: AC-007 (errors.Is)

- DEC-003 Repository interfaces на уровне пакета domain/shield (port), а не внутри entity
  Why: hex-arch/clean architecture: port принадлежит domain, adapter принадлежит infra
  Tradeoff: пакет domain/shield/repository.go импортирует entity/ — циклических зависимостей нет
  Affects: `repository.go`
  Validation: AC-005 (compile-time mock check)

- DEC-004 ScanPipeline синхронный последовательный
  Why: domain не должен управлять горутинами. Параллельность — infra/use case ответственность.
  Tradeoff: производительность ниже при большом числе детекторов — но это решается на уровне use case/adapters
  Affects: `service/scan.go`
  Validation: AC-003 (детекторы выполняются)

- DEC-005 Один файл на один type/aggregate, не barrell
  Why: Go не barrell-friendly; один type на файл упрощает навигацию и diff
  Tradeoff: больше файлов (но в domain это норма)
  Affects: все поверхности

## Incremental Delivery

### MVP (Первая ценность)

- Value objects + Entity + Sentinel errors + Repository interfaces
- AC-001, AC-002, AC-005, AC-006, AC-007, AC-008
- Валидация: `go build` + `go test`

### Итеративное расширение

- ScanPipeline + PolicyEvaluator (AC-003, AC-004): добавляются после того, как entity/types определены
- DetectorType enum/iota: вместе с entity

## Порядок реализации

1. Value objects (ни от чего не зависят)
2. Sentinel errors (ни от чего не зависят)
3. Entity (зависят от value + errors)
4. Repository interfaces (зависят от entity)
5. Services (зависят от entity)
6. Тесты (после каждого пакета)

Параллельно: value + errors не зависят друг от друга.

## Риски

- Риск: циклическая зависимость entity ↔ value (ProfileID в Profile, Profile в repository)
  Mitigation: value objects не импортируют entity; entity импортируют value. Repository.go импортирует entity. Циклов нет.
- Риск: переусложнение value objects (слишком много полей)
  Mitigation: value objects минимальны — только идентификаторы и перечисления.

## Rollout и compatibility

- Специальных rollout-действий не требуется — домен новый, обратной совместимости нет.

## Проверка

- `go build ./src/internal/domain/shield/...` — сборка без external зависимостей
- `go vet ./src/internal/domain/shield/...` — статический анализ
- `go test ./src/internal/domain/shield/...` — unit-тесты для каждого пакета
- AC-001..AC-008: каждый сценарий из Acceptance Approach

## Соответствие конституции

- нет конфликтов — чистый Go, DDD, Clean Architecture, ни одной external зависимости
