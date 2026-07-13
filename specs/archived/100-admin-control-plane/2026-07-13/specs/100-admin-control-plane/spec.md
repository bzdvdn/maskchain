# Admin Control Plane

## Scope Snapshot

- In scope: выделение admin control plane в отдельный сервис со своим entrypoint, Dockerfile и SPA UI; gateway остаётся лёгким data plane для LLM proxy; оба сервиса используют общие domain/ports/repository модули и одну базу.
- Out of scope: dashboard/analytics UI, bulk operations, profile versioning, импорт/экспорт профилей, Envoy data plane.

## Цель

Оператор получает два независимо масштабируемых binary: gateway (data plane для LLM proxy, ~15MB image, без node) и admin (control plane с UI и management API). Admin можно обновлять/масштабировать независимо от gateway. Разработчик получает возможность собирать и деплоить admin отдельно, не затрагивая data plane. Успех фичи измеряется тем, что оба сервиса запускаются из одного docker-compose, gateway обслуживает proxy-трафик, admin отдаёт SPA и API, а profile/incident CRUD доступны через оба сервиса.

## Основной сценарий

1. Разработчик запускает `docker compose up` — стартуют postgres, valkey, gateway (replicas: 2), admin.
2. Gateway принимает LLM proxy-запросы на `:8080`, admin отдаёт SPA на `:8081`.
3. Оператор открывает admin UI в браузере, создаёт профиль — POST идёт напрямую в admin API.
4. CI-скрипт создаёт профиль через gateway API (`POST /api/v1/profiles`) на `:8080`.
5. Оба запроса пишут в одну таблицу profiles, возвращают одинаковый response.
6. При обновлении UI (новый билд) пересобирается только admin image, gateway не передеплоивается.

## User Stories

- P1 (разработчик): как разработчик, я хочу собирать gateway без node, чтобы ускорить CI и уменьшить image с ~300MB до ~15MB.
- P1 (оператор): как оператор, я хочу масштабировать gateway горизонтально, не умножая инстансы UI/management API.
- P2 (оператор): как оператор, я хочу обновлять UI независимо от gateway, чтобы не рисковать data plane при выкатке косметических изменений.

## MVP Slice

Наименьший срез — `src/cmd/admin/main.go` + `Dockerfile.admin` + перенос `ui/embed.go` в admin + Gateway Dockerfile без node-стадии + docker-compose с двумя сервисами. Закрывает AC-001..AC-007.

## First Deployable Outcome

После первого implementation pass можно запустить `docker compose up`, gateway отвечает на `POST /v1/chat/completions`, admin отдаёт SPA на `:8081`, profile CRUD работает через оба сервиса, `make build-gateway && make build-admin` собирают два разных image.

## Scope

- `src/cmd/admin/main.go` — entrypoint admin binary (DI только для admin: UI, profile/incident handlers, health, metrics)
- `src/internal/api/admin.go` — отдельный Server для admin (отличающиеся middleware, CORS, rate limits, port)
- `ui/embed.go` — перенесён в admin binary, gateway его не импортирует
- `Dockerfile.gateway` — Go build → distroless, без node-стадии, CGO_ENABLED=0
- `Dockerfile.admin` — node build → Go build → distroless (текущая логика)
- `deployments/docker-compose/docker-compose.yml` — gateway + admin сервисы
- `src/cmd/gateway/main.go` — proxy routes, shield middleware, health, metrics, profile/incident API handlers
- `Makefile` — targets: `build-gateway`, `build-admin`, `docker-build-gateway`, `docker-build-admin`
- Graceful shutdown per-service с раздельными сигналами
- Admin API на отдельном порту (8081), доступен внутри сети docker-compose

## Контекст

- Gateway и admin используют общие пакеты `src/internal/{domain,app,ports,adapters,infra,api}` — код не дублируется
- Оба сервиса подключаются к одной БД и одному Valkey
- Gateway — stateless (кроме кэша), может масштабироваться горизонтально
- Admin — stateful через БД (profile CRUD), replica=1 в MVP
- Admin API напрямую доступен внутри docker-сети (не через gateway) для избежания лишнего network hop
- UI embedding остаётся в Go binary (admin), отдельный nginx не вводится
- Текущий `Dockerfile` остаётся как `Dockerfile.admin`, чтобы не ломать существующие CI-пайплайны

## Зависимости

