# 101: Gateway Diet — Minimal proxy-only binary

## Scope Snapshot

- In scope: разделить сборку gateway и admin так, чтобы gateway binary не содержал `ui/` embed, собирался без node/CGO и стартовал за <100ms, а admin — включал UI.
- Out of scope: рефакторинг domain-слоя, изменение бизнес-логики shield, маршрутизации или API-контрактов.

## Цель

Разработчик инфраструктуры и CI/CD получает два независимых бинарника: минимальный gateway (~50MB, старт <100ms) для data-plane без лишних зависимостей и admin для control-plane с UI. Gateway должен собираться без node, CGO, и не импортировать `ui/`. Успех измеряется чистотой `go build -tags gateway ./src/cmd/gateway/` и размером финального Docker image. Profile/incident хендлеры остаются общими для обоих бинарников.

## Основной сценарий

1. Gateway собирается через `make build-gateway` (или `make docker-build-gateway`) без node, без CGO.
2. Admin собирается через `make build-admin` (или `make docker-build-admin`) с node-стадией для UI.
3. Gateway содержит: proxy-роуты, shield/mask, profile/incident CRUD, health/ready/live, metrics, debug (pprof), tenant auth.
4. Admin содержит: всё то же, что gateway + UI static files.
5. Попытка импортировать `ui/` в gateway завершается ошибкой сборки на уровне build tag.

## User Stories

- P1 Story: Оператор может развернуть gateway отдельно от admin, не таская за собой node и UI-зависимости.
- P2 Story: Разработчик может собрать admin отдельно, включив UI, не влияя на gateway.

## MVP Slice

Gateway собирается с `-tags gateway`, не импортирует `ui/`. Admin собирается с `-tags admin` (или default), включает UI. Profile/incident хендлеры — общие для обоих. `AC-001`, `AC-005`, `AC-006` — первичная проверка.

## First Deployable Outcome

`make build-gateway` успешно собирает бинарник, который запускается и отвечает на /health. `make build-admin` успешно собирает бинарник, который раздаёт UI. Gateway может быть развёрнут независимо.

## Scope

- Разделение `src/internal/api/server.go` и `src/internal/api/admin.go` с build tags: `ui/` embed — только в admin-сборку
- `RegisterStaticFiles` остаётся только на `AdminServer` (за build tag `admin`)
- Makefile target `build-gateway` с `-tags gateway` и `build-admin` (сборка UI + `-tags admin`)
- Dockerfile.gateway без node-стадии (distroless static, ~50MB)
- Gateway без CGO
- Verification через `go vet`/`go build` с соответствующими тэгами

## Контекст

- Gateway и admin используют общий domain/adapters/infra — изменения только на уровне composition root и API-слоя
- `ui/embed.go` уже вынесен из gateway (100-admin-control-plane), но `Server` struct всё ещё содержит `RegisterStaticFiles`
- Gateway должен сохранить существующий proxy-флоу и shield/mask API
- build tag изоляция не должна требовать дублирования domain-кода

## Зависимости

- `specs/active/100-admin-control-plane/` — ui embed уже перенесён, gateway не импортирует ui/
- Никаких новых внешних зависимостей

## Требования

- RQ-001 Gateway binary НЕ ДОЛЖЕН импортировать `ui/` пакет при сборке с `-tags gateway`
- RQ-002 Admin binary ДОЛЖЕН включать UI (embed.FS)
- RQ-003 Gateway обязан собираться без CGO, без node, минимальный image — distroless static, ~50MB
- RQ-004 Gateway обязан стартовать и быть готов к приёму соединений за <100ms (при пустой конфигурации или минимальной)
- RQ-005 Makefile ДОЛЖЕН содержать targets: build-gateway, build-admin, docker-build-gateway, docker-build-admin

## Вне scope

- Рефакторинг domain/adapters/infra/ports слоёв
- Изменение логики shield/mask, маршрутизации, tenant auth, rate limiting
- Изменение API-контрактов (request/response остаются теми же)
- Оптимизация admin binary (размер admin не критичен)
- Удаление `RegisterProfileHandler` / `RegisterIncidentHandler` из `Server` struct (они остаются общими)
- Fallback-режим gateway при отказе admin (gateway не дублирует admin-функции)

