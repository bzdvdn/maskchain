# Admin UI Design Задачи

## Phase Contract

Inputs: plan.md (pass), data-model.md (changed), spec.md (pass).
Outputs: tasks.md с фазами, Surface Map, покрытие AC.
Stop if: нет — plan чёткий.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/infra/config/config.go` | T1.1 |
| `src/internal/adapters/repository/postgres/migrations/` | T1.2 |
| `src/internal/domain/admin_session/` | T1.3 |
| `src/internal/adapters/repository/postgres/admin_session_store.go` | T1.3 |
| `src/internal/adapters/repository/postgres/audit_log_store.go` | T1.4, T3.2 |
| `src/internal/api/middleware/adminauth.go` | T2.1 |
| `src/internal/api/handler/admin/admin_auth_handler.go` | T2.2 |
| `src/internal/api/admin.go` | T2.3 |
| `src/cmd/admin/main.go` | T2.3 |
| `ui/src/pages/Login/` | T2.4 |
| `ui/src/components/ProtectedRoute.tsx` | T2.4 |
| `ui/src/components/Layout.tsx` | T2.5 |
| `ui/src/App.tsx` | T2.5 |
| `ui/src/api/admin.ts` | T2.4 |
| `ui/src/api/client.ts` | T2.4 |
| `ui/src/pages/Dashboard.tsx` | T2.6 |
| `ui/src/pages/Analytics.tsx` | T3.1 |
| `ui/src/pages/Routing.tsx` | T3.1 |
| `ui/src/pages/Sessions.tsx` | T3.1 |
| `ui/src/pages/AuditLog.tsx` | T3.2 |
| `ui/src/pages/Settings.tsx` | T3.1 |
| `ui/src/pages/Swagger.tsx` | T3.1 |
| `src/internal/api/handler/admin/tenant_handler.go` | T3.2 |

## Implementation Context

- Цель MVP: admin login/logout + защита API + Dashboard с метриками + SPA layout с сайдбаром
- Границы приемки: AC-001 (login), AC-002 (dashboard), AC-003 (navigation), AC-004 (session expiry)
- Инварианты:
  - Admin сессии — отдельная таблица `admin_sessions`, не пересекается с tenant `sessions`
  - Audit log — append-only, async запись через buffered channel
  - Admin auth middleware применяется ко всем `/api/v1/*` кроме `/api/v1/admin/login`
- Контракты:
  - `POST /api/v1/admin/login` → `{username, password}` → `{token, expires_at}` + Set-Cookie
  - `POST /api/v1/admin/logout` → удаляет сессию по cookie
  - Все `/api/v1/*` (кроме login) проверяют admin session через cookie `admin_token` или `Authorization: Bearer <token>`
  - Существующий `X-Admin-Token` middleware (AdminAuth) сохранён для `/debug/pprof`
- Ошибки/коды:
  - Login неверный → 401 `{error: "invalid credentials"}`
  - Login при отсутствии `ADMIN_USERNAME` → 503 `{error: "admin not configured"}`
  - Истекшая/невалидная сессия → 401 `{error: "unauthorized"}`
- Key references: DEC-001 (admin session отдельная таблица), DEC-002 (audit async), DEC-003 (новый middleware), DEC-004 (SPA ProtectedRoute), DM-001 (AdminSession), DM-002 (AuditLogEntry)

## Фаза 1: Основа

Цель: миграции, конфиг, domain сущности и store для admin session и audit log.

- [x] T1.1 Добавить `AdminConfig` в конфиг (`ADMIN_USERNAME`, `ADMIN_PASSWORD`, `ADMIN_SESSION_TTL` default 30m, `DASHBOARD_POLL_INTERVAL` default 5s). Touches: `src/internal/infra/config/config.go`
- [x] T1.2 Создать SQL миграции: `011_admin_sessions.up.sql` + `012_audit_log.up.sql` (и down). Touches: `src/internal/adapters/repository/postgres/migrations/`
- [x] T1.3 Создать domain пакет `admin_session`: entity (ID, Username, TokenHash, CreatedAt, ExpiresAt), storage interface, use case. Touches: `src/internal/domain/admin_session/`
- [x] T1.4 Создать PG store для admin session и audit log. Audit log store с buffered channel + flush worker. Touches: `src/internal/adapters/repository/postgres/admin_session_store.go`, `src/internal/adapters/repository/postgres/audit_log_store.go`

## Фаза 2: MVP Slice

Цель: admin auth end-to-end + Dashboard + SPA layout.

- [x] T2.1 Реализовать новый middleware `AdminSessionAuth` — проверяет cookie/header admin session. Touches: `src/internal/api/middleware/adminauth.go`
- [x] T2.2 Реализовать `AdminAuthHandler` с HandleLogin и HandleLogout. Хеширование токена SHA256, constant-time сравнение. Touches: `src/internal/api/handler/admin/admin_auth_handler.go`
- [x] T2.3 Зарегистрировать admin auth handler и применить middleware ко всем `/api/v1/*` группам в AdminServer. Touches: `src/internal/api/admin.go`, `src/cmd/admin/main.go`
- [x] T2.4 Реализовать Login page, `ProtectedRoute`, api client с interceptor (редирект на `/login` при 401). Touches: `ui/src/pages/Login/`, `ui/src/components/ProtectedRoute.tsx`, `ui/src/api/admin.ts`, `ui/src/api/client.ts`
- [x] T2.5 Реализовать Layout (sidebar + header) и перестроить App.tsx на React Router с ProtectedRoute. Touches: `ui/src/components/Layout.tsx`, `ui/src/App.tsx`
- [x] T2.6 Реализовать Dashboard page с stat-карточками и 5s polling из `/api/v1/analytics/*`. Touches: `ui/src/pages/Dashboard.tsx`

## Фаза 3: Основная реализация

Цель: остальные страницы + audit log.

- [x] T3.1 Реализовать страницы: Analytics, Routing, Sessions (с кнопками Extend/Close), Settings, Swagger. Touches: `ui/src/pages/Analytics.tsx`, `ui/src/pages/Routing.tsx`, `ui/src/pages/Sessions.tsx`, `ui/src/pages/Settings.tsx`, `ui/src/pages/Swagger.tsx`
- [x] T3.2 Интегрировать audit log: tenant handler (create/update/delete) пишет событие в канал. Touches: `src/internal/adapters/repository/postgres/audit_log_store.go`, `src/internal/api/handler/admin/tenant_handler.go`, `ui/src/pages/AuditLog.tsx`

## Фаза 4: Проверка

Цель: automated tests + verify.

- [x] T4.1 Написать unit-тесты: admin auth handler (login/logout), admin session store, audit log store, middleware (valid/invalid/expired token). Touches: `src/internal/api/handler/admin/admin_auth_handler_test.go`, `src/internal/api/middleware/adminauth_test.go`, `src/internal/domain/admin_session/usecase_test.go`
- [x] T4.2 Выполнить `go build ./...`, `go vet ./...`, `npm run build`, ручной сценарий First Validation Path из плана. Touches: — (shell)

## Покрытие критериев приемки

- AC-001 Login/logout → T2.1, T2.2, T2.4, T4.1
- AC-002 Dashboard metrics → T2.6, T4.2
- AC-003 Navigation → T2.5, T4.2
- AC-004 Session expiry → T2.1, T2.4, T4.1
- AC-005 Audit log → T1.4, T3.2, T4.1
- AC-006 Routing page → T3.1, T4.2
