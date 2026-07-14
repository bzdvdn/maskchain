# Dependency-aware health/readiness probes

## Scope Snapshot

- In scope: замена статических health-эндпоинтов (`/health`, `/ready`, `/live`) на dependency-aware пробы с агрегированным статусом `ok/degraded/down`.
- Out of scope: health-check LLM-провайдеров (существует в `domain/routing/service/health.go`, остаётся независимым); health UI-панель; alerting на основе статуса пробы.

## Цель

Оператор получает возможность различать, жив ли процесс (`/health`), готов ли он принимать трафик (`/ready`) и принял ли он конфиг/инициализировал зависимости (`/live`). Успех определяется по тому, что при недоступности Valkey `/ready` возвращает `degraded`, а при недоступности PG — `down` (5xx), и k8s probe-ы (или load balancer) принимают верное решение.

## Основной сценарий

1. Сервис стартует, загружает конфиг, инициализирует пул PG, клиент Valkey, egress HTTP client.
2. После инициализации всех явных зависимостей `/live` начинает отвечать 200.
3. Каждый вызов `/ready` последовательно опрашивает каждый зарегистрированный probe (PG, Valkey, egress), собирает latency.
4. Если все probes вернули `ok` — общий статус `ok`.
5. Если хотя бы один не-критический probe не `ok` — общий статус `degraded`.
6. Если хотя бы один критический probe не `ok` — общий статус `down`, HTTP 503.
7. `/health` всегда 200 — процесс жив независимо от состояния зависимостей.

## User Stories

- P1: Оператор видит в k8s `readinessProbe`, что Pod не готов (PG недоступен), и трафик не направляется на него.
- P2: Оператор видит `degraded` при недоступности Valkey и понимает, что rate limiting временно отключён, но основной функционал работает.

## MVP Slice

AC-001, AC-005, AC-008 — статические эндпоинты с корректными сигналами (liveness всегда ok, startup — после init). Затем AC-002, AC-003, AC-004, AC-006, AC-007 — dynamic readiness.

## First Deployable Outcome

После первого implementation pass можно запустить сервис, вызвать `GET /health` (200), `GET /live` (200 после init), `GET /ready` (ok/degraded/down в зависимости от доступности PG/Valkey). Все три эндпоинта возвращают структурированный JSON. Результат проверяется `curl`-ом без поднятия PostgreSQL или Valkey (смоки-тест).

## Scope

- Реализация probe-интерфейса для PG (`SELECT 1`), Valkey (`PING`), egress (TCP dial к базовым URL провайдеров).
- Новый конфигурационный блок `server.health_check` с полем `critical_deps`.
- Замена `healthHandler` в `Server` и `AdminServer` на динамические обработчики.
- Response-формат с per-check статусом и latency.
- Auth bypass для всех трёх эндпоинтов (уже есть, сохранить).
- Unit-тесты с mock probes.
- Trace-маркер `@sk-task` на owning функциях/типах.

## Контекст

- Текущие хендлеры в `server.go:46-48` и `admin.go:47-49` используют общую `healthHandler`, возвращающую статический JSON.
- PG pool создаётся в `postgres/pool.go` с `Ping` при старте. Повторный Ping через существующий пул возможен.
- Valkey client создаётся в `main.go` (gateway/admin) без Ping на старте. Возможность `client.Do(ctx, "PING")` есть.
- Provider `HealthChecker` в `domain/routing/service/health.go` уже реализует похожую логику для LLM-провайдеров, но не используется в `main.go`.
- `HealthChecker` для провайдеров остаётся отдельной фичей — не смешивать с app-level probes.
- Auth bypass: `/health`, `/ready`, `/live` уже в `publicPaths` — не менять.

## Зависимости

- Зависит от существующего `*pgxpool.Pool` и `*valkey.Client` — интерфейсы для probe должны приниматься через DI.
- Зависит от конфигурации `server.health_check.critical_deps` (добавить в `ServerConfig`).
- `none` внешних сервисных зависимостей — все проверки используют уже инициализированные соединения.

