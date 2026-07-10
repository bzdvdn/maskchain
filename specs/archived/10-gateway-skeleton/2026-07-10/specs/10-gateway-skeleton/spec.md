# Gateway Skeleton

## Scope Snapshot

- In scope: базовый Gin HTTP сервер с graceful shutdown, health endpoints, цепочкой middleware (RequestID, Logger, Recovery, CORS), готовый к подключению бизнес-маршрутов.
- Out of scope: бизнес-логика, DLP инспекция, аутентификация/авторизация, маршрутизация запросов к upstream сервисам.

## Цель

Разработчик инфраструктуры получает готовый к расширению HTTP-сервер на Gin, который при запуске регистрирует health-эндпоинты и middleware, корректно завершает работу по SIGINT/SIGTERM и предоставляет единую точку входа для всего HTTP-трафика шлюза. Успех фичи определяется тем, что сервер запускается, отвечает на `/health`, `/ready`, `/live` и завершается без утечки ресурсов.

## Основной сценарий

1. Стартовая точка: `main.go` вызывает `server.New()` с конфигурацией, содержащей порт и параметры middleware.
2. Основное действие: сервер запускает Gin engine, регистрирует middleware (RequestID, Logger, Recovery, CORS), монтирует health-эндпоинты, начинает слушать порт.
3. Результат: сервер принимает HTTP-запросы, логирует их, добавляет request ID, корректно восстанавливается после паник, обрабатывает CORS.
4. Ошибка/fallback: если порт занят или инициализация не удалась — сервер возвращает ошибку caller'у, `main.go` логирует причину и завершается с exit code 1.

## User Stories

- P1 Story: как разработчик, я хочу запустить `main.go` и получить работающий HTTP-сервер на Gin, который отвечает на `/health` и `/ready`, чтобы я мог убедиться, что сервер жив.
- P2 Story: как разработчик, я хочу видеть request ID, логи запросов и защиту от паник в каждом HTTP-ответе, чтобы отлаживать инциденты в production.

## MVP Slice

Минимальный сервер с graceful shutdown, `/health`, `/ready`, `/live` и middleware Recovery + RequestID. Без Logger middleware и CORS — они добавляются во второй итерации.

## First Deployable Outcome

Собранный бинарник, который можно запустить, проверить `curl http://localhost:8080/health` → `200 OK`, `curl http://localhost:8080/ready` → `200 OK`, и остановить SIGTERM без паники.

## Scope

- Server struct в `src/internal/api/server.go` с методами `New`, `Start`, `Shutdown`
- Middleware: RequestID, Logger, Recovery, CORS
- Endpoints: `GET /health`, `GET /ready`, `GET /live`
- Graceful shutdown через signal.Notify для SIGINT/SIGTERM
- `main.go` в `src/cmd/gateway/main.go`, инициализирующий конфигурацию, создающий и запускающий сервер
- Конфигурация сервера (порт, таймауты) через viper или параметры `New`

## Контекст

- Репозиторий использует DDD + Clean Architecture; сервер размещается в `src/internal/api/` как точка входа HTTP-слоя
- Конституция требует Gin как HTTP-фреймворк
- Graceful shutdown обязателен для enterprise-развёртывания
- Система работает в enterprise-сетях с outbound proxy — CORS middleware нужно для UI
- Zap-логгер уже зафиксирован в `go.mod` как зависимость

## Зависимости

- `github.com/gin-gonic/gin` — необходимо добавить в `go.mod`
- `go.uber.org/zap` — уже в зависимостях
- `github.com/spf13/viper` — уже в зависимостях, для конфигурации
- `github.com/google/uuid` — для генерации request ID

## Требования

- RQ-001 При запуске сервер ДОЛЖЕН слушать TCP-порт, заданный в конфигурации (по умолчанию 8080).
- RQ-002 При получении SIGINT или SIGTERM сервер ДОЛЖЕН выполнить graceful shutdown: перестать принимать новые соединения, дать текущим запросам завершиться в течение таймаута (по умолчанию 10 с), затем закрыться.
- RQ-003 Каждый HTTP-ответ ДОЛЖЕН содержать заголовок `X-Request-ID` с уникальным значением (UUID).
- RQ-004 Каждый входящий HTTP-запрос ДОЛЖЕН логироваться с method, path, status code, duration, request ID.
- RQ-005 Любая паника в хендлере ДОЛЖНА быть перехвачена, залогирована, и клиенту возвращён `500 Internal Server Error`.
- RQ-006 CORS middleware ДОЛЖЕН разрешать cross-origin запросы с любым origin в development-режиме и из заданного списка origin'ов в production.
- RQ-007 `/health` ДОЛЖЕН возвращать `200 OK` с телом `{"status": "ok"}`.
- RQ-008 `/ready` ДОЛЖЕН возвращать `200 OK` с телом `{"status": "ok"}` (готовность к трафику).
- RQ-009 `/live` ДОЛЖЕН возвращать `200 OK` с телом `{"status": "alive"}` (факт работы процесса).

