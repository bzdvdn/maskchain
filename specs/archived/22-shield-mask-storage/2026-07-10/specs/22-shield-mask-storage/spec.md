# Mask Storage — обратимое template-based замещение

## Scope Snapshot

- In scope: Хранение цепочек маскинга для обратимого template-based replacement. `/mask` сканирует текст детекторами, заменяет найденные фрагменты на `{{<mask_id>.<N>}}`, сохраняет отображение placeholder→original в PG + Valkey (write-through). `/unmask` загружает цепочки по mask_ids и восстанавливает оригинальный текст (read-through: Valkey first, PG fallback).

- Out of scope: Preprocessors (CSV/JSON); Presidio-интеграция; справочники профилей; audit-log; реакции block/review/alert; потоковое unmask (SSE).

## Цель

Разработчик AI gateway получает возможность обратимо маскировать sensitive-данные в промптах: на `/mask` детекторы находят PII/secrets/финансовые данные, заменяют на плейсхолдеры, сохраняют цепочку замещений; на `/unmask` плейсхолдеры восстанавливаются в оригиналы. Успех фичи измеряется round-trip тестом (mask → unmask возвращает оригинальный текст) и прохождением unit-тестов для use case и overlap-фильтрации.

## Основной сценарий

1. Клиент отправляет `POST /api/v1/shield/mask?mask_id=<uuid>` с телом-промптом.
2. UseCase последовательно вызывает все зарегистрированные детекторы (PII, secrets, financial) через `DetectorRegistry`.
3. Результаты сортируются: более длинные совпадения имеют приоритет; пересекающиеся короткие отфильтровываются.
4. Каждый найденный фрагмент заменяется на `{{<mask_id>.<N>}}`. Замена выполняется right-to-left, чтобы избежать смещения позиций.
5. Отображение `placeholder → original` сохраняется в PG (`mask_entries.repalcements JSONB`) и кэшируется в Valkey (`key: mask:<id>`, TTL конфигурируемый).
6. Если `mask_id` занят — `409 Conflict`.
7. Если `mask_id` не передан — генерируется UUIDv7 и возвращается в `X-Mask-ID`.
8. Клиент отправляет `POST /api/v1/shield/unmask?mask_ids=id1,id2` с телом, содержащим плейсхолдеры.
9. UseCase загружает цепочки (Valkey first, PG fallback → refresh cache), выполняет `strings.ReplaceAll` для каждого placeholder.
10. Возвращается восстановленный текст.

## User Stories

- P1 Story: Разработчик может отправить промпт на `/mask`, получить текст с `{{<mask_id>.<N>}}` плейсхолдерами и сохранить цепочку в PG+Valkey.
- P1 Story: Разработчик может отправить ответ модели с плейсхолдерами на `/unmask` и получить восстановленный оригинальный текст.
- P2 Story: Разработчик может передать свой `mask_id`; если ID занят — получает 409.
- P2 Story: Система кэширует цепочки в Valkey и при `/unmask` читает из кэша, падая на PG только при промахе.

## MVP Slice

MaskUseCase с MaskText/UnmaskText + in-memory storage + тесты round-trip и overlap-фильтрации. Покрывает AC-001–AC-007.

## First Deployable Outcome

После первого implementation pass можно:
- Скомпилировать весь проект без ошибок
- Запустить `go test ./src/internal/domain/shield/mask/...` и получить зелёные тесты для MaskUseCase (round-trip, overlap, conflict)
- Продемонстрировать, что mask → unmask идентичен оригиналу

## Scope

- `src/internal/domain/shield/mask/` — MaskEntry entity, MaskStorage interface, MaskUseCase, UUIDv7
- `src/internal/adapters/repository/mask/` — PostgresMaskRepo, ValkeyMaskRepo, CachedMaskRepo
- `src/internal/api/mask_handler.go` — HTTP handlers для `/mask` и `/unmask`
- `src/internal/api/server.go` — RegisterMaskHandler method
- `src/internal/infra/config/config.go` — DatabaseConfig, ValkeyConfig, MaskConfig
- `src/cmd/gateway/main.go` — DI всех компонентов
- `src/internal/domain/shield/detector/composite.go` — CompositeDetector
- `deployments/migrations/001_mask_entries.sql` — таблица mask_entries

## Контекст

