# Profiles API

## Scope Snapshot

- In scope: REST API to manage Content Shield profiles (CRUD + inline dictionaries and preprocessors management), exposed as Gin handler group under `/api/v1/profiles`.
- Out of scope: standalone dictionaries/preprocessors endpoints, React UI integration, bulk import/export, versioning/history, tenant isolation wiring (tenant ID extracted from context — existing mechanism).

## Цель

Operator или React UI (в будущем) получает полный CRUD над профилями Content Shield: создание, чтение, обновление, удаление, а также точечное добавление/удаление записей словаря внутри профиля. Dictionaries и preprocessors передаются inline в теле профиля — отдельные endpoint'ы для них не предусмотрены. Успех определяется возможностью выполнить полный lifecycle профиля через REST без прямого доступа к PostgreSQL.

## Основной сценарий

1. Оператор отправляет `POST /api/v1/profiles` с телом, содержащим slug, name, dictionaries и preprocessors inline.
2. Система валидирует запрос (включая уникальность slug), сохраняет профиль в PostgreSQL, возвращает 201 с полной структурой.
3. Оператор получает список профилей (`GET /api/v1/profiles`) — краткая форма (slug, name, status).
4. Оператор получает профиль по slug (`GET /api/v1/profiles/:slug`) — полная структура со словарями и препроцессорами.
5. Оператор обновляет профиль (`PUT /api/v1/profiles/:slug`) — перезапись всех полей, включая dictionaries и preprocessors.
6. Оператор добавляет/удаляет запись словаря (`PATCH /api/v1/profiles/:slug/dictionary`) — без перезаписи всего профиля.
7. Оператор удаляет профиль (`DELETE /api/v1/profiles/:slug`) — каскадное удаление словарей.
8. При любой ошибке валидации или бизнес-логики возвращается единый формат ошибки.

## User Stories

- P1 Story: Как оператор, я хочу создавать, читать, обновлять и удалять профили Content Shield через REST API, чтобы управлять политиками безопасности без доступа к БД.
- P2 Story: Как оператор, я хочу добавлять/удалять отдельные entry словаря без перезаписи всего профиля, чтобы точечно корректировать политики.

## MVP Slice

Полный CRUD (`Create`, `List`, `GetBySlug`, `Update`, `Delete`) + `PATCH dictionary`. Все AC-* обязательны для MVP — фича неделимая.

## First Deployable Outcome

Запущенный gateway, к которому можно последовательно выполнить: create → list → get → update → patch dictionary → delete, наблюдая корректные HTTP статусы и тела ответов.

## Scope

- Gin handler group `src/internal/api/handler/profile/` — CreateProfile, GetProfile, ListProfiles, UpdateProfile, DeleteProfile
- DTO: `src/internal/api/dto/profile.go` — request/response структуры со встроенными dictionaries и preprocessors
- Request validation через go-playground/validator
- Проверка уникальности slug при create
- Error handling middleware — единый JSON-формат ошибок (`{"error": "...", "code": "..."}`)
- Route registration: `Server.RegisterProfileHandler` в `src/internal/api/server.go`
- `PATCH /api/v1/profiles/:slug/dictionary` — добавить или удалить entries словаря

## Контекст

- Tenant ID извлекается из контекста запроса существующим middleware-механизмом (не в scope этой фичи).
- Profile entity, ProfileRepository interface, PostgresProfileRepo уже реализованы в domain и adapters слоях.
- Dictionaries и preprocessors — inline-поля Profile entity, не отдельные агрегаты.
- Существующий `MaskHandler` живёт напрямую в `src/internal/api/`; новый код следует размещать в `src/internal/api/handler/profile/` для лучшей изоляции.

## Зависимости

- `github.com/go-playground/validator/v10` — валидация request body
- Существующий `shield.ProfileRepository` — persistence слой уже реализован
- Существующий `Server` + middleware (RequestID, Logger, Recovery, CORS) — registration pattern

