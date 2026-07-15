# Roadmap MaskChain

**MaskChain — платформа для обратимого маскирования данных в AI.**

```
MaskChain Gateway    — AI-прокси с маскированием, роутингом, fallback
MaskChain Mask API   — /mask и /unmask для обратимого преобразования
MaskChain Tenants    — тенанты, словари и политики маскирования
MaskChain Sessions   — карта замен на всю цепочку диалога (новое)
MaskChain Analytics  — токены, стоимость, трафик использования (новое)
```

**Легенда:** ✅ реализовано, ⬜ запланировано

---

## Группа 0: Фундамент ✅

### 00-project-foundation ✅

**Цель:** Go-модуль, DDD + Clean Architecture структура, билд-система, линтеры, Docker.

**Ключевые артефакты:** `go.mod`, Makefile, `.golangci.yml`, `.editorconfig`, структура `src/cmd/`, `src/internal/`, Dockerfile.

---

### 01-config-bootstrap ✅

**Цель:** cobra + viper конфигурация, YAML/ENV/флаги, валидация.

**Ключевые артефакты:** `src/internal/infra/config/` — Config, LoadConfig(), defaults, env-чтение, validation.

---

## Группа 1: Gateway Runtime ✅

### 10-gateway-skeleton ✅

**Цель:** Gin HTTP server с graceful shutdown, middleware chain, health endpoints.

**Ключевые артефакты:** Server struct, RequestID/Logger/Recovery/CORS middleware, `/health`, `/ready`, `/live`.

---

## Группа 2: Shield Domain ✅

### 20-shield-domain ✅

**Цель:** Domain-слой Content Shield: маскирование, детекция, реакции.

**Ключевые артефакты:** `src/internal/domain/shield/entity/` — Detector, Reaction, ScanResult, Pattern, Session. `src/internal/domain/shield/value/` — TenantID, Severity. Domain services.

---

### 21-shield-detectors ✅

**Цель:** PII (email, phone, SSN, паспорт), secrets (API key, JWT), financial (Luhn, IBAN, SWIFT).

**Ключевые артефакты:** Detector interface + registry + PIIDetector, SecretsDetector, FinancialDetector, CompositeDetector.

---

### 22-shield-mask-storage ✅

**Цель:** Обратимое template-based маскирование. Mask entries в PG + Valkey.

**Ключевые артефакты:** MaskEntry entity, MaskStorage interface, MaskUseCase. Write-through/read-through кэш. `POST /api/v1/shield/mask`, `POST /api/v1/shield/unmask`.

---

### 23-shield-reactions ✅

**Цель:** Механизм реакций: block, redact, mask, alert.

**Ключевые артефакты:** ReactionExecutor + BlockReaction (403), RedactReaction (`***`), MaskReaction (частичное скрытие), AlertReaction (лог). Pipeline через PolicyEvaluator.

---

### 24-shield-dictionaries ✅

**Цель:** Словари — именованные списки значений. MatchMode: exact, contains, regex, fuzzy.

**Ключевые артефакты:** Dictionary entity, DictionaryRepository, DictionaryDetector, WordlistMatcher. Exact — HashSet, Contains — Aho-Corasick, Regex — компиляция.

---

### 25-shield-preprocessors ✅

**Цель:** CSV/JSON препроцессоры для структурированных данных.

**Ключевые артефакты:** Processor interface, CSVProcessor, JSONProcessor (JSONPath, wildcard). PreprocessorDef в JSONB.

---

## Группа 3: Persistence ✅

### 30-shield-persistence ✅

**Цель:** PostgreSQL repository для словарей, mask entries, сессий.

**Ключевые артефакты:** Миграции, connection pool, DictionaryRepo, MaskEntryRepo, SessionRepo, PGXTransactionManager.

---

## Группа 4: Policies API & UI ✅

### 40-policies-api ✅

**Цель:** REST API CRUD политик маскирования (словари + препроцессоры inline).

**Ключевые артефакты:** Gin handlers: CreatePolicy, GetPolicy, ListPolicies, UpdatePolicy, DeletePolicy, PatchDictionary. DTO с inline-словарями и препроцессорами.

---

### 41-policies-ui ✅

**Цель:** React (Vite + TypeScript) интерфейс управления политиками.

**Ключевые артефакты:** Vite + React + TypeScript. PolicyList, PolicyDetail, PolicyForm. DictionaryEditor, PreprocessorEditor. Routing: `/policies`, `/policies/new`, `/policies/:slug`.

---

## Группа 5: Shield Runtime ✅

### 50-shield-engine ✅

**Цель:** Оркестратор сканирования: препроцессоры → детекторы → реакции → результат.

