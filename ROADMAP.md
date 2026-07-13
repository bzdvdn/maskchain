# Roadmap MaskChain

Фазы разработки в порядке выполнения. Каждая фаза = папка `specs/active/<slug>/`, проходящая полный speckeep flow: `/spk.spec <slug>` → plan → tasks → implement → verify → archive.

**Легенда:** ✅ реализовано, 🔄 в работе, ⬜ запланировано

---

## Группа 0: Фундамент ✅

### 00-project-foundation ✅

**Цель:** Инициализация Go-модуля, структуры директорий (DDD + Clean Architecture), базовый билд-системы, линтеров, Dockerfile.

**Ключевые артефакты:** `go.mod`, `go.sum`, Makefile, `.golangci.yml`, `.editorconfig`, структура директорий, пустой `main.go`, Dockerfile (multistage)

**Зависимости:** нет

---

### 01-config-bootstrap ✅

**Цель:** cobra + viper конфигурация. Загрузка YAML/ENV/флагов, валидация, структура конфига.

**Ключевые артефакты:** `src/internal/infra/config/` — Config struct, LoadConfig(), defaults. Поддержка `config.yaml`, `CONFIG_*` env vars, CLI флагов (`--config`, `--log-level`). Валидация required-полей.

**Зависимости:** 00

---

## Группа 1: Gateway Runtime ✅

### 10-gateway-skeleton ✅

**Цель:** Gin HTTP server с graceful shutdown, health endpoints, middleware chain.

**Ключевые артефакты:** `src/internal/api/server.go` — Server struct (Gin, Start/Shutdown). Middleware: RequestID, Logger, Recovery, CORS. Endpoints: `GET /health`, `GET /ready`, `GET /live`. Graceful shutdown.

**Зависимости:** 01

---

## Группа 2: Shield Domain ✅

### 20-shield-domain ✅

**Цель:** Domain-слой Content Shield: сущности, value objects, domain services, ошибки.

**Ключевые артефакты:** `src/internal/domain/shield/entity/` — Profile, Detector, DetectorType, Reaction, Incident, ScanResult, Pattern. `src/internal/domain/shield/value/` — ProfileID, ProfileSlug, TenantID, Severity. Domain services и repository interfaces.

**Зависимости:** 00

---

### 21-shield-detectors ✅

**Цель:** Базовый набор детекторов: PII (email, phone, SSN, паспорт), secrets (API key, JWT, private key), financial (Luhn, IBAN, SWIFT).

**Ключевые артефакты:** `src/internal/domain/shield/detector/` — Detector interface + registry + PIIDetector, SecretsDetector, FinancialDetector + CompositeDetector.

**Зависимости:** 20

---

### 22-shield-mask-storage ✅

**Цель:** Обратимое template-based маскирование. Мask entries в PG + Valkey.

**Ключевые артефакты:** `mask_entries` таблица, Valkey-кэш, write-through/read-through. `POST /api/v1/shield/mask`, `POST /api/v1/shield/unmask`. MaskEntry entity, MaskStorage interface, MaskUseCase.

**Зависимости:** 21, 01

---

### 23-shield-reactions ✅

**Цель:** Механизм реакций: block, redact, mask, alert.

**Ключевые артефакты:** `src/internal/domain/shield/reaction/` — ReactionExecutor interface + BlockReaction (403), RedactReaction (`***`), MaskReaction (частичное скрытие), AlertReaction (логирование). Pipeline выбора реакции через PolicyEvaluator.

**Зависимости:** 20, 21

---

### 24-shield-dictionaries ✅

**Цель:** Словари — именованные списки значений, привязанные к профилю. Поддержка MatchMode: exact, contains (Aho-Corasick), regex, fuzzy.

**Ключевые артефакты:** Dictionary entity, DictionaryRepository interface, DictionaryDetector, WordlistMatcher. Exact — HashSet, Contains — Aho-Corasick, Regex — компиляция per entry. Хранение в `dictionary_entries` таблице.

**Зависимости:** 20, 23

---

### 25-shield-preprocessors ✅

**Цель:** Препроцессоры CSV/JSON для маскирования структурированных данных до детекторов.