## Требования

- RQ-001 GET /health ДОЛЖЕН всегда возвращать HTTP 200 с `{"status":"ok"}`, независимо от состояния зависимостей.
- RQ-002 GET /ready ДОЛЖЕН опрашивать все зарегистрированные probes (PG, Valkey, egress) и возвращать агрегированный статус.
- RQ-003 GET /live ДОЛЖЕН возвращать HTTP 200 после того, как сервис загрузил конфиг и инициализировал все явные зависимости (PG pool, Valkey client, egress client).
- RQ-004 Система ДОЛЖНА классифицировать общий статус как `ok` (все probes healthy), `degraded` (не-критичный probe не healthy), `down` (критичный probe не healthy; HTTP 503).
- RQ-005 Каждый check в ответе ДОЛЖЕН содержать `status` и `latency_ms`.
- RQ-006 Конфигурационное поле `server.health_check.critical_deps` ДОЛЖНО определять, какие dependency names считаются критическими и вызывают статус `down`.
- RQ-007 Эндпоинты `/health`, `/ready`, `/live` ДОЛЖНЫ оставаться публичными (не требовать tenant-аутентификации).

## Вне scope

- Health-чекинг LLM-провайдеров (существует отдельно в `domain/routing/service/health.go`, не дублировать).
- Health-дашборд/UI.
- Prometheus метрики по health checks (могут быть добавлены отдельной фичой).
- Настройка интервалов probe-запросов — пробы вызываются синхронно при HTTP-запросе.
- Graceful degradation конкретных фич при недоступности Valkey — spec только про probe-сигнал.

## Критерии приемки

### AC-001 Liveness endpoint всегда ok

- Почему это важно: k8s livenessProbe должен видеть процесс живым независимо от состояния БД/кэша.
- **Given** работающий сервер (PG и Valkey могут быть недоступны)
- **When** выполняется GET /health
- **Then** ответ HTTP 200 с телом `{"status":"ok"}`
- Evidence: curl -f http://localhost:8080/health возвращает exit 0 и JSON с `"status":"ok"`

### AC-002 Readiness возвращает ok при всех здоровых зависимостях

- Почему это важно: сигнал k8s readinessProbe, что сервис может принимать трафик.
- **Given** PG pool отвечает на SELECT 1, Valkey отвечает на PING, egress TCP до провайдеров успешен
- **When** выполняется GET /ready
- **Then** ответ HTTP 200 с `{"status":"ok","checks":{"database":{"status":"ok","latency_ms":<число>},"valkey":{"status":"ok","latency_ms":<число>},"egress":{"status":"ok","latency_ms":<число>}}}`
- Evidence: curl -s http://localhost:8080/ready | jq .status выводит "ok"

### AC-003 Readiness возвращает degraded при недоступности не-критичной зависимости

