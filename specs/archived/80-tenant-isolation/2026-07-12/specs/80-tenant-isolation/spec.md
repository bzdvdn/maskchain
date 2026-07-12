# Multi-tenant Isolation

## Scope Snapshot

- In scope: Tenant domain entity, API key authentication, custom auth header per tenant, tenant-scoped profile isolation, tenant-scoped routing, X-Tenant-ID propagation to upstream providers.
- Out of scope: multi-tenant UI (user management, tenant admin panel), tenant provisioning API, RBAC/role-based permissions within a tenant, rate limiting per tenant.

## Цель

Операторы gateway получают возможность изолировать tenants друг от друга: каждый tenant аутентифицируется по API key (с возможностью настроить свой заголовок), видит только свои профили политик и маршрутизируется согласно своим RoutingRules. Успех фичи измеряется тем, что tenant A не может прочитать/изменить профили tenant B, запрос без валидного API key отклоняется, и routing учитывает tenant при выборе провайдера.

## Основной сценарий

1. Оператор конфигурирует tenants в конфигурации gateway: auth header, API key → tenant mapping, tenant → profile.
2. Клиент отправляет запрос с API key в заголовке, указанном в конфигурации его tenant.
3. Auth middleware извлекает API key из настроенного заголовка, находит tenant, устанавливает tenant ID в контекст запроса.
4. Все downstream handler-ы (profiles, incidents, shield, routing) читают tenant из контекста и применяют изоляцию.
5. Tenant ID пробрасывается в upstream LLM-провайдер.

## User Stories

- P1 Story: Оператор разворачивает gateway с tenant isolation; профили политик tenant A недоступны tenant B.
- P2 Story: Оператор настраивает для каждого tenant свой заголовок аутентификации (X-Api-Key, X-Token и т.д.) без изменений кода gateway.
- P3 Story: Оператор видит в логах и метриках tenant ID для каждого запроса.

## MVP Slice

Наименьший срез: Tenant entity + auth middleware (с поддержкой всех заголовков) + контекстная изоляция в profile handler. Покрывает AC-001, AC-002, AC-003, AC-004, AC-005.

## First Deployable Outcome

После первого implementation pass можно запустить gateway с конфигом tenants, отправить запросы к `/api/v1/profiles` с API key в `Authorization: Bearer`, `X-Mask-Authorization` или кастомном заголовке и убедиться, что каждый tenant видит только свои профили. Запрос без ключа возвращает 401.

## Scope

- Tenant domain entity в `src/internal/domain/tenant/`:
  - `Tenant` aggregate: Slug (уникальный ID), name, profile_slug (ссылка на профиль), API keys, auth_header (default: `X-Mask-Authorization`), auth_scheme (`bearer` | `raw`, default: `raw`)
  - `APIKey` value object (raw key string, validation)
  - TenantRepository (in-memory из static config, персистентность в БД — вне scope)
  - Tenant.Slug используется как tenant ID в RoutingRule, Profile, Incident и других сущностях для tenant-scoping
- Static tenant config через viper (`tenants` key)
- API key auth middleware:
  - На старте строится flat reverse index: `api_key → Tenant` (ключи уникальны в пределах всех tenants)
  - Middleware собирает candidate pairs `(header_name, key_value)` из запроса в порядке приоритета:
    1. `Authorization: Bearer <key>` — парсинг Bearer prefix
    2. `X-Mask-Authorization: <key>` — весь заголовок как raw key (default заголовок)
    3. Все кастомные заголовки из конфигов tenants (каждый как raw key)
  - Для каждого candidate: lookup key_value в reverse index → если найден tenant, проверяет header_name == tenant.auth_header
  - При совпадении: устанавливает tenant slug в контекст запроса
  - Если key найден, но header_name не совпадает с tenant.auth_header → 401 (защита от key theft)
  - Если ни один candidate не подошёл → 401
  - Если tenants config пуст/отсутствует — middleware пропускает все запросы (no-auth mode)
- Default заголовок `X-Mask-Authorization`: если tenant не указал auth_header, используется `X-Mask-Authorization` с auth_scheme=raw
- Tenant context propagation: установка tenant ID в `gin.Context`, вспомогательная функция `TenantFromContext`
- Замена `tenantIDFromContext` в profile handler — чтение tenant из контекста вместо "default"
- Адаптация shield middleware для чтения tenant из контекста (а не из header напрямую)
- Проброс `X-Tenant-ID` в upstream LLM provider через egress
- Tenant ID в structured logging (slog attribute)
- Tenant ID label в Prometheus-метриках (HTTP-метрики)
- Tenant → profile: 1:1 через `Profile.TenantID` (без изменений в Profile entity)

## Контекст

