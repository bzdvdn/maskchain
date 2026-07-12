# Multi-tenant Isolation Модель данных

## Scope

- Связанные `AC-*`: `AC-001`, `AC-002`, `AC-003`, `AC-005`
- Связанные `DEC-*`: `DEC-001`, `DEC-003`
- Статус: `changed`
- Новая сущность: `Tenant` (domain aggregate, in-memory из static config). Персистентное хранение — вне scope MVP.

## Сущности

### DM-001 Tenant

- Назначение: представляет организацию/клиента gateway, агрегирует API keys и ссылку на profile.
- Источник истины: static YAML config (секция `tenants`), загружается при старте.
- Инварианты:
  - Tenant.Slug уникален
  - API keys уникальны в пределах всех tenants (проверка при загрузке конфига)
  - auth_scheme ∈ {bearer, raw}
  - Хотя бы один API key
- Связанные `AC-*`: `AC-001`, `AC-002`, `AC-003`, `AC-005`
- Связанные `DEC-*`: `DEC-001` (reverse index), `DEC-003` (in-memory)
- Поля:
  - `Slug` — string, required, уникальный ID tenant (используется как TenantID в Profile, RoutingRule, Incident)
  - `Name` — string, optional, человекочитаемое имя
  - `ProfileSlug` — string, required, ссылка на профиль политик (Profile.Slug)
  - `APIKeys` — []string, required, список API keys (хэшированное или сырое хранение)
  - `AuthHeader` — string, required, имя HTTP-заголовка (например, "Authorization")
  - `AuthScheme` — string, required, `bearer` | `raw`
- Жизненный цикл:
  - Создаётся: при старте gateway из static config
  - Обновляется: при рестарте gateway (in-memory, hot-reload — вне scope)
  - Удаляется: удалением из конфига + рестарт

### DM-002 APIKey (value object)

- Назначение: представляет API key tenant.
- Инварианты: non-empty string.
- Поля:
  - `Value` — string, required, raw key (в обратном индексе — ключ карты)
  - `TenantSlug` — string, required, владелец ключа (для reverse index lookup)
- Жизненный цикл: живёт внутри Tenant entity, не существует самостоятельно.

## Связи

- `Tenant → Profile`: 1:1 через `Tenant.ProfileSlug = Profile.Slug`. Profile.TenantID = Tenant.Slug.
- `Tenant → RoutingRule`: 1:N через `RoutingRule.TenantID = Tenant.Slug`.
- `Tenant → Incident`: 1:N через `Incident.tenant = Tenant.Slug`.

## Производные правила

- Reverse index (`api_key → Tenant`) строится на старте из всех API keys всех tenants. Ключи должны быть уникальны — дубликат = fatal.
- При выборе заголовка для извлечения ключа: middleware проверяет `Authorization: Bearer` первым (auth_scheme=bearer), затем кастомные заголовки (auth_scheme=raw).

## Переходы состояний

- Жизненный цикл Tenant достаточно прост (create on start, delete on restart): отдельный список не требуется.

## Вне scope

- Хранение Tenant в PostgreSQL.
- Хэширование API keys (сырые ключи в конфиге — достаточный уровень безопасности для MVP).
- Hot-reload конфига без рестарта.
- Role-based permissions внутри tenant.