**Ключевые артефакты:** ScanUseCase, ApplyPolicyUseCase. ScanPipeline. ShieldEngine. Placeholder-based masking.

---

### 51-shield-gateway-integration ✅

**Цель:** Shield Engine в request lifecycle gateway.

**Ключевые артефакты:** ShieldMiddleware — перехват POST `/v1/chat/completions`. Policy resolution. Pre-request scan. Headers: `X-Shield-Status`.

---

## Группа 6: Observability ✅

### 61-observability ✅

**Цель:** OpenTelemetry, Prometheus metrics, structured logging, distributed tracing.

**Ключевые артефакты:** OTel SDK (traces + metrics). Prometheus counters (requests, shield_blocks, latency). slog handler с trace_id/span_id. Gin middleware (trace propagation, request duration). docker-compose: Prometheus + Grafana.

---

## Группа 7: Routing & Egress ✅

### 70-routing-engine ✅

**Цель:** Provider registry, model→provider mapping, fallback, health-aware routing.

**Ключевые артефакты:** Provider, Route, RoutingRule entities. ProviderRegistry, RouteSelector, FallbackHandler, HealthChecker. Config-based YAML routing. Прокси-хендлер `POST /v1/chat/completions`. Header `X-Provider`.

---

### 71-egress-streaming ✅

**Цель:** HTTP/HTTPS egress с proxy dialer, SSE streaming, retries, cancellation.

**Ключевые артефакты:** Egress HTTP client. SSE chunk forwarding. Retry (exponential backoff + full jitter). Context cancellation. Connection pooling. NO_PROXY.

---

## Группа 8: Multi-Tenancy ✅

### 80-tenant-isolation ✅

**Цель:** API key → tenant mapping, изоляция политик, tenant-scoped routing.

**Ключевые артефакты:** Tenant entity, APIKey value object, auth middleware. Tenant-scoped config overrides. `X-Tenant-ID` propagation.

---

## Группа 9: Production Hardening ✅

### 81-rate-limiting-budgets ✅

**Цель:** Rate limiting + token budgets. Valkey sliding window counters.

**Ключевые артефакты:** TokenBudget, RateLimit entities. Sliding window (Valkey Sorted Set). Per-tenant, per-model лимиты. 429 с Retry-After.

---

### 90-production-hardening ✅

**Цель:** Performance tuning, load testing, security audit.

**Ключевые артефакты:** pprof endpoints, security-check (gitleaks), load-test (python), pool tuning.

---

## Группа 10: Architecture Split ✅

### 100-admin-control-plane ✅

**Цель:** Выделение admin binary со своим Dockerfile. Gateway — лёгкий data plane.

**Ключевые артефакты:** `src/cmd/admin/main.go`, `src/internal/api/admin.go`, `Dockerfile.admin`, `Dockerfile.gateway`. Раздельные сигналы.

---

### 101-gateway-diet ✅

**Цель:** Минимальный gateway binary (<100ms старт, ~15MB image).

**Ключевые артефакты:** Build tag `gateway`/`admin`. UI embed только в admin. CGO_ENABLED=0.

---

### 102-policy-cache ✅

**Цель:** Read-through кэш политик на Valkey + in-memory LRU для gateway.

**Ключевые артефакты:** PolicyCache (Valkey + LRU). Write-through/read-through. PubSub-инвалидация. Graceful degradation.

---

## Группа 11: Provider Adapters ✅

### 110-provider-adapters ✅

**Цель:** Реальные HTTP-клиенты для LLM-провайдеров.

**Ключевые артефакты:** `openai.go` — OpenAI-compatible, `anthropic.go` — Anthropic Messages API. Фабрика `NewProviderClient(cfg)`. Call + Stream. Per-provider transport.

---

### 111-provider-auth-config ✅

**Цель:** API key management, secure storage, per-provider auth.

**Ключевые артефакты:** `ProviderConfig.APIKeys`, `AuthScheme` (bearer/api-key/basic), `AuthHeader`. Чтение из env. Validation. Sensitive-логи.

---

### 112-proxy-streaming-wiring ✅

**Цель:** SSE streaming через весь pipeline: клиент → shield → routing → provider.

**Ключевые артефакты:** `stream: true` detection. `gin.Context.Stream()` форвард. Cancellation propagation. FallbackHandler.Stream().

---

## Группа 12: Production Readiness ✅

### 113-shield-middleware-wiring ✅

**Цель:** Полный cycle: tenant → policy → scan → reaction.

**Ключевые артефакты:** Policy resolution per-request. Graceful degradation (default_action). Интеграционный тест.

---

### 114-real-health-probes ✅

