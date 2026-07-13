# Admin Control Plane — План

## Phase Contract

Inputs: spec, inspect (pass), минимальный repo-контекст.
Outputs: plan.md, data-model.md (stub — no-change).
Stop if: spec/inspect конфликтуют. Не конфликтуют.

## Цель

Разделить монолитный gateway на два binary: gateway (data plane, proxy + shield + admin API) и admin (control plane, UI + management API). Gateway image собирается без node (~15MB), admin image с UI. Оба используют общие `src/internal/` модули и одну БД. docker-compose запускает оба сервиса.

## MVP Slice

1. `src/cmd/admin/main.go` — минимальный entrypoint, запускает server на :8081
2. `src/internal/api/admin.go` — Server для admin (отличающиеся middleware, порт)
3. Перенос `ui/embed.go` из gateway в admin
4. `Dockerfile.gateway` — без node-стадии
5. Обновление `Makefile` — 4 новых target
6. Обновление `deployments/docker-compose/docker-compose.yml` — admin сервис

Покрывает: AC-001, AC-002, AC-003, AC-005, AC-006, AC-008, AC-009, AC-010.

## First Validation Path

```bash
cd deployments/docker-compose
docker compose up -d
curl -f http://localhost:8080/health && echo "gateway ok"
curl -f http://localhost:8081/health && echo "admin ok"
curl -s http://localhost:8081/ | head -1 | grep -q '<div id="root">' && echo "SPA ok"
```

## Scope

- `src/cmd/admin/main.go` — новый entrypoint
- `src/internal/api/admin.go` — новый server (дублирует структуру server.go, отличается middleware и порт)
- `ui/embed.go` — перенос: удалить из gateway, оставить только в admin
- `Dockerfile.gateway` — новый (Go → distroless, без node)
- `Dockerfile.admin` — текущий Dockerfile с новым именем
- `Makefile` — добавить 4 target (build-gateway, build-admin, docker-build-gateway, docker-build-admin)
- `deployments/docker-compose/docker-compose.yml` — добавить admin service
- `src/cmd/gateway/main.go` — удалить импорт ui, удалить RegisterStaticFiles

Границы, остающиеся нетронутыми:
- `src/internal/domain/`, `src/internal/app/`, `src/internal/ports/`, `src/internal/adapters/` — общий код, не дублируется
- миграции БД — не меняются
- API контракты — не меняются

## Performance Budget

- Gateway image size < 30MB (distroless static + Go binary)
- Admin image build time < 2 min (node build + Go build)

## Implementation Surfaces

| Surface | Статус | Почему |
|---|---|---|
| `src/cmd/admin/main.go` | NEW | Входная точка admin binary |
| `src/internal/api/admin.go` | NEW | Server с middleware для admin (другой CORS, rate limit, порт) |
| `ui/embed.go` | MODIFIED | Перенос ui пакета — импортируется только admin binary |
| `Dockerfile.gateway` | NEW | Без node; Go build → distroless |
| `Dockerfile` | RENAME | → `Dockerfile.admin` (обратная совместимость через symlink или Makefile alias) |
| `Dockerfile.admin` | RENAMED | Текущий Dockerfile с новым именем |
| `deployments/docker-compose/docker-compose.yml` | MODIFIED | Добавить admin service |
| `Makefile` | MODIFIED | 4 новых target |
| `src/cmd/gateway/main.go` | MODIFIED | Удалить `ui.DistFiles`, `RegisterStaticFiles`, убрать node-зависимости |
| `.github/workflows/*` | OPTIONAL | CI матрица (если есть) |

## Bootstrapping Surfaces

- `src/cmd/admin/` — создать директорию
- `src/internal/api/` — уже существует, добавить `admin.go`

## Влияние на архитектуру

- Локальное: gateway теряет одну зависимость (ui), admin получает собственный Server
- Интеграции: docker-compose получает второй сервис, оба смотрят в одну БД
- Миграция: текущий Dockerfile остаётся как Dockerfile.admin, CI, ломающиеся при rename, ничего не ломается
- Rollout: сначала собрать и запустить admin, потом переключить gateway на новый Dockerfile

## Acceptance Approach

- AC-001: сборка gateway + `go list -deps | grep -q ui` → non-zero
- AC-002: сборка admin + curl GET / → HTML с `<div id="root">`
- AC-003: docker compose up + curl health на обоих портах
- AC-004: POST profile через admin, GET через gateway → одинаковые slug/name/status
- AC-005: docker run gateway image → ls -lh /gateway, размер < 30MB
- AC-006: docker run admin image → curl GET / → HTML
- AC-007: SIGTERM → логи shutdown
- AC-008: make build-gateway && make build-admin → bin/ существуют
- AC-009: grep -E 'FROM node|npm' Dockerfile.gateway → 0
- AC-010: curl :8080/api/v1/profiles и :8081/api/v1/profiles → оба 200

## Данные и контракты

