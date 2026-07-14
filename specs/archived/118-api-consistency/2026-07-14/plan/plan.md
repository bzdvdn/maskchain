# API Consistency — единый /api/v1/ стандарт, OpenAPI, Swagger UI, единый envelope — План

## Phase Contract

Inputs: spec, inspect (pass).
Outputs: plan, data-model (no-change).
Stop if: — (spec чёткая, inspect pass).

## Цель

Унификация API-слоя без изменения бизнес-логики: все маршруты под `/api/v1/`, единый JSON envelope через middleware-обёртку, пагинация в формате `{data, pagination, error}`, OpenAPI 3.1 spec в `docs/openapi.yaml`, Swagger UI через embed в admin binary, SPA NoRoute с проверкой Accept.

## MVP Slice

- Единые DTO-типы envelope + middleware для оборачивания ответов
- `/api/v1/chat/completions` с redirect с `/v1/chat/completions`
- MaskHandler в JSON envelope
- AC-001, AC-002, AC-003, AC-006

## First Validation Path

Собрать gateway, выполнить `curl -v /v1/chat/completions` → 301; `curl /api/v1/chat/completions` → envelope; `curl -X POST /api/v1/shield/mask -d 'test'` → JSON; `curl /api/v1/docs` в admin → Swagger UI HTML.

## Scope

- DTO-слой: новый `ApiResponse`, обновлённый `PaginatedResponse`, перенос `ErrorResponse` в envelope
- Middleware: обёртка `ResponseEnvelope()` для JSON-ответов
- Route prefix: `/api/v1/chat/completions`, `/api/v1/completions` + 301 redirect со старых `/v1/*`
- MaskHandler: замена `c.String` / `gin.H` на `ApiResponse`
- OpenAPI spec: `docs/openapi.yaml` вручную или через кодогенерацию
- Swagger UI: embed dist-файлов + handler в admin binary
- Admin NoRoute: проверка `Accept: text/html` перед SPA fallback
- Все profile/incident/tenant handlers: новый формат пагинации

## Performance Budget

- none (обёртка ответа — O(1) аллокация, без влияния на latency)

## Implementation Surfaces

| Surface | Статус | Что меняется |
|---|---|---|
| `src/internal/api/dto/` | existing | Новый `ApiResponse`, обновлённый `PaginatedResponse` (per_page, error), `ErrorResponse` интеграция |
| `src/internal/api/middleware/` | existing | Новый `envelope.go` — ResponseEnvelope middleware |
| `src/internal/api/server.go` | existing | RegisterProxyRoute — /api/v1/ prefix + redirect group |
| `src/internal/api/mask_handler.go` | existing | HandleMask/HandleUnmask — c.JSON вместо c.String, ApiResponse |
| `src/internal/api/provider_handler.go` | existing | Ошибки через ApiResponse; прокси-ответы (provider body) не оборачиваются |
| `src/internal/api/handler/profile/handler.go` | existing | PaginatedResponse → новый формат |
| `src/internal/api/handler/incident/handler.go` | existing | PaginatedResponse → новый формат |
| `src/internal/api/admin.go` | existing | RegisterStaticFiles — Accept:text/html check; RegisterSwaggerUI |
| `src/internal/api/handler/admin/tenant_handler.go` | existing | ListTenants — pagination (если нужна) |
| `docs/openapi.yaml` | new | OpenAPI 3.1 spec |
| `src/internal/api/swagger_embed.go` | new | Embed swagger-ui dist + handler |
| `src/internal/api/middleware/errors.go` | existing | AbortWithError — ApiResponse вместо ErrorResponse |

## Bootstrapping Surfaces

- `src/internal/api/swagger_embed.go` — новый файл (embed + handler)
- `docs/openapi.yaml` — новый файл
- `docs/swagger-ui/` — директория с dist-файлами swagger-ui (скачать или скопировать)

## Влияние на архитектуру

- Локальное: все JSON-ответы проходят через middleware-обёртку
- Migration: PaginatedResponse меняет поля (page_size → per_page) — ломает существующих JSON-клиентов. UI-клиент адаптируется отдельной задачей.
- Compatibility: старые `/v1/...` пути сохраняются через 301 — клиенты с авторедиректом не ломаются
- SSE и прокси-ответы провайдеров не оборачиваются (прозрачный proxy)

## Acceptance Approach

| AC | Подход | Surfaces | Валидация |
|---|---|---|---|
| AC-001 | RegisterProxyRoute + /api/v1/ prefix для всех эндпоинтов | `server.go`, `admin.go` | curl по списку эндпоинтов — ни одного 404 |
| AC-002 | Gin redirect group `/v1/...` → 301 Location | `server.go` | curl -v /v1/chat/completions → 301 |
| AC-003 | ResponseEnvelope middleware оборачивает 200/201 JSON | `middleware/envelope.go`, все handlers | curl /api/v1/profiles → `{"data":..., "error":null}` |
| AC-004 | AbortWithError → ApiResponse | `middleware/errors.go`, `dto/` | curl /api/v1/profiles/nonexistent → `error.code` |
| AC-005 | PaginatedResponse → `{data, pagination, error}` | `dto/pagination.go`, profile/incident handlers | curl → jq .pagination.page/.per_page/.total |
| AC-006 | MaskHandler → JSON envelope | `mask_handler.go` | curl -X POST .../mask → `data.masked_text` |
| AC-007 | `docs/openapi.yaml` написан и валиден | `docs/openapi.yaml` | openapi validate |
| AC-008 | Swagger UI embed + GET /api/v1/docs | `swagger_embed.go`, `admin.go` | curl /api/v1/docs → HTML с swagger-ui |
| AC-009 | NoRoute Accept:text/html → index.html | `admin.go` RegisterStaticFiles | curl -H 'Accept:text/html' /unknown → HTML |
| AC-010 | DELETE → 204 без тела; export → CSV Content-Type | profile handler, incident/export.go | curl -X DELETE → 204; curl /export?format=csv → text/csv |

