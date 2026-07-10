# Gateway Skeleton Задачи

## Phase Contract

Inputs: plan 10-gateway-skeleton, data-model.md, spec.md.
Outputs: упорядоченные исполнимые задачи с покрытием AC.
Stop if: нет — plan конкретен, AC привязаны к чётким поверхностям.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `go.mod` | T1.1 |
| `src/internal/infra/config/config.go` | T1.2 |
| `src/internal/api/server.go` | T2.1 |
| `src/internal/api/middleware/requestid.go` | T2.2 |
| `src/internal/api/middleware/recovery.go` | T2.3 |
| `src/cmd/gateway/main.go` | T2.4 |
| `src/internal/api/middleware/logger.go` | T3.1 |
| `src/internal/api/middleware/cors.go` | T3.2 |
| `src/internal/api/server_test.go` | T4.1 |
| `src/internal/api/middleware/*_test.go` | T4.2 |

## Implementation Context

- Цель MVP: Gin сервер с health endpoints, RequestID + Recovery middleware, graceful shutdown (AC-001–AC-006)
- Инварианты/семантика: Server.Shutdown вызывается ровно один раз при SIGTERM/SIGINT; middleware порядок: RequestID → Logger → Recovery → CORS
- Ошибки/коды: порт занят → Start возвращает ошибку; паника → 500 + continue
- Контракты/протокол: `GET /health` → `200 {"status":"ok"}`; `GET /ready` → `200 {"status":"ok"}`; `GET /live` → `200 {"status":"alive"}`; X-Request-ID: UUIDv4; JSON Content-Type
- Границы scope: не делаем бизнес-маршруты, auth, rate limit, TLS, метрики, readiness с проверкой зависимостей
- Proof signals: go build; go test; curl health endpoints; SIGTERM → exit 0; curl -v показывает X-Request-ID
- References: DEC-001 (middleware отдельные файлы), DEC-002 (health inline), DEC-003 (ServerConfig в config.go), DEC-004 (signal.Notify в main.go), DEC-005 (CORS dev/prod), DM (ServerConfig)

## Фаза 1: Bootstrap

Цель: подготовить зависимости и конфигурацию, чтобы дальнейшая реализация не прерывалась на установку пакетов.

- [x] T1.1 Добавить зависимости gin и uuid в go.mod
  Touches: `go.mod`
  Outcome: `go build ./...` проходит, gin и uuid доступны

- [x] T1.2 Добавить ServerConfig в Config struct
  Touches: `src/internal/infra/config/config.go`
  Outcome: Config содержит `Server *ServerConfig` с полями Port, ShutdownTimeout, CORSOrigins; DefaultConfig инициализирует Server с Port=8080, ShutdownTimeout=10; тесты config не сломаны
  References: DM, DEC-003

## Фаза 2: MVP Slice

Цель: минимальный работающий сервер с health endpoints, RequestID, Recovery, graceful shutdown.

- [x] T2.1 Реализовать Server struct с New/Start/Shutdown и health endpoints inline
  Touches: `src/internal/api/server.go`
  Outcome: Server создаётся через `New(cfg *config.ServerConfig, logger *zap.Logger)`; Start запускает Gin на cfg.Port; Shutdown останавливает с cfg.ShutdownTimeout; health endpoints зарегистрированы (GET /health, /ready, /live)
  References: DEC-002

- [x] T2.2 Реализовать RequestID middleware
  Touches: `src/internal/api/middleware/requestid.go`
  Outcome: каждый запрос получает X-Request-ID с UUIDv4; если заголовок already set — не перезаписывается
  References: DEC-001

- [x] T2.3 Реализовать Recovery middleware
  Touches: `src/internal/api/middleware/recovery.go`
  Outcome: паника в хендлере перехватывается, логируется через zap, клиенту возвращается 500; сервер продолжает работу
  References: DEC-001

- [x] T2.4 Обновить main.go — инициализация server и signal handling
  Touches: `src/cmd/gateway/main.go`
  Outcome: main.go создаёт Server через server.New(cfg.Server, logger), запускает через Start в горутине, ожидает SIGINT/SIGTERM через signal.Notify, вызывает Shutdown; логи "Shutting down..." при остановке
  References: DEC-004

## Фаза 3: Расширение (Logger + CORS)

Цель: добавить наблюдаемость (Logger middleware) и поддержку CORS для UI.

- [x] T3.1 Реализовать Logger middleware
  Touches: `src/internal/api/middleware/logger.go`
  Outcome: каждый запрос логируется через zap с method, path, status, duration, request ID; middleware регистрируется после RequestID, до Recovery
  References: DEC-001

- [x] T3.2 Реализовать CORS middleware
  Touches: `src/internal/api/middleware/cors.go`
  Outcome: если cfg.CORSOrigins пуст — заголовки не добавляются; если содержит `*` — разрешён любой origin; иначе — проверка по списку; OPTIONS/preflight обрабатывается
  References: DEC-005

## Фаза 4: Проверка

Цель: automated тесты и verify всех AC.

- [x] T4.1 Написать unit-тесты для Server
  Touches: `src/internal/api/server_test.go`
  Outcome: тесты запускают тестовый сервер, проверяют health endpoints (AC-001, AC-002, AC-003), RequestID header (AC-004), panic recovery (AC-006), и Shutdown error handling

- [x] T4.2 Написать unit-тесты для middleware
  Touches: `src/internal/api/middleware/*_test.go`
  Outcome: тесты каждого middleware изолированно: RequestID генерирует UUID (AC-004), Recovery перехватывает панику (AC-006), Logger пишет все поля (AC-007), CORS возвращает правильные заголовки (AC-008)

- [x] T4.3 Выполнить verify path
  Touches: (нет — ручная проверка)
  Outcome: `go build ./src/cmd/gateway/` проходит; `go vet ./src/internal/api/...` без замечаний; `go test ./src/internal/api/...` — pass; ручной curl-сценарий из First Validation Path успешен
  References: AC-005 проверяется ручным SIGTERM

## Покрытие критериев приемки

- AC-001 -> T2.1, T4.1
- AC-002 -> T2.1, T4.1
- AC-003 -> T2.1, T4.1
- AC-004 -> T2.2, T4.1, T4.2
- AC-005 -> T2.4, T4.3
- AC-006 -> T2.3, T4.1, T4.2
- AC-007 -> T3.1, T4.2
- AC-008 -> T3.2, T4.2

## Заметки

- Порядок: T1.x → T2.x → [T3.x опционально] → T4.x. T1.x и T2.1 можно параллелить. T2.2, T2.3 независимы и могут выполняться параллельно с T2.1. T2.4 зависит от T2.1 + T2.2 + T2.3 (должны существовать). T3.x опциональны — если нужен только MVP, можно пропустить.
- trace-маркеры `@sk-task` над owning declaration (не package/import/file-header)
- Теги для задач: `@sk-task 10-gateway-skeleton#T2.1` и т.д.