- 90-production-hardening — pprof/debug маршруты, которые должны быть и в admin
- 40-profiles-api — ProfileHandler, DTO, валидация (переиспользуются admin)
- 41-profiles-ui — SPA build, embed.go (переезжает в admin)
- 60-audit-incidents — IncidentHandler (переиспользуется admin)
- 61-observability — OTel, метрики, логирование (оба сервиса)
- 10-gateway-skeleton — Server struct, middleware chain (admin использует свой экземпляр)
- 80-tenant-isolation — tenant auth middleware (оба сервиса)

## Требования

- RQ-001 Система ДОЛЖНА предоставлять два отдельных entrypoint: `src/cmd/gateway/` и `src/cmd/admin/`
- RQ-002 Gateway image ДОЛЖЕН собираться без node-зависимостей и содержать только Go runtime (distroless static)
- RQ-003 Admin image ДОЛЖЕН включать UI build stage и embedded SPA
- RQ-004 Оба binary ДОЛЖНЫ использовать общие модули `src/internal/` без копирования исходников
- RQ-005 Gateway ДОЛЖЕН обслуживать proxy routes: `POST /v1/chat/completions`, `POST /v1/completions`
- RQ-006 Gateway ДОЛЖЕН обслуживать shield endpoint: `POST /api/v1/shield/mask`
- RQ-007 Gateway ДОЛЖЕН обслуживать admin API для automation: `GET/POST/PUT/DELETE /api/v1/profiles/*`, `GET /api/v1/incidents/*`
- RQ-008 Gateway ДОЛЖЕН обслуживать health и metrics: `GET /health`, `GET /ready`, `GET /metrics`
- RQ-009 Admin ДОЛЖЕН обслуживать: SPA static files (catch-all `/*` → `index.html`), admin-only endpoints, `GET /health`, `GET /ready`, `GET /metrics`
- RQ-010 Admin API ДОЛЖЕН быть доступен на порту 8081 напрямую (без прокси через gateway)
- RQ-011 docker-compose ДОЛЖЕН содержать оба сервиса: gateway с `replicas: 2` и admin с `replica: 1`
- RQ-012 При изменении профиля через admin, изменения ДОЛЖНЫ быть немедленно видны через gateway API (единая БД)
- RQ-013 Makefile ДОЛЖЕН содержать цели `build-gateway`, `build-admin`, `docker-build-gateway`, `docker-build-admin`

## Вне scope

- Dashboard/analytics UI для admin
- Bulk import/export профилей
- Версионирование профилей (82-profile-versioning)
- Envoy data plane интеграция
- TLS termination (предполагается reverse proxy)
- Admin RBAC (роли, permissions)
- API gateway между admin и gateway
- WebSocket/SSE прокси для admin UI

## Критерии приемки

### AC-001 Gateway binary собирается без node

- Почему это важно: исключение node из CI пайплайна gateway сокращает время сборки и размер image
- **Given** репозиторий без установленного node.js
- **When** выполняется `make build-gateway` (или `go build -o /dev/null ./src/cmd/gateway/`)
- **Then** сборка завершается успешно, без ошибок о missing node/npm
- Evidence: `make build-gateway` exit code 0, и `go list -deps ./src/cmd/gateway/ | grep -q ui` возвращает non-zero (gateway не импортирует ui-пакет)

### AC-002 Admin binary включает UI

- Почему это важно: admin служит единственной точкой доступа к UI
- **Given** собранный admin binary (`make build-admin`)
- **When** admin запущен и выполняется `GET /` (или любой SPA route)
- **Then** возвращается HTML страница SPA (content-type: text/html, содержит `<div id="root">`)
- Evidence: curl response содержит `<!DOCTYPE html>` и `<div id="root">`

### AC-003 Gateway и admin запускаются из одного docker-compose

- Почему это важно: единый docker-compose для локальной разработки и production
- **Given** `deployments/docker-compose/docker-compose.yml` с gateway + admin сервисами
- **When** выполняется `docker compose up -d` в директории `deployments/docker-compose/`
- **Then** оба контейнера запущены, gateway отвечает на `:8080/health`, admin отвечает на `:8081/health`
- Evidence: `curl -f http://localhost:8080/health && curl -f http://localhost:8081/health` exit code 0

### AC-004 Profile CRUD работает через оба сервиса

- Почему это важно: automation (CI) может создавать профили через gateway, UI — через admin, данные консистентны
- **Given** запущенные gateway и admin, оба с доступом к одной БД
- **When** профиль создаётся через admin API (`POST :8081/api/v1/profiles`) и затем запрашивается через gateway API (`GET :8080/api/v1/profiles/:slug`)
- **Then** gateway возвращает тот же profile, что был создан через admin
- Evidence: response body совпадает

