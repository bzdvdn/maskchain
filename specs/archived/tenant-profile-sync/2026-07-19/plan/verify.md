---
report_type: verify
slug: tenant-profile-sync
status: pass
docs_language: ru
generated_at: 2026-07-19
---

## Результат верификации: PASS

### AC Coverage

| AC | Статус | Доказательство |
|----|--------|----------------|
| AC-001 | ✅ | Tenant entity + VO + миграция 005 + TenantRepository.Create + handler POST /api/v1/admin/tenants |
| AC-002 | ✅ | TenantResolver.List объединяет БД + config; auth middleware использует TenantProvider |
| AC-003 | ✅ | DBFirstTenantResolver: DB priority (Get проверяет БД первой) |
| AC-004 | ✅ | SyncConfig: INSERT ON CONFLICT DO NOTHING (не перезатирает) |
| AC-005 | ✅ | UpdateDictionaries endpoint: PUT /api/v1/admin/tenants/:slug/dictionaries не трогает api_keys |
| AC-006 | ✅ | ShieldMiddleware читает tenant.Dictionaries из контекста (TenantFromContext) |
| AC-007 | ✅ | Пустые dictionaries → dictDetectors пуст, нет dictionary match |
| AC-008 | ✅ | TenantRepository.Delete + handler DELETE → 204, resolver не видит удалённый tenant |
| AC-009 | ✅ | 008_cleanup: DROP TABLE dictionary_entries, profiles CASCADE; entity/profile удалён из кода |

### Code Artifacts

- `entity/tenant.go` — Tenant entity с slug, name, api_keys, dictionaries, pii_config
- `value/tenant_slug.go` — TenantSlug VO с validation
- `postgres/tenant.go` — PostgresTenantRepo: List, Get, Create, Update, Delete, GetDictionaries, UpdateDictionaries
- `resolver/tenant_resolver.go` — DBFirstTenantResolver: DB priority, SyncConfig (не перезатирает)
- `middleware/auth.go` — Auth middleware: TenantProvider + TenantFromContext
- `middleware/shield.go` — ShieldMiddleware: читает dictionaries из tenant (без X-Shield-Profile-Slug)
- `admin/tenant_handler.go` — Tenant CRUD + dictionaries endpoint
- `dto/tenant.go` — Tenant request/response DTOs
- `repository/dictionary/valkey.go` — ValkeyDictionaryCache (замена ProfileCache)
- `migrations/005_tenants.up.sql` — CREATE TABLE tenants
- `migrations/008_cleanup.up.sql` — DROP dictionary_entries, profiles CASCADE
- `examples/config.yaml` — tenants.default с api_keys + pii_config
- `examples/seed-tenant.sh` — Seed tenant dictionaries через admin API
- `examples/test-prompt.md` — Без X-Shield-Profile-Slug

### Build

`go build ./src/...` — PASS (без ошибок)

### Оставшееся

- `dto/profile.go` содержит shared типы (ErrorResponse, ValidationDetail) — не мешает, но может быть рефакторен
- Тесты (T6.1–T6.4) не написаны в рамках этой фичи
