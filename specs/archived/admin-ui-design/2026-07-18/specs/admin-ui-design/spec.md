# Admin UI Design

## Scope Snapshot

- In scope: полноценный административный интерфейс MaskChain с авторизацией, навигацией и набором страниц для управления gateway.
- Out of scope: доработка data-plane gateway (новые middleware, провайдеры, правила роутинга), API новых сущностей, RBAC с разными ролями.

## Цель

Администратор gateway получает единый веб-интерфейс для мониторинга и управления системой: дашборд метрик, CRUD тенантов, аналитика по использованию, просмотр сессий, статус провайдеров и настройки. Успех фичи — admin может выполнить все типовые операции не открывая терминал и не читая документацию.

## Основной сценарий

1. Администратор открывает `http://admin-host/`, видит страницу логина.
2. Вводит username/password, система создаёт admin-сессию и редиректит на Dashboard.
3. Dashboard показывает ключевые метрики: RPS, latency P50/P95/P99, error rate, активные тенанты.
4. Через боковое меню admin переходит к управлению тенантами, просмотру аналитики, сессий, статуса провайдеров.
5. После бездействия сессия истекает — admin перенаправляется на логин.

## MVP Slice

Login + Dashboard (статический макет с реальными метриками из API analytics) + навигация между существующими страницами (Tenants, Sessions — уже есть API). Боковое меню вместо хедера.

## First Deployable Outcome

После первого implementation pass: admin видит login page, вводит креды из `ADMIN_USERNAME`/`ADMIN_PASSWORD`, попадает на Dashboard с живыми метриками, может перейти в Tenants и Sessions без потери авторизации.

## Scope

- Admin login/logout с env-конфигурацией (`ADMIN_USERNAME`/`ADMIN_PASSWORD`)
- Session-based авторизация (cookie/токен) для всех admin API и SPA
- Dashboard с real-time метриками (RPS, latency, error rate, active tenants)
- Analytics page (графики token usage, cost breakdown по tenant/model)
- Routing page (список провайдеров со статусом, правила маршрутизации)
- Sessions page (просмотр активных сессий tenant-пользователей)
- Audit log page (лог изменений конфигурации)
- Settings page (просмотр глобальных конфигов: debug, log level, провайдеры)
- Layout: боковое меню (иконки + текст) + хедер с user info + logout
- HTML design mockup для визуального согласования до реализации
- Swagger/OpenAPI страница в админке

## Контекст

- Уже есть: `DebugConfig` с `admin_token`, `AdminAuth` middleware (X-Admin-Token), tenant auth middleware, session handler, analytics handler
- Admin SPA сейчас открыта без авторизации — tenant API доступны без токена
- Стили: тёмная тема, CSS variables, карточки (`card`), таблицы, хедер с логотипом
- Backend: Go + Gin, UI: React + TypeScript + Vite

## Зависимости

- Analytics API (`/api/v1/analytics/*`) — уже реализован
- Sessions API (`/api/v1/sessions/*`) — уже реализован
- Tenants API (`/api/v1/tenants/*`) — уже реализован

## Требования

- RQ-001 Admin username и password задаются через env `ADMIN_USERNAME` и `ADMIN_PASSWORD` (plain-text).
- RQ-002 `POST /api/v1/admin/login` принимает `{username, password}`, возвращает session token в теле ответа и Set-Cookie.
- RQ-003 Все admin API маршруты (`/api/v1/*`), кроме login, проверяют admin session token (через cookie или Authorization header).
- RQ-004 Admin сессия имеет TTL, настраиваемый через env `ADMIN_SESSION_TTL` (default 30m).
- RQ-005 Dashboard отображает метрики за последние N минут (RPS, latency P50/P95/P99, error rate, активные тенанты).
- RQ-006 Analytics page показывает token usage и cost breakdown per tenant/model с выбором периода.
- RQ-007 Routing page отображает список провайдеров (имя, тип, base_url, статус health) и routing rules.
- RQ-008 Sessions page показывает активные сессии с возможностью закрыть или продлить.
- RQ-009 Audit log хранит и отображает события: создание/изменение/удаление tenant, изменение правил роутинга.
- RQ-010 Settings page отображает текущую конфигурацию (debug, log level, server port).
- RQ-011 SPA использует layout с боковым меню (collapsible) и хедером.