**Ключевые артефакты:** Processor interface, CSVProcessor (обнаружение CSV-блоков, mask колонок), JSONProcessor (JSONPath, wildcard `[*]`, вложенные объекты). PreprocessorDef хранится в JSONB-поле профиля.

**Зависимости:** 20

---

## Группа 3: Persistence ✅

### 30-shield-persistence ✅

**Цель:** PostgreSQL repository для профилей, словарей, инцидентов. Миграции, CRUD, транзакции.

**Ключевые артефакты:** Миграции (001_profiles, 002_dictionary_entries, 003_incidents). Connection pool. ProfileRepo (с загрузкой словарей + препроцессоров), DictionaryRepo, IncidentRepo. PGXTransactionManager.

**Зависимости:** 20, 24, 25, 01

---

## Группа 4: Profiles API & UI ✅

### 40-profiles-api ✅

**Цель:** REST API CRUD профилей (словари и препроцессоры inline). Gin handlers, validation.

**Ключевые артефакты:** `src/internal/api/handler/profile/` — CreateProfile, GetProfile, ListProfiles, UpdateProfile, DeleteProfile, PatchDictionary. DTO с inline-словарями и препроцессорами. Error handling middleware.

**Зависимости:** 30, 10

---

### 41-profiles-ui ✅

**Цель:** React (Vite + TypeScript) для управления профилями.

**Ключевые артефакты:** `ui/` — Vite + React + TypeScript. ProfileList, ProfileDetail, ProfileForm. DictionaryEditor, PreprocessorEditor. Routing: `/profiles`, `/profiles/new`, `/profiles/:slug`.

**Зависимости:** 40

---

## Группа 5: Shield Runtime ✅

### 50-shield-engine ✅

**Цель:** Оркестратор сканирования: препроцессоры → детекторы (словари) → реакции → результат.

**Ключевые артефакты:** `src/internal/app/usecase/shield/` — ScanUseCase, ApplyPolicyUseCase. ScanPipeline: 1) препроцессоры 2) словари 3) детекторы 4) PolicyEvaluator 5) ReactionExecutor. ShieldEngine: `Scan(ctx, req) ScanResult`. Placeholder-based masking.

**Зависимости:** 20, 21, 22, 23, 24, 25, 30

---

### 51-shield-gateway-integration ✅

**Цель:** Интеграция Shield Engine в request lifecycle gateway.

**Ключевые артефакты:** ShieldMiddleware — перехват POST `/v1/chat/completions`. Profile resolution per-request. Pre-request scan. Headers: `X-Shield-Status`, `X-Shield-Incident-ID`.

**Зависимости:** 50, 10

---

## Группа 6: Audit & Observability ✅

### 60-audit-incidents ✅

**Цель:** Хранение и просмотр инцидентов Content Shield. API + UI.

**Ключевые артефакты:** `GET /api/v1/incidents` (list, filter), `GET /api/v1/incidents/:id`. Экспорт CSV/JSON. UI: Incidents page.

**Зависимости:** 30, 40, 41

---

### 61-observability ✅

**Цель:** OpenTelemetry, Prometheus metrics, structured logging, distributed tracing.

**Ключевые артефакты:** OTel SDK init (traces + metrics). Prometheus counters (requests, shield_blocks, shield_redacts, latency). slog adapter. Gin middleware: OTel trace propagation, request duration metric. docker-compose включает Prometheus + Grafana.

**Зависимости:** 10, 50

---

## Группа 7: Routing & Egress ✅

### 70-routing-engine ✅

**Цель:** Provider registry, model→provider mapping, fallback, health-aware routing.

**Ключевые артефакты:** `src/internal/domain/routing/` — Provider, Route, RoutingRule entities. `src/internal/domain/routing/service/` — ProviderRegistry, RouteSelector, FallbackHandler, HealthChecker. Config-based routing rules YAML. Прокси-хендлер `POST /v1/chat/completions`. Response header `X-Provider`.

**Зависимости:** 10

---

### 71-egress-streaming ✅

**Цель:** HTTP/HTTPS egress с proxy dialer, SSE streaming, retries, cancellation.

**Ключевые артефакты:** `src/internal/adapters/egress/` — HTTP client with proxy dialer (HTTP_PROXY/HTTPS_PROXY). SSE streaming (chunk forwarding). Retry с exponential backoff + full jitter. Request cancellation (context). Connection pooling. NO_PROXY support.

