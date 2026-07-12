# Multi-tenant Isolation План

## Phase Contract

Inputs: spec и inspect (pass).
Outputs: plan, data model.
Stop if: нет.

## Цель

Добавить слой multi-tenant isolation: Tenant entity, API key аутентификация (flat reverse index), tenant-scoped доступ к профилям, проброс tenant в routing/logging/metrics.

## MVP Slice

Tenant entity + auth middleware (все заголовки: Authorization:Bearer, X-Mask-Authorization по умолчанию, кастомные per-tenant) + контекстная изоляция в profile handler.
Покрывает AC-001, AC-002, AC-003, AC-004, AC-005, AC-010.

## First Validation Path

Запустить gateway с конфигом:
```yaml
tenants:
  alpha:
    name: "Alpha Corp"
    profile_slug: "alpha-profile"
    auth_header: "Authorization"
    auth_scheme: bearer
    api_keys: ["sk-abc123"]
  beta:
    name: "Beta Inc"
    profile_slug: "beta-profile"
    api_keys: ["mk-beta-key"]        # использует default X-Mask-Authorization
  gamma:
    name: "Gamma LLC"
    profile_slug: "gamma-profile"
    auth_header: "X-Custom-Token"
    auth_scheme: raw
    api_keys: ["custom-gamma-token"]
```

Проверить:
1. `curl -H "Authorization: Bearer sk-abc123" /api/v1/profiles` → 200, профили tenant alpha
2. `curl -H "X-Mask-Authorization: mk-beta-key" /api/v1/profiles` → 200, профили tenant beta
3. `curl -H "X-Custom-Token: custom-gamma-token" /api/v1/profiles` → 200, профили tenant gamma
4. `curl /api/v1/profiles` без заголовка → 401
5. `curl -H "Authorization: Bearer wrong-key" /api/v1/profiles` → 401

## Scope

- Новый пакет `src/internal/domain/tenant/` — Tenant entity, APIKey value object, TenantRepository interface
- Новый in-memory TenantRepository в `src/internal/adapters/repository/tenant/`
- Новый auth middleware в `src/internal/api/middleware/auth.go`
- Конфиг: секция `tenants` в viper
- Profile handler: замена `tenantIDFromContext` на чтение из gin context
- Shield middleware: чтение tenant из контекста вместо прямого header
- Egress: проброс `X-Tenant-ID`
- Logging: tenant ID через slog attribute
- Metrics: tenant label на HTTP-метрики
- `server.go`: подключение auth middleware ко всем защищённым группам
- Нетронуты: Profile entity, ProfileRepository, RoutingRule, Incident entity, UI

## Performance Budget

- Auth middleware overhead: < 1ms p99 (map lookup + multi-header candidate collection)
- `none` — прочих значимых ограничений нет

## Implementation Surfaces

| Surface | Change | Почему |
|---------|--------|--------|
| `src/internal/domain/tenant/` | new | Новая domain entity |
| `src/internal/adapters/repository/tenant/` | new | In-memory реализация TenantRepository |
| `src/internal/api/middleware/auth.go` | new | Multi-header auth middleware |
| `src/internal/infra/config/config.go` | modify | Добавить `tenants` map в viper config |
| `src/internal/api/handler/profile/handler.go` | modify | Замена tenantIDFromContext |
| `src/internal/api/middleware/shield.go` | modify | Чтение tenant из контекста |
| `src/internal/adapters/egress/` | modify | Проброс X-Tenant-ID |
| `src/internal/api/server.go` | modify | Wire auth middleware, skip public endpoints |
| `src/internal/infra/logging/` | modify | Добавить tenant attribute |
| `src/internal/infra/metrics/` | modify | Добавить tenant label |

## Bootstrapping Surfaces

- `src/internal/domain/tenant/` — создать первым (Tenant entity, APIKey VO, TenantRepository interface)
- `src/internal/adapters/repository/tenant/` — сразу после domain (in-memory repo)

## Влияние на архитектуру

- Tenant emerge как новый domain aggregate (до этого tenant = только строка `value.TenantID` в shield value)
- Auth становится явным слоем middleware вместо отсутствия аутентификации
- Profile handler перестаёт возвращать "default" — реальная изоляция
- Public endpoints (/health, /ready, /live, /metrics) исключены из auth

## Acceptance Approach

- **AC-001** — unit test: mock TenantRepository, auth middleware с валидным Bearer → tenant в контексте. Integration: curl с валидным ключом → 200.
- **AC-002** — unit test: middleware без заголовка → abort с 401. Integration: curl без ключа → 401.
- **AC-003** — unit test: middleware с невалидным Bearer → abort с 401. Integration: curl с неизвестным ключом → 401.
- **AC-004** — unit test: middleware с кастомным заголовком (X-Custom) → tenant в контексте.
- **AC-005** — integration: два tenants с разными профилями, запрос tenant alpha → только профиль alpha.
- **AC-006** — integration: tenant-scoped routing через RouteSelector.Select с tenant из контекста.
- **AC-007** — unit test egress: проверить, что X-Tenant-ID установлен в исходящем запросе.
- **AC-008** — unit test logging: slog handler проверяет наличие tenant_id attribute.
- **AC-009** — integration: `/metrics` содержит `tenant` label.
- **AC-010** — unit test: middleware с дефолтным `X-Mask-Authorization` → tenant в контексте.

