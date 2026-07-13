# 101: Gateway Diet — План

## Phase Contract

Inputs: spec (101-gateway-diet), inspect (pass).
Outputs: plan.md, data-model.md (no-change).
Stop if: spec неоднозначна — нет, spec чёткая.

## Цель

Убрать `RegisterStaticFiles` из `Server` struct (пакет `api`), закрепить build tags `gateway`/`admin` как взаимоисключающие, обновить Makefile. Domain/adapters/infra не меняются. Profile/incident хендлеры остаются общими. Изменения только в API-слое и сборке.

## MVP Slice

1. Удалить `RegisterStaticFiles` из `Server` в `server.go` — она остаётся только на `AdminServer` в `admin.go`.
2. Очистить импорты `server.go` от `io/fs` и `strings`.
3. Makefile: `build-gateway` → `-tags gateway`, `build-admin` → `-tags admin`.
4. Проверить: `go build -tags gateway ./src/cmd/gateway/` успешен, `ui/` не импортируется.

Покрывает: AC-001, AC-005, AC-006.

## First Validation Path

```sh
make build-gateway && ./bin/gateway --help
make build-admin && ./bin/admin --help
go tool nm bin/gateway | grep 'ui/'    # → пусто
```

## Scope

- `src/internal/api/server.go` — удалить `RegisterStaticFiles`, очистить импорты
- `Makefile` — build-gateway/build-admin с build tags
- `Dockerfile.gateway` — проверить CGO_ENABLED=0 (уже есть)
- Никаких изменений: domain, adapters, infra, ports, ui/, cmd/gateway/main.go, cmd/admin/main.go

## Performance Budget

- Gateway image <= 55MB (SC-002)
- Startup < 100ms (SC-001)
- `none` для alloc/op — изменения не затрагивают hot path

## Implementation Surfaces

| Surface | Изменение |
|---|---|
| `src/internal/api/server.go` | Удалить метод `RegisterStaticFiles`, убрать `io/fs` и `strings` из imports |
| `Makefile` | `build-gateway`: +`-tags gateway`; `build-admin`: +`-tags admin` |

## Bootstrapping Surfaces

- `none` — все нужные файлы существуют

## Влияние на архитектуру

- Локальное: один метод уходит из `Server` struct
- Никакого влияния на интеграции, контракты, data model
- Совместимость: `AdminServer.RegisterStaticFiles` остаётся без изменений

## Acceptance Approach

| AC | Подход | Surfaces | Наблюдение |
|---|---|---|---|
| AC-001 | `go build -tags gateway` + `go tool nm` проверка | Makefile, server.go | `nm` не показывает `ui/` символов |
| AC-002 | time-to-ready замер | (runtime) | curl /health после старта <100ms |
| AC-003 | `docker build` gateway | Dockerfile.gateway | image size <= 20MB |
| AC-004 | `docker build` admin + curl / | Dockerfile.admin | HTML, не 404 |
| AC-005 | `go build -tags gateway` + `go vet` | Makefile, server.go | оба успешны |
| AC-006 | `make build-gateway && make build-admin` | Makefile | `bin/gateway` и `bin/admin` существуют |

## Данные и контракты

- Data model: без изменений (см. `data-model.md`)
- API-контракты: без изменений
- Никаких новых зависимостей

## Стратегия реализации

### DEC-001 Build tag convention: `gateway`/`admin` взаимоисключающие

- Why: явные тэги понятнее, чем `!admin` по умолчанию; при сборке без тэга можно явно потребовать указать target
- Tradeoff: нужно указать тэг всегда (Makefile скрывает это от разработчика)
- Affects: Makefile, CI scripts
- Validation: `go build -tags gateway ./src/cmd/gateway/` и `go build -tags admin ./src/cmd/admin/` оба успешны

### DEC-002 `RegisterStaticFiles` удаляется из `Server`, а не прячется за build tag

- Why: `Server` не должен знать об `io/fs` и статике; единственный caller — `AdminServer`
- Tradeoff: если future feature захочет добавить статику в gateway — вернёт метод осознанно
- Affects: `src/internal/api/server.go`
- Validation: `go build -tags gateway ./src/cmd/gateway/` не требует `io/fs`

## Incremental Delivery

### MVP (Первая ценность)

Удалить `RegisterStaticFiles` из `Server`, обновить Makefile, проверить сборку.
AC: AC-001, AC-005, AC-006.

### Итеративное расширение

- AC-003: проверить Docker image gateway
- AC-004: проверить Docker image admin
- AC-002: замерить startup time

## Порядок реализации

1. `server.go` — удалить метод и импорты (самый дешёвый шаг)
2. `Makefile` — добавить build tags в targets
3. Сборка и проверка AC-001/AC-005/AC-006
4. Dockerfile проверки AC-003/AC-004
5. Startup time замер AC-002

Шаги 1-2 можно делать в одном коммите. Шаги 3-5 — параллельные проверки.

## Риски

- **Риск 1**: Gateway собран без `-tags gateway` может нечаянно импортировать `ui/` через будущие изменения. *Mitigation:* CI должен явно использовать `make build-gateway`, а не `go build ./src/cmd/gateway/`.
- **Риск 2**: Startup time > 100ms из-за PG/Valkey подключения. *Mitigation:* AC-002 замеряется с минимальным конфигом (без БД); это уже документировано в spec.
- **Риск 3**: `go vet` может жаловаться на неиспользуемые импорты после удаления `RegisterStaticFiles`. *Mitigation:* предварительно проверить, не используется ли `io/fs`/`strings` в других методах `server.go`.

## Rollout и compatibility

- Специальных rollout-действий не требуется
- `RegisterStaticFiles` на `Server` сейчас не вызывается нигде (admin использует `AdminServer`)
- После изменения gateway бинарник становится чище, но API-поведение не меняется

## Проверка

- `go build -tags gateway ./src/cmd/gateway/` — успех (AC-001, AC-005)
- `go vet -tags gateway ./src/cmd/gateway/` — чист (AC-005)
- `make build-gateway && make build-admin` — оба бинарника созданы (AC-006)
- `go tool nm bin/gateway | grep 'ui/'` — пусто (AC-001)
- `docker build -f Dockerfile.gateway ...` + size check (AC-003)
- `docker build -f Dockerfile.admin ...` + curl / (AC-004)
- `time` замер startup (AC-002)

## Соответствие конституции

- нет конфликтов