**Зависимости:** 70

---

## Группа 8: Multi-Tenancy ✅

### 80-tenant-isolation ✅

**Цель:** Multi-tenant: API key → tenant mapping, изоляция профилей, tenant-scoped routing.

**Ключевые артефакты:** Tenant entity, APIKey value object, auth middleware. Profile isolation. Tenant-scoped config overrides. `X-Tenant-ID` header propagation.

**Зависимости:** 30, 40, 50

---

## Группа 9: Production Hardening (реализовано частично, требуется доработка)

### 81-rate-limiting-budgets ⬜

**Цель:** Rate limiting и token budgets. Valkey-based sliding window counters.

**Ключевые артефакты:** TokenBudget, RateLimit entities. Sliding window rate limiter (Valkey Sorted Set). Token budget tracking per-tenant, per-model. Valkey repository для counters.

**Статус:** domain-сущности и конфиг готовы, middleware не подключена. Перенесено в фазу 113.

**Зависимости:** 10, 80

---

### 82-profile-versioning ⬜

**Цель:** Версионирование профилей. История изменений, diff, rollback (PostMVP).

**Статус:** отложено до PostMVP.

**Зависимости:** 40, 30

---

### 90-production-hardening ⬜

**Цель:** Performance tuning, load testing, security audit, production runbook.

**Статус:** частично (pprof endpoints, pool tuning). Требует расширения. Задачи декомпозированы в фазы 113, 114, 115, 116.

**Зависимости:** все предыдущие

---

## Group 10: Architecture Split ✅

### 100-admin-control-plane ✅

**Цель:** Выделение admin control plane в отдельный binary со своим Dockerfile. Gateway — лёгкий data plane.

**Ключевые артефакты:** `src/cmd/admin/main.go` — entrypoint admin. `src/internal/api/admin.go` — admin server. Gateway Dockerfile (`Dockerfile.gateway`) без node-стадии. Admin Dockerfile (`Dockerfile.admin`) с node build. Раздельные сигналы, graceful shutdown.

**Зависимости:** 90, 40, 41

---

### 101-gateway-diet ✅

**Цель:** Минимальный gateway binary (<100ms старт, ~15MB image).

**Ключевые артефакты:** Build tag `gateway`/`admin`. UI embed только в admin. CGO_ENABLED=0. Makefile target: `build-gateway`, `build-admin`, `docker-build-gateway`, `docker-build-admin`.

**Зависимости:** 100

---

### 102-profile-cache ✅

**Цель:** Read-through кэш профилей на Valkey + in-memory LRU для gateway.

**Ключевые артефакты:** ProfileCache (Valkey + LRU). Кэш-ключи `profile:<slug>:v<version>`. Write-through/read-through. PubSub-инвалидация. Graceful degradation. Метрики.

**Зависимости:** 30, 80, 61

---

## Группа 11: Provider Adapters (НОВОЕ — необходимо для 1.0.0)

### 110-provider-adapters ⬜

**Цель:** Реальные HTTP-клиенты для LLM-провайдеров: OpenAI, Anthropic (Claude), DeepSeek, Mistral. Каждый адаптер реализует `ports.ProviderClient`, преобразует запросы/ответы в формат провайдера и обратно в OpenAI-совместимый.

**Ключевые артефакты:**
- `src/internal/adapters/provider/openai.go` — OpenAI-compatible (DeepSeek, Mistral, Groq): Bearer auth, `/v1/chat/completions`, стандартный SSE
- `src/internal/adapters/provider/anthropic.go` — Anthropic Messages API: `x-api-key` header, `/v1/messages`, свой формат стриминга
- Фабрика `NewProviderClient(cfg)` в main.go, создающая адаптер по `api_type`
- Каждый адаптер: Call + Stream, преобразование ошибок провайдера в стандартный формат

**Критично для:** фактической поддержки Anthropic, DeepSeek, Mistral. Без этой фазы gateway не может общаться ни с одним реальным провайдером (только stub).

**Зависимости:** 70, 71, 80

---

### 111-provider-auth-and-config ⬜

**Цель:** API key management, secure storage, per-provider auth в конфиге. Добавить в `ProviderConfig` поля `api_key`, `auth_type`, `auth_header`. Поддержка чтения ключей из env/Vault (не только из YAML).