- Data model: не меняется (см. `data-model.md`)
- API контракты: не меняются — те же маршруты, те же DTO, те же ошибки
- Добавляется admin-only endpoint space (порт 8081), но контракты идентичны gateway API

## Стратегия реализации

- DEC-001 Прямой доступ к admin API на :8081 (без прокси через gateway)
  Why: исключает лишний network hop, упрощает отладку, не создаёт точки отказа (gateway не должен падать, чтобы открыть UI)
  Tradeoff: admin должен быть доступен в сети (не public), требуется внутренняя сеть docker-compose
  Affects: `src/internal/api/admin.go`, docker-compose networking
  Validation: AC-010

- DEC-002 Общий код в `src/internal/` без build tags
  Why: build tags усложняют CI и IDE-поддержку; profile/incident handlers используются обоими binary без конфликтов
  Tradeoff: gateway binary будет на ~2-3MB больше из-за admin handler кода; это приемлемо для data plane
  Affects: `src/cmd/gateway/main.go`, `src/cmd/admin/main.go`
  Validation: AC-001 (binary < 30MB), AC-005 (image size)

- DEC-003 NoRoute SPA handler остаётся catch-all (без фильтрации по Accept)
  Why: текущий код `RegisterStaticFiles` перехватывает все NoRoute; фильтрация по Accept — это 104-api-consistency
  Tradeoff: неизвестные API пути будут возвращать SPA вместо 404; риск низкий — admin API пути не меняются
  Affects: `src/internal/api/admin.go`
  Validation: AC-002, AC-010

- DEC-404 Graceful shutdown — существующий механизм в server.go переиспользуется
  Why: admin server копирует структуру gateway server.go, shutdown логика идентична
  Affects: `src/internal/api/admin.go`
  Validation: AC-007

## Incremental Delivery

### MVP (Первая ценность)

- Создать `src/cmd/admin/main.go` (пустой server на :8081)
- Создать `src/internal/api/admin.go` (копия server.go с другим портом и stripped middleware)
- Перенести `ui/embed.go` и `RegisterStaticFiles` в admin
- Удалить ui из gateway
- Создать `Dockerfile.gateway` (Go → distroless)
- Переименовать `Dockerfile` → `Dockerfile.admin`
- Обновить `Makefile`
- Обновить docker-compose
- Покрывает: все AC-*

### Итеративное расширение

- CI матрица: `build-gateway` и `build-admin` в parallel jobs (post-MVP)
- Health/ready/metrics shared handler для admin (если дублирование кода в admin.go заметно)

## Порядок реализации

1. `Dockerfile.gateway` + `Dockerfile.admin` — чтобы сразу иметь собираемые image
2. `src/internal/api/admin.go` — server для admin
3. `src/cmd/admin/main.go` — entrypoint (минимальный DI)
4. Удалить ui из gateway, перенести в admin
5. `Makefile` targets
6. docker-compose admin service
7. Валидация: `make docker-build-gateway && make docker-build-admin && docker compose up`

Параллельно: 2+3 (admin.go и main.go можно писать вместе).

## Риски

- Риск 1: Dupplicated code между server.go и admin.go
  Mitigation: вынести общую middleware цепочку (RequestID, Logger, Recovery, CORS) в shared builder. Если дублирование < 30 строк — оставить как есть, вынести при 105-health-enhancements.
- Риск 2: Сломанные CI-пайплайны из-за rename Dockerfile
  Mitigation: Dockerfile.admin — rename, старый Dockerfile — symlink на Dockerfile.admin или Makefile alias
- Риск 3: Gateway собирается дольше из-за лишних зависимостей
  Mitigation: проверить `go mod tidy` после удаления ui; gateway не должен иметь ui в go.sum

## Rollout и compatibility

- Сначала собрать и запустить admin на порту 8081
- Потом переключить gateway на новый Dockerfile.gateway
- Старый `Dockerfile` НЕ удаляем — оставляем как `Dockerfile.admin` плюс symlink для обратной совместимости
- docker-compose profiles: admin можно добавить в профиль `admin` (не стартует по умолчанию, только явно)

## Проверка

- Automated: `make test` не ломается, `make build-gateway` и `make build-admin` проходят
- Manual:
  1. `docker compose up -d` — оба сервиса стартуют
  2. `curl localhost:8080/health && curl localhost:8081/health` — 200
  3. `curl localhost:8081/ | grep '<div id="root">'` — SPA отдаётся
  4. `curl -X POST localhost:8081/api/v1/profiles -d '{"slug":"test","name":"Test"}'` — 201
  5. `curl localhost:8080/api/v1/profiles/test` — 200, тот же profile
  6. `docker run --rm gateway:test ls -lh /gateway` — size < 30MB
- Подтверждает: AC-001..AC-010, DEC-001..DEC-004

## Соответствие конституции

- нет конфликтов: конституция не накладывает ограничений на количество binary или Dockerfile