## Данные и контракты

- Новая сущность Tenant (in-memory) — добавляется в data model.
- Никаких изменений API-контрактов (REST paths, payloads остаются теми же, только добавляется проверка авторизации).
- `data-model.md` описывает Tenant aggregate.

## Стратегия реализации

### DEC-001 Flat reverse index для auth

- Why: lookup по ключу за O(1), не нужно перебирать tenants. Альтернатива — tenant lookup по ключу через итерацию всех tenants — O(n) на каждый запрос.
- Tradeoff: ключи должны быть уникальны в пределах всех tenants (валидация на старте). Память: index хранит копию ключей.
- Affects: auth middleware, TenantRepository (buildIndex).
- Validation: benchmark lookup < 1μs.

### DEC-002 Multi-header auth middleware (все заголовки сразу)

- Why: пользователям нужна гибкость с первого дня. Default `X-Mask-Authorization` удобен для mask/unmask, `Authorization: Bearer` для совместимости, кастомные — для специфических провайдеров.
- Tradeoff: middleware чуть сложнее (перебор до 3+ заголовков), но overhead всё ещё negligible (< 10μs).
- Affects: auth middleware (multi-header candidate collection + header match validation).
- Validation: AC-001 (Bearer), AC-004 (custom), AC-010 (default X-Mask-Authorization) все pass.

### DEC-003 TenantRepository in-memory на старте

- Why: MVP не требует динамического управления tenants. Static YAML config + in-memory map достаточны.
- Tradeoff: перезагрузка конфига требует рестарта gateway. При переходе на БД — замена реализации TenantRepository.
- Affects: TenantRepository interface (уже рассчитан на DB-impl), DI контейнер.
- Validation: config загружается, reverse index строится, lookup работает.

### DEC-004 Auth middleware — глобальный, с allowlist для public endpoints

- Why: гарантирует, что новый endpoint не появится без auth (default-deny). Публичные endpoint-ы явно исключаются.
- Tradeoff: при добавлении нового публичного endpoint-а надо не забыть внести в allowlist.
- Affects: server.go (router setup), auth middleware.
- Validation: health/ready/metrics доступны без ключа, всё остальное — 401.

## Incremental Delivery

### MVP (Первая ценность)

Multi-header auth + изоляция профилей. Задачи 1–8 (см. Порядок реализации). Критерий: AC-001, AC-002, AC-003, AC-004, AC-005, AC-010.

### Итеративное расширение

1. **Второй increment**: per-tenant rate limiting (не в scope этого spec).

## Порядок реализации

1. **Tenant domain entity** — `domain/tenant/` tenant.go, api_key.go, repository.go
2. **Config loader** — `infra/config/` секция tenants, парсинг в Tenant entity
3. **In-memory TenantRepository** — `adapters/repository/tenant/` + reverse index
4. **Auth middleware** — `api/middleware/auth.go` multi-header (Bearer + X-Mask-Authorization + custom)
5. **Wire в server.go** — глобальная middleware + allowlist для public endpoints
6. **Profile handler** — замена tenantIDFromContext
7. **Shield middleware** — чтение tenant из контекста
8. **Egress propagation** — X-Tenant-ID в upstream
9. **Logging** — tenant ID attribute
10. **Metrics** — tenant label

1→3 (domain + config + repo) можно параллелить с 9 (logging) и 10 (metrics). 4 (middleware) блокирует 5→8.

## Риски

- **Риск 1: Auth middleware пропущена на новом endpoint-е.** Mitigation: DEC-004 — глобальная middleware с default-deny, allowlist.
- **Риск 2: Tenant key collision (duplicate API key across tenants).** Mitigation: валидация на старте — panic/fatal при дубликате.
- **Риск 3: Совместимость — клиенты, полагающиеся на отсутствие auth.** Mitigation: это новая фича, старых клиентов нет. Документировать breaking change.

## Rollout и compatibility

- Фича включается наличием секции `tenants` в конфиге. Если секция пуста/отсутствует — gateway работает без auth (backward compat для dev).
- Требуется обновление deploy-конфига с tenants.
- Monitoring: добавить алерт на 401 rate spike (признак неправильно настроенного клиента).

## Проверка

- Unit: Tenant entity, APIKey VO, TenantRepository, auth middleware, egress propagation.
- Integration: profile handler с tenant из контекста, shield middleware.
- Manual: curl-команды из First Validation Path.
- AC-001/002/003/004/005/010 покрываются MVP. AC-006/007/008/009 — второй проход после MVP.

## Соответствие конституции

- нет конфликтов. Go + Gin + viper, DDD, enterprise outbound proxy — всё согласовано.

Готово к: `/spk.tasks 80-tenant-isolation`
