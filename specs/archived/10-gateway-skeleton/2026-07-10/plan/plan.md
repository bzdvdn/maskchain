# Gateway Skeleton План

## Phase Contract

Inputs: spec 10-gateway-skeleton, кодовая база с существующим main.go (загрузка config + zap) и пустым `src/internal/api/`.
Outputs: plan, data-model stub.
Stop if: нет — spec однозначна, границы ясны.

## Цель

Создать Gin HTTP Server с middleware и health endpoints. Основная работа: новый пакет `src/internal/api/server.go` (Server struct), middleware в `src/internal/api/middleware/`, обновление `main.go` для вызова `server.Start()` и обработки сигналов. Существующая загрузка config и инициализация логгера остаётся. План безопасен, т.к. server.go — новая точка расширения, не ломающая существующие модули.

## MVP Slice

Минимальный сервер (Recovery + RequestID middleware, health endpoints, graceful shutdown) — покрывает AC-001, AC-002, AC-003, AC-004, AC-005, AC-006.

## First Validation Path

```bash
cd src/cmd/gateway && go run . --config ../../../config.yaml
# в другом терминале:
curl -s http://localhost:8080/health   # → {"status":"ok"}
curl -s http://localhost:8080/ready    # → {"status":"ok"}
curl -s http://localhost:8080/live     # → {"status":"alive"}
curl -sv http://localhost:8080/health 2>&1 | grep X-Request-ID  # → заголовок присутствует
kill -TERM %1                          # graceful shutdown, код 0
```

## Scope

- Новый файл `src/internal/api/server.go` — Server struct с конструктором, Start, Shutdown
- Новый пакет `src/internal/api/middleware/` — файлы: `requestid.go`, `logger.go`, `recovery.go`, `cors.go`, `middleware.go` (опционально barrel)
- Health handler в `src/internal/api/handlers/health.go` (или inline в server.go)
- Обновление `src/cmd/gateway/main.go` — интеграция server.Start + signal handling
- `go.mod` — добавление `github.com/gin-gonic/gin`, `github.com/google/uuid`
- Config: добавление `ServerConfig` в `src/internal/infra/config/config.go` (порт, shutdown timeout, CORS origins)

## Performance Budget

- P99 latency health endpoints: < 5 ms (без бизнес-логики)
- Peak RSS: < 32 MB
- `none` для alloc/op — health endpoints не являются горячим путём

## Implementation Surfaces

| Surface | Статус | Почему участвует |
|---------|--------|-----------------|
| `src/internal/api/server.go` | новая | Server struct — ядро фичи |
| `src/internal/api/middleware/*.go` | новая | Каждый middleware отдельный файл |
| `src/internal/api/handlers/health.go` | новая (опционально) | Health endpoint handler |
| `src/cmd/gateway/main.go` | existing, update | Интеграция сервера |
| `src/internal/infra/config/config.go` | existing, update | ServerConfig |
| `go.mod` | existing, update | Зависимости gin, uuid |

## Bootstrapping Surfaces

- `src/internal/api/middleware/` — создать директорию
- `src/internal/api/handlers/` — создать директорию (или inline health в server.go)

## Влияние на архитектуру

- Server struct становится единственной точкой входа HTTP-слоя — все будущие маршруты регистрируются через него
- Clean Architecture: server.go в `api` слое, слой `ports` остаётся для интерфейсов, слой `app` для use cases
- `Config` расширяется полем `Server` — это обратно совместимо (defaults)

## Acceptance Approach

| AC | Подход | Surfaces | Наблюдение |
|----|--------|----------|------------|
| AC-001 | Запуск + GET /health → 200 + `{"status":"ok"}` | server.go, main.go | curl |
| AC-002 | GET /ready → 200 + `{"status":"ok"}` | server.go | curl |
| AC-003 | GET /live → 200 + `{"status":"alive"}` | server.go | curl |
| AC-004 | Любой запрос → X-Request-ID в ответе | middleware/requestid.go | curl -v |
| AC-005 | SIGTERM → graceful stop | main.go (signal.Notify) | логи + exit code |
| AC-006 | panic в хендлере → 500, сервер жив | middleware/recovery.go | curl + повторный health |
| AC-007 | GET /health → лог с method/path/status/duration/requestID | middleware/logger.go | stderr содержит структ. запись |
| AC-008 | OPTIONS + Origin → CORS headers | middleware/cors.go | curl -v |

