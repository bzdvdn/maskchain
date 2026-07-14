# API Consistency — единый /api/v1/ стандарт, OpenAPI, Swagger UI, единый envelope — Задачи

## Phase Contract

Inputs: plan.md, spec.md, data-model.md (no-change).
Outputs: исполнимые задачи с покрытием AC.
Stop if: — (план чёткий, AC покрываемы).

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/api/dto/` | T1.1 |
| `src/internal/api/middleware/envelope.go` | T1.2 |
| `src/internal/api/middleware/errors.go` | T3.2 |
| `src/internal/api/mask_handler.go` | T2.1 |
| `src/internal/api/provider_handler.go` | T2.2, T3.2 |
| `src/internal/api/server.go` | T2.2 |
| `src/internal/api/handler/profile/handler.go` | T3.1 |
| `src/internal/api/handler/incident/handler.go` | T3.1 |
| `src/internal/api/handler/admin/tenant_handler.go` | T3.1 |
| `src/internal/api/admin.go` | T3.4, T3.5 |
| `src/internal/api/swagger_embed.go` | T3.4 |
| `docs/openapi.yaml` | T3.3 |
| `docs/swagger-ui/` | T3.4 |

## Implementation Context

- Цель MVP: единый `/api/v1/` префикс, JSON envelope через middleware, MaskHandler в JSON, 301 redirect со старых `/v1/`
- Границы приемки: AC-001, AC-002, AC-003, AC-006 (MVP); AC-004, AC-005, AC-007–AC-010 (расширение)
- Инварианты/семантика:
  - JSON envelope: `{"data": <payload>, "error": null}` — успех; `{"data": null, "error": {"code": "...", "message": "..."}}` — ошибка
  - Пагинация: `{"data": [...], "pagination": {"page": int, "per_page": int, "total": int}, "error": null}`
  - На входе сервер принимает `page_size` и `per_page`; в ответе всегда `per_page`
  - 204, SSE, proxy body (provider response) — не оборачиваются
- Контракты/протокол:
  - `/v1/chat/completions` → 301 → `/api/v1/chat/completions`
  - `/v1/completions` → 301 → `/api/v1/completions`
  - Health endpoints (`/health`, `/ready`, `/live`), `/metrics` — без изменений
- Границы scope:
  - Не меняем domain/app/repository слои
  - Не меняем SSE streaming data format
  - Не меняем React UI (адаптация отдельной задачей)
- Proof signals:
  - `curl /api/v1/profiles | jq` → `data` + `error` keys
  - `curl -v /v1/chat/completions` → 301 + Location
  - `curl -X POST /api/v1/shield/mask -d 'test' | jq` → `data.masked_text`
  - `curl /api/v1/docs` (admin) → HTML with swagger-ui
- References: DEC-001 (envelope middleware), DEC-002 (redirect group), DEC-003 (per_page alias), DEC-004 (mask JSON), DEC-005 (Swagger embed)

## Фаза 1: Core DTO и middleware

Цель: подготовить базовые типы envelope и middleware, от которых зависят все остальные задачи.

- [x] T1.1 Создать `ApiResponse` и обновить `PaginatedResponse` в dto-пакете. `ApiResponse` — generic обёртка `{data, error}` для успеха и ошибок. `PaginatedResponse` — формат `{data, pagination: {page, per_page, total}, error}`. Сохранить старый `ErrorResponse` как внутренний тип для middleware. Touches: `src/internal/api/dto/pagination.go`, `src/internal/api/dto/profile.go` (ErrorResponse), `src/internal/api/dto/` (новый файл `envelope.go`). AC-003, AC-004, AC-005.

- [x] T1.2 Реализовать `ResponseEnvelope` middleware в новом `middleware/envelope.go`. Middleware перехватывает JSON-ответы (200, 201, 4xx, 5xx) и оборачивает их в `ApiResponse`. Исключения (204, SSE, proxy body) — через context key `skipEnvelope`. Настроить Gin `c.Next()` + обёртку `c.Writer`. Touches: `src/internal/api/middleware/envelope.go`. AC-003, AC-004, AC-010.

## Фаза 2: MVP — MaskHandler, route prefix, redirect

Цель: минимальная демонстрируемая ценность — MaskHandler в JSON, `/api/v1/chat/completions`, 301 redirect.

- [x] T2.1 Перевести `MaskHandler.HandleMask` и `HandleUnmask` с `c.String`/`gin.H` на `ApiResponse`. Ответ mask: `{"data": {"masked_text": "...", "mask_id": "...", "document_mask_id": "..."}, "error": null}`. Ошибки: `{"data": null, "error": {"code": "...", "message": "..."}}`. Заголовки `X-Mask-ID`, `X-Document-Mask-ID` сохранить. Touches: `src/internal/api/mask_handler.go`. AC-006, RQ-005.

- [x] T2.2 Добавить `/api/v1/chat/completions` и `/api/v1/completions` в `RegisterProxyRoute`. Создать отдельную Gin-группу `/v1` только для redirect: handler возвращает 301 + `Location: /api/v1/...`. Прокси-ответы от провайдера (raw body) не оборачиваются — установить context key `skipEnvelope` перед `c.Data()`/`c.Stream()`. Touches: `src/internal/api/server.go`, `src/internal/api/provider_handler.go`. AC-001, AC-002.

## Фаза 3: Основная реализация

Цель: handler migration, error envelope, OpenAPI, Swagger UI, NoRoute.

- [x] T3.1 Обновить `ProfileHandler.ListProfiles`, `incident.Handler.ListIncidents`, `TenantHandler.ListTenants` на новый `PaginatedResponse` формат. Заменить `PageSize` на `PerPage` в ответе. Парсить query params: `page`, `per_page` (или `page_size` как alias). Touches: `src/internal/api/handler/profile/handler.go`, `src/internal/api/handler/incident/handler.go`, `src/internal/api/handler/admin/tenant_handler.go`. AC-005.

- [x] T3.2 Обновить `AbortWithError` в `middleware/errors.go` — использовать `ApiResponse` вместо `ErrorResponse`. Все ошибки в handlers (profile, incident, tenant, provider, mask) автоматически оборачиваются через middleware. Touches: `src/internal/api/middleware/errors.go`, `src/internal/api/provider_handler.go` (error paths). AC-004.

- [x] T3.3 Написать OpenAPI 3.1 спецификацию `docs/openapi.yaml`. Описать все публичные эндпоинты: profiles CRUD, incidents list/get/export, tenants CRUD, shield mask/unmask, chat/completions, completions. Включить компоненты envelope, pagination, error schemas. Touches: `docs/openapi.yaml`. AC-007.

- [x] T3.4 Встроить Swagger UI в admin binary: скачать/скопировать swagger-ui dist в `docs/swagger-ui/`, создать `swagger_embed.go` с `//go:embed` + handler, зарегистрировать `GET /api/v1/docs` в `AdminServer.RegisterSwaggerUI()`. Swagger UI загружает `openapi.yaml` из docs/. Touches: `src/internal/api/swagger_embed.go` (new), `src/internal/api/admin.go`, `docs/swagger-ui/`. AC-008, RQ-010.

