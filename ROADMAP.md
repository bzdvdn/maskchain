# Roadmap MaskChain

**MaskChain — платформа для обратимого маскирования данных в AI.**

```
MaskChain Gateway    — AI-прокси с маскированием, роутингом, fallback
MaskChain Mask API   — /mask и /unmask для обратимого преобразования
MaskChain Tenants    — тенанты, словари и политики маскирования
MaskChain Sessions   — статистика сессий по тенантам и моделям (новое)
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

## Группа 13: Sessions ✅ (MaskChain 2.0)

**Контекст:** Сессия трекает диалог: сколько сообщений, токенов, какая модель, какой тенант. Клиент отправляет `X-Session-ID`, shield middleware создаёт/обновляет сессию. Оператор смотрит статистику использования через REST API.

### sessions ✅

**Цель:** Session tracking — создание, обновление счётчиков, TTL, cleanup.

**Ключевые артефакты:**

- `Session` entity: `SessionID` (UUIDv7), `TenantID`, `Model`, `TokenCount`, `MessageCount`, `TotalMasks`, `DictMaskCount`, `PIIMaskCount`, `PreprocessorCount`, `Status` (active/expired/closed), `TTL`, `CreatedAt`, `ExpiresAt`
- `SessionID` value object (UUIDv7)
- `SessionStore` port interface: `Save`, `Get`, `IncrementCounts`, `ExtendTTL`, `Close`, `DeleteExpired`, `ListByTenant`
- `SessionUseCase`: create, increment counts, close, extend TTL, list, delete expired
- `PostgresSessionStore` — CRUD + атомарный UPDATE всех счётчиков
- `ValkeySessionCache` — TTL-based кэш (fail-open)
- `CleanupWorker` — фоновый interval-based cleanup expired сессий
- REST API: `POST/GET /api/v1/sessions`, `PATCH .../extend`, `DELETE .../id`
- Middleware: чтение `X-Session-ID`, создание/обновление сессии
- Tenant-scoped, пагинация, OpenAPI spec
- Graceful degradation: Valkey недоступен — работа через PG

**Зависимости:** 20-shield-domain, 80-tenant-isolation, 30-shield-persistence, 01-config-bootstrap

---

## Группа 14: Analytics ⬜ (MaskChain 2.0)

**Контекст:** Аналитика использования: токены, стоимость, трафик по tenant-ам и моделям. Замена метрикам инцидентов — теперь считаем не «нарушения», а «использование».

### 130-analytics-domain ✅

**Цель:** Domain-слой аналитики.

**Ключевые артефакты:**

- `TokenUsage` entity: `TenantID`, `Model`, `InputTokens`, `OutputTokens`, `Cost`, `Timestamp`
- `UsageRecord` value object: агрегированная запись за период
- `CostRate` value object: стоимость токена по модели (config-driven)
- `UsageStore` port interface: `Record`, `QueryByTenant`, `QueryByModel`, `AggregateByDay`
- `Aggregation` entity: агрегаты по tenant/model/day: total_tokens, total_cost, request_count, avg_latency

**Зависимости:** 80-tenant-isolation, 01-config-bootstrap

---

### 131-analytics-pipeline ✅

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

### 132-analytics-api ✅

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

## Группа 15: Platform Maturity ✅ (дополнительно к плану 1.0)

**Контекст:** Cross-cutting улучшения, выполненные параллельно с основным планом: CI/CD, Helm, egress proxy, качество кода, документация.

### 119-provider-egress-proxy ✅

**Цель:** Per-provider egress proxy: HTTP/HTTPS/SOCKS5 для каждого LLM-провайдера.

**Ключевые артефакты:**
- `ProviderConfig.ProxyURL` — опциональный `proxy_url` в конфиге провайдера
- `proxyFuncFromURL(url)` — HTTP/HTTPS proxy DialContext
- `socks5DialContext()` — SOCKS5 через `golang.org/x/net/proxy`
- `NewTransport(cfg, proxyURL)` — расширенная сигнатура
- Fallback: пустой `proxy_url` → `HTTP_PROXY` env var
- 8 unit-тестов в `proxy_test.go`

**Зависимости:** 71-egress-streaming, 110-provider-adapters, 01-config-bootstrap

---

### 120-tenant-profile-sync ✅

**Цель:** Миграция с ProfileRepository на Tenant entity + словари inline. Удаление deprecated ProfileRepository.

**Ключевые артефакты:**
- Tenant entity со встроенными Dictionaries + PIIConfig (вместо Profile)
- PostgresTenantRepo с 7 методами
- DBFirstTenantResolver с SyncConfig для синхронизации YAML→DB
- Auth middleware через TenantProvider
- DictionaryCache (бывший ProfileCache) с warm-up
- Миграции 005 (создание) + 008 (cleanup)
- Tenant CRUD handler + dictionaries endpoint

**Зависимости:** 80-tenant-isolation, 30-shield-persistence

---

### 121-ci-cd-pipeline ✅

**Цель:** GitHub Actions CI/CD — lint, test, build, docker, helm, smoke, docker-push.

**Ключевые артефакты:**
- `.github/workflows/ci.yml` — 6 стадий (lint → test → build → docker → helm → smoke)
- Docker Push на push в main/master
- `.github/dependabot.yml` — 4 экосистемы (Go, GitHub Actions, Docker, Helm)
- `src/pkg/version/version.go` — Version/Commit/Date через ldflags
- Makefile: `ci` target, `helm-lint`, `security-check`, `test` с `-race -count=1`

**Зависимости:** 00-project-foundation

---

### 122-docs-community ✅

**Цель:** Полная документация и community standards.

**Ключевые артефакты:**
- README: бейджи, comparison table (MaskChain vs LiteLLM vs privacy-filter), Use Cases
- `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`
- `docs/TUTORIAL.md` — 5-minute walkthrough
- `docs/DEPLOYMENT.md` — Docker Compose + Helm + Bare Binary
- `docs/SHIELD.md` — architecture deep-dive content shield
- `docs/PERFORMANCE.md` — benchmarks and tuning
- `demo/pii-demo.sh` — демо PII-маскирования
- `.github/ISSUE_TEMPLATE/` (bug_report.yml, feature_request.yml, config.yml)
- `.github/PULL_REQUEST_TEMPLATE.md`

**Зависимости:** все фазы

---

### 123-quality-config-refactoring ✅

**Цель:** Качество кода: расширение линтеров, рефакторинг GOD-объекта конфига.

**Ключевые артефакты:**
- `.golangci.yml`: 6→14 linters (+gosec, gosimple, bodyclose, noctx, thelper, prealloc, misspell, exportloopref)
- `config.go` рефакторинг: 864→7 файлов (config.go, validator.go, watcher.go, defaults.go, diff.go, serialize.go, cmd.go)
- Docker security: `.dockerignore` (5→30 entries), hardened multi-stage, nonroot, LABELs
- 3 Dockerfile: `Dockerfile`, `Dockerfile.gateway`, `Dockerfile.admin`

**Зависимости:** 01-config-bootstrap, 00-project-foundation

---

### 124-helm-chart ✅

**Цель:** Полноценный Helm chart для Kubernetes deployment.

**Ключевые артефакты:**
- `deployments/helm/maskchain/` — 20+ файлов
- Bitnami PostgreSQL/Valkey subchart dependencies
- ConfigMap split: base (инфраструктура) + runtime (бизнес-логика)
- Secret `apiKeys` с `${VAR}` placeholder resolution
- Deployment/Service/Ingress для gateway/admin/all режимов
- Gateway API HTTPRoute, ServiceMonitor, PDB, NetworkPolicy
- `helm-lint` в CI

**Зависимости:** 100-admin-control-plane, 101-gateway-diet

---

## PostMVP (после 2.0)

| Slug                 | Описание                                                      |
| -------------------- | ------------------------------------------------------------- |
| `advanced-detectors` | ML-based классификаторы, context-aware PII, multi-language    |
| `policy-engine`      | OPA/Rego policies, WASM filters, declarative policy model     |
| `envoy-data-plane`   | Envoy ext_proc gRPC + xDS control plane. Build tag `envoy`    |
| `k8s-operator`       | Kubernetes Operator: CRDs, reconciliation, Gateway API        |
| `shield-benchmark`   | Performance benchmark: throughput, latency, accuracy          |
| `mcp-integration`    | Model Context Protocol — маскирование на уровне MCP-тулов     |
| `agent-support`      | Маскирование в multi-agent цепочках (агент вызывает агента)   |
| `cache-prompt`       | Кэширование общих prefix-ов промптов для экономии токенов     |
| `model-selector`     | Автоматический выбор модели под задачу (cost/quality balance) |

---

## Порядок разработки

```
1.0.0 (основной план):
00 → 01 → 10 → 20 → 21 → 22 → 23 → 24 → 25
                                             ↓
30 → 40 → 41 → 50 → 51 → 61
                                    ↓
70 → 71 → 80 → 81 → 90 → 100 → 101 → 102
                                    ↓
110 → 111 → 112 → 113 → 116 → 114 → 115 → 117 → 118

2.0.0 (в разработке):
sessions → 130 → 131 → 132

Дополнительно (out-of-order, параллельно 1.0.0):
119-provider-egress-proxy ── после 71 (per-provider proxy)
120-tenant-profile-sync   ── после 80 (чистка ProfileRepository)
121-ci-cd-pipeline        ── параллельно 100+ (CI/CD)
122-docs-community        ── финальная документация
123-quality-config-refact ── рефакторинг в любой момент
124-helm-chart            ── после 101 (Helm deployment)
```

Все фазы 1.0.0 + 2.0.0 выполнены.  
Очередь — PostMVP.
