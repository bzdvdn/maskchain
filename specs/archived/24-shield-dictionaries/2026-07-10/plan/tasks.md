# Shield Dictionaries — Задачи

## Phase Contract

Inputs: plan (`specs/active/24-shield-dictionaries/plan.md`), data-model (`specs/active/24-shield-dictionaries/data-model.md`).
Stop if: AC расплывчаты — нет, все 8 AC имеют конкретные подходы.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/domain/shield/dictionary/dictionary.go` | T1.1 |
| `src/internal/domain/shield/dictionary/match_mode.go` | T1.1 |
| `src/internal/domain/shield/dictionary/wordlist.go` | T1.2 |
| `src/internal/domain/shield/dictionary/repository.go` | T2.1 |
| `src/internal/domain/shield/detector/dictionary_detector.go` | T2.1, T3.1, T3.2, T3.3 |
| `src/internal/domain/shield/entity/detector_type.go` | T2.2 |
| `src/internal/domain/shield/entity/profile.go` | T2.2 |
| `src/internal/infra/migrations/002_dictionary_entries.sql` | T4.1 |
| `src/internal/adapters/repository/dictionary/postgres.go` | T4.2 |
| `src/internal/adapters/repository/profile/postgres.go` | T5.1 |
| `src/cmd/gateway/main.go` | T5.1 |
| `src/internal/domain/shield/dictionary/*_test.go` | T6.1 |
| `src/internal/domain/shield/detector/dictionary_detector_test.go` | T6.1 |

## Implementation Context

- Цель MVP: Dictionary + exact match DictionaryDetector + CRUD репозиторий + загрузка словаря с профилем
- Инварианты/семантика:
  - Dictionary — ValueObject (сравнивается по полям, не имеет id), ProfileSlug — уникальный ключ
  - Один профиль = один словарь; entries группируются соглашением именования
  - DictionaryDetector получает Dictionary через конструктор — не ходит в БД
  - MatchMode определяет стратегию: exact (HashSet), contains (Aho-Corasick), regex (re.Compile), fuzzy (Levenshtein)
  - Невалидный regex entry: логируется, entry пропускается, детектор продолжает
  - Пустой Dictionary (entries=[]) — детектор возвращает пустой результат
- Ошибки/коды:
  - DictionaryRepository: Save возвращает ошибку при nil-словаре; FindBySlug возвращает nil (не ошибку) если словаря нет
  - Detector.Scan: невалидный regex не вызывает ошибку Scan — только лог + пропуск entry
- Контракты/протокол:
  - DictionaryRepository: `Save(ctx, *Dictionary) error`, `FindByProfileSlug(ctx, ProfileSlug) (*Dictionary, error)`, `Delete(ctx, ProfileSlug) error`
  - DictionaryDetector: `NewDictionaryDetector(dict *Dictionary) *DictionaryDetector`, `Scan(ctx, text) ([]DetectorResult, error)`
  - Profile: `WithDictionaries(dicts []Dictionary) ProfileOption` — nil-безопасно
- Proof signals:
  - `go test ./src/internal/domain/shield/dictionary/...` — все AC пакета
  - `go test ./src/internal/...` — все AC фичи + без регрессий
  - `go vet ./...` — без циклических зависимостей
- Вне scope:
  - Standalone API/UI для словарей
  - Импорт/экспорт
  - Кэширование (Valkey)
  - Разделение по tenant (через профиль)

## Фаза 1: Domain foundation

Цель: Dictionary + MatchMode value objects и WordlistMatcher (Aho-Corasick) — чистые типы без зависимостей от других слоёв.

- [x] T1.1 Реализовать Dictionary (ProfileSlug, Name, Entries, MatchMode) и MatchMode (exact, contains, regex, fuzzy) в новом пакете `src/internal/domain/shield/dictionary/`. Dictionary — ValueObject с геттерами, MatchMode — отдельный тип с методом String(). Touches: `src/internal/domain/shield/dictionary/dictionary.go`, `src/internal/domain/shield/dictionary/match_mode.go`
- [x] T1.2 Реализовать WordlistMatcher с Aho-Corasick automaton: Build(entries) строит trie + failure links, Match(text) возвращает все совпадения `[]Match{Pattern, Start, End}`. Чистый Go, без external deps. Touches: `src/internal/domain/shield/dictionary/wordlist.go`

## Фаза 2: MVP slice

Цель: DictionaryRepository, DictionaryDetector (exact match), Profile extension, DetectorType — минимальная ценность.

- [x] T2.1 Реализовать DictionaryRepository interface (Save/FindByProfileSlug/Delete) + DictionaryDetector с exact match (HashSet lookup). DictionaryDetector принимает *Dictionary через конструктор, реализует Detector.Scan. Touches: `src/internal/domain/shield/dictionary/repository.go`, `src/internal/domain/shield/detector/dictionary_detector.go`
- [x] T2.2 Добавить DetectorTypeDictionary ("dictionary") в entity.DetectorType enum. Добавить в Profile поле dictionaries + WithDictionaries option. Touches: `src/internal/domain/shield/entity/detector_type.go`, `src/internal/domain/shield/entity/profile.go`

## Фаза 3: Расширение режимов детекции

Цель: contains, regex, fuzzy — полный набор MatchMode в DictionaryDetector.

- [x] T3.1 Добавить contains mode в DictionaryDetector — использует WordlistMatcher (Aho-Corasick) для поиска подстрок. Touches: `src/internal/domain/shield/detector/dictionary_detector.go`, `src/internal/domain/shield/dictionary/wordlist.go`
- [x] T3.2 Добавить regex mode в DictionaryDetector — каждый entry компилируется в regex; невалидный regex логируется и пропускается. Touches: `src/internal/domain/shield/detector/dictionary_detector.go`
- [x] T3.3 Добавить fuzzy mode в DictionaryDetector — метрика Levenshtein, порог 0.8 (нормализованная схожесть). Touches: `src/internal/domain/shield/detector/dictionary_detector.go`

## Фаза 4: Persistence

Цель: SQL migration + Postgres adapter для DictionaryRepository.

- [x] T4.1 Создать SQL migration `002_dictionary_entries.sql`: таблица dictionary_entries (profile_slug TEXT PK, name TEXT, entries JSONB, match_mode TEXT, created_at/updated_at TIMESTAMPTZ). Идемпотентный (IF NOT EXISTS). Touches: `src/internal/infra/migrations/002_dictionary_entries.sql`
- [x] T4.2 Реализовать PostgresDictionaryRepo (pgxpool) — CRUD операции через параметризованные запросы. Save — UPSERT (ON CONFLICT). Touches: `src/internal/adapters/repository/dictionary/postgres.go`

## Фаза 5: Integration

Цель: ProfileRepository adapter загружает словари, DI регистрирует DictionaryDetector.

- [x] T5.1 Реализовать PostgresProfileRepo — адаптер ProfileRepository, композирует DictionaryRepository для загрузки словарей при FindBySlug/FindByID. Отсутствие словаря = nil (не ошибка). Touches: `src/internal/adapters/repository/profile/postgres.go`, `src/cmd/gateway/main.go`

## Фаза 6: Проверка

Цель: полное тестовое покрытие + verify.

- [x] T6.1 Написать unit-тесты для Dictionary (AC-001), DictionaryRepository in-memory (AC-002), DictionaryDetector exact/contains/regex/fuzzy (AC-003/004/005), WordlistMatcher (AC-008), registry registration (AC-007). Тесты на Profile загрузку со словарём (AC-006) — через mock. Touches: `src/internal/domain/shield/dictionary/dictionary_test.go`, `src/internal/domain/shield/dictionary/repository_test.go`, `src/internal/domain/shield/dictionary/wordlist_test.go`, `src/internal/domain/shield/detector/dictionary_detector_test.go`
- [x] T6.2 Выполнить `go vet ./...`, `go build ./...`, `go test ./src/internal/...`. Touches: CI/no file changes

## Покрытие критериев приемки

- AC-001 -> T1.1, T6.1
- AC-002 -> T2.1, T4.2, T6.1
- AC-003 -> T2.1, T6.1
- AC-004 -> T3.1, T6.1
- AC-005 -> T3.2, T6.1
- AC-006 -> T5.1, T6.1
- AC-007 -> T2.2, T6.1
- AC-008 -> T1.2, T6.1

## Заметки

- T1.1 и T1.2 независимы — можно параллелить
- T2.1 зависит от T1.1 (нужен Dictionary)
- T3.1 зависит от T1.2 (нужен WordlistMatcher)
- T4.2 зависит от T2.1 (нужен DictionaryRepository interface)
- T5.1 зависит от T2.1, T2.2, T4.1 (нужен DictionaryRepository + Profile + migration)
- T6.1 — после всех остальных
