# Roadmap MaskChain

**Платформа для обратимого маскирования данных в AI — Enterprise AI Gateway.**

```
Текущее:   v1.0 — Content Shield DLP + Routing + Multi-Tenancy ✅
           v2.0 — Sessions + Analytics + Platform Maturity ✅
Стратегия: v3.0 — Enterprise AI Gateway (Feature Parity + Differentiation)
```

---

## Статус: v1.0 + v2.0 — Выполнено ✅

| Домен | Ключевое |
|---|---|
| **Content Shield** | PII/PHI/Financial/Secrets/Dictionary детекторы, reversible mask, streaming unmask, Aho-Corasick, реакции block/redact/mask/alert, CSV/JSON препроцессоры |
| **Routing & Egress** | Provider registry, model→provider mapping, fallback, health-aware routing, circuit breaker, retry, per-provider proxy (HTTP/SOCKS5), SSE streaming, connection pooling |
| **Multi-Tenancy** | API key → tenant mapping, tenant-scoped политики, словари, PII-правила, rate limiting (Valkey sliding window) |
| **Observability** | OTel distributed tracing, Prometheus metrics, structured logging, dependency-aware health probes |
| **Platform** | Go single binary (~18MB), 3 Dockerfile (gateway/admin/combined), Helm chart, GitHub Actions CI/CD, 14 linters |
| **Admin UI** | React (Vite+TS) management UI: политики, словари, инциденты, тенанты |
| **Analytics** | Token usage, cost tracking, per-tenant/per-model агрегация, API + Prometheus |
| **Sessions** | Session tracking per dialog: токены, маски, модель, tenant, TTL, cleanup |

---

## Стратегия: v3.0 — Enterprise AI Gateway

**Цель:** Feature parity с LiteLLM в ключевых enterprise-возможностях при сохранении дифференциации в Content Shield DLP.

### Принципы (из конституции)

1. **Content Shield — core domain**: всё новое не должно ослаблять DLP
2. **Tenant-driven policies**: каждое расширение — tenant-scoped
3. **Extensibility over hardcoding**: plugins, interfaces, adapters
4. **Native-only data plane**: Go, никаких external runtime-зависимостей
5. **AI traffic is network traffic**: passthrough, не translation
6. **Infrastructure, not chatbot**: никакого agent framework, prompt playground

---

## Фаза 1: Identity & Access Management (Virtual Keys)

**Проблема:** Tenant API keys — raw-ключи без scoping. Невозможно дать доступ "только к GPT-4 с бюджетом $50".

**Конкуренты:** LiteLLM virtual keys — scoped per model, budget, team, metadata.

### 300-virtual-keys

| Артефакт | Описание |
|---|---|
| `VirtualKey` entity | `key_hash`, `tenant_id`, `label`, `allowed_models []string`, `blocked_models []string`, `budget_cap`, `spent`, `expires_at`, `metadata`, `enabled` |
| `VirtualKeyRepository` port/impl (PG) | CRUD + `FindByKeyHash()` + `ListByTenant()` + soft-delete |
| `VirtualKeyAuth` middleware | Извлекает ключ из `Authorization`, резолвит tenant, проверяет `enabled`/`expires_at`, добавляет scope в контекст |
| `ModelAccess` middleware | Проверяет `allowed_models`/`blocked_models` перед routing |
| Admin API CRUD | `POST/GET /api/v1/admin/keys`, `GET /api/v1/admin/keys/:id`, `DELETE /api/v1/admin/keys/:id` |
| Admin UI — Key Management | Создание, просмотр, отзыв ключей; копирование ключа один раз при создании |
| Audit log | Все операции с ключами логируются в audit_trail |
| Миграция существующих tenant keys | Backfill: создать VirtualKey для каждого существующего tenant API key |

**Зависимости:** 80-tenant-isolation ✅

**Критично:** Virtual keys — фундамент для бюджетов, spend tracking и team management.

---

## Фаза 2: Financial Operations (Spend + Budgets)

**Проблема:** Аналитика есть, но нет enforcement — тенанты могут превысить бюджет без блокировки.

**Конкуренты:** LiteLLM spend tracking per key/user/team + hard/soft budgets + alerts.

### 301-budget-enforcement

