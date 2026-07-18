---
report_type: inspect
slug: helm-chart
status: pass
docs_language: ru
generated_at: 2026-07-18
---

# Inspect Report: helm-chart

## Scope

- snapshot: Проверка spec для Helm chart MaskChain с Bitnami dependencies (registry bitnamilegacy)
- artifacts:
  - CONSTITUTION.md (через .speckeep/constitution.summary.md)
  - specs/active/helm-chart/spec.md

## Verdict

- status: pass
- конституция не нарушена, spec самосогласован, AC измеримы

## Errors

- none

## Warnings

- **W-001 ConfigMap sections incomplete**: Scope и RQ-003 перечисляют секции для ConfigMap как `shield, routing, egress, session, admin, otel, ratelimit, analytics, dictionary_cache, tenants`, но в `config.go` также есть `server`, `log`, `mask`, `debug` — они не упомянуты. Рекомендуется либо явно исключить их (с обоснованием: server/log/env-based), либо дополнить список. На planning'е нужно зафиксировать решение.

## Questions

- Не указано, какие секции конфига должны быть в ConfigMap, а какие — только в Secret/env. `database.dsn`, `valkey.password`, `admin.password`, `tenants[*].api_keys` уходят в Secret (RQ-005), но `server.port`, `log.level`, `mask.cache_ttl`, `debug.enabled` — куда? В ConfigMap? Вопрос к планированию.

## Suggestions

- **S-001 AC-004 grep pattern**: `grep -iE '(password|api_key.*[^s]|token)'` может ложно срабатывать на ключе YAML `api_keys:` из-за backtracking в regex. Рекомендуется явный grep по значениям ConfigMap (data.config.yaml), а не по всему stdout. Или упростить проверку: `grep -E 'password:|token:'` плюс ручная проверка tenants.

## Traceability

- 8 AC покрывают: lint (AC-001), template для трёх deployMode (AC-002), полнота ConfigMap (AC-003), secret isolation (AC-004), e2e install (AC-005), ingress toggle (AC-006), servicemonitor toggle (AC-007), networkpolicy (AC-008)
- Каждый AC имеет уникальный observable outcome, Given/When/Then + Evidence
- RQ-001–008 согласованы с AC-001–008 (прямое 1:1 или 2:1 отображение)

## Next Step

- safe to continue to plan; W-001 и S-001 — не блокеры, но учесть на планировании
