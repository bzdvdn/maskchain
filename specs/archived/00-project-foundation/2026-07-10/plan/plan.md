# Project Foundation План

## Phase Contract

Inputs: spec 00-project-foundation, constitution summary.
Outputs: plan.md, data-model.md (no-change stub).
Stop if: spec неоднозначна — spec чёткая, scope изолирован.

## Цель

Создать пустой, но полностью настроенный Go-проект с DDD-структурой, билд-системой, линтерами и Dockerfile. Никакой бизнес-логики. Разработчик клонирует репо и сразу получает рабочий `make build` → `bin/gateway`.

## MVP Slice

Весь spec — это один неделимый MVP. Все AC (1–6) должны быть закрыты за один проход, так как структура без Makefile или Makefile без Dockerfile не дают независимой ценности. AC-003 (Makefile) и AC-005 (линтер) можно валидировать первыми.

## First Validation Path

```bash
make build    # → bin/gateway
./bin/gateway # → exit 0
make lint     # → no errors
make test     # → ok
make docker-build && docker images maskchain/gateway
```

## Scope

- Создание директорий согласно DDD/Clean Architecture (из секции Scope spec)
- `go.mod` + `go.sum` (module github.com/bzdvdn/maskchain)
- `src/cmd/gateway/main.go` — пустой entrypoint
- Makefile: build, test, lint, clean, docker-build
- `.golangci.yml` — стандартные линтеры
- `.editorconfig` — Go tabs, остальное 2 spaces
- `.gitignore` — bin/, .idea/, .env, *.exe
- Dockerfile (multistage: golang:1.23-alpine → distroless/static)
- Никаких go-файлов кроме `main.go`, никаких external зависимостей

## Performance Budget

- `none` — фаза не содержит исполняемой логики. Единственный criterion: образ < 50 MB (SC-002).

## Implementation Surfaces

Все surfaces — новые (greenfield):

| Surface | Зачем | Тип |
|---------|-------|-----|
| `src/cmd/gateway/main.go` | Entrypoint бинарника | новый |
| `src/internal/{domain,app,ports,adapters,infra,api}/` | DDD/Clean Architecture слои | новый |
| `src/pkg/` | Публичные библиотеки | новый |
| `ui/` | React frontend (пустая директория) | новый |
| `specs/active/`, `specs/archived/` | Spec-артефакты | новый |
| `deployments/docker-compose/` | Docker Compose (пустая) | новый |
| `docs/`, `examples/`, `bin/` | Документация, примеры, артефакты | новый |
| `go.mod` | Go-модуль | новый |
| `Makefile` | Билд-система | новый |
| `.golangci.yml` | Линтер | новый |
| `.editorconfig` | Редактор | новый |
| `.gitignore` | Игнор-файл | новый |
| `Dockerfile` | Контейнеризация | новый |

## Bootstrapping Surfaces

Все директории и корневые конфигурационные файлы создаются до любых исходников (кроме `main.go`, который минимален). Порядок: go.mod → Makefile → директории → main.go → Dockerfile → linter config → editorconfig → gitignore.

## Влияние на архитектуру

- Закладывается структура для всех будущих фаз. Любые изменения DDD-структуры после этой фазы — scope violation.
- `internal/` пакеты не экспортируются за пределы модуля (Go convention), `pkg/` — для потенциально переиспользуемых публичных компонентов.
- `ui/` на одном уровне с `src/` — frontend изолирован от backend.

## Acceptance Approach

- AC-001: создать `go.mod`, `main.go` → `go build ./...` → проверить exit code и наличие `bin/gateway`
- AC-002: создать все директории → `ls -d` проверить каждую
- AC-003: реализовать Makefile → `make build`, `make test`, `make lint`, `make clean` — каждый exit 0
- AC-004: реализовать Dockerfile → `make docker-build` → проверить `docker images maskchain/gateway` и размер
- AC-005: настроить `.golangci.yml` → `make lint` → exit 0
- AC-006: `go build ./...` → `./bin/gateway` → exit 0