- [x] T3.5 Обновить `RegisterStaticFiles` в `admin.go`: NoRoute handler проверяет `Accept: text/html` перед SPA fallback. Если запрос без `Accept: text/html` или путь начинается с `/api/` — вернуть JSON 404. Touches: `src/internal/api/admin.go`. AC-009.

## Фаза 4: Проверка

Цель: automated тесты + валидация.

- [x] T4.1 Написать unit-тесты: `ApiResponse` marshaling, `PaginatedResponse` format, `ResponseEnvelope` middleware (success/error/skip cases), MaskHandler JSON response. Прогнать `make test`. Touches: `src/internal/api/dto/envelope_test.go`, `src/internal/api/middleware/envelope_test.go`, `src/internal/api/mask_handler_test.go`. AC-001–AC-010, SC-001.

- [x] T4.2 Валидировать `docs/openapi.yaml` через `openapi validate`. Написать curl-based integration smoke test (список эндпоинтов, проверка envelope). Touches: `docs/openapi.yaml`, скрипт `test/integration/api-consistency.sh`. AC-007, SC-002.

## Покрытие критериев приемки

- AC-001 -> T2.2, T4.1
- AC-002 -> T2.2, T4.1
- AC-003 -> T1.1, T1.2, T4.1
- AC-004 -> T1.1, T1.2, T3.2, T4.1
- AC-005 -> T1.1, T3.1, T4.1
- AC-006 -> T2.1, T4.1
- AC-007 -> T3.3, T4.2
- AC-008 -> T3.4, T4.1
- AC-009 -> T3.5, T4.1
- AC-010 -> T1.2, T3.1, T4.1

## Заметки

- Фазы 1–2 обязательны перед фазой 3 (dependency chain: DTO → middleware → handlers)
- T3.3 (OpenAPI) и T3.5 (NoRoute) независимы — можно параллелить с T3.1/T3.2
- T3.4 (Swagger UI) зависит от T3.3 (openapi.yaml)
- T4.2 интеграционный smoke test — ручной/CI, не обязателен для `make test`
