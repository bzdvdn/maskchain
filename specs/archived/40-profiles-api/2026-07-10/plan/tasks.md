# Profiles API Задачи

## Phase Contract

Inputs: plan, data-model (no-change), spec, repo-контекст.
Outputs: упорядоченные исполнимые задачи с покрытием 11 AC.
Stop if: задачи расплывчаты или AC без покрытия.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/api/dto/profile.go` | T1.1 |
| `src/internal/api/middleware/errors.go` | T1.1 |
| `src/internal/api/handler/profile/` | T2.1, T2.2, T3.1, T3.2 |
| `src/internal/api/server.go` | T4.1 |
| `src/internal/api/handler/profile/handler_test.go` | T4.2 |

## Implementation Context

- Цель MVP: Gin handler group `/api/v1/profiles` с CRUD + PATCH dictionary, единый формат ошибок
- Инварианты/семантика:
  - Tenant ID из контекста (fallback `"default"` при отсутствии) — DEC-003
  - Slug unique pre-check: SELECT before INSERT — DEC-004
  - PATCH dictionary: action=add создаёт словарь при отсутствии, action=remove → 404 при отсутствии
- Ошибки/коды:
  - `SLUG_CONFLICT` (409) — дубликат slug
  - `VALIDATION_ERROR` (400) — невалидные поля + `details: [{"field":"...","message":"..."}]`
  - `NOT_FOUND` (404) — slug не найден
  - Формат: `{"error":"...", "code":"...", "details": [...]}`
- Контракты/протокол:
  - `POST /api/v1/profiles` → 201 + `ProfileResponse`
  - `GET /api/v1/profiles` → 200 + `[ProfileListItem]`
  - `GET /api/v1/profiles/:slug` → 200 + `ProfileResponse`
  - `PUT /api/v1/profiles/:slug` → 200 + `ProfileResponse`
  - `DELETE /api/v1/profiles/:slug` → 204
  - `PATCH /api/v1/profiles/:slug/dictionary` → 200 + `ProfileResponse`
- Границы scope:
  - Не делаем tenant extraction middleware (ожидается из контекста)
  - Не делаем отдельные endpoint'ы для dictionaries/preprocessors
- Proof signals:
  - `go test ./src/internal/api/...` проходит
  - Полный lifecycle (create → list → get → update → patch → delete) работает через httptest
- References: DEC-001..005 из plan.md, RQ-001..010, AC-001..011

## Фаза 1: Основа

Цель: подготовить DTO, error middleware и handler scaffold — без них ни один handler не работает.

- [x] T1.1 Создать DTO и error middleware
  - Файл `src/internal/api/dto/profile.go`: `CreateProfileRequest`, `UpdateProfileRequest`, `ProfileResponse`, `ProfileListItem`, `PatchDictionaryRequest`, `ErrorResponse` с тегами `json` и `validate`
  - Файл `src/internal/api/middleware/errors.go`: middleware-функция, перехватывающая aborted requests и формирующая `{"error","code","details"}`
  - Touches: `src/internal/api/dto/profile.go`, `src/internal/api/middleware/errors.go`

- [x] T1.2 Создать handler package scaffold
  - Директория `src/internal/api/handler/profile/` с файлом `handler.go`, содержащим struct `ProfileHandler` с полем `repo shield.ProfileRepository`
  - Touches: `src/internal/api/handler/profile/handler.go`

## Фаза 2: MVP Slice

Цель: реализовать базовые handler'ы — Create, List, GetBySlug (чтение + создание).

- [x] T2.1 Implement CreateProfile handler
  - POST /api/v1/profiles: валидация DTO, unique slug pre-check (FindBySlug), создание Profile entity + Dictionary + сохранение через repo, возврат ProfileResponse 201
  - Touches: `src/internal/api/handler/profile/handler.go`
  - AC: AC-001, AC-002, AC-003, AC-011

- [x] T2.2 Implement ListProfiles и GetProfile handlers
  - GET /api/v1/profiles: ListByTenant → массив ProfileListItem (slug, name, status)
  - GET /api/v1/profiles/:slug: FindBySlug → ProfileResponse, 404 если nil
  - Touches: `src/internal/api/handler/profile/handler.go`
  - AC: AC-004, AC-005, AC-006, AC-011

## Фаза 3: Основная реализация

Цель: реализовать Update, Delete, PatchDictionary (модификация + удаление).

- [x] T3.1 Implement UpdateProfile и DeleteProfile handlers
  - PUT /api/v1/profiles/:slug: FindBySlug → 404 если нет, полная перезапись полей (включая dictionaries, preprocessors), сохранение, возврат ProfileResponse 200
  - DELETE /api/v1/profiles/:slug: FindBySlug → 404 если нет, repo.Delete (каскад), 204
  - Touches: `src/internal/api/handler/profile/handler.go`
  - AC: AC-007, AC-008, AC-011

- [x] T3.2 Implement PatchDictionary handler
  - PATCH /api/v1/profiles/:slug/dictionary: FindBySlug → 404 если нет профиля; action=add: создать/дополнить entries; action=remove: удалить entries из словаря; 404 если словаря нет при remove
  - Touches: `src/internal/api/handler/profile/handler.go`
  - AC: AC-009, AC-010, AC-011

## Фаза 4: Проверка

Цель: зарегистрировать handler'ы и покрыть тестами.

- [x] T4.1 Зарегистрировать профильные handler'ы в Server
  - Добавить метод `RegisterProfileHandler(h *profile.ProfileHandler)` в `src/internal/api/server.go`
  - Подключить error middleware в цепочку Server.New
  - Touches: `src/internal/api/server.go`

- [x] T4.2 Добавить тесты для всех handler'ов
  - Unit tests: каждый handler с mock repository — покрытие всех 11 AC
  - Integration test: полный lifecycle через httptest + mock репозиторий
  - Touches: `src/internal/api/handler/profile/handler_test.go`
  - AC: все (AC-001..AC-011)

## Покрытие критериев приемки

- AC-001 -> T2.1, T4.2
- AC-002 -> T2.1, T4.2
- AC-003 -> T2.1, T4.2
- AC-004 -> T2.2, T4.2
- AC-005 -> T2.2, T4.2
- AC-006 -> T2.2, T4.2
- AC-007 -> T3.1, T4.2
- AC-008 -> T3.1, T4.2
- AC-009 -> T3.2, T4.2
- AC-010 -> T3.2, T4.2
- AC-011 -> T1.1, T2.1, T2.2, T3.1, T3.2, T4.2

## Заметки

- Все handler'ы используют mock repository в тестах — БД не требуется для unit-проверки
- Error middleware ставится после Recovery и CORS в `Server.New`
- tenantID fallback `"default"` используется до появления tenant extraction middleware
- Использовать `github.com/go-playground/validator/v10` для валидации DTO
