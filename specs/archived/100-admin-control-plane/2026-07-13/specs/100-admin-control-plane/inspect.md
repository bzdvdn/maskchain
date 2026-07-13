---
report_type: inspect
slug: 100-admin-control-plane
status: pass
docs_language: ru
generated_at: 2026-07-13
---

# Inspect Report: 100-admin-control-plane

## Scope

- snapshot: проверка spec на выделение admin control plane в отдельный сервис, 2 Dockerfile, docker-compose, shared domain code
- artifacts:
  - CONSTITUTION.md
  - .speckeep/constitution.summary.md
  - specs/active/100-admin-control-plane/spec.md

## Verdict

- status: pass

## Errors

- none

## Warnings (resolved)

- **W-001** (ambiguous): исправлено — "минимальный размер image для горизонтального масштабирования gateway"
- **W-002** (compound requirement): исправлено — RQ-005 разбит на RQ-005..RQ-008 (proxy, shield, admin API, health)
- **W-003** (scope ambiguity): не относится к spec — spec последовательно использует "прямой доступ на :8081"

## Questions

- **Q-001**: AC-004 говорит "response body совпадает" — какие именно поля проверяются? Предлагается зафиксировать: `slug`, `name`, `status`, `preprocessors`, `dictionaries`.

## Suggestions

- **S-001** (constitution alignment): Конституция требует "React UI — только для управления профилями и логов". Spec это соблюдает, но хорошо бы явно упомянуть в Допущениях, что admin не будет обрастать dashboard-функциями в этой фиче (сейчас вне scope только dashboard, но хорошо закрепить принцип).
- **S-002** (edge case): Не описан сценарий, когда gateway и admin запущены с разными версиями миграций БД. Предлагается добавить в краевые случаи: "миграции идут от первого сервиса, второй проверяет version и пропускает".
- **S-003** (testability): AC-001 проверяет сборку без node, но не проверяет, что binary не импортирует ui-пакет. Предлагается добавить: `go build -o /dev/null ./src/cmd/gateway/` проходит, и `go list -deps ./src/cmd/gateway/ | grep -q ui` возвращает non-zero.

## Traceability

- 10 AC, 10 RQ — скоуп одной фичи
- Все AC имеют Given/When/Then + Evidence
- Нет плана/задач пока — inspect проводится перед plan

## Этап реализации — Verification Results

Проверено после implement (2026-07-13):

| Check | Status |
|---|---|
| `make build-gateway` | ✅ exit 0, binary `bin/gateway` (44MB) |
| `make build-admin` | ✅ exit 0, binary `bin/admin` (44MB), UI built successfully |
| `go list -deps ./src/cmd/gateway/ \| grep maskchain/ui` | ✅ NOT FOUND (no ui dependency) |
| `grep -c -E 'FROM node\|npm' Dockerfile.gateway` | ✅ 0 (no node stages) |
| `readlink Dockerfile` → `Dockerfile.admin` | ✅ symlink correct |
| `rg 'ui\.\|RegisterStaticFiles' src/cmd/gateway/main.go` | ✅ CLEAN |
| `go build ./src/cmd/admin/` | ✅ compiles |
| `go mod tidy` | ✅ clean |

## Next Step

- safe to proceed to verify phase
