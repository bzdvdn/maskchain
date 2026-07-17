# Admin UI Design План

## Phase Contract

Inputs: spec (pass), inspect (pass), минимальный repo-контекст (admin.go, adminauth.go, session_handler.go, config.go, main.go, UI).
Outputs: plan.md, data-model.md.
Stop if: нет — spec чёткая.

## Цель

Реализовать admin-авторизацию (username/password из env + session-based), защитить admin API, перестроить SPA на layout с сайдбаром, добавить страницы Dashboard, Analytics, Routing, Sessions, Audit Log, Settings, Swagger.

## MVP Slice

Login + admin session middleware + Dashboard с живыми метриками + защита существующих tenant/sessions API. Одна новая страница Dashboard остальные — итеративно.

Покрывает: AC-001 (login/logout), AC-002 (dashboard metrics), AC-003 (navigation), AC-004 (session expiry).

## First Validation Path

1. Запустить admin: `ADMIN_USERNAME=admin ADMIN_PASSWORD=test go run -tags admin ./src/cmd/admin/`
2. Открыть `http://localhost:8081/` → редирект на `/login`
3. Ввести admin/test → редирект на Dashboard
4. Проверить карточки метрик через `/api/v1/analytics/tokens`
5. Перейти в Tenants/Sessions — данные загружаются

## Scope

- `src/internal/api/admin.go` — новый login route, переключение tenant/sessions/analytics на admin auth middleware
- `src/internal/api/middleware/adminauth.go` — замена: новый middleware проверяет admin session cookie/token
- `src/internal/api/handler/admin/` — новый `admin_auth_handler.go`: login/logout handler
- `src/internal/domain/session/` — расширение: admin session use case или новый domain object
- `src/internal/adapters/repository/postgres/` — admin session store, audit log store
- `src/internal/infra/config/config.go` — `AdminConfig` (username, password, session_ttl)
- `src/cmd/admin/main.go` — wire нового handler, middleware, config
- `ui/src/` — login page, sidebar layout, Dashboard, Analytics, Routing, Sessions, Audit, Settings страницы

## Performance Budget

- none — admin plane, не data plane

## Implementation Surfaces