## Данные и контракты

- Data model расширяется: `Config.Server` с полями `Port`, `ShutdownTimeout`, `CORSOrigins` (см. `data-model.md`)
- API-контракты: health endpoints (GET, JSON body) — новые, внутренние
- Совместимость: существующий main.go сигнатура не меняется; только добавляется вызов server.Start

## Стратегия реализации

- DEC-001 Middleware в отдельных файлах, не в server.go
  Why: каждый middleware имеет свою логику и тесты; смешивание в server.go создаст >300 строк монолита
  Tradeoff: больше файлов, но чище модульность и изоляция тестов
  Affects: `src/internal/api/middleware/`
  Validation: go build проходит, middleware применяются в правильном порядке

- DEC-002 Health handlers inline в server.go, без отдельного хендлер-пакета
  Why: health endpoints — тривиальные one-liner без бизнес-логики; отдельный пакет избыточен
  Tradeoff: если позже health станет сложным (проверка зависимостей), придётся выносить
  Affects: `src/internal/api/server.go`
  Validation: go build проходит, curl возвращает ожидаемые JSON

- DEC-003 ServerConfig добавляется в существующий Config struct, отдельный файл не создаётся
  Why: конфигурация сервера — часть общей конфигурации приложения; отдельный файл увеличивает связи
  Tradeoff: Config.go может вырасти — но пока это одно поле
  Affects: `src/internal/infra/config/config.go`
  Validation: go build, тесты config не сломаны

- DEC-004 graceful shutdown через signal.Notify в main.go, не внутри Server
  Why: Server.Shutdown — чистая функция остановки; кто вызывает — тот решает, на какой сигнал
  Tradeoff: main.go получает больше ответственности; но это стандартный паттерн в Go
  Affects: `src/cmd/gateway/main.go`
  Validation: SIGTERM → лог "Shutting down..." + exit code 0

- DEC-005 CORS в dev-mode `*`, в production из списка
  Why: спецификация требует; список origins конфигурируется через `server.cors_origins`
  Tradeoff: dev-mode безопасен только в dev; production требует явной конфигурации
  Affects: `src/internal/api/middleware/cors.go`, config.go
  Validation: curl с Origin проверяет поведение

## Incremental Delivery

### MVP (Первая ценность)

- Server struct, health endpoints, RequestID + Recovery middleware, graceful shutdown
- AC-001, AC-002, AC-003, AC-004, AC-005, AC-006
- Валидация: ручной curl + SIGTERM (см. First Validation Path)

### Итеративное расширение

- Logger middleware (AC-007): добавляется во второй итерации, т.к. не блокирует health endpoints
- CORS middleware (AC-008): добавляется, когда UI-разработка потребует cross-origin запросов

## Порядок реализации

1. `src/internal/api/server.go` + health handlers (зависимость: gin добавлен в go.mod)
2. `middleware/requestid.go`, `middleware/recovery.go` (можно параллельно с server.go)
3. Обновление `main.go` + signal handling (зависит от server.go)
4. `middleware/logger.go` (опционально, после MVP)
5. `middleware/cors.go` (опционально, после MVP)

Параллельно: обновление `config.go` + `go.mod` (не зависят от реализации)

## Риски

- Риск: Gin может не быть совместим с go 1.26.3
  Mitigation: проверить go.mod gin-версии при добавлении; fallback — использовать net/http напрямую, но это нарушит конституцию
- Риск: double import при тестировании middleware из server_test.go
  Mitigation: middleware тестируются независимо в своём пакете; server_test.go использует готовый engine

## Rollout и compatibility

- Полностью обратно совместимо: main.go расширяется, существующий код не ломается
- Специальных rollout-действий не требуется
- Наproduction-ready: добавить health-check в orchestration (K8s probe) отдельным тикетом

## Проверка

- `go build ./src/cmd/gateway/` — сборка без ошибок
- `go vet ./src/internal/api/...` — статический анализ
- `go test ./src/internal/api/...` — unit-тесты для сервера и middleware
- Ручной сценарий: `curl` всех health endpoints + SIGTERM
- AC-001..AC-008: каждый сценарий из Acceptance Approach

## Соответствие конституции

- нет конфликтов — Gin, zap, viper, Clean Architecture — всё соответствует