- Почему это важно: Valkey — не-критичная зависимость (rate limiting отключается, но трафик проходит), оператор должен видеть degraded, а не down.
- **Given** критическая зависимость (PG) доступна, но Valkey не отвечает на PING
- **When** выполняется GET /ready
- **Then** ответ HTTP 200 с `{"status":"degraded","checks":{"database":{"status":"ok","latency_ms":<число>},"valkey":{"status":"down","error":"<описание>"}}}}
- Evidence: curl -s http://localhost:8080/ready | jq -r .status выводит "degraded"

### AC-004 Readiness возвращает down при недоступности критической зависимости

- Почему это важно: k8s readinessProbe должен убрать Pod из rotation, если PG (core domain) недоступен.
- **Given** PG pool не отвечает на SELECT 1 (или возвращает ошибку), Valkey в любом состоянии
- **When** выполняется GET /ready
- **Then** ответ HTTP 503 с `{"status":"down","checks":{"database":{"status":"down","error":"<описание>"}}}`
- Evidence: curl -w "%{http_code}" -s -o /dev/null http://localhost:8080/ready выводит 503

### AC-005 Startup probe отражает успешную инициализацию

- Почему это важно: k8s startupProbe должен ждать завершения инициализации зависимостей перед readiness-чеком.
- **Given** сервис только что запущен, инициализация PG pool и Valkey client завершена
- **When** выполняется GET /live
- **Then** ответ HTTP 200 с `{"status":"ok"}`
- Evidence: curl -f http://localhost:8080/live возвращает JSON с `"status":"ok"`

### AC-006 Конфигурация critical_deps управляет порогом down/degraded

- Почему это важно: оператор должен иметь возможность переопределить, какие зависимости считать критическими, без изменения кода.
- **Given** `server.health_check.critical_deps: ["database", "egress"]` в конфиге
- **When** Valkey недоступен, но PG и egress доступны
- **Then** GET /ready возвращает HTTP 200 с `"status":"ok"` (Valkey не в critical_deps)
- **When** egress недоступен, но PG доступен
- **Then** GET /ready возвращает HTTP 503 с `"status":"down"` (egress в critical_deps)
- Evidence: curl -s http://localhost:8080/ready последовательно показывает "ok" и "down"

### AC-007 Формат ответа един для всех эндпоинтов

- Почему это важно: единый контракт для consumer-ов (k8s, LB, оператор).
- **Given** любой из эндпоинтов `/health`, `/ready`, `/live`
- **When** ответ парсится как JSON
- **Then** корневой ключ `"status"` присутствует; для `/ready` также присутствует `"checks"` с объектом, где каждый ключ — имя зависимости, значение — `{"status":"<ok|down>","latency_ms":<число>}`; поле `latency_ms` — целое число миллисекунд
- Evidence: jq eval с проверкой структуры проходит для всех трёх эндпоинтов

### AC-008 Health endpoints остаются публичными

- Почему это важно: k8s probe-ы не имеют tenant-токена, auth bypass должен сохраниться.
- **Given** запрос без заголовка авторизации
- **When** GET /health, /ready, /live
- **Then** ответ HTTP 200 (не 401)
- Evidence: curl без headers возвращает 200 для всех трёх путей

## Допущения

- Valkey считается не-критичной зависимостью по умолчанию (rate limiting и кэш маски/словарей могут временно отсутствовать).
- PG считается критической зависимостью (профили, shield-конфигурация, инциденты — core domain).
- Egress проба — TCP dial к базовым URL всех настроенных LLM провайдеров с timeout (наследуется из существующего egress config).
- При отсутствии `server.health_check.critical_deps` значение по умолчанию: `["database"]`.
- `/live` устанавливается в ok сразу после вызова конструктора Server/AdminServer (все зависимости уже переданы через DI).

## Критерии успеха

- SC-001 GET /ready latency < 100ms при всех доступных зависимостях (PG local, Valkey local, 3 провайдера).
- SC-002 Отсутствие false-positive degraded/down при нормальной работе (0 false срабатываний за 24ч в staging).

## Краевые случаи

- Все зависимости не настроены (PG/Valkey addr пуст) — probe пропускается или возвращает "not configured" со статусом "ok".
- Таймаут probe-запроса — статус "down" для этой зависимости.
- Egress провайдеров нет в конфиге — egress probe не регистрируется.
- `critical_deps` содержит имя зависимости, которая не зарегистрирована — игнорировать (log warning).
- Параллельные запросы к /ready — каждый запрос выполняет probes независимо (no shared mutable state между запросами).

## Открытые вопросы

- Должен ли egress probe проверять все провайдеры последовательно или достаточно первого успешного? — решается на фазе plan. Пока: проверяем все, статус "ok" если хотя бы один доступен.
- Формат error сообщения при недоступности — что включать (таймаут, connection refused, DNS)? — решается на фазе plan.