| Артефакт | Описание |
|---|---|
| `Budget` entity | `id`, `tenant_id`, `virtual_key_id` (optional), `scope` (tenant|key|model), `type` (monthly|daily|custom), `soft_limit`, `hard_limit`, `currency`, `notify_at []float64` (проценты) |
| `BudgetRepository` port/impl (PG + Valkey) | Valkey counter `budget:{scope}:{period}`, PG для persistence и истории |
| `BudgetMiddleware` | После завершения запроса: инкремент spent + проверка превышения hard_limit → 429/403 |
| `BudgetAlert` | При превышении `notify_at` → webhook + лог |
| `SpendAggregation` | Materialized hourly/daily spend per key + tenant + model |
| Admin API | `POST/GET /api/v1/admin/budgets`, `GET /api/v1/admin/budgets/:id/history` |
| Admin UI — Budget Dashboard | Прогресс-бары, уведомления, история spend |

**Зависимости:** 300-virtual-keys, 132-analytics-api ✅

### 302-cost-rates-auto

| Артефакт | Описание |
|---|---|
| `CostRateRegistry` | Автоматическое обновление цен моделей (из community provider defs или встроенной таблицы) |
| `CostRate` entity | `model`, `provider`, `input_price_per_1k`, `output_price_per_1k`, `currency`, `updated_at` |
| Fallback cost estimation | Если модели нет в таблице — fallback по `input_price`/`output_price` из конфига провайдера |

**Зависимости:** 301-budget-enforcement, 110-provider-adapters ✅

---

## Фаза 3: Provider Ecosystem

**Проблема:** 6 провайдеров vs 100+ у LiteLLM. Каждый новый провайдер — hardcode.

**Конкуренты:** LiteLLM — 100+ provider definitions в Python + community contributions.

### 310-provider-registry-plugin

| Артефакт | Описание |
|---|---|
| `ProviderDefinition` entity | `name`, `api_type`, `base_url_pattern`, `auth_scheme`, `supported_endpoints []string`, `models []ModelDef`, `cost_rates` |
| Community provider definitions | YAML-файлы в `providers/` или external репозиторий. Регистрация через `routing.providers.definitions_path` в конфиге |
| `DynamicProviderClient` adapter | Passthrough-клиент, конфигурируемый `ProviderDefinition` (endpoints, auth, headers) |
| Provider health probe per definition | Generic health check: `GET {base_url}/health` или `HEAD {base_url}` |
| Fallback cost estimation | Если модели нет в таблице — fallback по `input_price`/`output_price` из конфига |
| CLI tool | `maskchain provider add <name> --api-type openai --base-url ...` |

**Дизайн-решение:** MaskChain — passthrough, не translation. DynamicProviderClient не конвертирует форматы (кроме уже реализованных gemini/bedrock). Все неподдерживаемые форматы — raw passthrough с заголовками из конфига. Это в 100x упрощает добавление новых провайдеров.

**Зависимости:** 110-provider-adapters ✅, 111-provider-auth-config ✅

### 311-more-api-endpoints

| Артефакт | Описание |
|---|---|
| `POST /v1/embeddings` | Passthrough handler + optional shield scan |
| `POST /v1/images/generations` | Passthrough handler + shield scan для prompt |
| `POST /v1/audio/transcriptions` | Passthrough handler |
| `POST /v1/audio/speech` | Passthrough handler |
| `POST /v1/completions` | Legacy completions passthrough |
| `POST /v1/models` | Proxy: aggregator моделей от всех активных провайдеров |
| Generic passthrough route | `POST /v1/:provider/*path` — raw proxy для любого provider-specific endpoint |

**Дизайн:** Каждый новый endpoint — 20-30 строк handler + route registration. Никакого translation. Content Shield применяется только к текстовым полям.

**Зависимости:** 310-provider-registry-plugin (для discovery supported endpoints)

---

## Фаза 4: Advanced Routing

**Проблема:** Только sequential fallback. Нет weighted routing, A/B testing, traffic mirroring.

**Конкуренты:** LiteLLM weighted routing, traffic mirroring, A/B testing, router plugins.

### 320-weighted-routing

| Артефакт | Описание |
|---|---|
| `weight` field в provider config | `providers[].weight` (int, default 1) |
| `WeightedRouteSelector` | Weighted random selection (reservoir sampling) |
| Fallback chain per weight group | Если выбранный упал — следующий по весу, не по порядку |
| Metrics | `maskchain_route_selected{tenant,model,provider}` с тегом выбора |

**Зависимости:** 70-routing-engine ✅

### 321-traffic-mirroring

