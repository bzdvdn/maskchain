# Shield Preprocessors: CSV/JSON Masking Pipeline — План

## Phase Contract

Inputs: spec (pass), inspect (pass), repo surfaces (Shield Engine pipeline, Profile entity, Profile Repository).
Outputs: plan, data model.
Stop if: spec intent неоднозначна для safe sequencing.

## Цель

Реализовать препроцессоры CSV/JSON в Shield Engine pipeline: новый domain-пакет `preprocessor`, расширение Profile entity (JSONB-поле `preprocessors`), интеграция в `MaskHandler` до вызова детекторов. Подход безопасен — препроцессоры опциональны (пустой список = no-op), детекторы не требуют изменений.

## MVP Slice

CSVProcessor (`full` + `surname`), JSONProcessor (`full` + JSONPath с `[*]`), фабрика `NewPreprocessor`, `PreprocessorDef` в Profile entity, интеграция в `MaskHandler`. Закрывает AC-001, AC-002, AC-003, AC-005, AC-007, AC-008.

## First Validation Path

1. `go test ./src/internal/domain/shield/preprocessor/...` — unit-тесты CSVProcessor, JSONProcessor, factory
2. Ручная проверка: запустить gateway, отправить `POST /api/v1/shield/mask` с телом, содержащим CSV/JSON, убедиться по логам, что детекторы получили текст с плейсхолдерами

## Scope

- `src/internal/domain/shield/preprocessor/` — новая директория: Processor interface, CSVProcessor, JSONProcessor, factory, PreprocessorDef, JSONPath walker
- `src/internal/domain/shield/entity/profile.go` — новое поле `preprocessors []PreprocessorDef`
- `src/internal/domain/shield/mask/handler.go` или аналогичный — интеграция вызова препроцессоров (опционально)
- `src/internal/api/mask_handler.go` — вызов препроцессоров до детекторов
- `src/internal/adapters/repository/profile/postgres.go` — JSONB-сериализация `preprocessors`

**Не меняется:** Detector interface, CompositeDetector, DetectorRegistry, MaskUseCase, API contracts (request/response), конфигурация gateway.

## Performance Budget

- SC-003: CSV-блок 1000×10 колонок < 100ms в `Process()`
- SC-002: overhead на неструктурированном тексте < 1% (benchmark)
- `none` для memory budget на данном этапе

## Implementation Surfaces

| Surface | Статус | Роль |
|---|---|---|
| `src/internal/domain/shield/preprocessor/` | Новая | Processor interface, CSVProcessor, JSONProcessor, factory, ProcessResult, PreprocessorDef, JSONPath walker |
| `src/internal/domain/shield/entity/profile.go` | Сущ. | Добавить поле `preprocessors []PreprocessorDef` + `WithPreprocessors()` option + `Preprocessors()` getter |
| `src/internal/api/mask_handler.go` | Сущ. | Интегрировать вызов препроцессоров перед циклом детекторов (строка 41) |
| `src/internal/adapters/repository/profile/postgres.go` | Сущ. | JSONB marshaling/unmarshaling поля `preprocessors` |

## Bootstrapping Surfaces

- `src/internal/domain/shield/preprocessor/` — создать первой, т.к. типы из неё нужны entity и handler
- `none` для остального — структура репозитория уже готова

## Влияние на архитектуру

- Локальное: новое domain-пакет `preprocessor` — следует паттерну `detector/` (интерфейс + реализации в domain)
- Profile entity расширяется: новое поле, не ломающее существующие конструкторы (functional option)
- Shield Engine pipeline расширяется: pre-processing step перед детекторами
- Нет миграции данных: новое JSONB-поле nullable со default `NULL` или `[]`
- Нет влияния на API/REST-контракты

## Acceptance Approach