## Вне scope

- Бизнес-маршруты (chat, DLP, proxy) — подключаются в отдельных feature spec
- Аутентификация и авторизация
- Rate limiting
- TLS/HTTPS — добавляется отдельно
- Метрики и tracing (Prometheus, OpenTelemetry)
- Readiness probe с проверкой зависимостей (БД, Valkey) — только факт запуска
- Middleware для тела запроса (размер, валидация)
- Swagger/OpenAPI документация

## Критерии приемки

### AC-001 Запуск и ответ health endpoint

- Почему это важно: базовый smoke test, что сервер работает
- **Given** сервер запущен с конфигурацией по умолчанию
- **When** выполняется `GET /health`
- **Then** возвращается `200 OK` с телом `{"status": "ok"}`
- Evidence: curl-запрос возвращает HTTP 200 и JSON `{"status": "ok"}`

### AC-002 Ready endpoint

- Почему это важно: сигнал для orchestration (K8s probe), что сервер готов принимать трафик
- **Given** сервер запущен
- **When** выполняется `GET /ready`
- **Then** возвращается `200 OK` с телом `{"status": "ok"}`
- Evidence: curl-запрос возвращает HTTP 200 и JSON `{"status": "ok"}`

### AC-003 Live endpoint

- Почему это важно: сигнал для orchestration, что процесс жив
- **Given** сервер запущен
- **When** выполняется `GET /live`
- **Then** возвращается `200 OK` с телом `{"status": "alive"}`
- Evidence: curl-запрос возвращает HTTP 200 и JSON `{"status": "alive"}`

### AC-004 Request ID middleware

- Почему это важно: request ID необходим для трассировки запросов через логи
- **Given** сервер запущен
- **When** выполняется любой запрос к любому endpoint
- **Then** ответ содержит заголовок `X-Request-ID` с валидным UUID
- Evidence: `curl -v` показывает заголовок `X-Request-ID` в ответе

### AC-005 Graceful shutdown

- Почему это важно: предотвращает потерю запросов при рестарте/деплое
- **Given** сервер запущен и обрабатывает запрос
- **When** процесс получает SIGTERM
- **Then** сервер завершает обработку текущих запросов (не более таймаута) и закрывается без паники
- Evidence: после SIGTERM сервер завершается с exit code 0; логи показывают "Shutting down..." и "Server stopped"

### AC-006 Panic recovery

- Почему это важно: паника в хендлере не должна ронять весь сервер
- **Given** сервер запущен
- **When** хендлер вызывает `panic("test")`
- **Then** сервер возвращает `500 Internal Server Error` и продолжает работу
- Evidence: curl на эндпоинт, вызывающий панику, получает 500; следующий запрос к `/health` успешен

### AC-007 Request logging

- Почему это важно: наблюдаемость запросов для отладки и аудита
- **Given** сервер запущен
- **When** выполняется `GET /health`
- **Then** в лог выводится запись с method (`GET`), path (`/health`), status code (`200`), duration, request ID
- Evidence: в stderr/stdout присутствует структурированная запись со всеми полями

### AC-008 CORS middleware

- Почему это важно: UI (React) должен иметь возможность делать запросы к API из браузера
- **Given** сервер запущен с CORS, разрешающим origin `http://localhost:5173`
- **When** выполняется `OPTIONS /health` с заголовком `Origin: http://localhost:5173`
- **Then** ответ содержит `Access-Control-Allow-Origin: http://localhost:5173` и `Access-Control-Allow-Methods`
- Evidence: curl с Origin показывает CORS-заголовки в ответе

## Допущения

- Gin версия будет взята latest compatible с Go 1.26.3
- Request ID генерируется через `google/uuid` v4 (не crypto-safe, достаточно для трассировки)
- CORS в dev-режиме разрешает любой origin (`*`); в production — конфигурируется через список
- По умолчанию сервер слушает порт 8080; конфигурация через viper с ключом `server.port`
- Graceful shutdown таймаут по умолчанию 10 секунд
- Logger middleware использует `go.uber.org/zap`

## Критерии успеха

- SC-001 Сервер стартует за < 500 мс от вызова `Start` до готовности принимать соединения
- SC-002 Graceful shutdown завершается за < 15 с при штатной нагрузке

## Краевые случаи

- Сервер не может запуститься на указанном порту (порт занят) — `Start` возвращает ошибку
- SIGTERM при простое — сервер завершается немедленно и чисто
- Паника в middleware (не в хендлере) — recovery middleware должен перехватить на любом уровне
- Пустой список origin'ов для CORS — CORS middleware не добавляет заголовков (запрет cross-origin)
- Двойной вызов `Shutdown` — безопасен (idempotent)

## Открытые вопросы

- `none`