- Gateway работает в enterprise-сетях с outbound proxy — аутентификация не должна требовать внешних IdP.
- Routing domain уже tenant-scoped (RoutingRule.TenantID, RouteSelector.Select принимает tenantID).
- Profile repository уже tenant-scoped (Profile.TenantID, все методы принимают TenantID).
- Profile handler `tenantIDFromContext` — заглушка, всегда возвращает "default".
- X-Tenant-ID header уже читается в provider_handler и shield middleware напрямую из заголовка — нужно унифицировать через контекст.
- Tenant как сущность сейчас не существует — только `value.TenantID` строка в `domain/shield/value/`.
- Tenant.Slug используется как tenant ID во всех доменах: Profile.TenantID, RoutingRule.TenantID, Incident.tenant.
- `profile_slug` в Tenant — это конфигурация "какой profile slug назначен tenant", а не дублирование Profile.TenantID.
- В конфигурации routing уже есть `tenant` поле (tenant → RoutingRule), но нет единого Tenant entity.
- Static YAML config достаточен для MVP; динамическое управление — вне scope.

## Зависимости

- `value.TenantID` — существует в `src/internal/domain/shield/value/tenant_id.go` (может быть перенесён в domain/tenant при рефакторинге).
- ProfileRepository — уже tenant-scoped.
- Routing domain — уже tenant-scoped.
- HTTP egress client — требуется добавить propagation X-Tenant-ID.
- `none` — внешних сервисных зависимостей нет.

## Требования

- RQ-001 Система ДОЛЖНА проверять API key на всех защищённых endpoints и возвращать 401 при отсутствии или невалидном ключе. Заголовок для извлечения ключа определяется конфигурацией tenant.
- RQ-002 Система ДОЛЖНА извлекать tenant ID из валидного API key и делать tenant ID доступным downstream handler-ам через контекст запроса.
- RQ-003 Система ДОЛЖНА фильтровать профили политик по tenant ID — каждый tenant видит только свои профили.
- RQ-004 Система ДОЛЖНА применять tenant-scoped RoutingRules при выборе провайдера для LLM-запроса.
- RQ-005 Система ДОЛЖНА пробрасывать `X-Tenant-ID` header в upstream LLM-провайдер.
- RQ-006 Система ДОЛЖНА включать tenant ID в structured логи и HTTP-метрики.

## Вне scope

- Динамическое управление tenants через API или UI (только static config).
- Tenant provisioning/adminsitration UI.
- RBAC/permissions внутри tenant.
- Rate limiting per tenant.
- Tenant-scoped инциденты (уже tenant-scoped в data model, требуется только контекст).
- JWT, OAuth2, OpenID Connect — пока только static API keys.
- Персистентное хранение Tenant entity в PostgreSQL — in-memory из config.

## Критерии приемки

### AC-001 API key аутентификация — успешный сценарий

- Почему это важно: без аутентификации нет изоляции.
- **Given** сконфигурирован tenant `alpha` с API key `sk-abc123`, auth header `Authorization` и Bearer scheme
- **When** клиент отправляет GET `/api/v1/profiles` с заголовком `Authorization: Bearer sk-abc123`
- **Then** запрос проходит аутентификацию, tenant ID `alpha` доступен handler-у
- Evidence: handler получает tenantID = `alpha` в контексте, запрос не отклонён

### AC-002 API key аутентификация — отсутствующий ключ

- Почему это важно: неаутентифицированные запросы должны быть отклонены.
- **Given** ни один API key не сконфигурирован для данного запроса
- **When** клиент отправляет GET `/api/v1/profiles` без заголовка с API key
- **Then** система возвращает 401 Unauthorized
- Evidence: HTTP-ответ с status code 401 и телом ошибки

### AC-003 API key аутентификация — невалидный ключ

- Почему это важно: запросы с неизвестными ключами не должны обрабатываться.
- **Given** сконфигурирован только API key `sk-abc123` для tenant `alpha`
- **When** клиент отправляет GET `/api/v1/profiles` с заголовком `Authorization: Bearer sk-unknown`
- **Then** система возвращает 401 Unauthorized
- Evidence: HTTP-ответ с status code 401 и телом ошибки

### AC-004 Кастомный заголовок аутентификации

- Почему это важно: разные окружения используют разные заголовки для API key.
- **Given** tenant `beta` настроен с auth header `X-Api-Key`, auth_scheme `raw` и API key `token-xyz`
- **When** клиент отправляет GET `/api/v1/profiles` с заголовком `X-Api-Key: token-xyz`
- **Then** запрос проходит аутентификацию, tenant ID `beta` доступен handler-у
- Evidence: handler получает tenantID = `beta` в контексте, запрос не отклонён

### AC-010 Default заголовок X-Mask-Authorization

