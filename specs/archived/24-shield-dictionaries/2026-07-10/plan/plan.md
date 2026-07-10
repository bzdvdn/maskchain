# Shield Dictionaries — План

## Phase Contract

Inputs: spec (`specs/active/24-shield-dictionaries/spec.md`), inspect (`pass`), repo-контекст.
Outputs: plan.md, data-model.md.
Stop if: spec неоднозначна — нет, spec чёткая.

## Цель

Добавить словари как ValueObject, привязанный к Profile через ProfileSlug, с DictionaryRepository (CRUD), DictionaryDetector (реализует Detector) и WordlistMatcher (Aho-Corasick). Словари загружаются вместе с профилем через адаптер ProfileRepository. Работа без standalone API/UI — управление через inline entries профиля.

## MVP Slice

Exact match (HashSet) + CRUD DictionaryRepository + загрузка словаря через ProfileRepository + регистрация DictionaryDetector в DetectorRegistry. AC-001, AC-002, AC-003, AC-006, AC-007.

## First Validation Path

Собрать и запустить `go test ./src/internal/domain/shield/dictionary/...` — unit-тесты на Dictionary, DictionaryRepository (in-memory), DictionaryDetector (exact). Затем `go test ./src/internal/...` — убедиться, что существующие тесты не сломаны.

## Scope

- Новый пакет: `src/internal/domain/shield/dictionary/` — Dictionary, MatchMode, DictionaryRepository, WordlistMatcher, DictionaryDetector
- Миграция: `src/internal/infra/migrations/002_dictionary_entries.sql`
- Адаптер: `src/internal/adapters/repository/dictionary/` — PostgresDictionaryRepo
- Profile entity: добавление поля `dictionaries []Dictionary` (через ProfileOption)
- DetectorType: добавление константы `DetectorTypeDictionary`
- DictionaryRepository не заменяет ProfileRepository — оба существуют раздельно, адаптер ProfileRepository вызывает DictionaryRepository при загрузке профиля
- API/UI не затрагиваются

## Performance Budget

- SC-001: exact match < 1ms на 10KB текста для 1000 entries — гарантируется HashSet O(n) по длине текста
- SC-002: contains (Aho-Corasick) < 5ms на 100KB текста для 100 паттернов
- Memory: автомат Aho-Corasick ≈ O(total characters in entries) — разумно для типовых размеров словаря

## Implementation Surfaces

| Surface | Тип | Почему |
|---|---|---|
| `src/internal/domain/shield/dictionary/dictionary.go` | новый | Dictionary ValueObject |
| `src/internal/domain/shield/dictionary/match_mode.go` | новый | MatchMode enum |
| `src/internal/domain/shield/dictionary/repository.go` | новый | DictionaryRepository interface |
| `src/internal/domain/shield/dictionary/detector.go` | новый | DictionaryDetector (implements Detector) |
| `src/internal/domain/shield/dictionary/wordlist.go` | новый | Aho-Corasick WordlistMatcher |
| `src/internal/domain/shield/entity/detector_type.go` | изменение | + DetectorTypeDictionary |
| `src/internal/domain/shield/entity/profile.go` | изменение | + dictionaries поле, WithDictionaries option |
| `src/internal/infra/migrations/002_dictionary_entries.sql` | новый | таблица dictionary_entries |
| `src/internal/adapters/repository/dictionary/postgres.go` | новый | Postgres словарей |
| `src/internal/adapters/repository/profile/` | изменение | адаптер ProfileRepository загружает словари |

## Bootstrapping Surfaces

- `src/internal/domain/shield/dictionary/` — создать директорию и пакет первыми
- Миграция БД — до adapter-слоя

## Влияние на архитектуру

- **Profile entity**: добавлено необязательное поле `dictionaries`. Существующие тесты не ломаются — поле nil-безопасно.
- **DetectorType**: новая константа — регистрация в `DetectorRegistry` остаётся за вызывающим кодом (main.go DI).
- **ProfileRepository adapter**: требуется внедрение DictionaryRepository для загрузки словарей при FindBySlug/FindByID. Не breaking change — интерфейс ProfileRepository не меняется, меняется только реализация.
- **CompositeDetector**: автоматически включает DictionaryDetector, если он зарегистрирован в registry (регистрация в main.go).

## Acceptance Approach

| AC | Подход | Surfaces | Observability |
|---|---|---|---|
| AC-001 | Unit: NewDictionary + геттеры | `dictionary.go` | `go test` |
| AC-002 | In-memory DictionaryRepository + Save/Find/Delete | `repository.go`, in-memory impl в тесте | `go test` |
| AC-003 | DictionaryDetector.Scan с exact MatchMode | `detector.go`, `match_mode.go` | `go test` |
| AC-004 | DictionaryDetector.Scan с contains (Aho-Corasick) | `detector.go`, `wordlist.go` | `go test` |
| AC-005 | DictionaryDetector.Scan с regex (entry compile error handling) | `detector.go` | `go test` |
| AC-006 | ProfileRepository.FindBySlug возвращает профиль со словарём | profile adapter, dictionary adapter | integration test |
| AC-007 | Registry.Register + Registry.Get | `registry.go` (сущ.) | `go test` |
| AC-008 | WordlistMatcher.Match — multiple overlapping matches | `wordlist.go` | `go test` |

## Данные и контракты

- Новая таблица `dictionary_entries` — см. `data-model.md`
- Profile entity: новое опциональное поле `dictionaries []Dictionary`
- DictionaryRepository interface: `Save(ctx, *Dictionary) error`, `FindByProfileSlug(ctx, slug) (*Dictionary, error)`, `Delete(ctx, profileSlug) error`
- Никакие API/event контракты не меняются — словари не имеют standalone API
- `data-model.md` прилагается с описанием новой таблицы