| Поверхность | Роль | Новая/сущ. |
|---|---|---|
| `src/internal/api/handler/admin/admin_auth_handler.go` | Login/logout handler | Новая |
| `src/internal/api/middleware/adminauth.go` | Admin session middleware (замена X-Admin-Token) | Сущ. (переписать) |
| `src/internal/domain/admin_session/` | Admin session entity + use case | Новая |
| `src/internal/adapters/repository/postgres/admin_session_store.go` | PG store для admin sessions | Новая |
| `src/internal/adapters/repository/postgres/audit_log_store.go` | PG store для audit log | Новая |
| `src/internal/api/admin.go` | RegisterAdminAuth, защита всех /api/v1/* групп | Сущ. (расширить) |
| `src/internal/infra/config/config.go` | AdminConfig block | Сущ. (расширить) |
| `ui/src/` | Login page, layout, все страницы | Сущ. (расширить) |

## Bootstrapping Surfaces

- `src/internal/domain/admin_session/` — пакет домена admin-сессий
- Миграция: `migrations/XXXXXX_create_admin_sessions.sql`, `migrations/XXXXXX_create_audit_log.sql`

## Влияние на архитектуру

- Admin auth middleware будет применяться ко всем `/api/v1/*` группам (кроме `/login`)
- Существующий `AdminAuth` (X-Admin-Token) сохранить для `/debug/pprof`, не менять
- Tenant auth middleware (tenant auth, не admin) остаётся как есть для gateway data plane
- Session handler: tenant sessions и admin sessions — разные домены, не путать

## Acceptance Approach

| AC | Подход | Поверхности |
|---|---|---|
| AC-001 | Login handler + admin session middleware + SPA redirect | `admin_auth_handler.go`, `adminauth.go`, `Login.tsx`, `ProtectedRoute.tsx` |
| AC-002 | Dashboard читает `/api/v1/analytics/*` с admin auth, polling 5s | `Dashboard.tsx`, analytics handler (уже есть) |
| AC-003 | React Router + sidebar компонент | `App.tsx`, `Sidebar.tsx`, `Layout.tsx` |
| AC-004 | Admin session TTL проверяется middleware, 401 → SPA redirect | `adminauth.go`, `api.ts` interceptor |
| AC-005 | Tenant handler после create/update/delete пишет audit event async | `audit_log_store.go`, tenant handler расширение |
| AC-006 | Routing page читает config health probe | `RoutingPage.tsx`, health handler |

## Данные и контракты

- Новая таблица `admin_sessions` — отдельно от tenant sessions
- Новая таблица `audit_log` — async запись через channel + batch insert
- Существующие API контракты не меняются — только добавляется auth guard
- `POST /api/v1/admin/login` — новый endpoint
- `POST /api/v1/admin/logout` — новый endpoint
- Подробнее: `data-model.md`

## Стратегия реализации

### DEC-001 Admin sessions: отдельная таблица, не tenant sessions

- Why: tenant sessions — это сессии gateway-пользователей с моделью, масками и TTL. Admin sessions — это сессии админа UI. Разные домены, разный lifecycle, разный TTL. Смешивание создаст耦合.
- Tradeoff: дублирование кода store (но домен маленький, ≈50 строк).
- Affects: `domain/admin_session/`, `postgres/admin_session_store.go`
- Validation: admin login → запись в `admin_sessions`, middleware читает из той же таблицы.

### DEC-002 Audit log: async channel + batch insert

- Why: синхронная запись в том же request добавляет latency и может упасть вместе с транзакцией. Async через buffered channel позволяет не терять события при перегрузке, но не блокирует admin.
- Tradeoff: события могут быть потеряны при падении процесса до flush (mitigation: sync on graceful shutdown). AC-005 accepts eventual consistency.
- Affects: `audit_log_store.go`, tenant handler (вызов audit), `main.go` (запуск worker)
- Validation: создать tenant → через <1s проверить `audit_log` таблицу.

### DEC-003 Admin middleware: новый, не расширять существующий AdminAuth

- Why: существующий `AdminAuth` проверяет статический `X-Admin-Token` для pprof. Новый middleware проверяет admin session cookie/token. Разная логика, разный контракт.
- Tradeoff: два middleware вместо одного. Но их не спутать — один для `/debug/pprof`, другой для `/api/v1/*`.
- Affects: `middleware/adminauth.go` (новый `AdminSessionAuth`), `admin.go` (применение к API группам)
- Validation: запрос к `/api/v1/tenants` без cookie → 401. С cookie → 200.

### DEC-004 SPA: React Router с ProtectedRoute и Layout

- Why: централизованная проверка auth на каждый route, единый layout (sidebar + header), lazy loading страниц.
- Tradeoff: небольшой бойлерплейт ProtectedRoute.
- Affects: `ui/src/App.tsx`, `ui/src/components/Layout.tsx`, `ui/src/components/ProtectedRoute.tsx`
- Validation: неавторизованный → редирект на `/login`. Авторизованный → видит sidebar.

### DEC-005 Dashboard polling: 5s через setInterval с cleanup

- Why: простой и надёжный polling, не требует WebSocket или SSE. 5s достаточно для admin panel.
- Tradeoff: лишние запросы при открытой вкладке. Mitigation: pause polling при `document.hidden`.
- Affects: `ui/src/pages/Dashboard.tsx`
- Validation: открыть Dashboard — видно, что числа обновляются каждые 5s.

## Incremental Delivery

### MVP (Первая ценность)

- Admin auth (login/logout + middleware) — AC-001, AC-004
- Dashboard с метриками из analytics API — AC-002
- SPA layout с сайдбаром и переключением страниц — AC-003
- Защита существующих API (Tenants, Sessions) admin middleware

### Итеративное расширение

1. **Analytics page** — вывести token usage/cost/traffic из `/api/v1/analytics/*`
2. **Routing page** — провайдеры (health status) + routing rules (read-only)
3. **Sessions page** — список tenant-сессий, кнопки Close/Extend
4. **Audit log** — таблица + async worker + tenant handler integration
5. **Settings page** — отображение глобальной конфигурации
6. **Swagger page** — embedded openapi.yaml viewer

## Порядок реализации

1. Backend: config (AdminConfig) + миграции
2. Backend: admin_session domain + store + middleware
3. Backend: login/logout handler + router wiring
4. UI: Login page + ProtectedRoute + api interceptor
5. UI: Layout (sidebar + header) + routing
6. UI: Dashboard page с polling
7. UI: Остальные страницы (можно параллельно)
8. Backend: audit log store + worker
9. UI: Audit page

Шаги 1–6 — MVP. 7–9 — итеративное расширение.

## Риски

- **Сломанные существующие тесты**: tenant handler тесты используют текущий auth. Admin auth middleware может сломать их.
  - Mitigation: tenant handler тесты не ходят через middleware — они вызывают handler напрямую. Но если тесты идут через `gin.Context`, надо проверить.
- **Корректность SPA fallback**: NoRoute handler не должен мешать новым API роутам.
  - Mitigation: NoRoute срабатывает только для `/api/` (возвращает 404) или для SPA путей. Новый `/api/v1/admin/login` не попадёт в NoRoute.

## Rollout и compatibility

- Никаких feature flags не требуется — admin UI новая функциональность
- Старый `X-Admin-Token` для `/debug/pprof` сохраняется
- При отсутствии `ADMIN_USERNAME`/`ADMIN_PASSWORD` admin server стартует, но login возвращает 503

## Проверка

- Unit тесты: admin auth handler, admin session store, audit log store, middleware
- UI: `npm run build` + ручная проверка всех страниц
- Integration: admin login → dashboard → tenants → sessions → analytics
- AC-001–006: каждый проверяется через ручной сценарий First Validation Path

## Соответствие конституции

- нет конфликтов
