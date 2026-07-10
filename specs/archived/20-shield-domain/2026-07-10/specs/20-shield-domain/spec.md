# Content Shield Domain

## Scope Snapshot

- In scope: domain-слой Content Shield — сущности, value objects, domain services, domain errors и repository interfaces в пакете `src/internal/domain/shield/`.
- Out of scope: реализация repository (PostgreSQL), Presidio integration, HTTP/gRPC handlers, use cases, UI, configuration связывания.

## Цель

Разработчик domain-слоя получает чистый, тестируемый набор Go-типов и интерфейсов, описывающих предметную область Content Shield: профили политик безопасности, детекторы, реакции на инциденты, pipeline сканирования и интерфейсы репозиториев. Успех определяется тем, что все entity, value objects и service signature собраны в пакете `domain/shield` без external-зависимостей, и domain-сервисы покрыты unit-тестами.

## Основной сценарий

1. Стартовая точка: разработчик создаёт entity и value objects на основе описанных требований.
2. Основное действие: Profile содержит детекторы (Detector) с типом (DetectorType), Pattern для сопоставления, Reaction на событие. ScanPipeline оркестрирует детекторы и возвращает ScanResult. PolicyEvaluator выбирает реакцию по severity.
3. Результат: domain-слой готов к использованию из use case слоя; repository interfaces определены для реализации в infrastructure слое.
4. Ошибка/fallback: domain errors (ErrProfileNotFound, ErrInvalidPattern, ErrInvalidSlug, ErrDetectorFailed, ErrDuplicateSlug) возвращаются из domain services и repository interfaces.

## User Stories

- P1 Story: как разработчик, я хочу создать Profile с набором Detector и Pattern, чтобы определить политику безопасности для заданного типа контента.
- P2 Story: как разработчик, я хочу запустить ScanPipeline с набором детекторов и получить ScanResult с статусом и инцидентами.
- P3 Story: как разработчик, я хочу вызвать PolicyEvaluator по результату сканирования и получить Reaction (allow/block/review/log).

## MVP Slice

Сущности Profile, Detector, DetectorType, Pattern, Reaction, Incident, ScanResult + value objects (ProfileID, ProfileSlug, TenantID, PatternID, Severity, ScanStatus) + базовый ScanPipeline без расширенной оркестрации.

## First Deployable Outcome

Пакет `src/internal/domain/shield/` компилируется без external-зависимостей, entity типы и value objects корректно создаются через конструкторы, domain errors определны и тестируемы.

## Scope

- `src/internal/domain/shield/entity/` — Profile, Detector, DetectorType, Reaction, Incident, ScanResult, Pattern
- `src/internal/domain/shield/value/` — ProfileID, ProfileSlug, TenantID, PatternID, Severity, ScanStatus
- `src/internal/domain/shield/service/` — ScanPipeline, PolicyEvaluator
- `src/internal/domain/shield/errors/` — ErrProfileNotFound, ErrInvalidPattern, ErrInvalidSlug, ErrDetectorFailed, ErrDuplicateSlug
- `src/internal/domain/shield/repository.go` — ProfileRepository, IncidentRepository interfaces
- Все типы иммутабельны (constructors, value semantics)

## Контекст

- Domain-слой не имеет external-зависимостей — чистый Go standard library
- DDD: entity имеют идентичность (ProfileID, etc.), value objects сравниваются по значению
- ProfileSlug — уникальный человеко-читаемый идентификатор профиля (латиница + цифры + дефис)
- Конституция требует Clean Architecture + PostgreSQL; domain model не должна зависеть от инфраструктуры
- Microsoft Presidio integration будет в adapters, не в domain

## Зависимости

- `none` — domain-слой использует только стандартную библиотеку Go

## Требования

- RQ-001 Profile ДОЛЖЕН содержать ProfileID, ProfileSlug, TenantID, название, набор Detector, опциональный описание, enabled/disabled флаг, временные метки создания и обновления.
- RQ-002 ProfileSlug ДОЛЖЕН быть уникальным в пределах одного TenantID, состоять только из латинских букв, цифр и дефисов.
- RQ-003 Detector ДОЛЖЕН иметь уникальный в рамках Profile DetectorID, DetectorType, набор Pattern, Severity, enabled/disabled флаг.
- RQ-004 Pattern ДОЛЖЕН содержать PatternID, регулярное выражение или ключевое слово, описание и тип (regex/keyword).
- RQ-005 ScanResult ДОЛЖЕН включать ScanStatus (clean/suspicious/blocked/error), список Incident, временную метку.
- RQ-006 Incident ДОЛЖЕН содержать DetectorID, PatternID, Severity, найденный фрагмент, позицию в контенте.
- RQ-007 Reaction ДОЛЖЕН быть одним из: allow, block, review, log — с опциональным полем для метаданных.
- RQ-008 ScanPipeline ДОЛЖЕН принимать набор Detector и контент, запускать каждый enabled детектор, собирать результат и возвращать ScanResult.
- RQ-009 PolicyEvaluator ДОЛЖЕН принимать ScanResult и возвращать Reaction (по highest severity).
- RQ-010 ProfileRepository интерфейс ДОЛЖЕН включать методы Save, FindByID, FindBySlug, ListByTenant, Delete.
- RQ-011 IncidentRepository интерфейс ДОЛЖЕН включать методы Save, FindByID, ListByProfile, ListByTenant.

