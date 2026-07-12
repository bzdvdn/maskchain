---
status: config-only
---

# Data Model: 90-production-hardening

## Status

`config-only` — нет изменений схемы БД, domain entities, API contracts или event schemas.

## Изменения

- `src/internal/infra/config/config.go`:
  - Новая структура `DebugConfig` с полями `Enabled bool` и `AdminToken string`
  - `EgressConfig`: добавлены поля `MaxIdleConnsPerHost int` и `DisableKeepAlives bool`
  - `Config`: добавлено поле `Debug *DebugConfig`
  - Defaults: `DebugConfig.Enabled = false`, `DebugConfig.AdminToken = ""`, `EgressConfig.MaxIdleConnsPerHost = 2`, `EgressConfig.DisableKeepAlives = false`

## Почему не DB/domain

- Connection pool параметры — runtime-конфигурация, не domain-сущность
- Admin token — операционный секрет, не tenant entity
- Все значения задаются через config/env, не требуют persistence
