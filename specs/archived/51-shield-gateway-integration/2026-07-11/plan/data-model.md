---
status: no-change
reason: Фича не требует изменений DB модели; расширяется только конфиг
---

# Data Model: 51-shield-gateway-integration

## Status

**no-change** — существующая модель данных не меняется.

## Обоснование

- Profile resolution использует существующий `ProfileRepository.FindBySlug()` — ни new table, ни new column не требуется
- TenantID извлекается из HTTP заголовка, не хранится в БД
- Fallback mapping (tenant→model→profile_slug) хранится в config YAML, не в БД
- Инциденты логируются через middleware logger, не через IncidentRepository (отложено)

## Что расширяется (не DB)

- `Config.Shield` — новая секция в config.go (ActionOnSuspicious, TenantModelMapping)
- HTTP response headers — X-Shield-Status, X-Shield-Incident-ID (новые, без persistence)