## Данные и контракты

- Data model не меняется — только API-формат ответов
- Контракты: JSON envelope (response-only), query params пагинации (page, page_size/per_page)
- SSE data stream не меняется
- `data-model.md` — no-change stub

## Стратегия реализации

### DEC-001: ResponseEnvelope middleware vs explicit per-handler

**Why:** Middleware единообразно оборачивает все JSON-ответы (200/201). Не нужно менять каждый handler вручную. Исключения (204, SSE, proxy body) обрабатываются через middleware skip-логику.

**Tradeoff:** middleware не может различать уже обёрнутые ответы — нужен признак (context key или ResponseWriter wrapper). 204 и SSE требуют явного `Abort()` или флага.

**Affects:** `middleware/envelope.go` (new), `mask_handler.go`, `provider_handler.go`, profile/incident/tenant handlers

**Validation:** AC-003, AC-004, AC-010

### DEC-002: Gin redirect group для /v1/ → /api/v1/

**Why:** `engine.Group("/v1")` с одним handler, возвращающим 301 + Location, минимально invasive. Не требует дублирования middleware.

**Tradeoff:** Если в будущем /v1/ нужно будет полностью убрать — группа удаляется одной строкой. При redirect клиент теряет тело запроса — это норма для GET/POST redirect.

**Affects:** `server.go` RegisterProxyRoute

**Validation:** AC-002

### DEC-003: Принимать `page_size` и `per_page` на входе

**Why:** Обратная совместимость с существующими клиентами. `page_size` → `per_page` маппинг в хендлере или bind-структуре.

**Tradeoff:** Два имени для одного поля — небольшой когнитивный overhead.

**Affects:** profile, incident handlers (parse params)

**Validation:** AC-005

### DEC-004: MaskHandler — структурированный JSON ответ

**Why:** Envelope требует единого формата. Вместо text/plain возвращаем `{"data": {"masked_text": "...", "mask_id": "...", "document_mask_id": "..."}}`. Заголовки X-Mask-ID/X-Document-Mask-ID сохраняются.

**Tradeoff:** Ломает клиентов, парсящих text/plain. Спека явно декларирует это изменение.

**Affects:** `mask_handler.go`

**Validation:** AC-006

### DEC-005: Swagger UI — embed dist в admin binary

**Why:** Пользователь явно указал Go bin. Никаких CDN-зависимостей, работает в enterprise-сетях без внешнего доступа. Аналогично ui/embed.go.

**Tradeoff:** Размер бинарника увеличивается на ~5 MB (swagger-ui dist). Обновление версии swagger-ui требует пересборки.

**Affects:** `swagger_embed.go` (new), `admin.go`, `docs/` (openapi.yaml)

**Validation:** AC-008

## Incremental Delivery

### MVP (Первая ценность)

1. DTO: ApiResponse, обновлённый PaginatedResponse
2. ResponseEnvelope middleware + skip для 204/SSE/proxy
3. MaskHandler → JSON
4. /api/v1/chat/completions + 301 redirect
5. AC-001, AC-002, AC-003, AC-006

### Итеративное расширение

6. Handler migration: profile, incident, tenant на новый PaginatedResponse + ApiResponse
7. OpenAPI spec: docs/openapi.yaml
8. Swagger UI: embed + /api/v1/docs
9. NoRoute Accept:text/html
10. AC-004, AC-005, AC-007, AC-008, AC-009, AC-010

## Порядок реализации

1. DTO-типы (ApiResponse, PaginatedResponse) — без них ничего не собрать
2. ResponseEnvelope middleware + тесты
3. MaskHandler → JSON (изолированно, один handler)
4. Route prefix: /api/v1/ + redirect
5. Profile/Incident/Tenant handlers — новый PaginatedResponse
6. AbortWithError → ApiResponse
7. OpenAPI spec docs/openapi.yaml
8. Swagger UI embed + /api/v1/docs
9. NoRoute Accept:text/html

Параллельно: 1 + 7, 8 + 9.

## Риски

- **R-001: Прокси-ответы провайдера не должны оборачиваться** — middleware должен детектить raw proxy response. Tag via context key.
  Mitigation: context.WithValue с флагом `skipEnvelope` перед `c.Data()` в прокси-хендлере.
- **R-002: Регрессия UI-клиента** — новый PaginatedResponse формат ломает существующие запросы.
  Mitigation: UI адаптируется отдельной задачей; spec явно выносит это из scope.
- **R-003: OpenAPI 3.1 спецификация рассинхронизируется с кодом** — ручное поддержание.
  Mitigation: На этапе implement документировать только стабильные endpoint-ы; spec описывает intent, не обязывает к генерации из кода.

## Rollout и compatibility

- Gateway/admin деплоятся одновременно (один репозиторий)
- UI-клиент обновляется после deploy API (backward compat через accept обоих pagination params)
- Старые `/v1/...` пути сохраняются (301) — legacy-клиенты не ломаются
- Feature flag не требуется — изменения касаются только API-формата, не логики

## Проверка

- Unit-тесты: ResponseEnvelope middleware, ApiResponse marshaling, PaginatedResponse format
- Integration-тесты: curl-скрипт по списку эндпоинтов (AC-001, AC-002)
- Manual: curl /api/v1/docs → HTML; curl -H 'Accept:text/html' /unknown → SPA
- Go test: `make test` — all existing tests pass (SC-001)
- OpenAPI validate: `openapi validate docs/openapi.yaml` (SC-002)

## Соответствие конституции

- нет конфликтов