| Артефакт | Описание |
|---|---|
| `MirrorConfig` в route config | `mirror: { provider: "openai", sample_rate: 0.1, headers: {...} }` |
| `MirroringClient` adapter | Оборачивает `ProviderClient`. После успешного Call/Stream — fire-and-forget к mirror-провайдеру в отдельной горутине |
| Metrics | `maskchain_mirror_sent{primary_provider,mirror_provider}` |
| Логирование | Результаты mirror записываются в `mirror_log` (PG или S3), не влияют на response клиенту |

**Зависимости:** 320-weighted-routing

### 322-router-plugins

| Артефакт | Описание |
|---|---|
| `RoutingPlugin` port interface | `Process(ctx *RoutingContext) error` — может сужать список кандидатов, добавлять сигналы в метаданные |
| `RoutingPluginRegistry` | Pipeline в `RouteSelector.Select()`: plugin[0] → plugin[1] → ... → final selection |
| Plugin sources | Встроенные (language-detector, cost-optimizer, latency-prioritizer) + WASM-hosted (`wasmtime-go`) |
| Built-in plugin: `cost-optimizer` | Выбирает дешёвого провайдера для модели, если не задан explicit routing |
| Built-in plugin: `latency-prioritizer` | Выбирает провайдера с наименьшей latency (исторической, из метрик) |
| Config | `routing.plugins: [{name: "cost-optimizer", config: {...}}]` |

**Зависимости:** 70-routing-engine ✅

**Дизайн-решение:** WASM-host для community plugins — кроссплатформенный, безопасный (sandbox), работает с любой версией Go. Встроенные плагины — нативные Go-структуры.

---

## Фаза 5: Observability & Compliance

**Проблема:** Только OTel + Prometheus. Нет интеграции с внешними observability-платформами.

**Конкуренты:** LiteLLM — 15+ logging integrations (Langfuse, LangSmith, Datadog, S3, GCS, Azure, etc.).

### 330-log-exporters

| Артефакт | Описание |
|---|---|
| `LogExporter` port interface | `Export(ctx, *AuditRecord) error` |
| `WebhookLogExporter` | POST JSON на внешний endpoint (batch или per-event) |
| `S3LogExporter` | Периодическая загрузка batch-файлов в S3-compatible storage (JSON Lines / Parquet) |
| `DatadogLogExporter` | Datadog API logs intake |
| `LogExporterRegistry` | per-tenant: какие exporter включены, с каким sampling rate |
| Audit record enrichment | `X-Request-ID`, `X-Session-ID`, virtual key label, route decision, shield verdict |
| Sampling | Per-exporter `sample_rate` для high-volume логов (список разрешённых событий всегда 100%) |
| Config | `logging.exporters: [{type: webhook, url: ..., sample_rate: 0.1}]` |

**Зависимости:** 61-observability ✅

### 331-jwt-oidc-auth

| Артефакт | Описание |
|---|---|
| `JWTValidator` middleware | Парсинг и валидация JWT (RS256/ES256), проверка `iss`, `aud`, `exp` |
| `OIDCProvider` | Discovery URL → JWKS → кэширование ключей |
| OIDC integration для Admin UI | Login через external IdP (Keycloak, Azure AD, Okta) |
| `AdminSession` JWT bridge | JWT → внутренняя admin сессия (для совместимости с существующим session store) |
| Config | `auth.jwt: { jwks_url: ..., audience: ..., issuer: ... }` |

**Дизайн-решение:** MaskChain НЕ становится identity provider — это external. SSO/SAML — через oauth2-proxy перед admin, не в ядро. Принцип **Infrastructure, Not Chatbot** (III).

**Зависимости:** 80-tenant-isolation ✅

---

## Фаза 6: Agent Infrastructure (PostMVP → Active)

**Проблема:** Клиенты хотят проксировать MCP/A2A трафик через MaskChain для DLP-сканирования.

### 340-mcp-gateway

| Артефакт | Описание |
|---|---|
| MCP protocol detection | Content-Type `application/vnd.mcp+json` или path prefix `/mcp/` |
| MCP proxy handler | Passthrough с shield scan для текстового содержимого tool calls |
| MCP tool discovery blocking | Blacklist/whitelist тулов per-tenant (через словари — названия тулов как dictionary entries с action=block) |
| `@sk-masked` аннотация | MCP tool response может содержать masked данные — автоматический unmask при возврате клиенту |

**Зависимости:** 51-shield-gateway-integration ✅, 24-shield-dictionaries ✅

### 341-a2a-gateway

