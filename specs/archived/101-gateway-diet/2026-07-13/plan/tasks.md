# 101: Gateway Diet — Задачи

## Phase Contract

Inputs: plan.md, data-model.md, spec.md.
Outputs: исполнимые задачи с покрытием AC.
Stop if: план расплывчат — нет, план чёткий.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/api/server.go` | T1.1 |
| `Makefile` | T1.2 |
| `Dockerfile.gateway` | T2.1 |
| `Dockerfile.admin` | T2.2 |
| (shell — проверки) | T3.1, T3.2 |

## Implementation Context

- Цель MVP: удалить `RegisterStaticFiles` из `Server`, добавить build tags в Makefile, проверить изоляцию `ui/`
- Инварианты:
  - Profile/incident хендлеры остаются в gateway (DEC-002). `RegisterStaticFiles` — единственный удаляемый метод
  - Build tags `gateway`/`admin` — взаимоисключающие (DEC-001)
  - `AdminServer` в `admin.go` не меняется
- Контракты: API-контракты не меняются
- Proof signals:
  - `go build -tags gateway ./src/cmd/gateway/` успешен
  - `go tool nm bin/gateway | grep 'ui/'` → пусто
  - `make build-gateway && make build-admin` → оба бинарника
- Вне scope: domain, adapters, infra, ui/, cmd/gateway/main.go, cmd/admin/main.go

## Фаза 1: Core changes

Цель: убрать admin-зависимость из gateway и закрепить build tags.

- [x] T1.1 **Удалить `RegisterStaticFiles` из `Server` struct** — убрать метод и его вызовы из `server.go`; удалить импорты `io/fs` и `strings`, если они больше не используются.
  Touches: `src/internal/api/server.go`
  AC: AC-001, AC-005

- [x] T1.2 **Обновить Makefile targets** — `build-gateway`: добавить `-tags gateway`; `build-admin`: добавить `-tags admin`.
  Touches: `Makefile`
  AC: AC-006

## Фаза 2: Dockerfiles

Цель: синхронизировать Dockerfiles с build tag convention.

- [x] T2.1 **Обновить Dockerfile.gateway** — добавить `-tags gateway` в команду `go build`.
  Touches: `Dockerfile.gateway`
  AC: AC-003

- [x] T2.2 **Обновить Dockerfile.admin** — добавить `-tags admin` в команду `go build`.
  Touches: `Dockerfile.admin`
  AC: AC-004

## Фаза 3: Проверка

Цель: доказать, что gateway собран чисто, без `ui/`, и все AC выполняются.

- [x] T3.1 **Проверить сборку gateway** — `go build -tags gateway ./src/cmd/gateway/`, `go vet -tags gateway ./src/cmd/gateway/`, `go tool nm bin/gateway | grep 'ui/'`.
  Touches: (shell — CI-шаги, скрипты проверки)
  AC: AC-001, AC-005

- [x] T3.2 **Проверить Makefile и Docker сборки** — `make build-gateway && make build-admin`, `docker build -f Dockerfile.gateway`, `docker build -f Dockerfile.admin` + curl `/`.
  Touches: (shell — CI-шаги)
  AC: AC-002 (startup), AC-003 (image size), AC-004 (UI), AC-006 (make targets)

## Покрытие критериев приемки

- AC-001 -> T1.1, T3.1
- AC-002 -> T3.2
- AC-003 -> T2.1, T3.2
- AC-004 -> T2.2, T3.2
- AC-005 -> T1.1, T3.1
- AC-006 -> T1.2, T3.2

## Заметки

- T1.1 и T1.2 можно выполнять параллельно
- T2.1 и T2.2 — после T1.2
- T3.1 и T3.2 — после всех изменений
- Никаких новых файлов не создаётся — только модификация существующих