## Требования

- RQ-001 Система ДОЛЖНА предоставлять REST endpoint `POST /api/v1/profiles` для создания профиля с тела запроса, содержащего slug, name, опционально description, dictionaries, preprocessors.
- RQ-002 Система ДОЛЖНА возвращать 409 Conflict при попытке создать профиль с существующим slug.
- RQ-003 Система ДОЛЖНА возвращать 400 Bad Request с единым форматом ошибки при невалидном теле запроса.
- RQ-004 Система ДОЛЖНА предоставлять `GET /api/v1/profiles` — список всех профилей в краткой форме (slug, name, status).
- RQ-005 Система ДОЛЖНА предоставлять `GET /api/v1/profiles/:slug` — полную структуру профиля со словарями и препроцессорами.
- RQ-006 Система ДОЛЖНА возвращать 404 Not Found при запросе несуществующего slug.
- RQ-007 Система ДОЛЖНА предоставлять `PUT /api/v1/profiles/:slug` для полной перезаписи профиля (включая dictionaries и preprocessors).
- RQ-008 Система ДОЛЖНА предоставлять `DELETE /api/v1/profiles/:slug` с каскадным удалением словарей.
- RQ-009 Система ДОЛЖНА предоставлять `PATCH /api/v1/profiles/:slug/dictionary` для добавления или удаления entries словаря без перезаписи профиля.
- RQ-010 Система ДОЛЖНА использовать единый формат ошибок JSON: `{"error": "<message>", "code": "<error_code>"}` через error handling middleware. Для validation errors ДОЛЖЕН присутствовать массив `details: [{"field": "<field_path>", "message": "<reason>"}]`.

## Вне scope

- Отдельные endpoint'ы для dictionaries или preprocessors — управляются только через профиль.
- Tenant isolation — tenant ID ожидается в контексте запроса, механизм не в scope этой фичи.
- Versioning / history изменений профиля.
- Bulk import/export профилей.
- React UI интеграция — только backend API.
- Enums/справочники типов детекторов — валидируются существующей domain логикой.

## Критерии приемки

### AC-001 Create profile with full payload

- Почему это важно: базовый сценарий — создание профиля со словарями и препроцессорами за один запрос
- **Given** valid profile payload (slug, name, dictionaries with entries, preprocessors with rules)
- **When** POST /api/v1/profiles
- **Then** response 201 Created с полной структурой профиля в теле
- Evidence: response body содержит id, slug, name, dictionaries, preprocessors, status, created_at, updated_at

### AC-002 Create profile — duplicate slug rejection

- Почему это важно: slug — уникальный идентификатор профиля в рамках tenant
- **Given** существующий профиль с slug "my-profile"
- **When** POST /api/v1/profiles с slug "my-profile"
- **Then** response 409 Conflict с `{"error": "slug already exists", "code": "SLUG_CONFLICT"}`
- Evidence: второй create возвращает 409, первый профиль остаётся неизменным

### AC-003 Create profile — validation error

- Почему это важно: защита от невалидных данных на уровне API
- **Given** payload с пустым name
- **When** POST /api/v1/profiles
- **Then** response 400 Bad Request с `{"error": "...", "code": "VALIDATION_ERROR", "details": [...]}`
- Evidence: response содержит `details` с записью для каждого невалидного поля, включая `field` и `message`

### AC-004 List profiles — brief representation

- Почему это важно: оператор должен видеть все профили summary без лишних данных
- **Given** 3 существующих профиля
- **When** GET /api/v1/profiles
- **Then** response 200 OK с массивом профилей, каждый содержит только slug, name, status
- Evidence: response body не содержит dictionaries, preprocessors, description, id

### AC-005 Get profile by slug — full structure

