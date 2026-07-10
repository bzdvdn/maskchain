# Content Shield Domain Задачи

## Phase Contract

Inputs: plan 20-shield-domain, data-model.md, spec.md.
Outputs: исполнимые задачи с покрытием AC.
Stop if: нет — plan конкретен, AC привязаны к чётким поверхностям.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/domain/shield/value/*.go` | T1.1, T5.1 |
| `src/internal/domain/shield/errors/*.go` | T1.2, T5.1 |
| `src/internal/domain/shield/entity/profile.go` | T2.1, T5.2 |
| `src/internal/domain/shield/entity/detector.go` | T2.2, T5.2 |
| `src/internal/domain/shield/entity/pattern.go` | T2.2, T5.2 |
| `src/internal/domain/shield/entity/incident.go` | T2.3, T5.2 |
| `src/internal/domain/shield/entity/scan_result.go` | T2.3, T5.2 |
| `src/internal/domain/shield/entity/reaction.go` | T2.3, T5.2 |
| `src/internal/domain/shield/entity/detector_type.go` | T2.2 |
| `src/internal/domain/shield/repository.go` | T3.1, T5.4 |
| `src/internal/domain/shield/service/scan.go` | T4.1, T5.3 |
| `src/internal/domain/shield/service/evaluate.go` | T4.2, T5.3 |

## Implementation Context

- Цель MVP: value objects + entity + errors + repository interfaces (AC-001, AC-002, AC-005, AC-006, AC-007, AC-008). Services (AC-003, AC-004) — вторая итерация.
- Инварианты/семантика: value objects иммутабельны (== сравнение); entity через конструкторы с валидацией; sentinel errors через `var ErrX = errors.New(...)`; repository interfaces — port для infra слоя
- Ошибки/коды: ErrProfileNotFound, ErrInvalidPattern, ErrInvalidSlug, ErrDetectorFailed, ErrDuplicateSlug. Валидация slug: `^[a-zA-Z0-9-]{3,}$`. Detector без Pattern → ErrInvalidPattern.
- Контракты/протокол: все типы в standard library; ProfileRepository.Save(context, *Profile) error; IncidentRepository.Save(context, *Incident) error
- Границы scope: не делаем реализацию repository, Presidio integration, use cases, валидацию контента через regex
- Proof signals: `go build ./src/internal/domain/shield/...` (без новых dependecies); `go test ./src/internal/domain/shield/...` pass; `go vet` pass
- References: DEC-001 (value structs), DEC-002 (sentinel errors), DEC-003 (interfaces in domain/shield), DEC-004 (sync sequential), DEC-005 (one file per type), DM (entity/value/errors specs)

## Фаза 1: Foundation (value + errors)

Цель: value objects и sentinel errors — фундамент, от которого зависят все entity.

- [x] T1.1 Реализовать value objects: ProfileID, ProfileSlug, TenantID, PatternID, Severity, ScanStatus
  Touches: `src/internal/domain/shield/value/*.go`
  Outcome: 6 типов, каждый в отдельном файле; ProfileSlug с конструктором NewProfileSlug с валидацией (`^[a-zA-Z0-9-]{3,}$`); Severity с методом Reaction(); все иммутабельны, сравнение через ==
  References: DEC-001, DEC-005, DM

- [x] T1.2 Реализовать sentinel errors: ErrProfileNotFound, ErrInvalidPattern, ErrInvalidSlug, ErrDetectorFailed, ErrDuplicateSlug
  Touches: `src/internal/domain/shield/errors/*.go`
  Outcome: 5 sentinel errors, var ErrX = errors.New("..."), различаются через errors.Is
  References: DEC-002, DM

## Фаза 2: Core entities

Цель: entity — ядро домена, зависит от value + errors.

- [x] T2.1 Реализовать Profile entity
  Touches: `src/internal/domain/shield/entity/profile.go`
  Outcome: Profile struct с ProfileID, ProfileSlug, TenantID, Name, Description *string, Detectors []Detector, Enabled bool, CreatedAt/UpdatedAt time.Time; конструктор NewProfile с валидацией slug; геттеры
  References: DM (Profile), AC-001, AC-002

- [x] T2.2 Реализовать Detector, DetectorType, Pattern entities
  Touches: `src/internal/domain/shield/entity/detector.go`, `src/internal/domain/shield/entity/detector_type.go`, `src/internal/domain/shield/entity/pattern.go`
  Outcome: Detector struct с DetectorID, Type DetectorType, Patterns []Pattern, Severity, Enabled; DetectorType enum (regex, keyword, presidio); Pattern struct с PatternID, Expression, Description, Type PatternType; конструкторы с валидацией (Pattern без Expression → ErrInvalidPattern); геттеры
  References: DM (Detector, DetectorType, Pattern), AC-008

- [x] T2.3 Реализовать Incident, ScanResult, Reaction entities
  Touches: `src/internal/domain/shield/entity/incident.go`, `src/internal/domain/shield/entity/scan_result.go`, `src/internal/domain/shield/entity/reaction.go`
  Outcome: Incident struct с DetectorID, PatternID, Severity, Fragment, Position; ScanResult struct с Status ScanStatus, Incidents []Incident, ScannedAt; Reaction type string (allow, block, review, log) с опциональным Metadata; конструкторы с валидацией; геттеры
  References: DM (Incident, ScanResult, Reaction)

## Фаза 3: Repository interfaces

Цель: port interfaces для infrastructure слоя.

- [x] T3.1 Реализовать ProfileRepository и IncidentRepository interfaces
  Touches: `src/internal/domain/shield/repository.go`
  Outcome: ProfileRepository interface (Save, FindByID, FindBySlug, ListByTenant, Delete); IncidentRepository interface (Save, FindByID, ListByProfile, ListByTenant); compile-time mock structs для verify (implements check)
  References: DM (repository interfaces), DEC-003, AC-005

## Фаза 4: Domain services

Цель: ScanPipeline и PolicyEvaluator.

- [x] T4.1 Реализовать ScanPipeline service
  Touches: `src/internal/domain/shield/service/scan.go`
  Outcome: ScanPipeline struct с методом Execute(detectors []Detector, content string) *ScanResult; все enabled детекторы выполняются последовательно; disabled пропускаются; пустой набор детекторов → ScanStatus=clean
  References: DM (ScanResult, Detector), DEC-004, AC-003

- [x] T4.2 Реализовать PolicyEvaluator service
  Touches: `src/internal/domain/shield/service/evaluate.go`
  Outcome: PolicyEvaluator struct с методом Evaluate(result *ScanResult) Reaction; выбор по highest severity (critical→block, high→review, medium/low→log); пустой ScanResult (clean) → allow; неизвестный severity → block (fail-safe)
  References: DM (Reaction, Severity), AC-004

## Фаза 5: Verification

Цель: automated тесты и verify всех AC.

- [x] T5.1 Написать unit-тесты для value objects и errors
  Touches: `src/internal/domain/shield/value/*_test.go`, `src/internal/domain/shield/errors/*_test.go`
  Outcome: тесты для каждого value object: сравнение через == (AC-006), валидация ProfileSlug (AC-002), Severity.Reaction маппинг; тесты для sentinel errors: errors.Is (AC-007)
  References: AC-002, AC-006, AC-007

- [x] T5.2 Написать unit-тесты для entity
  Touches: `src/internal/domain/shield/entity/*_test.go`
  Outcome: тесты: NewProfile с валидными/невалидными полями (AC-001, AC-002); NewDetector с Pattern (AC-008); Detector без Pattern → ErrInvalidPattern; NewIncident; NewScanResult
  References: AC-001, AC-002, AC-008

- [x] T5.3 Написать unit-тесты для services
  Touches: `src/internal/domain/shield/service/*_test.go`
  Outcome: тесты: ScanPipeline.Execute с enabled/disabled детекторами (AC-003); PolicyEvaluator.Evaluate с разными severity (AC-004); пустой детектор; disabled детектор; пустой ScanResult
  References: AC-003, AC-004

- [x] T5.4 Выполнить verify path
  Touches: (нет — ручная проверка)
  Outcome: `go build ./src/internal/domain/shield/...` (без external dependencies); `go vet ./src/internal/domain/shield/...` без замечаний; `go test ./src/internal/domain/shield/...` — pass; compile-time mock check (AC-005)
  References: AC-005

## Покрытие критериев приемки

- AC-001 -> T2.1, T5.2
- AC-002 -> T1.1, T2.1, T5.1, T5.2
- AC-003 -> T4.1, T5.3
- AC-004 -> T4.2, T5.3
- AC-005 -> T3.1, T5.4
- AC-006 -> T1.1, T5.1
- AC-007 -> T1.2, T5.1
- AC-008 -> T2.2, T5.2

## Заметки

- Порядок: T1.x → T2.x → T3.x → T4.x → T5.x. T1.1 и T1.2 независимы. T2.x зависит от T1.x. T3.x зависит от T2.x. T4.x зависит от T2.x. T5.x — после каждой фазы.
- Для value objects формат: `type X struct { value string }` (или type alias для string). Для Severity: `type Severity int` с iota.
- trace-маркеры `@sk-task` над owning declaration (не package/import/file-header)
- Теги для задач: `@sk-task 20-shield-domain#T1.1` и т.д.