## Данные и контракты

- Data model не затрагивается — ни одна БД/схема/миграция не создаётся.
- API-контракты отсутствуют — HTTP-сервер не в scope.
- `data-model.md` — stub no-change.

## Стратегия реализации

- DEC-001 Одна директория на DDD-слой (без вложенных пакетов)
  Why: greenfield, слои фиксированы конституцией, вложенность появится в следующих фазах с реальными сущностями. Избыточная вложенность сейчас = пустые пакеты без файлов.
  Tradeoff: при добавлении первой бизнес-логики придётся создать вложенные пакеты внутри domain/. Это нормально — паттерн grow-as-you-go.
  Affects: все `src/internal/*/` директории
  Validation: `ls -d src/internal/*/` показывает ровно 6 директорий

- DEC-002 Makefile использует shell-команды без внешних инструментов (кроме golangci-lint)
  Why: минимум зависимостей для разработчика. `go build`, `go test`, `go vet` — встроенные. `golangci-lint` — единственный external, но опционален (make lint проверяет наличие).
  Tradeoff: `make docker-build` требует Docker. Это константа — проект контейнеризирован.
  Affects: `Makefile`
  Validation: `make build` без установленного golangci-lint не падает

- DEC-003 Multistage Dockerfile: golang:1.23-alpine → distroless/static
  Why: distroless — минимальный образ (~5 MB base + бинарник), alpine на этапе build для удобства (apk add). Статическая линковка (CGO_ENABLED=0) для совместимости с любым runtime.
  Tradeoff: CGO_ENABLED=0 исключает cgo-зависимости. Если будущие фазы потребуют CGO (напр., SQLite), Dockerfile придётся менять. Пока CGO не нужен.
  Affects: `Dockerfile`
  Validation: образ < 50 MB, бинарник статический (`file bin/gateway`)

## Incremental Delivery

### MVP (Первая ценность)

Все AC-001–006 одним блоком. Критерий: `make build && make lint && make test && make docker-build` проходят. Время проверки: < 1 минута.

### Итеративное расширение

Не применимо — фаза атомарна.

## Порядок реализации

1. `go mod init github.com/bzdvdn/maskchain` — модуль
2. Создать все директории (`mkdir -p`)
3. `src/cmd/gateway/main.go` — entrypoint
4. `Makefile` — build, test, lint, clean, docker-build
5. `Dockerfile` — multistage
6. `.golangci.yml`, `.editorconfig`, `.gitignore`
7. `go mod tidy` + финальная проверка `go build ./...`

Параллельно: `.editorconfig` и `.gitignore` не зависят от остального.

## Риски

- Риск 1: `golangci-lint` не установлен у разработчика
  Mitigation: `make lint` проверяет наличие (`which golangci-lint`) и выдаёт понятную инструкцию по установке, не падает с ошибкой
- Риск 2: docker build падает из-за отсутствия Docker
  Mitigation: `make docker-build` проверяет `docker` availability, fallback с сообщением
- Риск 3: несоответствие структуры директорий между AC-002 и реальностью
  Mitigation: зафиксировать точный список в Makefile цель `check-structure` (lint-подобная)

## Rollout и compatibility

- Специальных rollout-действий не требуется. Фаза не имеет обратной совместимости — это первый коммит в репозиторий.

## Проверка

- `go build ./...` → exit 0 (AC-001, AC-006)
- `ls -d` проверка директорий (AC-002)
- `make {build,test,lint,clean}` → exit 0 (AC-003, AC-005)
- `make docker-build && docker images maskchain/gateway` (AC-004)
- `file bin/gateway` → статический бинарник (DEC-003)

## Соответствие конституции

- нет конфликтов. Конституция требует DDD/Clean Architecture структуру — план её создаёт. Языковая политика (docs=ru, comments=en) соблюдена.
