# Project Foundation Задачи

## Phase Contract

Inputs: plan 00-project-foundation, spec 00-project-foundation.
Outputs: упорядоченные исполнимые задачи с покрытием всех AC.
Stop if: задачи расплывчаты — план чёткий, scope изолирован.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `go.mod`, `go.sum` | T1.1 |
| `src/cmd/gateway/main.go` | T1.2 |
| `src/internal/{domain,app,ports,adapters,infra,api}/` | T1.2 |
| `src/pkg/`, `ui/`, `specs/`, `deployments/`, `docs/`, `examples/`, `bin/` | T1.2 |
| `Makefile` | T2.1 |
| `Dockerfile` | T2.2 |
| `.golangci.yml` | T3.1 |
| `.editorconfig` | T3.2 |
| `.gitignore` | T3.3 |

## Implementation Context

- Цель MVP: пустой Go-проект с DDD-директориями, Makefile, Dockerfile, линтером — `go build ./...` и `make lint` проходят
- Границы приемки: AC-001–006
- Ключевые правила: слои DDD/Clean Architecture фиксированы конституцией; `internal/` не экспортируется; CGO_ENABLED=0 для статической линковки (DEC-003)
- Инварианты данных/домена: нет бизнес-логики; `main.go` только `os.Exit(0)`; никаких external зависимостей
- Контракты/протокол: отсутствуют — фаза без HTTP и без API
- Proof signals: exit code 0 для build/lint/test; `ls -d` для директорий; `docker images` для образа
- Вне scope: HTTP-сервер, конфигурация, middleware, React UI, Docker Compose, CI/CD

## Фаза 1: Инициализация модуля и структуры

Цель: Go-модуль и DDD-директории существуют.

- [x] T1.1 Инициализировать Go-модуль `github.com/bzdvdn/maskchain`. Touches: `go.mod`, `go.sum`
  - `go mod init github.com/bzdvdn/maskchain`
  - `go mod tidy` — фиксация `go.sum`
  - AC-001

- [x] T1.2 Создать DDD/Clean Architecture структуру директорий и пустой entrypoint. Touches: `src/cmd/gateway/main.go`, `src/internal/{domain,app,ports,adapters,infra,api}/.gitkeep`, `src/pkg/.gitkeep`, `ui/.gitkeep`, `specs/active/.gitkeep`, `specs/archived/.gitkeep`, `deployments/docker-compose/.gitkeep`, `docs/.gitkeep`, `examples/.gitkeep`, `bin/.gitkeep`
  - `mkdir -p` всех директорий из секции Scope spec (AC-002)
  - `.gitkeep` в пустых директориях для сохранения в git
  - `src/cmd/gateway/main.go` — `package main; func main() { os.Exit(0) }` (DEC-001)
  - AC-001, AC-002, AC-006 (main.go компилируется)

## Фаза 2: Билд-система и контейнеризация

Цель: Makefile и Dockerfile работают.

- [x] T2.1 Реализовать Makefile с целями build, test, lint, clean, docker-build. Touches: `Makefile`
  - `build`: `go build -o bin/gateway ./src/cmd/gateway/`
  - `test`: `go test ./...`
  - `lint`: check `golangci-lint` exists → run, else инструкция (DEC-002)
  - `clean`: `rm -rf bin/`
  - `docker-build`: check `docker` exists → `docker build -t maskchain/gateway .`
  - AC-003

- [x] T2.2 Реализовать multistage Dockerfile (golang:1.26-alpine + distroless/static). Touches: `Dockerfile`
  - Stage 1 (build): `golang:1.23-alpine`, CGO_ENABLED=0, build binary
  - Stage 2 (runtime): `gcr.io/distroless/static-debian12`, copy binary
  - Static linking `CGO_ENABLED=0` (DEC-003)
  - AC-004

## Фаза 3: Качество кода

Цель: линтер, editorconfig, gitignore настроены.

- [x] T3.1 Настроить `.golangci.yml` с базовым набором линтеров (go vet, staticcheck, errcheck, govet, ineffassign). Touches: `.golangci.yml`
  - run: `golangci-lint run` (молча)
  - AC-005

- [x] T3.2 Создать `.editorconfig` (Go: indent_style=tab, indent_size=8; остальное: indent_style=space, indent_size=2). Touches: `.editorconfig`
  - RQ-007

- [x] T3.3 Создать `.gitignore` (bin/, .idea/, .env, *.exe, *.test, coverage.out). Touches: `.gitignore`

## Фаза 4: Финальная верификация

Цель: доказать, что все AC закрыты.

- [x] T4.1 Выполнить финальную проверку: `go build ./...`, `make build`, `make test`, `make lint`, `make clean`, `make docker-build`, `./bin/gateway`. Touches: все созданные файлы
  - `go build ./...` → exit 0 (AC-001)
  - `make build` → `bin/gateway` exists (AC-003)
  - `make test` → exit 0 (AC-003)
  - `make lint` → exit 0 / "golangci-lint not installed" message (AC-003, AC-005)
  - `make clean` → `bin/` удалена (AC-003)
  - `make docker-build && docker images maskchain/gateway` → образ < 50MB (AC-004)
  - `./bin/gateway` → exit 0 (AC-006)
  - AC-001, AC-003, AC-004, AC-005, AC-006
  - (AC-002 проверен в T1.2)

## Покрытие критериев приемки

- AC-001 -> T1.1, T1.2, T4.1
- AC-002 -> T1.2
- AC-003 -> T2.1, T4.1
- AC-004 -> T2.2, T4.1
- AC-005 -> T3.1, T4.1
- AC-006 -> T1.2, T4.1

## Заметки

- T2.1 и T2.2 можно параллелить после T1.2
- T3.1–T3.3 можно делать параллельно с T2.1–T2.2
- T4.1 — финальная интеграционная проверка, зависит от всех предыдущих
- Никаких external зависимостей в `go.mod` не добавлять
