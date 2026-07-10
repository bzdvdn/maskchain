# Shield Dictionaries

## Scope Snapshot

- In scope: словари — именованные списки строк (ключ + entries), привязанные к профилю, с детекцией текста против словарей (exact/contains/regex/fuzzy match)
- Out of scope: standalone API/UI для управления словарями; импорт/экспорт словарей; fuzzy-match engine (порог и метрика)

## Цель

Оператор получает возможность создавать именованные списки значений (запрещённые имена, коды, домены, IP) в контексте профиля Content Shield и детектировать их в трафике без написания кода или regex для каждого элемента. Успех определяется появлением словарей в форме профиля и срабатыванием DictionaryDetector на содержимом словарей.

## Основной сценарий

1. Администратор создаёт/редактирует профиль и добавляет словарь — задаёт имя, список строк и режим сопоставления.
2. При применении профиля словарь сохраняется в таблицу `dictionary_entries`, привязанную к profile_slug.
3. При обработке запроса DictionaryDetector сканирует текст против entries выбранным методом (exact HashSet, contains Aho-Corasick, regex, fuzzy).
4. Результат детекции возвращается как `[]DetectorResult` с указанием совпавшего фрагмента.
5. Ошибка: если entry не компилируется (regex/fuzzy) — entry пропускается, детектор логирует и продолжает.

## User Stories

- P1 Story: как администратор, я хочу задать список запрещённых имён для профиля в режиме exact match, чтобы блокировать их в трафике.
- P2 Story: как администратор, я хочу использовать contains match для поиска подстрок (домены, IP) и regex для гибких шаблонов.

## MVP Slice

Словарь с exact match (HashSet), CRUD через DictionaryRepository, интеграция с ProfileRepository (загрузка словарей вместе с профилем). AC-001, AC-002, AC-003, AC-007.

## First Deployable Outcome

После первого implementation pass: объект Dictionary создаётся через repository, DictionaryDetector матчит exact-совпадения, результат доступен через Profile (словари загружены). Можно проверить unit-тестами.

## Scope

- Domain: Dictionary как ValueObject (ProfileSlug, Name, Entries, MatchMode) в пакете `src/internal/domain/shield/dictionary/`
- Domain: MatchMode (exact | contains | regex | fuzzy), каждый режим с отдельной стратегией матчинга
- Domain: DictionaryRepository — CRUD по ProfileSlug
- Domain: DictionaryDetector — реализация Detector из `detector/detector.go`, регистрируется в DetectorRegistry
- Domain: WordlistMatcher — утилита для мульти-паттерн поиска (Aho-Corasick для contains)
- DB: миграция для таблицы `dictionary_entries` (profile_slug, entries[])
- Integration: расширение ProfileRepository — загрузка словарей вместе с профилем
- No standalone API/UI: управление через inline entries в форме профиля (React UI — PostMVP)

## Контекст

- Dictionary привязан к Profile через ProfileSlug и не существует вне контекста профиля
- ProfileRepository.FindBySlug и FindByID должны возвращать профиль со словарями
- DetectorType "dictionary" будет добавлен в enum entity.DetectorType
- Aho-Corasick реализация не должна требовать внешних зависимостей (чистый Go)
- Fuzzy-match: порог и метрика не фиксируются в spec (определяются в plan)

## Зависимости

- Зависит от: `Detector` interface и `DetectorRegistry` из `src/internal/domain/shield/detector/`
- Зависит от: `ProfileRepository` из `src/internal/domain/shield/repository.go`
- Зависит от: `entity.Profile`, `value.ProfileSlug` из `src/internal/domain/shield/entity/`
- Внешних зависимостей нет

## Требования

- RQ-001 Система ДОЛЖНА поддерживать Dictionary как ValueObject с полями ProfileSlug, Name, Entries []string, MatchMode
- RQ-002 Система ДОЛЖНА поддерживать MatchMode: exact (HashSet O(1)), contains (Aho-Corasick), regex, fuzzy
- RQ-003 Система ДОЛЖНА предоставлять DictionaryRepository с CRUD-операциями по ProfileSlug
- RQ-004 Система ДОЛЖНА предоставлять DictionaryDetector, реализующий Detector.Scan для всех MatchMode
- RQ-005 Система ДОЛЖНА загружать словари профиля при вызове ProfileRepository (FindBySlug, FindByID)
- RQ-006 Система ДОЛЖНА хранить словари в таблице `dictionary_entries` (profile_slug → entries[])
- RQ-007 Система ДОЛЖНА предоставлять WordlistMatcher для мульти-паттерн поиска (Aho-Corasick)
- RQ-008 Словари НЕ ДОЛЖНЫ иметь standalone API или UI — управление через inline entries профиля

## Вне scope

- Standalone API/UI для словарей (REST-эндпоинты, отдельная страница)
- Импорт/экспорт словарей (CSV, JSON)
- Fuzzy-match engine (порог схожести, метрика Levenshtein/Jaro-Winkler — определяется в plan)
- Кэширование словарей (Valkey — PostMVP, если потребуется)
- Разделение словарей по tenant (привязка через профиль, tenant контекст профиля)