- Почему это важно: оператор получает полную конфигурацию профиля для review
- **Given** существующий профиль со словарями и препроцессорами
- **When** GET /api/v1/profiles/my-profile
- **Then** response 200 OK с полной структурой, включая dictionaries и preprocessors
- Evidence: dictionaries содержат name, entries, matchMode; preprocessors содержат name, type, rules

### AC-006 Get profile by slug — not found

- Почему это важно: предсказуемый ответ при запросе несуществующего ресурса
- **Given** несуществующий slug "nonexistent"
- **When** GET /api/v1/profiles/nonexistent
- **Then** response 404 Not Found с `{"error": "profile not found", "code": "NOT_FOUND"}`
- Evidence: статус 404, тело содержит error и code

### AC-007 Update profile — full replacement

- Почему это важно: перезапись профиля включая dictionaries и preprocessors
- **Given** существующий профиль со словарём ["foo"] и препроцессором CSV
- **When** PUT /api/v1/profiles/my-profile с новым словарём ["bar"] и без препроцессоров
- **Then** response 200 OK с обновлённой структурой
- Evidence: словарь содержит ["bar"], препроцессоры пусты, updated_at обновлён

### AC-008 Delete profile — cascade dictionaries

- Почему это важно: удаление профиля должно очищать связанные словари
- **Given** существующий профиль со словарём
- **When** DELETE /api/v1/profiles/my-profile
- **Then** response 204 No Content
- Evidence: GET /api/v1/profiles/my-profile возвращает 404; словарь удалён из БД

### AC-009 Patch dictionary — add entry

- Почему это важно: точечное добавление entry без перезаписи всего профиля
- **Given** существующий профиль со словарём с entries ["foo"]
- **When** PATCH /api/v1/profiles/my-profile/dictionary с `{"action": "add", "name": "default", "entries": ["bar"]}`
- **Then** response 200 OK с обновлённым словарём
- Evidence: dictionary содержит ["foo", "bar"], остальные поля профиля не изменились

### AC-010 Patch dictionary — remove entry

- Почему это важно: точечное удаление entry без перезаписи всего профиля
- **Given** существующий профиль со словарём с entries ["foo", "bar"]
- **When** PATCH /api/v1/profiles/my-profile/dictionary с `{"action": "remove", "name": "default", "entries": ["bar"]}`
- **Then** response 200 OK с обновлённым словарём
- Evidence: dictionary содержит ["foo"], entries удалены

### AC-011 Unified error format on all endpoints

- Почему это важно: предсказуемый формат ошибок для клиентов API
- **Given** любой endpoint профилей
- **When** запрос приводит к 4xx или 5xx ошибке
- **Then** тело ответа содержит JSON с полями `error` (string) и `code` (string)
- Evidence: все error responses соответствуют формату `{"error": "...", "code": "..."}`

## Допущения

- Tenant ID присутствует в контексте запроса — middleware tenant extraction уже реализована или будет до этой фичи.
- Существующий `ProfileRepository` и `PostgresProfileRepo` корректно работают и не требуют изменений в persistence слое для этой фичи.
- `go-playground/validator/v10` уже добавлен в go.mod или будет добавлен в рамках implementation фазы.
- Gin используется в режиме `gin.TestMode` в тестах и `gin.ReleaseMode` в production, как в существующем Server.

## Критерии успеха

- SC-001 Create + Get + Delete cycle выполняется за < 500ms (P99) на локальной PostgreSQL.
- SC-002 List 1000 профилей возвращается за < 200ms.

## Краевые случаи

- Пустой список профилей — GET /api/v1/profiles возвращает `[]`.
- Словарь с пустыми entries — допустимо.
- Создание профиля без dictionaries и preprocessors — допустимо.
- PATCH dictionary для профиля без словаря — создаёт новый словарь (action=add) или 404 (action=remove).
- Обновление несуществующего профиля через PUT — 404.
- Удаление несуществующего профиля — 404.
- Попытка create/update с некорректным slug (не проходит `NewProfileSlug`) — 400.

## Открытые вопросы

- `none`