## Вне scope

- Реализация repository (PostgreSQL, in-memory)
- Presidio/ML integration
- Валидация контента (regex engine execution)
- HTTP/gRPC API endpoints
- Use case orchestration
- Caching layer

## Критерии приемки

### AC-001 Profile создаётся с правильными полями

- Почему это важно: Profile — центральная сущность, от которой зависит вся конфигурация политик
- **Given** конструктор NewProfile с валидными ProfileID, ProfileSlug, TenantID, названием
- **When** Profile создан
- **Then** все поля установлены корректно, Slug валидирован, временные метки не zero
- Evidence: профиль создаётся без ошибок; геттеры возвращают ожидаемые значения

### AC-002 ProfileSlug валидируется при создании

- Почему это важно: slug используется в URL и идентификации, невалидный slug приведёт к ошибкам маршрутизации
- **Given** конструктор NewProfile с невалидным ProfileSlug (содержит пробелы, кириллицу или спецсимволы)
- **When** Profile создаётся
- **Then** возвращается ErrInvalidSlug
- Evidence: тест с невалидным slug ожидает ошибку

### AC-003 ScanPipeline запускает детекторы и возвращает ScanResult

- Почему это важно: pipeline — основной workflow обработки контента
- **Given** ScanPipeline с набором enabled детекторов и входным контентом
- **When** ScanPipeline.Execute вызывается
- **Then** все enabled детекторы запущены, результат содержит ScanStatus и список Incident
- Evidence: ScanResult.Status не пустой; количество Incident соответствует сработавшим детекторам

### AC-004 PolicyEvaluator выбирает Reaction по highest severity

- Почему это важно: автоматический выбор реакции — ключевая бизнес-логика
- **Given** PolicyEvaluator и ScanResult с инцидентами разных severity
- **When** PolicyEvaluator.Evaluate вызывается
- **Then** возвращается Reaction, соответствующий highest severity (critical→block, high→review, medium/low→log)
- Evidence: для каждого severity уровня возвращается ожидаемая Reaction

### AC-005 Repository interfaces определены с полной сигнатурой

- Почему это важно: интерфейсы — контракт для infrastructure слоя
- **Given** ProfileRepository и IncidentRepository интерфейсы
- **When** типы проверены на полноту методов
- **Then** все методы из RQ-010 и RQ-011 присутствуют с корректными сигнатурами
- Evidence: проверка через компиляцию — mock-реализации удовлетворяют интерфейсам

### AC-006 Value objects сравниваются по значению

- Почему это важно: value objects должны быть взаимозаменяемы при равенстве полей
- **Given** два ProfileID с одинаковым значением UUID
- **When** сравниваются через ==
- **Then** они равны
- Evidence: тест подтверждает равенство value объектов

### AC-007 Domain errors определны и различаются

- Почему это важно: caller должен различать тип ошибки для правильной обработки
- **Given** определённые domain errors
- **When** errors.Is / errors.As используются
- **Then** каждый error соответствует своему типу
- Evidence: тест проверяет errors.Is для каждого типа ошибки

### AC-008 Detector содержит Pattern и Severity

- Почему это важно: детектор без pattern и severity не имеет смысла
- **Given** конструктор NewDetector с Pattern, Severity и типом
- **When** Detector создан
- **Then** геттеры возвращают корректные Pattern и Severity
- Evidence: тест создаёт детектор и проверяет поля

## Допущения

- UUID генерируется в infrastructure слое; domain принимает уже сгенерированный ProfileID/IncidentID/PatternID
- Slang — только латиница, цифры и дефис; минимальная длина 3 символа
- ScanPipeline выполняет детекторы последовательно (параллельность — infra оптимизация)
- Reaction mapping: critical → block, high → review, medium/low → log
- Временные метки — time.Time (UTC) из стандартной библиотеки

## Критерии успеха

- SC-001 Пакет `domain/shield` компилируется без external-зависимостей (`go build` без новых require в go.mod)
- SC-002 Покрытие domain unit-тестами > 80% для entity и services

## Краевые случаи

- Пустой набор детекторов в профиле — ScanPipeline возвращает ScanStatus=clean
- Отключённый (disabled) детектор — ScanPipeline пропускает его
- Дублирующийся slug в пределах tenant — ErrDuplicateSlug
- Detector без Pattern — ErrInvalidPattern
- ScanResult без инцидентов — Reaction = allow
- Неизвестный severity — Reaction = block (fail-safe)

## Открытые вопросы

- `none`