- Detector-пакет уже реализован (21-shield-detectors): 4 детектора возвращают `DetectorResult` с позициями и фрагментами.
- Registry (`DetectorRegistry`) умеет регистрировать и получать детекторы — но keyed by `entity.DetectorType`, а все regex-детекторы одного типа. Для mask use case добавлен `CompositeDetector`, позволяющий объединить несколько детекторов под одним ключом.
- Текущий ScanPipeline (`service.ScanPipeline`) не умеет модифицировать текст — MaskUseCase работает напрямую с registry для получения позиций и замены.
- Маскинг обратимый: placeholder → original сохраняется, а не выбрасывается.
- PG + Valkey — внешние зависимости, но код спроектирован так, что оба могут быть nil (no-op).

## Зависимости

- Зависит от `detector.DetectorRegistry` и `detector.DetectorResult` из `21-shield-detectors`
- Внешние библиотеки: `github.com/jackc/pgx/v5`, `github.com/valkey-io/valkey-go`
- `deployments/migrations/001_mask_entries.sql` — DDL для PG

## Требования

- RQ-001 Система ДОЛЖНА предоставлять MaskStorage interface с методами Save, Get, Delete
- RQ-002 MaskEntry ДОЛЖЕН содержать mask_id (string, UUIDv7), profile_id (опционально), замены (map[string]string), created_at
- RQ-003 MaskUseCase.MaskText ДОЛЖЕН выполнять все зарегистрированные детекторы, заменять фрагменты на `{{<mask_id>.<N>}}`, сохранять entry через MaskStorage
- RQ-004 MaskUseCase.UnmaskText ДОЛЖЕН загружать entry по mask_ids, мерджить замены, выполнять ReplaceAll для восстановления
- RQ-005 При пересечении совпадений ДОЛЖНО использоваться более длинное (longer match wins)
- RQ-006 PostgresMaskRepo ДОЛЖЕН использовать `ON CONFLICT DO NOTHING` и возвращать ErrMaskIDConflict при повторе mask_id
- RQ-007 ValkeyMaskRepo ДОЛЖЕН использовать ключ `mask:<id>` с конфигурируемым TTL
- RQ-008 CachedMaskRepo ДОЛЖЕН реализовывать write-through (PG→Valkey) и read-through (Valkey→PG fallback→refresh)
- RQ-009 HTTP handler ДОЛЖЕН принимать `POST /api/v1/shield/mask?mask_id=<uuid>` и `POST /api/v1/shield/unmask?mask_ids=id1,id2`
- RQ-010 UUIDv7 ДОЛЖЕН генерироваться системой, если mask_id не передан; возвращаться в заголовке X-Mask-ID
- RQ-011 All repos ДОЛЖНЫ корректно работать с nil pool/client (graceful degradation)
- RQ-012 Замена плейсхолдеров ДОЛЖНА выполняться без сохранения оригинального текста — хранится только отображение placeholder→original

## Вне scope

- Интеграция с Presidio — вынесено в отдельный spec
- Audit-log детекций и маскинга
- Профили справочников и preprocessors (CSV/JSON shield)
- Потоковый unmask (SSE/streaming)
- Реакции block/review/alert — только replace
- Batch-операции (массовое архивирование цепочек)
- Админка/UI для просмотра цепочек
- Semantic cache для LLM-ответов

## Критерии приемки

### AC-001 MaskStorage interface определён и реализуем

- Почему это важно: единый контракт для PG, Valkey и композитного репозитория.
- **Given** пустой модуль mask
- **When** определён MaskStorage с методами Save, Get, Delete
- **Then** все репозитории реализуют этот интерфейс
- **Evidence**: компиляция проход

### AC-002 MaskUseCase.MaskText сохраняет entry и возвращает masked text

- Почему это важно: ядро фичи — замена sensitive-данных на плейсхолдеры.
- **Given** текст "Hi test@example.com!" и mask_id "abc"
- **When** вызван MaskUseCase.MaskText с детектором, находящим email на [3,19)
- **Then** возвращается "Hi {{abc.1}}!" и entry.Replacements["{{abc.1}}"] == "test@example.com"
- **Evidence**: assertion в тесте

### AC-003 MaskUseCase.UnmaskText восстанавливает оригинальный текст

- Почему это важно: unmask — обратная операция, без которой маскинг бесполезен.
- **Given** запись с заменой "{{abc.1}}" → "test@example.com"
- **When** вызван UnmaskText с текстом "Hi {{abc.1}}!" и mask_ids=["abc"]
- **Then** возвращается "Hi test@example.com!"
- **Evidence**: assertion в тесте

### AC-004 Round-trip mask → unmask идентичен оригиналу

