# Profiles API План

## Phase Contract

Inputs: spec, inspect (pass), repo-контекст (domain entity, repository, server, middleware).
Outputs: plan, data-model stub.
Stop if: spec ambiguity или multi-feature creep.

## Цель

Реализовать Gin handler group для CRUD профилей (+ PATCH dictionary) в новом подпакете `src/internal/api/handler/profile/`, используя существующие domain entity, repository и DTO-слой с валидацией через go-playground/validator. Error handling — единый middleware, формат `{"error","code","details"}`.

## MVP Slice

Весь spec — неделимая фича. Все 11 AC должны быть закрыты в одном implementation pass. Порядок: DTO → middleware → handler (Create → List → Get → Update → Delete → Patch) → регистрация → тесты.

## First Validation Path

Запустить `go test ./src/internal/api/...` и integration test, проходящий полный lifecycle: POST create → GET list → GET by slug → PUT update → PATCH dictionary → DELETE → GET 404.

## Scope

- `src/internal/api/handler/profile/` — новый пакет с 5 Gin handler funcs
- `src/internal/api/dto/profile.go` — DTO request/response типы
- `src/internal/api/middleware/errors.go` — error response middleware (или helper)
- `src/internal/api/server.go` — добавление `RegisterProfileHandler`
- Тесты: handler unit tests + интеграционный lifecycle-тест
- Dictionaries/preprocessors endpoint'ов — НЕТ (только через профиль)

## Performance Budget

- `none` — профили управляются оператором, RPS низкий, latency не критична

## Implementation Surfaces

| Surface | Status | Почему |
|---------|--------|--------|
| `src/internal/api/handler/profile/` | NEW | Изоляция handler'ов профилей; spec требует отдельный пакет |
| `src/internal/api/dto/profile.go` | NEW | Request/response DTO с тегами validator |
| `src/internal/api/middleware/errors.go` | NEW | Единый формат ошибок |
| `src/internal/api/server.go` | EXISTING | Добавить метод `RegisterProfileHandler` |
| `src/internal/domain/shield/repository.go` | EXISTING | `ProfileRepository` уже есть, не меняется |
| `src/internal/domain/shield/entity/profile.go` | EXISTING | Profile entity не меняется |
| `src/internal/domain/shield/dictionary/dictionary.go` | EXISTING | Dictionary не меняется |

## Bootstrapping Surfaces

- `src/internal/api/handler/profile/` — создать директорию
- `src/internal/api/dto/` — создать директорию (если нет)
- `src/internal/api/middleware/errors.go` — создать файл

## Влияние на архитектуру

- Локальное: новый handler-пакет по домену (profile) задаёт шаблон для будущих handler-групп
- Нет изменений domain/adapters/infra слоёв
- Нет новых зависимостей (validator уже в go.mod)

## Acceptance Approach

- AC-001 (Create): POST /api/v1/profiles с полным payload → 201 + полная структура
- AC-002 (Duplicate): POST с существующим slug → 409 + SLUG_CONFLICT
- AC-003 (Validation): POST с пустым name → 400 + VALIDATION_ERROR + details
- AC-004 (List): GET /api/v1/profiles → 200 + массив {slug, name, status}
- AC-005 (GetBySlug): GET /api/v1/profiles/:slug → 200 + полная структура
- AC-006 (NotFound): GET /api/v1/profiles/nonexistent → 404 + NOT_FOUND
- AC-007 (Update): PUT /api/v1/profiles/:slug → 200 + обновлённая структура
- AC-008 (Delete): DELETE /api/v1/profiles/:slug → 204 + каскад
- AC-009 (PatchAdd): PATCH dictionary action=add → 200 + entries обновлены
- AC-010 (PatchRemove): PATCH dictionary action=remove → 200 + entries удалены
- AC-011 (ErrorFormat): все 4xx/5xx → `{"error","code"}` + опционально `details`

Все AC наблюдаются через HTTP response status/body.

## Данные и контракты

- Data model domain не меняется — `data-model.md` со статусом `no-change`
- API контракты: специфицированы в spec как request/response форматы
- Миграции БД не нужны — существующая схема (profiles, dictionary_entries) покрывает

## Стратегия реализации