**Цель:** Dependency-aware health checks.

**Ключевые артефакты:** PGProbe, ValkeyProbe, EgressProbe. Три состояния: ok/degraded/down. Configurable critical deps.

---

### 115-rate-limit-wiring ✅

**Цель:** Rate limiting в gateway lifecycle.

**Ключевые артефакты:** Valkey repos → RateLimit middleware → RegisterRateLimit(). Per-tenant overrides. Token budgets. Метрики.

---

### 116-connection-pool-fixes ✅

**Цель:** Исправление багов egress-клиента, circuit breaker, TLS/mTLS.

**Ключевые артефакты:** MaxIdleConnsPerHost. Per-provider timeout. TLS config (CA, mTLS, insecure). Circuit breaker. Per-provider transport.

---

### 117-critical-test-coverage ✅

**Цель:** Тесты на critical path.

**Ключевые артефакты:** routing handler tests, shield middleware tests, mask handler tests, server tests. Integration test (full cycle).

---

### 118-api-consistency ✅

**Цель:** Единый API-стандарт `/api/v1/*`, OpenAPI, envelope.

**Ключевые артефакты:** `POST /api/v1/chat/completions`. Envelope middleware. OpenAPI 3.1 spec. Swagger UI. SPA fallback. `/v1/*` → 301 redirect.

---

## Группа 13: Sessions ⬜ (MaskChain 2.0)

**Контекст:** Сессия хранит карту замен `{placeholder: original}` на протяжении всей цепочки диалога. Клиент отправляет `X-Session-ID`, shield middleware пишет замены в сессию, `/unmask` по SessionID восстанавливает оригинал всего диалога.

### 120-sessions-domain ⬜

**Цель:** Domain-слой Sessions.

**Ключевые артефакты:**
- `Session` entity: `SessionID`, `TenantID`, `Model`, `ReplacementMap` (map[string]string), `TokenCount`, `MessageCount`, `Status` (active/expired/closed), `TTL`, `CreatedAt`, `ExpiresAt`
- `SessionID` value object (UUIDv7)
- `ReplacementMap` value object — упорядоченная карта замен с поддержкой diff
- `SessionStore` port interface: `Save`, `Get`, `AppendReplacements`, `ExtendTTL`, `Close`, `DeleteExpired`
- `SessionUseCase`: create session, append replacements, unmarshal by session, close/expire

**Зависимости:** 20-shield-domain, 80-tenant-isolation, 01-config-bootstrap

---

### 121-sessions-storage ⬜

**Цель:** Postgres + Valkey repository для Sessions.

**Ключевые артефакты:**
- Миграция: `CREATE TABLE sessions (id UUID, tenant_id TEXT, model TEXT, replacements JSONB, token_count BIGINT, message_count INT, status TEXT, ttl INTERVAL, created_at TIMESTAMPTZ, expires_at TIMESTAMPTZ)`
- Индекс по `tenant_id`, `status`, `expires_at`
- `PostgresSessionStore` — CRUD + `AppendReplacements` (JSONB merge)
- `ValkeySessionCache` — TTL-based кэш с read-through/write-through
- `CleanupWorker` — фоновый inverval removal expired sessions (optional)
- Graceful degradation: если Valkey недоступен — работа через PG (fail-open)

**Зависимости:** 120-sessions-domain, 30-shield-persistence

---

### 122-sessions-api ⬜

**Цель:** REST API для управления сессиями.

**Ключевые артефакты:**
- `POST /api/v1/sessions` — создать сессию, вернуть `session_id`
- `GET /api/v1/sessions/:id` — получить метаданные сессии
- `GET /api/v1/sessions/:id/replacements` — получить карту замен (только для admin/tenant owner)
- `PATCH /api/v1/sessions/:id/extend` — продлить TTL
- `DELETE /api/v1/sessions/:id` — закрыть сессию
- `POST /api/v1/unmask/session/:id` — unmarshal всего диалога по SessionID
- Пагинация для `GET /api/v1/sessions`
- Tenant-scoped: тенант видит только свои сессии
- OpenAPI spec + Swagger UI

**Зависимости:** 121-sessions-storage, 118-api-consistency, 80-tenant-isolation

---

### 123-sessions-gateway ⬜

**Цель:** Интеграция Sessions в request lifecycle gateway.

**Ключевые артефакты:**
- Middleware: читает `X-Session-ID` из запроса, создаёт сессию если нет, кладёт в контекст
- Shield middleware: **пишет** `{placeholder → original}` в активную сессию вместо создания Incident
- Session middleware: пробрасывает `X-Session-ID` в ответ
- Headers: `X-Session-ID`, `X-Session-Expires-At`, `X-Session-Message-Count`
- Config: `session.ttl` (default 30m), `session.max_messages` (default 100)
- Metrics: `session_active_total`, `session_replacements_total`
- Graceful degradation: если session store недоступен — маскирование работает без сохранения сессии (fail-open)