| Артефакт | Описание |
|---|---|
| A2A agent registration | Статическая конфигурация: `agents: [{agent_name, url, capabilities}]` |
| A2A proxy handler | Proxy A2A запросов с shield scan на границе |
| Agent-to-agent DLP | Сканирование данных, которыми обмениваются агенты (body A2A messages) |

**Дизайн-решение:** MCP и A2A — это протоколы, а не agent framework. MaskChain не запускает агентов, а проксирует их трафик с DLP. Принцип **Infrastructure, Not Chatbot** соблюдается.

**Зависимости:** 340-mcp-gateway

---

## Фаза 7: Shield Deepening (Content Differentiation)

**Проблема:** LiteLLM guardrails поверхностные. MaskChain должен углубить отрыв в DLP.

### 350-moderation-detector

| Артефакт | Описание |
|---|---|
| `ModerationDetector` | Вызов OpenAI Moderation API (или self-hosted) как Detector в pipeline |
| Policy action per category | `action_on_hate: block`, `action_on_sexual: mask` |
| Config | `shield.detectors.moderation: { provider: openai, api_key: ..., categories: [hate, harassment, self-harm, sexual, violence] }` |

**Зависимости:** 21-shield-detectors ✅

### 351-context-aware-detectors

| Артефакт | Описание |
|---|---|
| `ContextAwareDetector` | Анализирует не фрагмент, а весь prompt + предыдущие turn-ы (из session history) |
| Sliding window context | Последние N токенов диалога для выявления data leakage через контекст |
| NLP-based PII | Использование spaGO или внешнего ONNX-early для NER |

**Зависимости:** 21-shield-detectors ✅, sessions ✅

---

## Порядок разработки

```
v3.0 Enterprise AI Gateway:

Фаза 1: Identity & Access
  300-virtual-keys ──── foundation, нет внешних зависимостей

Фаза 2: Financial Operations
  301-budget-enforcement ─── после 300 (virtual keys) + 132 ✅ (analytics)
  302-cost-rates-auto    ─── параллельно 301

Фаза 3: Provider Ecosystem
  310-provider-registry-plugin ─── после 110 ✅
  311-more-api-endpoints       ─── после 310 (discovery)

Фаза 4: Advanced Routing
  320-weighted-routing  ─── после 70 ✅
  321-traffic-mirroring ─── после 320
  322-router-plugins    ─── после 320

Фаза 5: Observability & Compliance
  330-log-exporters ─── после 61 ✅
  331-jwt-oidc-auth ─── после 80 ✅, optional external

Фаза 6: Agent Infrastructure (PostMVP → Active)
  340-mcp-gateway  ─── после 51 ✅
  341-a2a-gateway  ─── после 340

Фаза 7: Shield Deepening
  350-moderation-detector     ─── после 21 ✅
  351-context-aware-detectors ─── после sessions ✅
```

Приоритет: **Фаза 1 → Фаза 2 → Фаза 3** (core enterprise adoption).  
Фазы 4-7 — параллельно по готовности.

---

## Отстройка от LiteLLM (Differentiation Strategy)

| Область | MaskChain advantage | Действие |
|---|---|---|
| **Content Shield DLP** | Reversible mask, streaming unmask, Aho-Corasick dictionary | Углублять (Фаза 7) |
| **Performance** | Go binary, <100ms startup, ~18MB | Сохранять |
| **Tenant isolation** | Словари + PII-правила per-tenant | Virtual keys (Фаза 1) усилит |
| **Streaming unmask** | Уникальная фича | Маркетинг |
| **Ecosystem** | Отставание (6 vs 100+ providers) | Provider registry (Фаза 3) |

### Что НЕ делаем (сознательное no-go)

- Translation между форматами провайдеров (кроме gemini/bedrock) — passthrough только
- Agent framework / agent runtime — только проксирование протоколов
- Prompt playground / chatbot UI — инфраструктурный продукт
- SSO/SAML в ядре — external proxy
- Low-code / visual workflow

---

## Метрики успеха v3.0

| Метрика | Цель | Фаза |
|---|---|---|
| Virtual keys created | >80% tenant используют | Фаза 1 |
| Budget enforcement | 100% тенантов с active budget | Фаза 2 |
| Provider coverage | 50+ provider definitions | Фаза 3 |
| Log exporters | 4+ exporter types | Фаза 5 |
| Community providers | 10+ contributed provider YAML | Фаза 3 |
| MCP integration | E2E working MCP proxy | Фаза 6 |