## Стратегия реализации

### DEC-001 Dictionary как отдельный пакет domain/shield/dictionary, не в entity

- Why: Dictionary — value object со своей логикой (MatchMode, WordlistMatcher). entity содержит только агрегаты (Profile). Маски хранятся отдельно (domain/shield/mask/) — симметрично.
- Tradeoff: требуется импорт entity/value пакетов для ProfileSlug.
- Affects: `src/internal/domain/shield/dictionary/`
- Validation: пакет импортируется из entity и adapter без циклических зависимостей.

### DEC-002 DictionaryRepository отдельный от ProfileRepository

- Why: SRP. ProfileRepository не должен знать о схеме словарей. Адаптер ProfileRepository композирует DictionaryRepository внутри.
- Tradeoff: два SQL-запроса вместо одного JOIN при загрузке профиля. Некритично — 1 доп. запрос на profile fetch.
- Affects: `src/internal/domain/shield/dictionary/repository.go`, `src/internal/adapters/repository/dictionary/`
- Validation: AC-002 (Save/Find/Delete) + AC-006 (profile загружается со словарём).

### DEC-003 DictionaryDetector получает Dictionary через конструктор

- Why: детектор не ходит в БД. Он — чистая функция: Dictionary in, text in, results out.
- Tradeoff: вызывающий код (main.go при сборке CompositeDetector) должен иметь загруженный Dictionary. Это нормально — профиль уже загружен.
- Affects: `src/internal/domain/shield/dictionary/detector.go`
- Validation: AC-003, AC-004, AC-005.

### DEC-004 Aho-Corasick реализуется встроенным кодом, без external dependency

- Why: spec требует. Алгоритм устоявшийся, реализация занимает ~100 строк.
- Tradeoff: нет proven production implementation; нужно покрыть тестами краевые случаи.
- Affects: `src/internal/domain/shield/dictionary/wordlist.go`
- Validation: AC-008 (множественные совпадения, перекрытие).

### DEC-005 Profile entity получает поле dictionaries через WithDictionaries option

- Why: минимальное изменение Profile — не breaking, сохраняет существующий API. nil = нет словаря.
- Tradeoff: Profile хранит слайс словарей, хотя spec допускает один. Готовность к будущему расширению.
- Affects: `src/internal/domain/shield/entity/profile.go`
- Validation: AC-006.

## Incremental Delivery

### MVP (Первая ценность)

Dictionary value object + DictionaryRepository (in-memory) + exact match DictionaryDetector + загрузка с профилем + регистрация в DetectorRegistry.
AC-001, AC-002, AC-003, AC-006, AC-007.
Validation: `go test ./src/internal/domain/shield/dictionary/...` + `go test ./src/internal/domain/shield/...`

### Итеративное расширение 1

Contains match через Aho-Corasick (WordlistMatcher) + DictionaryDetector support.
AC-004, AC-008.
Validation: `go test ./src/internal/domain/shield/dictionary/...` (wordlist + detector contains)

### Итеративное расширение 2

Regex match + error handling для невалидных regex.
AC-005.
Validation: `go test ./src/internal/domain/shield/dictionary/...` (detector regex)

### Итеративное расширение 3

Fuzzy match (Levenshtein) + Postgres adapter.
AC-002 (with DB), AC-002 расширение, validation с интеграционным тестом.

## Порядок реализации

1. Dictionary + MatchMode value objects — нет зависимостей
2. WordlistMatcher (Aho-Corasick) — чистый алгоритм, без зависимостей от других слоёв
3. DictionaryRepository interface + in-memory impl для тестов
4. DictionaryDetector (exact → contains → regex → fuzzy)
5. Миграция `002_dictionary_entries.sql`
6. PostgresDictionaryRepo adapter
7. Profile entity — WithDictionaries option
8. ProfileRepository adapter — композиция DictionaryRepository
9. DetectorType константа + регистрация в main.go
10. Тесты всех AC

Шаги 1-4 и 5 можно параллелить после шага 1. Шаги 7-8 завязаны на 3 и 5.

## Риски

- **Aho-Corasick реализация**: алгоритм требует аккуратной обработки failure links и overlap.
  Mitigation: покрыть WordlistMatcher unit-тестами (AC-008) с разными кейсами (пересечения, вложенность, повторы).
- **Зависимость от ProfileRepository adapter**: изменение существующего адаптера может сломать существующую логику загрузки профиля.
  Mitigation: dictionaries загружаются опционально — отсутствие таблицы/записей не ошибка, а пустой словарь.
- **JSONB колонка для entries**: размер entries может быть большим.
  Mitigation: ограничение на уровне приложения (max entries = 10000, max entry length = 1KB) — уточнить при реализации.

## Rollout and compatibility

- Миграция `002_dictionary_entries.sql` идемпотентна (IF NOT EXISTS).
- Profile без словарей продолжает работать — поле `dictionaries` nil.
- Регистрация DictionaryDetector опциональна — если DetectorType не зарегистрирован, CompoundDetector просто не использует его.
- Feature flag не требуется — новая функциональность, не меняющая существующее поведение.
- Специальных rollout-действий не требуется.

## Проверка

- `go test ./src/internal/domain/shield/dictionary/...` — AC-001, AC-003, AC-004, AC-005, AC-007, AC-008
- `go test ./src/internal/domain/shield/...` — AC-001, AC-002 (in-memory репозиторий)
- `go test ./src/internal/adapters/repository/...` — AC-002 (DB), AC-006
- `go vet ./...` — отсутствие циклических зависимостей
- `go build ./...` — успешная компиляция

## Соответствие конституции

- нет конфликтов