## Вне scope

- RBAC с разными ролями (admin/viewer/editor)
- OAuth2/OIDC интеграция для admin login
- Управление провайдерами и routing rules через UI (только просмотр)
- Настройка глобальных конфигов через UI (только просмотр)
- Email/push-уведомления об ошибках

## Критерии приемки

### AC-001 Login/logout

- Почему важно: admin не может получить доступ к панели без аутентификации
- **Given** admin не аутентифицирован
- **When** он открывает `/` в браузере
- **Then** он перенаправляется на `/login`
- **Given** admin вводит неверные креды и нажимает Login
- **Then** отображается ошибка "Invalid credentials"
- **Given** admin вводит верные `ADMIN_USERNAME`/`ADMIN_PASSWORD` и нажимает Login
- **Then** создаётся admin сессия, браузер получает cookie, происходит редирект на `/`
- Evidence: UI показывает login page → после ввода верных кредов виден Dashboard

### AC-002 Dashboard metrix

- Почему важно: admin видит состояние gateway без CLI
- **Given** admin аутентифицирован и находится на Dashboard
- **Then** он видит карточки: RPS, P50 latency, P95 latency, P99 latency, error rate, active tenants
- **Then** значения обновляются автоматически (polling 5s)
- Evidence: числа на карточках соответствуют данным из `/api/v1/analytics/*`

### AC-003 Navigation

- Почему важно: admin может переключаться между страницами
- **Given** admin аутентифицирован
- **When** он кликает на пункт бокового меню
- **Then** открывается соответствующая страница без перезагрузки
- **Then** активный пункт меню подсвечен
- Evidence: каждый пункт меню ведёт на свою страницу, URL меняется

### AC-004 Session expiry

- Почему важно: admin сессия не должна жить вечно
- **Given** admin аутентифицирован и бездействует дольше `ADMIN_SESSION_TTL`
- **When** он делает запрос к API
- **Then** API возвращает 401, SPA перенаправляет на `/login`
- Evidence: после TTL страница сама переключается на login

### AC-005 Audit log

- Почему важно: изменения конфигурации должны быть traceable
- **Given** admin изменяет tenant (создаёт, редактирует, удаляет)
- **Then** событие записывается в audit log с timestamp, admin ID, типом операции, diff
- **Given** admin открывает Audit page
- **Then** он видит таблицу событий с фильтром по типу и дате
- Evidence: после создания tenant в audit log появляется запись

### AC-006 Routing page (read-only)

- Почему важно: admin видит текущую конфигурацию провайдеров и правил
- **Given** admin аутентифицирован
- **When** он открывает Routing page
- **Then** он видит список провайдеров: name, api_type, base_url, health status
- **Then** он видит routing rules: tenant → model → providers
- Evidence: данные соответствуют секции `routing` в config.yaml

## Допущения

- `ADMIN_USERNAME` и `ADMIN_PASSWORD` задаются в docker-compose environment или .env файле
- Admin session хранится в той же PostgreSQL, что и tenant sessions
- Audit log пишется async (через канал/очередь) в отдельную таблицу PostgreSQL
- Провайдеры не меняются через API — только через config.yaml

## Критерии успеха

- SC-001 Страница Dashboard загружается <2s при 10k tenants
- SC-002 Login проверяется и выполняется <500ms

## Краевые случаи

- `ADMIN_USERNAME`/`ADMIN_PASSWORD` не заданы — admin видит предупреждение при старте, login возвращает 503
- Admin пытается открыть страницу с истекшей сессией — редирект на login без потери URL (return URL)
- Несколько admin сессий одновременно (один пользователь с разных вкладок)
- Dashboard при отсутствии данных (новый gateway, нет трафика) — отображать "0" или "No data" вместо пустоты

## Открытые вопросы

- Принятые решения (закрыты):
  - Пароль в env: plain-text (`ADMIN_USERNAME`/`ADMIN_PASSWORD`)
  - Audit log: async (event bus / очередь)
  - Dashboard polling: 5s, сделать конфигурируемым через env `DASHBOARD_POLL_INTERVAL`
  - Swagger: да, отдельный пункт меню
  - Метрики Dashboard: RPS, P95 latency, error rate, active tenants как baseline
  - Health endpoint провайдеров: использовать существующий health probe