- Почему это важно: tenant может не указывать заголовок — используется стандартный.
- **Given** tenant `gamma` настроен без auth_header (используется default `X-Mask-Authorization`), с API key `mk-key-789`
- **When** клиент отправляет GET `/api/v1/profiles` с заголовком `X-Mask-Authorization: mk-key-789`
- **Then** запрос проходит аутентификацию, tenant ID `gamma` доступен handler-у
- Evidence: handler получает tenantID = `gamma` в контексте, запрос не отклонён

### AC-005 Изоляция профилей по tenant

- Почему это важно: tenant A не должен видеть профили tenant B.
- **Given** tenant `alpha` имеет профиль `p1`, tenant `beta` — профиль `p2`
- **When** клиент tenant `alpha` запрашивает GET `/api/v1/profiles`
- **Then** в ответе присутствует только профиль `p1`
- Evidence: response body содержит `[{"slug": "p1", ...}]` без `p2`

### AC-006 Tenant-scoped routing

- Почему это важно: routing правила должны применяться в контексте tenant.
- **Given** tenant `alpha` имеет RoutingRule c моделью `gpt-4`, провайдер `ProviderA`
- **When** клиент tenant `alpha` отправляет POST `/v1/chat/completions` с `model: gpt-4` через API key `sk-abc123`
- **Then** система маршрутизирует запрос к `ProviderA`
- Evidence: вызов `RouteSelector.Select("gpt-4", "alpha")` возвращает provider из правила tenant-alpha

### AC-007 Propagation X-Tenant-ID в upstream

- Почему это важно: upstream может использовать tenant ID для своей маршрутизации/биллинга.
- **Given** аутентифицированный запрос tenant `alpha`
- **When** система отправляет запрос к upstream LLM-провайдеру
- **Then** исходящий HTTP-запрос содержит заголовок `X-Tenant-ID: alpha`
- Evidence: egress-клиент устанавливает `X-Tenant-ID` header в upstream-запросе

### AC-008 Tenant ID в логах

- Почему это важно: observability для multi-tenant операций.
- **Given** аутентифицированный запрос tenant `alpha`
- **When** система логирует событие в рамках этого запроса
- **Then** лог-запись содержит атрибут `tenant_id=alpha`
- Evidence: structured log entry содержит `"tenant_id": "alpha"`

### AC-009 Tenant ID в метриках

- Почему это важно: мониторинг использования по tenants.
- **Given** аутентифицированный запрос tenant `alpha`
- **When** система записывает HTTP-метрику
- **Then** метрика содержит label `tenant="alpha"`
- Evidence: `/metrics` endpoint содержит `http_requests_total{tenant="alpha", ...}`

## Допущения

- API key → tenant mapping статичен на уровне файла конфигурации (YAML).
- Default auth header: `X-Mask-Authorization` (auth_scheme=raw), если tenant не указал свой auth_header.
- API key передаётся как `Bearer <key>` (auth_scheme=bearer) или сырым значением в кастомном заголовке (auth_scheme=raw).
- Tenant ID из API key валиден (совпадает с существующим tenant ID в RoutingRules/Profile).
- Tenant → profile = 1:1. Tenant.profile_slug указывает, какой профиль назначен tenant. Profile.TenantID = Tenant.Slug.
- `value.TenantID` — value object существует, но может быть переиспользован или заменён на Tenant.Slug.
- Public endpoints (/health, /ready, /live, /metrics) не требуют аутентификации.

## Критерии успеха

- SC-001 Время обработки запроса не увеличивается более чем на 5ms (auth middleware overhead).
- SC-002 Все защищённые endpoint-ы (profiles, incidents, shield, routing) возвращают 401 при отсутствии API key. Endpoint-ы /health, /ready, /live, /metrics остаются доступными без аутентификации.

## Краевые случаи

- API key передан в неправильном заголовке (например, tenant ждёт `X-Api-Key`, а клиент шлёт `Authorization`) — 401.
- Несколько tenants с одинаковым ключом — не поддерживается (key → tenant, 1:1).
- Tenant не найден (ключ есть, tenant с таким ID не сконфигурирован в RoutingRules) — fallback на "default" при routing, пустой список профилей.
- Пустой API key — 401.
- Tenant ID в контексте отсутствует (запрос не прошёл auth middleware) — handler возвращает 401.
- Custom auth header с auth_scheme=raw: весь заголовок = API key, без парсинга префиксов.
- API key найден в reverse index, но tenant.auth_header не совпадает с заголовком, из которого извлечён ключ — 401 (защита от перебора ключей через чужой заголовок).
- `value.TenantID` из domain/shield/value — при появлении Tenant entity может быть рефакторинг, обратная совместимость не гарантируется на уровне value object.

## Открытые вопросы

1. `value.TenantID` в `domain/shield/value/` — во время MVP остаётся как есть. При рефакторинге: перенести в `domain/tenant/` или заменить на Tenant.Slug напрямую.
2. Персистентное хранение Tenant entity в PostgreSQL — пока in-memory из static config. При появлении потребности в динамическом управлении — переходить на БД.