### AC-005 Gateway image не содержит UI файлов

- Почему это важно: минимальный размер image для горизонтального масштабирования gateway
- **Given** собранный gateway image (`docker build -f Dockerfile.gateway -t gateway:test .`)
- **When** выполняется `docker run --rm gateway:test ls /gateway`
- **Then** binary `/gateway` существует и не имеет embedded UI-файлов (размер binary < 30MB)
- Evidence: `docker run --rm gateway:test ls -lh /gateway` показывает размер

### AC-006 Admin image содержит UI

- Почему это важно: admin binary отдаёт SPA без внешнего веб-сервера
- **Given** собранный admin image (`docker build -f Dockerfile.admin -t admin:test .`)
- **When** выполняется `docker run --rm admin:test /admin` и проверяется endpoint `/`
- **Then** возвращается SPA HTML
- Evidence: curl response с `<!DOCTYPE html>`

### AC-007 Graceful shutdown работает для обоих сервисов

- Почему это важно: при rolling update или сбое не теряются in-flight запросы
- **Given** запущенный gateway и admin
- **When** посылается SIGTERM каждому процессу
- **Then** завершение происходит в рамках shutdown timeout, активные запросы успевают завершиться
- Evidence: логи содержат `"server stopping"` и `"server stopped"`, процесс завершается без `exit code 1`

### AC-008 Makefile цели собирают оба binary

- Почему это важно: разработчик собирает нужный binary одной командой
- **Given** Makefile с новыми целями
- **When** выполняются `make build-gateway` и `make build-admin`
- **Then** `bin/gateway` и `bin/admin` существуют и запускаются без ошибок
- Evidence: `bin/gateway --help` и `bin/admin --help` exit code 0

### AC-009 Gateway Dockerfile не содержит node стадии

- Почему это важно: node не должен быть транзитивной зависимостью gateway image
- **Given** `Dockerfile.gateway`
- **When** выполняется grep на `FROM node` или `npm`
- **Then** ни одна строка не содержит node/npm
- Evidence: `grep -c -E 'FROM node|npm' Dockerfile.gateway` возвращает 0

### AC-010 Admin API маршруты не пересекаются с gateway

- Почему это важно: роутинг предсказуем, нет конфликтов при проксировании
- **Given** запущенные gateway и admin
- **When** на gateway и admin посылается запрос `GET /api/v1/profiles`
- **Then** оба возвращают корректный ответ (200), профили из одной БД
- Evidence: `curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/api/v1/profiles` = 200, `curl -s -o /dev/null -w '%{http_code}' http://localhost:8081/api/v1/profiles` = 200

## Допущения

- Admin и gateway работают в одной docker-сети, admin не требует внешнего доступа (проброс порта только для dev)
- Gateway replica count управляется через `docker compose up --scale gateway=N`, дефолт 2
- Profile/incident handler код не дублируется — оба binary импортируют один пакет
- Текущий `Dockerfile` остаётся как `Dockerfile.admin` для обратной совместимости
- Admin не имеет отдельной аутентификации в MVP (использует ту же AdminAuth, что gateway)
- Оба сервиса используют один экземпляр Valkey для кэша и rate limiting
- UI embed стабилен — `embed.FS` не требует изменений

## Критерии успеха

- SC-001 Gateway image size < 30MB (distroless static)
- SC-002 Admin image build time < 2 min (node build + go build)
- SC-003 Gateway start-up time < 500ms (без node, минимум инициализации)
- SC-004 Profile CRUD latency через gateway < 50ms p99 (без дополнительного network hop)

## Краевые случаи

- Gateway запущен без admin — работает proxy и profile API, UI недоступен (ожидаемо)
- Admin запущен без gateway — UI и profile CRUD работают, proxy недоступен (ожидаемо)
- Profile создан через gateway во время недоступности admin — профиль сохранён в БД, при старте admin UI его видит
- Оба сервиса падают одновременно — graceful shutdown не теряет данные (PG транзакции)
- Миграции БД выполняются при старте первого сервиса (gateway или admin), второй пропускает (idempotent)

## Открытые вопросы

- Стоит ли выносить health/ready/metrics роуты в общий shared handler, чтобы не дублировать? — Решается на фазе plan.
- Нужен ли admin свой middleware chain (например, менее строгий rate limit)? — Да, но детали на plan.
- Как быть с build tag'ами (`//go:build gateway || admin`) для опционального кода? — Если дублирование минимально, tag'и не вводить; если profile handlers потянут лишние зависимости — tag'и на plan.
