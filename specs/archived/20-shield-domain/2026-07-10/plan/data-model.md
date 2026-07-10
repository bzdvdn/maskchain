# Content Shield Domain Data Model

## Status: new

## Описание

Полностью новая модель данных — domain-слой Content Shield. Ниже описаны entity, value objects и их отношения.

## Value Objects

**Пакет:** `src/internal/domain/shield/value/`

### ProfileID
- `type ProfileID string` — UUID v4
- Иммутабельный, сравнение по значению

### ProfileSlug
- `type ProfileSlug string` — латиница, цифры, дефис, мин. 3 символа
- Конструктор `NewProfileSlug(s string) (ProfileSlug, error)` валидирует формат

### TenantID
- `type TenantID string` — UUID v4
- Иммутабельный

### PatternID
- `type PatternID string` — UUID v4
- Иммутабельный

### Severity
- `type Severity int` — iota: Critical, High, Medium, Low
- Метод `Reaction() Reaction` — маппинг: Critical→block, High→review, Medium/Low→log

### ScanStatus
- `type ScanStatus string` — clean, suspicious, blocked, error

## Entities

**Пакет:** `src/internal/domain/shield/entity/`

### Profile
- ProfileID, ProfileSlug, TenantID, Name string, Description *string
- Detectors []Detector
- Enabled bool
- CreatedAt, UpdatedAt time.Time
- Конструктор: `NewProfile(id, slug, tenant, name, detectors, opts...) (*Profile, error)`

### Detector
- DetectorID string (UUID), Type DetectorType
- Patterns []Pattern, Severity Severity
- Enabled bool
- Конструктор: `NewDetector(id, typ, patterns, severity) (*Detector, error)`

### DetectorType
- `type DetectorType string` — regex, keyword, presidio (enum)

### Pattern
- PatternID PatternID, Expression string, Description string
- Type PatternType (regex, keyword)
- Конструктор: `NewPattern(id, expr, desc, typ) (*Pattern, error)`

### Reaction
- `type Reaction string` — allow, block, review, log
- Metadata *string (опционально)

### Incident
- DetectorID string, PatternID PatternID, Severity Severity
- Fragment string, Position int
- Конструктор: `NewIncident(detectorID, patternID, severity, fragment, position) (*Incident, error)`

### ScanResult
- Status ScanStatus, Incidents []Incident
- ScannedAt time.Time
- Конструктор: `NewScanResult(status, incidents) *ScanResult`

## Domain Errors

**Пакет:** `src/internal/domain/shield/errors/`

- `var ErrProfileNotFound = errors.New("profile not found")`
- `var ErrInvalidPattern = errors.New("invalid pattern")`
- `var ErrInvalidSlug = errors.New("invalid profile slug")`
- `var ErrDetectorFailed = errors.New("detector execution failed")`
- `var ErrDuplicateSlug = errors.New("duplicate profile slug")`

## Repository Interfaces

**Файл:** `src/internal/domain/shield/repository.go`

### ProfileRepository
```go
type ProfileRepository interface {
    Save(ctx context.Context, profile *entity.Profile) error
    FindByID(ctx context.Context, id value.ProfileID) (*entity.Profile, error)
    FindBySlug(ctx context.Context, tenantID value.TenantID, slug value.ProfileSlug) (*entity.Profile, error)
    ListByTenant(ctx context.Context, tenantID value.TenantID) ([]*entity.Profile, error)
    Delete(ctx context.Context, id value.ProfileID) error
}
```

### IncidentRepository
```go
type IncidentRepository interface {
    Save(ctx context.Context, incident *entity.Incident) error
    FindByID(ctx context.Context, id value.PatternID) (*entity.Incident, error) // note: IncidentID TBD
    ListByProfile(ctx context.Context, profileID value.ProfileID) ([]*entity.Incident, error)
    ListByTenant(ctx context.Context, tenantID value.TenantID) ([]*entity.Incident, error)
}
```

## Связи

- Profile 1──* Detector (Profile содержит список детекторов)
- Detector 1──* Pattern (детектор содержит список паттернов)
- ScanResult 1──* Incident (результат содержит список инцидентов)
- Incident ссылается на DetectorID + PatternID (по значению, не по ссылке)

## Инварианты

- ProfileSlug уникален в пределах TenantID (валидируется на уровне repository, не в domain конструкторе — ErrDuplicateSlug)
- Severity маппинг в Reaction жёстко задан в domain service (PolicyEvaluator)
- ScanResult без инцидентов = ScanStatus=clean
- Detector без Pattern = ErrInvalidPattern