- DEC-001 Пакет handler'ов: `handler/profile` вместо flat `api`
  Why: spec явно требует `src/internal/api/handler/profile/` для изоляции; существующий MaskHandler в `api` — legacy, новая группа не должна повторять flat-структуру.
  Tradeoff: небольшой boilerplate на импорт/экспорт; handler'ы не имеют доступа к private API пакета.
  Affects: `src/internal/api/handler/profile/`, `src/internal/api/server.go`
  Validation: handler'ы регистрируются через `RegisterProfileHandler` и работают из `server_test.go`

- DEC-002 Error middleware — отдельный файл в `middleware/errors.go`
  Why: единый формат ошибок (RQ-010) должен применяться ко всем handler'ам, а не дублироваться в каждом. Gin middleware подходит для перехвата abort'ов и формирования `{"error","code","details"}`.
  Tradeoff: middleware не поймает ошибки до его установки; нужно ставить после Recovery/CORS.
  Affects: `src/internal/api/middleware/errors.go`, `src/internal/api/server.go` (порядок middleware)
  Validation: AC-011 — любой 4xx/5xx ответ содержит error и code.

- DEC-003 Unique slug pre-check в handler
  Why: `PostgresProfileRepo.Save` — upsert, а не строгий create. Handler должен вызвать `FindBySlug` перед create и вернуть 409 если найден.
  Tradeoff: дополнительный SELECT перед INSERT; acceptable для admin API.
  Affects: handler CreateProfile
  Validation: AC-002 — 409 при дубликате.

- DEC-004 PATCH dictionary через handler логику (не middleware)
  Why: операция специфична для профиля — чтение, модификация entries, сохранение. Требует обращения к repository, не может быть middleware.
  Tradeoff: handler содержит бизнес-логику модификации entries внутри profile.
  Affects: handler PatchDictionary
  Validation: AC-009, AC-010.

- DEC-005 DTO-слой в отдельном пакете `dto`
  Why: spec требует `src/internal/api/dto/profile.go`. Отделяем request/response типы от handler'ов для переиспользования в тестах и документации.
  Tradeoff: дополнительный пакет с циклическими импортами, если domain типы использовать напрямую в DTO.
  Affects: `src/internal/api/dto/profile.go`
  Validation: handler'ы принимают/возвращают DTO, сериализуются корректно.

## Incremental Delivery

### MVP (Первая ценность)

Все 11 AC — единый delivery. Разбивка на задачи внутри implementation фазы:

1. DTO и error middleware (bootstrapping)
2. Base handler (Create + Лист + GetBySlug)
3. Update + Delete
4. PATCH dictionary
5. Route registration + интеграционные тесты

### Итеративное расширение

- `none` — фича неделимая

## Порядок реализации

1. `middleware/errors.go` — без него handler'ы не смогут выдавать единый формат
2. `dto/profile.go` — DTO нужны handler'ам
3. `handler/profile/` — Create, List, Get (базовые GET)
4. `handler/profile/` — Update, Delete
5. `handler/profile/` — PatchDictionary
6. `server.go` — RegisterProfileHandler + middleware
7. Тесты (unit + integration lifecycle)

Параллельно: 1+2, 3+4+5, 6+7.

## Риски

- **Риск: tenantID из контекста** — spec допускает, что middleware tenant extraction существует. Если её нет — handler'ам неоткуда взять tenantID.
  Mitigation: plan не включает tenant middleware; handler'ы используют fallback tenantID (hardcoded "default") для MVP. Если tenant middleware появится позже — замена тривиальна.

- **Риск: PATCH dictionary при отсутствующем словаре** — в текущей модели 1 словарь на профиль. PATCH action=add без существующего словаря должен создать новый.
  Mitigation: handler проверяет `FindByProfileSlug`, если nil — создаёт новый Dictionary при action=add, 404 при action=remove.

- **Риск: validator теги** — go-playground/validator требует теги на struct fields.
  Mitigation: DTO structs получают `validate` теги; ошибки валидации маппятся в `details` формат.

## Rollout and compatibility

- Новые endpoint'ы не меняют существующие (/api/v1/shield/mask, /health)
- Нет migration/feature flag/rollback сценария — API добавляется, старые не меняются
- После деплоя: проверить lifecycle вручную через curl

## Проверка

- Unit tests: каждый handler (с mock repository) — 1 test per AC
- Integration test: полный lifecycle через `httptest.NewRecorder` + `Server` (без запуска БД — mock repository)
- `go test ./src/internal/api/...` — все тесты проходят
- AC-011: проверяется в каждом handler тесте через assert на error format

## Соответствие конституции

- нет конфликтов