- Почему это важно: гарантия обратимости для downstream потребителя.
- **Given** оригинальный текст с несколькими sensitive-данными
- **When** выполнен MaskText → UnmaskText с теми же mask_ids
- **Then** восстановленный текст совпадает с оригиналом
- **Evidence**: тест round-trip

### AC-005 Пересекающиеся совпадения фильтруются, длинное побеждает

- Почему это важно: короткое совпадение не должно «разрезать» длинное, иначе unmask сломается.
- **Given** текст "john@example.com" и два совпадения: [0,16) "john@example.com" и [5,16) "example.com"
- **When** выполнен MaskText
- **Then** заменяется только "john@example.com", "example.com" отфильтровано как пересечение
- **Evidence**: тест OverlapFilter содержит 1 замену

### AC-006 Duplicate mask_id возвращает 409 Conflict

- Почему это важно: пользовательские mask_id должны быть уникальны.
- **Given** сохранённая запись с mask_id "dup"
- **When** выполнен MaskText с тем же mask_id "dup"
- **Then** возвращается ErrMaskIDConflict
- **Evidence**: errors.Is(err, ErrMaskIDConflict)

### AC-007 UUIDv7 генерируется и соответствует формату

- Почему это важно: глобально уникальные ID для цепочек маскинга.
- **Given** вызов NewUUIDv7
- **Then** возвращается строка 36 символов, версия 7 на позиции 14, variant 10xx на позиции 19
- **Evidence**: assertion в тесте

### AC-008 PostgresMaskRepo использует ON CONFLICT DO NOTHING

- Почему это важно: атомарная проверка уникальности mask_id на уровне БД.
- **Given** существующая запись в PG
- **When** выполнен Save с тем же mask_id
- **Then** tag.RowsAffected() == 0 → ErrMaskIDConflict
- **Evidence**: код использует `ON CONFLICT (mask_id) DO NOTHING` и проверяет RowsAffected

### AC-009 ValkeyMaskRepo использует TTL и префикс mask:

- Почему это важно: кэш не должен расти бесконечно; префикс предотвращает коллизии ключей.
- **Given** экземпляр ValkeyMaskRepo с TTL=3600
- **When** выполнен Save
- **Then** ключ `mask:<id>` установлен с TTL
- **Evidence**: код использует `Ex(r.ttl)` и key prefix

### AC-010 CachedMaskRepo реализует write-through и read-through

- Почему это важно: производительность чтения (Valkey first) и надёжность (PG fallback).
- **Given** CachedMaskRepo с primary=PG и secondary=Valkey
- **When** Save → пишет в PG, потом best-effort в Valkey
- **When** Get → читает Valkey, при промахе PG → refresh cache
- **Evidence**: последовательность вызовов в коде

### AC-011 CompositeDetector объединяет несколько детекторов

- Почему это важно: registry keyed by DetectorType, а все regex-детекторы одного типа.
- **Given** CompositeDetector с PIIDetector + SecretsDetector
- **When** выполнен Scan
- **Then** возвращаются объединённые результаты всех вложенных детекторов
- **Evidence**: код перебирает все детекторы и мерджит результаты

### AC-012 nil pool/client обрабатывается без паники

- Почему это важно: приложение должно стартовать без PG/Valkey.
- **Given** PostgresMaskRepo с pool=nil или ValkeyMaskRepo с client=nil
- **When** вызван Save/Get/Delete
- **Then** не паникует, возвращает ErrMaskNotFound при Get или no-op
- **Evidence**: nil-guards в коде

## Допущения

- Mask_id — UUIDv7 text, уникален глобально. User может передать свой (если занят — 409).
- Привязка к profile_id опциональна.
- Все детекторы уже зарегистрированы в registry при старте приложения.
- Overlap-фильтрация предпочитает более длинное совпадение (по длине фрагмента).
- Замена выполняется right-to-left, позиции стабильны.
- PG и Valkey могут быть недоступны — код не падает, а ведёт себя как no-op.
- Плейсхолдеры — уникальные строки; `strings.ReplaceAll` безопасен.

## Критерии успеха

- SC-001: `go test ./...` проходит без ошибок
- SC-002: `go vet ./...` чистый
- SC-003: `go build ./...` успешен

## Краевые случаи

- Пустой текст → пустая замена, entry с пустым replacements
- Нет детекторов в registry → текст не меняется
- mask_id уже существует → 409
- Пересекающиеся совпадения разной длины → longer wins
- Текст без sensitive-данных → возвращается как есть
- nil pool/client в репозиториях → no-op
- Пустой mask_ids в unmask → текст возвращается без изменений

## Открытые вопросы

- `none`