**Критично для:** цепочки диалогов (multi-turn). Без сессий каждый запрос маскируется независимо, и unmarshal одного запроса не восстанавливает контекст предыдущих.

**Зависимости:** 122-sessions-api, 51-shield-gateway-integration, 61-observability

---

## Группа 14: Analytics ⬜ (MaskChain 2.0)

**Контекст:** Аналитика использования: токены, стоимость, трафик по tenant-ам и моделям. Замена метрикам инцидентов — теперь считаем не «нарушения», а «использование».

### 130-analytics-domain ⬜

**Цель:** Domain-слой аналитики.

**Ключевые артефакты:**
- `TokenUsage` entity: `TenantID`, `Model`, `InputTokens`, `OutputTokens`, `Cost`, `Timestamp`
- `UsageRecord` value object: агрегированная запись за период
- `CostRate` value object: стоимость токена по модели (config-driven)
- `UsageStore` port interface: `Record`, `QueryByTenant`, `QueryByModel`, `AggregateByDay`
- `Aggregation` entity: агрегаты по tenant/model/day: total_tokens, total_cost, request_count, avg_latency

**Зависимости:** 80-tenant-isolation, 01-config-bootstrap

---

### 131-analytics-pipeline ⬜

**Цель:** Сбор и агрегация метрик использования.

**Ключевые артефакты:**
- `UsageMiddleware` — пост-обработка каждого запроса: читает токены из response, записывает UsageRecord
- Token counting: из response body (usage поле в OpenAI-формате) или fallback через tiktoken
- Async worker: буферизованная запись в UsageStore (batch insert каждые 5s)
- Aggregation worker: материализованные агрегаты (per-hour, per-day), cleanup сырых данных
- Prometheus: `maskchain_tokens_total{tenant,model,type=input|output}`, `maskchain_cost_total{tenant,model}`, `maskchain_request_total{tenant,model}`

**Критично для:** — операторы хотят видеть сколько тратят на каждый tenant и модель.

**Зависимости:** 130-analytics-domain, 61-observability, 70-routing-engine

---

### 132-analytics-api ⬜

**Цель:** API и дашборды аналитики.

**Ключевые артефакты:**
- `GET /api/v1/analytics/tokens` — токены по tenant/model за период (day/week/month)
- `GET /api/v1/analytics/cost` — стоимость по tenant/model
- `GET /api/v1/analytics/traffic` — количество запросов, latency P50/P95/P99
- `GET /api/v1/analytics/tenants/:slug/summary` — сводка по конкретному tenant
- Grafana dashboard provisioning: `deployments/grafana/dashboards/analytics.json`
- OpenAPI spec
- Tenant-scoped: тенант видит только свою аналитику
- CSV/JSON export для reporting

**Зависимости:** 131-analytics-pipeline, 118-api-consistency, 41-policies-ui

---

## PostMVP (после 2.0)

| Slug | Описание |
|---|---|
| `advanced-detectors` | ML-based классификаторы, context-aware PII, multi-language |
| `policy-engine` | OPA/Rego policies, WASM filters, declarative policy model |
| `envoy-data-plane` | Envoy ext_proc gRPC + xDS control plane. Build tag `envoy` |
| `k8s-operator` | Kubernetes Operator: CRDs, reconciliation, Gateway API |
| `shield-benchmark` | Performance benchmark: throughput, latency, accuracy |
| `mcp-integration` | Model Context Protocol — маскирование на уровне MCP-тулов |
| `agent-support` | Маскирование в multi-agent цепочках (агент вызывает агента) |
| `cache-prompt` | Кэширование общих prefix-ов промптов для экономии токенов |
| `model-selector` | Автоматический выбор модели под задачу (cost/quality balance) |

---

## Порядок разработки (2.0.0)

```
Уже реализовано (1.0.0):
00 → 01 → 10 → 20 → 21 → 22 → 23 → 24 → 25
                                             ↓
30 → 40 → 41 → 50 → 51 → 61
                                    ↓
70 → 71 → 80 → 81 → 90 → 100 → 101 → 102
                                    ↓
110 → 111 → 112 → 113 → 116 → 114 → 115 → 117 → 118

Новые фазы 2.0.0:
120 → 121 → 122 → 123
  ↓
130 → 131 → 132
```

Все фазы 1.0.0 выполнены. Фазы 120–132 — план MaskChain 2.0.
