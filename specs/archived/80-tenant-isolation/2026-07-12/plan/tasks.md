# Multi-tenant Isolation Задачи

## Phase Contract

Inputs: plan, data model, spec.
Outputs: упорядоченные задачи с покрытием AC.
Stop if: нет.

## Surface Map

| Surface | Change | Tasks |
|---------|--------|-------|
| `src/internal/domain/tenant/tenant.go` | new | T1.1 |
| `src/internal/domain/tenant/api_key.go` | new | T1.1 |
| `src/internal/domain/tenant/repository.go` | new | T1.1 |
| `src/internal/adapters/repository/tenant/in_memory.go` | new | T1.3 |
| `src/internal/infra/config/config.go` | modify | T1.2 |
| `src/internal/api/middleware/auth.go` | new | T2.1 |
| `src/internal/api/server.go` | modify | T2.2 |
| `src/internal/api/handler/profile/handler.go` | modify | T2.3 |
| `src/internal/api/middleware/shield.go` | modify | T2.4 |
| `src/internal/adapters/egress/pool.go` | modify | T3.1 |
| `src/internal/infra/logging/logging.go` | modify | T3.2 |
| `src/internal/infra/metrics/metrics.go` | modify | T3.3 |
| `src/internal/api/middleware/shield_test.go` | modify | T4.5 |
| `src/internal/api/provider_handler_test.go` | modify | T4.5, T4.6 |
| `src/internal/adapters/egress/egress_test.go` | modify | T4.6 |
| `src/internal/api/middleware/middleware_test.go` | modify | T4.7 |
| `src/internal/infra/metrics/metrics_test.go` | modify | T4.8 |

## Implementation Context

- **Цель MVP:** Multi-header API key auth (Authorization:Bearer, X-Mask-Authorization, custom) + tenant-scoped profile isolation + observability (tenant in logs/metrics/upstream).
- **Вторая фаза (verify-driven):** Добавить unit-тесты для AC-006 (routing), AC-007 (egress), AC-008 (logging), AC-009 (metrics) — покрытие было идентифицировано как concerns в verify.
- **Инварианты/семантика:**
  - API keys уникальны в пределах всех tenants — duplicate = fatal at startup (DEC-001).
  - Tenant.Slug используется как TenantID в Profile, RoutingRule, Incident (DM-001).
  - Tenant → Profile = 1:1 через ProfileSlug (DM-001).
  - auth_scheme ∈ {bearer, raw}. Default: raw (DEC-002).
  - Reverse index строится при старте: flat map `api_key → Tenant`.
  - Default auth header: `X-Mask-Authorization` (auth_scheme=raw), если tenant не указал свой.
- **Ошибки/коды:**
  - 401 Unauthorized — отсутствие/невалидный API key.
  - 401 + "key theft" — ключ найден, но передан в чужом заголовке.
- **Контракты/протокол:**
  - Middleware пробует заголовки в порядке: Authorization:Bearer → X-Mask-Authorization → кастомные per-tenant.
  - Public endpoints (без auth): GET /health, /ready, /live, /metrics.
  - `TenantFromContext(c *gin.Context) (value.TenantID, bool)` — helper для handler-ов.
  - Если tenants config пуст/отсутствует — gateway работает без auth (backward compat).
- **Границы scope:** не делаем per-tenant rate limiting, не делаем persistence для Tenant.
- **Proof signals:** curl с валидным ключом → 200 и профили tenant; curl без ключа → 401.
- **References:** `DEC-001` (reverse index), `DEC-003` (in-memory repo), `DEC-004` (global middleware + allowlist), `DM-001` (Tenant entity).

## Фаза 1: Bootstrapping — Domain + Config + Repository

Цель: создать Tenant domain entity, загрузку конфига и in-memory репозиторий.

- [x] T1.1 **Создать Tenant domain entity**
  Tenant aggregate в новом пакете `domain/tenant/`: поля Slug, Name, ProfileSlug, APIKeys, AuthHeader, AuthScheme + APIKey value object + TenantRepository interface.
  Touches: `src/internal/domain/tenant/tenant.go`, `src/internal/domain/tenant/api_key.go`, `src/internal/domain/tenant/repository.go`