**Ключевые артефакты:**
- `ProviderConfig.APIKey` — строковый ключ (чтение из `CONFIG_ROUTING_PROVIDERS_0_API_KEY` env)
- `ProviderConfig.AuthHeader` — кастомный заголовок (по умолчанию `Authorization`)
- `ProviderConfig.AuthType` — bearer | api-key | basic
- Секция `routing.providers[].api_key` в YAML (опционально, можно env)
- Валидация: если провайдер сконфигурирован, APIKey — required
- Метки sensitive в логах: ключи не выводятся в debug-конфиге

**Зависимости:** 110, 80, 01

---

### 112-proxy-streaming-wiring ⬜

**Цель:** Провести SSE streaming через весь pipeline от провайдера до клиента. `HandleChatCompletion` сейчас вызывает только `Call()`, игнорируя `Stream()`. Нужно: определение streaming-запроса (по `stream: true` в body), проксирование SSE-чанков до клиента, корректная обработка cancellation.

**Ключевые артефакты:**
- Детекция streaming-запроса в `chatRequest` (поле `Stream bool`)
- `Middleware.WrapSSE()` — установка `Content-Type: text/event-stream`, `Transfer-Encoding: chunked`, flush-per-chunk
- RoutingProxyHandler.HandleChatCompletion: при `stream: true` вызывает `Stream()`, форвардит чанки через `gin.Context.Stream()`
- Обработка cancellation от клиента → cancellation upstream provider
- Обработка ошибок в середине стрима: запись ошибки в стрим + закрытие
- FallbackHandler.Stream() для стримингового fallback между провайдерами

**Критично для:** UX — большинство LLM-запросов идут в streaming-режиме. Без этой фазы клиенты не получают токены по SSE.

**Зависимости:** 110, 71, 70

---

## Группа 12: Production Readiness (критично для 1.0.0)

### 113-shield-middleware-wiring ⬜

**Цель:** Подключить реальный ProfileRepository в ShieldMiddleware (сейчас — `nil`). Content Shield фактически не работает в production.

**Ключевые артефакты:**
- Wire ProfileRepository (из PG + кэш) в `ShieldMiddleware` вместо `nil`
- В `main.go`: загрузка профилей из tenant routing config → ShieldEngine инициализация
- Profile resolution per-request: tenant `X-Tenant-ID` → профиль по `tenant_model_mapping`
- Если профиль не найден: fallback (block | allow) — настраивается в `ShieldConfig.default_action`
- Интеграционный тест: отправка PII-содержащего промпта → shield блокирует → 403
- Graceful degradation: если PG недоступен для загрузки профиля, действие определяется `default_action`

**Критично для:** соответствия конституции (Content Shield — обязательная фича, не opt-in).

**Зависимости:** 51, 102, 80

---

### 114-real-health-probes ⬜

**Цель:** Dependency-aware health/readiness probes. `/health`, `/ready`, `/live` сейчас возвращают статический `{"status":"ok"}`.

**Ключевые артефакты:**
- `GET /health` — liveness: всегда 200 (процесс жив)
- `GET /ready` — readiness: проверка PG (SELECT 1), Valkey (PING), egress (TCP к провайдерам)
- `GET /live` — startup probe: сервис принял конфиг, инициализировал зависимости
- Формат ответа: `{"status":"ok|degraded|down","checks":{"database":{"status":"ok","latency_ms":2}}}`
- `degraded` — не-критичная зависимость недоступна (Valkey для rate limiter)
- `down` — критическая недоступна (PG для shield)
- Config: `server.health_check.critical_deps: ["database"]`
- Тесты с mock probes

**Зависимости:** 10, 61

---

### 115-rate-limit-wiring ⬜

**Цель:** Подключить rate limiting в gateway request lifecycle. Middleware и Valkey-репозиторий существуют, но не завязаны в main.

**Ключевые артефакты:**
- Инициализация Valkey repos → RateLimit middleware → `srv.RegisterRateLimit()` в main.go
- Per-tenant rate limits из `config.ratelimit.tenant_overrides`
- Per-model token budgets из `config.ratelimit.default_token_budget`
- 429 Too Many Requests с Retry-After header
- Метрики: rate_limit_exceeded_total, budget_exceeded_total
- Graceful degradation: при недоступности Valkey — fail-open с warning