## Критерии приемки

### AC-001 Dictionary ValueObject создаётся с корректными полями

- Почему это важно: без ValueObject невозможно моделировать словарь
- **Given** профиль существует
- **When** создаётся Dictionary с ProfileSlug, Name, Entries, MatchMode
- **Then** все поля Dictionary доступны через геттеры; Name не пуст; Entries не nil
- Evidence: unit test создаёт Dictionary и читает поля

### AC-002 DictionaryRepository сохраняет и возвращает словари по ProfileSlug

- Почему это важно: CRUD — основа управления словарями
- **Given** пустая таблица `dictionary_entries`
- **When** словарь сохранён через DictionaryRepository, затем найден по ProfileSlug
- **Then** возвращается тот же словарь с теми же Entries и MatchMode
- Evidence: unit test (in-memory impl) или integration test (DB) подтверждает Save/Find/Delete

### AC-003 DictionaryDetector находит exact-совпадения

- Почему это важно: базовый use case — точная блокировка по списку
- **Given** Dictionary с entries ["secret", "admin"] и MatchMode=exact
- **When** DictionaryDetector.Scan вызывается с текстом "the admin password is secret"
- **Then** возвращается 2 результата: "admin" и "secret" с корректными StartPos/EndPos
- Evidence: unit test с in-memory DictionaryRepository

### AC-004 DictionaryDetector находит contains-совпадения через Aho-Corasick

- Почему это важно: поиск подстрок без составления точного списка
- **Given** Dictionary с entries ["example.com", "test"] и MatchMode=contains
- **When** DictionaryDetector.Scan вызывается с текстом "visit sub.example.com for test"
- **Then** возвращаются совпадения "example.com" и "test" в правильных позициях
- Evidence: unit test c WordlistMatcher

### AC-005 DictionaryDetector применяет regex-совпадения

- Почему это важно: гибкие шаблоны для доменов, IP, нестандартных форматов
- **Given** Dictionary с entries ["^(192\.168\.)\d+\.\d+$", "admin.*"] и MatchMode=regex
- **When** DictionaryDetector.Scan вызывается с текстом "192.168.1.1 and admin_test"
- **Then** возвращаются совпадения по обоим regex; невалидный regex логируется и пропускается
- Evidence: unit test подтверждает корректные совпадения и graceful skip кривого regex

### AC-006 Словари загружаются вместе с профилем через ProfileRepository

- Почему это важно: словарь не существует вне контекста профиля
- **Given** профиль с сохранённым словарём в таблице
- **When** ProfileRepository.FindBySlug возвращает профиль
- **Then** профиль содержит словарь с корректными Entries и MatchMode
- Evidence: integration test или unit test через mock ProfileRepository

### AC-007 Dictionary регистрируется в DetectorRegistry под DetectorType "dictionary"

- Почему это важно: детектор должен быть доступен через стандартный registry
- **Given** DetectorRegistry
- **When** DictionaryDetector зарегистрирован с типом "dictionary"
- **Then** DetectorRegistry.Get("dictionary") возвращает DictionaryDetector
- Evidence: unit test регистрирует и получает детектор

### AC-008 WordlistMatcher с Aho-Corasick строит автомат и возвращает совпадения

- Почему это важно: contains-режим требует эффективного мульти-паттерн поиска
- **Given** WordlistMatcher с entries ["foo", "bar"]
- **When** вызывается Match("foo and bar and foo")
- **Then** возвращаются 3 совпадения: "foo" (0,3), "bar" (8,11), "foo" (16,19)
- Evidence: unit test подтверждает все позиции и множественные совпадения одной entry

## Допущения

- DictionaryEntries хранятся как массив строк в JSONB-колонке PostgreSQL
- fuzzy-режим: метрика Levenshtein, порог 0.8 (нормализованная схожесть 1 − distance / max(|a|,|b|))
- Aho-Corasick реализуется встроенным кодом, без внешних библиотек
- Aho-Corasick автомат строится один раз при создании DictionaryDetector (lazy, при первом Scan), не перестраивается на каждый вызов
- Один профиль = один словарь. Если нужны несколько именованных списков — entries группируются соглашением именования внутри одного Dictionary

## Критерии успеха

- SC-001 Exact-детекция для словаря из 1000 entries занимает <1ms на 10KB текста
- SC-002 Aho-Corasick contains-детекция для 100 паттернов занимает <5ms на 100KB текста

## Краевые случаи

- Пустой словарь (Entries = []): детектор возвращает пустой результат
- Entry-дубликаты: exact match дедуплицирует через HashSet; contains/regex учитывает каждое совпадение
- Невалидный regex: entry логируется и пропускается, детектор продолжает с остальными
- Dictionary с ProfileSlug несуществующего профиля: DictionaryRepository возвращает пустой список (не ошибку)
- Очень длинные entries ( > 1MB): не специфицировано, ожидается обсуждение в реализации

## Открытые вопросы

- none