- [x] T1.2 **Добавить секцию tenants в viper config**
  Добавить поле `Tenants map[string]TenantConfig` в `Config` struct. TenantConfig: Name, ProfileSlug, APIKeys, AuthHeader, AuthScheme. Валидация: непустой Slug, хотя бы один API key, auth_scheme ∈ {bearer, raw}. Duplicate keys → fatal.
  Touches: `src/internal/infra/config/config.go`

- [x] T1.3 **Реализовать in-memory TenantRepository**
  In-memory repo: `map[string]*Tenant` keyed by Slug. Метод `BuildIndex()` — строит reverse index `map[string]string` (api_key → tenant_slug), проверяет уникальность ключей. `FindByAPIKey(key string) (*Tenant, bool)`.
  Touches: `src/internal/adapters/repository/tenant/in_memory.go`

## Фаза 2: MVP Slice — Auth + Profile Isolation

Цель: multi-header auth middleware, tenant в контексте, изоляция профилей.

- [x] T2.1 **Реализовать multi-header auth middleware**
  Middleware: собирает candidate pairs (header_name, key_value) в порядке: Authorization:Bearer → X-Mask-Authorization → кастомные per-tenant заголовки. Для каждого candidate: lookup key по reverse index → если найден tenant, проверяет header_name == tenant.auth_header. При совпадении → tenant slug в gin context. Если ключ найден в чужом заголовке → 401. Функция `TenantFromContext(c) (string, bool)` для handler-ов.
  Default: `X-Mask-Authorization` с auth_scheme=raw если tenant не указал auth_header.
  Touches: `src/internal/api/middleware/auth.go`

- [x] T2.2 **Подключить auth middleware в server.go**
  Добавить auth middleware глобально (после metrics middleware). Public endpoints (/health, /ready, /live, /metrics) исключить из auth через allowlist: или отдельный router group без auth, или skip-условие. Если конфиг tenants пуст — auth middleware пропускает все запросы (no-op).
  Touches: `src/internal/api/server.go`

- [x] T2.3 **Заменить tenantIDFromContext в profile handler**
  Убрать заглушку `tenantIDFromContext`, читать tenant из gin context через `TenantFromContext()`. Если tenant в контексте отсутствует — возвращать 401.
  Touches: `src/internal/api/handler/profile/handler.go`

- [x] T2.4 **Адаптировать shield middleware**
  Заменить прямой вызов `c.GetHeader("X-Tenant-ID")` в `resolveTenantID()` на чтение tenant из контекста через `TenantFromContext()`. Убрать fallback на "default".
  Touches: `src/internal/api/middleware/shield.go`

## Фаза 3: Observability + Propagation

Цель: tenant ID в upstream, логах и метриках.

- [x] T3.1 **Добавить X-Tenant-ID propagation в egress**
  В egress-клиенте (pool.go) при отправке запроса к upstream LLM-провайдеру добавить заголовок `X-Tenant-ID` из контекста запроса. Заголовок добавляется только если tenant ID присутствует.
  Touches: `src/internal/adapters/egress/pool.go`

- [x] T3.2 **Добавить tenant ID в structured logging**
  В logging middleware (или middleware/logger.go) добавить атрибут `tenant_id` в slog-запись, читая tenant из gin context.
  Touches: `src/internal/api/middleware/logger.go`

- [x] T3.3 **Добавить tenant label в HTTP-метрики**
  В metrics middleware добавить label `tenant` на HTTP-метрики (http_requests_total и др.). Значение — tenant slug из контекста или "unknown".
  Touches: `src/internal/infra/metrics/metrics.go`

## Фаза 4: Проверка

Цель: automated тесты + manual validation script.

- [x] T4.1 **Unit-тесты для domain entities**
  Tenant creation, invariant validation (empty slug, empty API key, duplicate keys). APIKey value object validation. TenantRepository BuildIndex duplicate detection.
  Touches: `src/internal/domain/tenant/tenant_test.go`, `src/internal/adapters/repository/tenant/in_memory_test.go`