**Зависимости:** 81, 61

---

### 116-connection-pool-fixes ⬜

**Цель:** Исправить найденные баги в egress-клиенте и добавить per-provider timeout.

**Ключевые артефакты:**
- **Fix bug** `pool.go:23`: `MaxIdleConnsPerHost: cfg.MaxIdleConns` → `cfg.MaxIdleConnsPerHost`
- **Per-provider timeout**: пробросить `ProviderConfig.Timeout` в контекст egress-клиента
- **TLS config**: поддержка кастомных CA-сертификатов, отключение проверки (для инетрнала), mTLS через конфиг `egress.tls`
- **Circuit breaker**: простая имплементация (после N последовательных ошибок — skip provider на T секунд)
- **Per-provider connection pool**: выделенный `http.Transport` на провайдера (изоляция от "шумных соседей")

**Зависимости:** 71, 70

---

### 117-critical-test-coverage ⬜

**Цель:** Закрыть пробелы в тестировании на critical path.

**Ключевые артефакты:**
- `routing_proxy_handler_test.go` — fallback, health-aware выбор, tenant-scoped routing
- `shield_middleware_test.go` — блокировка PII, allow чистого промпта, graceful degradation
- `mask_handler_test.go` — mask/unmask pipeline
- `server_test.go` — health endpoints, graceful shutdown
- Integration тест: полный цикл запрос → auth → shield → routing → egress → response

**Зависимости:** 110, 112, 113, 114

---

### 118-api-consistency ⬜

**Цель:** Единый API-стандарт `/api/v1/*`, OpenAPI/Swagger, единый envelope.

**Ключевые артефакты:**
- `POST /api/v1/chat/completions` (основной), `/v1/chat/completions` — deprecated с redirect 301
- Единый envelope: `{"data": ..., "error": {"code": "...", "message": "..."}}`
- Пагинация: `{"data": [...], "pagination": {"page": 1, "per_page": 20, "total": 100}}`
- OpenAPI 3.1 spec: `docs/openapi.yaml`
- Swagger UI: `GET /api/v1/docs` (admin binary)
- SPA NoRoute handler с проверкой `Accept: text/html`

**Зависимости:** 100, 40, 60, 22

---

## Порядок разработки (1.0.0)

```
Текущее состояние (реализовано):
00 → 01 → 10 → 20 → 21 → 22 → 23 → 24 → 25
                                             ↓
30 → 40 → 41 → 50 → 51 → 60 → 61
                                    ↓
70 → 71 → 80 → 100 → 101 → 102
                                    ↓
                          81(partial)

Группа 11: Provider Adapters (критично)
110 → 111 → 112
  ↓
112 (streaming wiring)

Группа 12: Production Readiness (критично)
113 → 114 → 115 → 116 → 117 → 118
  ↓     ↓       ↓
113   114     116
(shield) (health) (pool fixes)

Рекомендуемый порядок для 1.0.0:
110 → 111 → 112 → 113 → 116 → 114 → 115 → 117 → 118
```

Каждая фаза — полный speckeep цикл:

1. `/spk.spec <slug>` — создание ветки `feature/<slug>` + spec-файл
2. `/spk.plan` — план реализации
3. `/spk.tasks` — декомпозиция на задачи
4. `/spk.implement` — реализация
5. `/spk.verify` — верификация (тесты + AC)
6. `speckeep archive <slug> .` — архив (после verify: pass)

---

## PostMVP (после 1.0.0)

| Slug | Описание |
|---|---|
| `82-profile-versioning` | Версионирование профилей: история, diff, rollback, draft→active workflow |
| `envoy-data-plane` | Envoy ext_proc gRPC + xDS control plane. Build tag `envoy` |
| `k8s-operator` | Kubernetes Operator: CRDs, reconciliation, Gateway API |
| `advanced-detectors` | ML-based classifiers, context-aware PII, multi-language |
| `policy-engine` | OPA/Rego policies, WASM filters, declarative policy model |
| `shield-benchmark` | Performance benchmark suite для детекторов: throughput, latency, accuracy |
| `audit-dashboard` | Admin dashboard с графиками инцидентов по tenant/severity/timeline |