| AC | Подход | Surfaces | Наблюдение |
|---|---|---|---|
| AC-001 | Unit: CSVProcessor с тестовым CSV | `CSVProcessor` | ModifiedText + Replacements |
| AC-002 | Unit: CSVProcessor с mask surname | `CSVProcessor` | ModifiedText |
| AC-003 | Unit: JSONProcessor с вложенным JSON | `JSONProcessor` | ModifiedText |
| AC-004 | Unit: JSONProcessor с ```json фенсами | `JSONProcessor` | ModifiedText |
| AC-005 | Unit: JSONProcessor с `[*]` wildcard | `JSONProcessor` | ModifiedText |
| AC-006 | Unit: CSVProcessor с quoted/escaped CSV | `CSVProcessor` | ModifiedText |
| AC-007 | Unit: factory с csv/json/unknown type | `preprocessor` factory | Processor type / error |
| AC-008 | Integration: mock handler + preprocessor + mock detector | `mask_handler.go`, `preprocessor` | detector.Scan получает замаскированный текст |

## Данные и контракты

- `data-model.md`: Profile.Preprocessors — новое поле, тип `[]PreprocessorDef`, JSONB в PG
- API-контракты не меняются — препроцессоры применяются прозрачно внутри pipeline
- Результат ProcessResult.Replacements пока не экспортируется через API (downstream use — вне scope)

## Стратегия реализации

### DEC-001: Inline препроцессоры в Profile (JSONB) vs отдельная таблица

- **Why:** препроцессоры неотделимы от профиля — загружаются и применяются вместе. JSONB-массив в той же строке — zero JOINs, атомарная загрузка, не ломает существующие запросы. Отдельная таблица добавила бы CRUD без реальной выгоды до появления shared-preprocessors.
- **Tradeoff:** sharing между профилями потребует рефакторинга (выделение preprocessor entity). Это отложено (Вне scope).
- **Affects:** `entity/profile.go`, `adapters/repository/profile/postgres.go`
- **Validation:** тест: `Profile.Preprocessors()` возвращает установленные значения

### DEC-002: Встраивание препроцессоров в MaskHandler vs новый middleware/сервис

- **Why:** handler уже читает body, вызывает детекторы и возвращает результат. Добавление pre-processing step в то же место — минимальный diff и простая traceability. Новый middleware усложнил бы передачу Replacements между слоями.
- **Tradeoff:** handler получает дополнительную ответственность. При разрастании pre-processing — вынести в отдельный `PreprocessingService`.
- **Affects:** `src/internal/api/mask_handler.go`
- **Validation:** AC-008: детекторы получают текст с плейсхолдерами

### DEC-003: JSONPath walker через `map[string]interface{}` vs string-based regex

- **Why:** unmarshal JSON в `any`, пройти по дереву через segments (`user`, `email`, `[*]`), мутировать значения, re-marshal — надёжнее и проще, чем regex на сырой строке. Решает экранирование, вложенность, типы.
- **Tradeoff:** требует полного парсинга JSON — не подходит для streaming. Но AI-запросы целиком в памяти на этапе проверки.
- **Affects:** `preprocessor/json_processor.go`, `preprocessor/jsonpath.go`
- **Validation:** AC-003, AC-004, AC-005

### DEC-004: Fail-open при ошибке препроцессора

- **Why:** ошибка в одном препроцессоре не должна блокировать проверку всего запроса. Логируем ошибку и продолжаем со следующим препроцессором / без препроцессинга.
- **Tradeoff:** риск что чувствительные данные не будут замаскированы. Mitigation: observability (алерт на ошибку препроцессора).
- **Affects:** точка интеграции в handler
- **Validation:** юнит-тест: Process() бросает ошибку → pipeline продолжает с оригинальным текстом

## Incremental Delivery

### MVP (первые 3 задачи)

1. **PreprocessorDef + Processor interface + factory + ProcessResult** — типы данных и фабрика
2. **CSVProcessor** — full mask + surname + quoting/escaping → AC-001, AC-002, AC-006
3. **JSONProcessor + JSONPath walker** — full mask, вложенные объекты, `[*]` → AC-003, AC-004, AC-005

Критерий: `go test ./src/internal/domain/shield/preprocessor/...` — зелёный после каждой задачи.

### Итеративное расширение

4. **Profile entity + repository** — поле `preprocessors`, функциональная опция, JSONB сериализация → основа для AC-008
5. **MaskHandler интеграция** — загрузка препроцессоров из профиля, вызов до детекторов, fail-open → AC-008
6. **Integration test** — полный pipeline с mock repository, preprocessor, mock detector → AC-008

## Порядок реализации

1. **Domain types first** — PreprocessorDef, Processor interface, ProcessResult, factory — независимы от всего
2. **Processors** — CSVProcessor, JSONProcessor — зависят только от domain types
3. **Profile extension** — entity поле + опция — зависит от PreprocessorDef
4. **Repository** — JSONB в postgres адаптере — зависит от entity
5. **Handler integration** — зависит от всего выше

Параллельно можно: (1)+(2) параллельно с (3), (4)+(5) последовательно после.

## Риски

- **R1: CSV detection false positives.** Текст с обычными запятыми может быть ошибочно принят за CSV.
  - Mitigation: эвристика — только блоки с >= 2 строк (заголовок + данные) с одинаковым числом запятых. В плане предусмотреть unit-test corner cases.
- **R2: JSONPath с экранированными точками в ключах** (e.g., `"key.with.dots"`).
  - Mitigation: в MVP не поддерживаем — документировать в Допущения. Парсер segment-сплит по `.` без учёта escaping.
- **R3: Profile Repository — заглушки.** Текущая имплементация PostgresProfileRepo — скелетная (все методы `return nil`). JSONB-поле добавится, но реальное сохранение/чтение недоступно до фазы profile repository.
  - Mitigation: unit-тесты препроцессоров не зависят от PG (чистые функции). AC-008 integration test через mock repository.

## Rollout и compatibility

- Новое JSONB-поле `preprocessors` nullable — обратная совместимость со старыми профилями (NULL = пустой список, препроцессоры не запускаются).
- Feature flag не требуется — пустой список препроцессоров = no-op.
- Никаких новых API endpoints, миграций данных или change of behavior для существующих запросов без препроцессоров.

## Проверка

| Шаг | Тип | Проверяет |
|---|---|---|
| Unit: CSVProcessor | automated | AC-001, AC-002, AC-006 |
| Unit: JSONProcessor | automated | AC-003, AC-004, AC-005 |
| Unit: factory | automated | AC-007 |
| Unit: JSONPath walker | automated | segments, wildcard, edge cases |
| Unit: Profile entity (preprocessors field) | automated | поле + option + getter |
| Integration: handler + preprocessors + mock detector | automated | AC-008, DEC-002, DEC-004 |
| Manual: gateway smoke | manual | весь pipeline end-to-end |

## Соответствие конституции

- нет конфликтов: Content Shield core domain (I) ✓, DDD + Clean Architecture (processor interface в domain) ✓, PostgreSQL persistence (II) ✓, native-only data plane (VI) ✓, extensibility over hardcoding (X) ✓, feature-ветка (XIV) ✓