## Критерии приемки

### AC-001 Gateway не импортирует ui/ пакет

- Почему это важно: гарантирует, что gateway не тащит node-зависимости и лишний embed-код
- **Given** чистая рабочая копия репозитория
- **When** выполняется `go build -tags gateway ./src/cmd/gateway/`
- **Then** сборка успешна, и `go tool nm` на полученном бинарнике не показывает символов из `ui/`
- Evidence: `go tool nm bin/gateway | grep 'ui/'` возвращает пустой вывод

### AC-002 Gateway binary стартует <100ms

- Почему это важно: data-plane обязан минимально влиять на latency при перезапусках и scale-from-zero
- **Given** собранный gateway бинарник и валидный минимальный конфиг
- **When** gateway запускается и начинается прослушивание порта
- **Then** time-to-ready (от exec до ответа 200 на /health) не превышает 100ms
- Evidence: `timeout 5 ./bin/gateway --config testdata/minimal.yaml & sleep 0.2 && curl -f http://localhost:8080/health && kill %1` — curl возвращает 200

### AC-003 Gateway Docker image ~50MB (distroless static)

- Почему это важно: минимальный image ускоряет pull/deploy в кластерах и уменьшает surface для атак
- **Given** Dockerfile.gateway в корне репозитория
- **When** выполняется `docker build -f Dockerfile.gateway -t gateway:test .`
- **Then** финальный image использует `gcr.io/distroless/static-debian12` (или аналогичный static base) и его размер не превышает 55MB
- Evidence: `docker images gateway:test --format '{{.Size}}'` и проверка base image в Dockerfile

### AC-004 Admin image включает UI

- Почему это важно: admin без UI бесполезен для оператора
- **Given** Dockerfile.admin в корне репозитория
- **When** выполняется `docker build -f Dockerfile.admin -t admin:test .`
- **Then** admin binary содержит встроенный UI (embed.FS) и раздаёт его по /
- Evidence: `docker run admin:test` и curl к `/` возвращает HTML (не 404)

### AC-005 `go build -tags gateway` не ссылается на admin-код

- Почему это важно: гарантирует, что компилятор с tag gateway не захватывает admin-файлы
- **Given** clean checkout
- **When** выполняется `go build -tags gateway ./src/cmd/gateway/`
- **Then** команда завершается успешно, `go vet ./src/cmd/gateway/` не выдаёт ошибок неиспользуемых импортов
- Evidence: `go build -tags gateway ./src/cmd/gateway/ && go vet -tags gateway ./src/cmd/gateway/` — оба успешны

### AC-006 Makefile targets работают

- Почему это важно: разработчик и CI используют единый entrypoint для сборки
- **Given** репозиторий с Makefile
- **When** выполняются `make build-gateway` и `make build-admin`
- **Then** `bin/gateway` и `bin/admin` созданы и запускаются
- Evidence: `make build-gateway && test -f bin/gateway && make build-admin && test -f bin/admin`

## Допущения

- Profile/incident хендлеры общие для gateway и admin — build tag изолирует только `ui/` embed
- `Server` и `AdminServer` остаются в одном пакете `api`; разделение — через build tag-файлы, а не отдельные пакеты
- Node.js не требуется для сборки gateway ни в каком виде
- Размер gateway image считается по финальному слою distroless, не включая go-builder stage

## Критерии успеха

- SC-001 Время старта gateway < 100ms (p99 при минимальной конфигурации)
- SC-002 Gateway image size <= 55MB
- SC-003 `go vet -tags gateway ./src/cmd/gateway/` чист (0 предупреждений)

## Краевые случаи

- Gateway без БД (offline/reduced mode) работает без БД-зависимых хендлеров (runtime guard, не build tag)
- Admin без UI (например, отвалился embed) — паника при старте, оператор видит fatal log
- Gateway собран без `-tags gateway` — поведение undefined (ответственность CI/CD)
- Миграция: оператор должен явно переключиться на admin binary для UI; API-хендлеры остаются в gateway

## Открытые вопросы

1. Какой tag convention выбрать: `//go:build gateway` и `//go:build admin` как взаимоисключающие, или `!admin` как default gateway?