- [x] T4.2 **Unit-тесты для auth middleware**
  Mock TenantRepository. Test cases: valid Bearer → tenant in context; default X-Mask-Authorization → tenant in context; custom header → tenant in context; missing header → 401; invalid key → 401; empty token → 401; key in wrong header → 401; TenantFromContext helper.
  Touches: `src/internal/api/middleware/auth_test.go`

- [x] T4.3 **Integration-тесты для изоляции профилей**
  Запуск gateway с двумя tenants. Проверка: tenant A видит только свои профили (AC-005); tenant B видит только свои; запрос без ключа → 401.
  Touches: `src/internal/api/middleware/auth_test.go`, `src/internal/api/handler/profile/handler_test.go`

- [x] T4.4 **Manual validation script**
  README или shell-скрипт с curl-командами из First Validation Path плана.
  Touches: `specs/active/80-tenant-isolation/README.md` (optional)

- [x] T4.5 **Unit-тесты для tenant-scoped routing (AC-006)**
  Test `resolveTenantID()` в shield middleware: установить `tenant_slug` в gin context → проверить, что возвращается корректный TenantID.
  Test в provider handler: установить tenant в контекст → проверить, что `Select()` вызывается с этим tenant.
  Touches: `src/internal/api/middleware/shield_test.go`, `src/internal/api/provider_handler_test.go`

- [x] T4.6 **Unit-тесты для X-Tenant-ID propagation (AC-007)**
  Test в egress client: `Call()` передаёт заголовки из `ProviderRequest.Headers` в исходящий HTTP-запрос.
  Test в provider handler: `ProviderRequest.Headers` содержит `X-Tenant-ID` при наличии tenant в контексте.
  Touches: `src/internal/adapters/egress/egress_test.go`, `src/internal/api/provider_handler_test.go`

- [x] T4.7 **Unit-тесты для tenant ID в логах (AC-008)**
  Test в middleware_test.go: расширить `TestLogger` — установить `tenant_slug` в gin context → проверить, что лог-запись содержит `tenant_id`.
  Touches: `src/internal/api/middleware/middleware_test.go`

- [x] T4.8 **Unit-тесты для tenant label в метриках (AC-009)**
  Test в metrics_test.go: установить `tenant_slug="test-tenant"` в gin context → проверить, что `/metrics` содержит `tenant="test-tenant"` label.
  Touches: `src/internal/infra/metrics/metrics_test.go`

## Покрытие критериев приемки

- AC-001 -> T2.1, T4.2, T4.3
- AC-002 -> T2.1, T4.2, T4.3
- AC-003 -> T2.1, T4.2, T4.3
- AC-004 -> T2.1, T4.2, T4.3
- AC-005 -> T2.3, T4.3
- AC-010 -> T2.1, T4.2
- AC-006 -> T2.4, T4.3, T4.5
- AC-007 -> T3.1, T4.2, T4.6
- AC-008 -> T3.2, T4.2, T4.7
- AC-009 -> T3.3, T4.3, T4.8

## Заметки

- Порядок: T1.1 → T1.2 → T1.3 → T2.1 → T2.2 → T2.3 → T2.4 → T3.1 → T3.2 → T3.3 → T4.1 → T4.2 → T4.3 → T4.4 → T4.5 → T4.6 → T4.7 → T4.8.
- T1.1–T1.3 можно параллелить с T3.2 и T3.3 (независимые surfaces).
- T1.3 необходим для T2.1.
- T2.2 необходим для T2.3 и T2.4.
- T4.4 опционален — достаточно если T4.2 и T4.3 проходят.
- T4.5–T4.8 независимы друг от друга — можно параллелить (разные файлы).
- Trace-маркер `@sk-task` добавлять над owning type/function declaration при реализации. `@sk-test` — над тестовой функцией.

Готово к: `/spk.implement 80-tenant-isolation`
