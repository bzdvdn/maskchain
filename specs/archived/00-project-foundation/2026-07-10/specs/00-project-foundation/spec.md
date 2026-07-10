# Project Foundation

## Scope Snapshot

- In scope: инициализация Go-модуля, DDD-структуры директорий, билд-системы, линтеров и Dockerfile для проекта MaskChain.
- Out of scope: любой исполняемый код бизнес-логики, конфигурация приложения, HTTP-сервер.

## Цель

Разработчик получает пустой, но полностью настроенный Go-проект с DDD/Clean Architecture структурой, готовый к добавлению доменной логики. Успех фазы — `go build ./...` проходит без ошибок, `make lint` не выдаёт предупреждений, `make test` проходит, Docker-образ собирается.

## Основной сценарий

1. Разработчик клонирует репозиторий и выполняет `make build`.
2. `go build ./...` компилирует `cmd/gateway/main.go` в бинарник `bin/gateway`.
3. `make lint` запускает golangci-lint, проходит без ошибок.
4. `make test` запускает тесты (пока пустые), возвращает успех.
5. `make docker-build` собирает multistage Docker-образ.
6. При запуске `./bin/gateway` программа завершается с `os.Exit(0)`.

## User Stories

- P1 Story: Разработчик может склонировать репо, выполнить `make build` и получить бинарник.
- P2 Story: Разработчик может выполнить `make docker-build` и получить production-ready Docker-образ.

## MVP Slice

Базовая структура + `go.mod` + пустой `main.go` + Makefile с целями `build`, `lint`, `test`. AC-001, AC-002, AC-003.

## First Deployable Outcome

Пустой бинарник gateway (`bin/gateway`), который завершается сразу после запуска. Можно запустить, замерить время старта (< 100ms), убедиться что бинарник статически слинкован.

## Scope

- Go-модуль `github.com/bzdvdn/maskchain` с `go.mod` и `go.sum`
- Директории DDD/Clean Architecture:
  - `src/cmd/gateway/`
  - `src/internal/domain/`
  - `src/internal/app/`
  - `src/internal/ports/`
  - `src/internal/adapters/`
  - `src/internal/infra/`
  - `src/internal/api/`
  - `src/pkg/`
  - `ui/`
  - `specs/active/`, `specs/archived/`
  - `deployments/docker-compose/`
  - `docs/`, `examples/`, `bin/`
- Makefile с целями: `build`, `test`, `lint`, `clean`, `docker-build`
- `.golangci.yml` — конфигурация линтера
- `.editorconfig` — настройки редактора
- `.gitignore` — игнор артефактов
- Dockerfile multistage: `golang:1.23-alpine` build, `gcr.io/distroless/static-debian12` runtime
- `main.go` в `src/cmd/gateway/main.go`, только `package main` + `func main() { os.Exit(0) }`

## Контекст

- Проект стартует с нуля (greenfield), никакого существующего кода нет.
- Используется Go 1.23, зависимостей пока ноль.
- Docker — для локальной разработки и production-сборки.
- Makefile — единственный entrypoint для разработчика.

## Зависимости

- `none`

## Требования

- RQ-001 Go-модуль `github.com/bzdvdn/maskchain` инициализирован, `go build ./...` успешен.
- RQ-002 Структура директорий повторяет DDD/Clean Architecture согласно конституции.
- RQ-003 Makefile содержит команды `build`, `test`, `lint`, `clean`, `docker-build`.
- RQ-004 `.golangci.yml` настроен на стандартный набор линтеров (go vet, staticcheck, errcheck, govet, ineffassign).
- RQ-005 Dockerfile собирает статический бинарник в multistage сборке.
- RQ-006 `main.go` компилируется в бинарник `bin/gateway` и завершается с `os.Exit(0)`.
- RQ-007 `.editorconfig` задаёт отступы (Go: tabs, YAML/MD: 2 spaces).

## Вне scope

- Любая бизнес-логика, HTTP-сервер, конфигурация, middleware.
- Система сборки для React UI.
- CI/CD конфигурация.
- Docker Compose файлы.

## Критерии приемки

### AC-001 Go-модуль инициализирован и компилируется

- Почему это важно: основа для всего проекта, без неё ни одна фаза не начнётся.
- **Given** пустая директория проекта с `go.mod`
- **When** выполнен `go build ./...`
- **Then** команда завершается успешно, бинарник появляется (если есть `main` package)
- Evidence: exit code 0, файл `bin/gateway` существует

### AC-002 Структура директорий соответствует DDD/Clean Architecture

- Почему это важно: единая архитектурная конвенция для всей команды.
- **Given** корень репозитория
- **When** проверено наличие директорий `src/cmd/gateway/`, `src/internal/{domain,app,ports,adapters,infra,api}/`, `ui/`
- **Then** все указанные директории существуют
- Evidence: `ls -d src/*/` показывает все требуемые директории

### AC-003 Makefile работает

- Почему это важно: единый интерфейс для разработчика.
- **Given** Makefile в корне
- **When** выполнены `make build`, `make test`, `make lint`, `make clean`
- **Then** каждая команда завершается успешно (или с осмысленным сообщением, если нет тестов/линтера)
- Evidence: exit code 0 для каждой цели Makefile

### AC-004 Dockerfile собирает образ

- Почему это важно: контейнеризация для dev和生产.
- **Given** Dockerfile в корне
- **When** выполнен `make docker-build`
- **Then** Docker-образ создан с тегом `maskchain/gateway:latest`
- Evidence: `docker images maskchain/gateway` показывает образ < 50MB

### AC-005 Линтер настроен и проходит

- Почему это важно: качество кода с первого коммита.
- **Given** `.golangci.yml` настроен
- **When** выполнен `make lint` (или `golangci-lint run ./...`)
- **Then** линтер не выводит ошибок (пустой проект проходит)
- Evidence: exit code 0, нет warning/error сообщений

### AC-006 main.go завершается корректно

- Почему это важно: точка входа в приложение.
- **Given** собранный `bin/gateway`
- **When** выполнен `./bin/gateway`
- **Then** процесс завершается с exit code 0, без паники
- Evidence: exit code 0, stdout/stderr пуст

## Допущения

- Go 1.23+ установлен на машине разработчика.
- Docker установлен для сборки образа.
- `golangci-lint` установлен (или будет донастроен в CI).
- Имя модуля `github.com/bzdvdn/maskchain` не будет меняться.

## Критерии успеха

- SC-001 `make build` выполняется за < 5 секунд на свежей системе.
- SC-002 Docker-образ меньше 50 MB (distroless base).
- SC-003 `make docker-build` выполняется за < 30 секунд (при пустом кэше).

## Краевые случаи

- `make lint` без установленного `golangci-lint` — вывод осмысленной ошибки установки.
- Повторный `make build` — бинарник пересобирается, старый удаляется.
- `make clean` удаляет `bin/` и артефакты сборки.
- Docker build с --no-cache — образ всё ещё собирается успешно.

## Открытые вопросы

- `none`
