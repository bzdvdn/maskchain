# Roadmap MaskChain

Фазы разработки в порядке выполнения. Каждая фаза = один spec-файл (`specs/active/<slug>.md`), проходящий полный speckeep flow: `/spk.spec <slug>` → plan → tasks → implement → verify → archive.

---

## Группа 0: Фундамент

### 00-project-foundation

**Цель:** Инициализация Go-модуля, структуры директорий (DDD + Clean Architecture), базовый билд-системы, линтеров, Dockerfile. Никакой бизнес-логики.

**Ключевые артефакты:**

- `go.mod`, `go.sum` (module: `github.com/bzdvdn/maskchain`)
- Makefile (lint, test, build, run)
- `.golangci.yml`, `.editorconfig`
- Директории: `src/cmd/gateway/`, `src/internal/{domain,app,ports,adapters,infra,api}/`, `ui/`, `deployments/docker-compose/`
- Пустой `main.go` (просто `os.Exit(0)`)
- Dockerfile (multistage: golang build + distroless runtime)

**Зависимости:** нет

---

### 01-config-bootstrap

**Цель:** cobra + viper конфигурация. Загрузка YAML/ENV/флагов, валидация, структура конфига.

**Ключевые артефакты:**

- `src/internal/infra/config/` — Config struct, LoadConfig(), defaults
- Поддержка `config.yaml`, `CONFIG_*` env vars, CLI флагов (`--config`, `--log-level`)
- Валидация required-полей
- `main.go` загружает конфиг и логирует его

**Зависимости:** 00

---

## Группа 1: Gateway Runtime

### 10-gateway-skeleton

**Цель:** Gin HTTP server с graceful shutdown, health endpoints, middleware chain (request ID, logging, recovery, panic handling).

**Ключевые артефакты:**

- `src/internal/api/server.go` — Server struct (Gin engine, Start/Shutdown)
- Middleware: RequestID, Logger, Recovery, CORS
- Endpoints: `GET /health`, `GET /ready`, `GET /live`
- Graceful shutdown (SIGINT/SIGTERM)
- `main.go`: инициализация сервера + запуск

**Зависимости:** 01

---

## Группа 2: Shield Domain

### 20-shield-domain

**Цель:** Domain-слой Content Shield: сущности, value objects, domain services, ошибки. Чистый Go без external зависимостей.

**Ключевые артефакты:**

- `src/internal/domain/shield/entity/` — Profile, Detector, DetectorType, Reaction, Incident, ScanResult, Pattern
- `src/internal/domain/shield/value/` — ProfileID, ProfileSlug (уникальный slug профиля), TenantID, PatternID, Severity, ScanStatus
- `src/internal/domain/shield/service/` — ScanPipeline (оркестрация детекторов), PolicyEvaluator (reaction selection)
- `src/internal/domain/shield/errors/` — ErrProfileNotFound, ErrInvalidPattern, ErrDetectorFailed, ErrDuplicateSlug
- `src/internal/domain/shield/repository.go` — ProfileRepository, IncidentRepository interfaces (port)

**Зависимости:** 00

---

### 21-shield-detectors

**Цель:** Базовый набор детекторов Content Shield. PII (email, phone, SSN, паспорт), secrets (API key, JWT, private key patterns), financial (Luhn, IBAN, SWIFT).

**Ключевые артефакты:**

- `src/internal/domain/shield/detector/` — Detector interface + registry
  - `piidetector.go` — email, phone, SSN, passport regexes
  - `secretsdetector.go` — API key, JWT, PEM private key patterns
  - `financialdetector.go` — Luhn check, IBAN regex, SWIFT regex
  - `phidetector.go` — медицинские коды (ICD-10, SSN-like)
- Unit-тесты для каждого детектора (включая граничные случаи)
- `DetectorResult` — найденные совпадения с позициями и confidence

**Зависимости:** 20

---

### 22-shield-mask-storage

**Цель:** Хранение цепочек маскинга для обратимого template-based replacement. `/mask` сохраняет найденные детекторами фрагменты в PG + Valkey, `/unmask` достаёт и восстанавливает оригинальный текст.

**Ключевые артефакты:**

- `mask_entries` таблица в PG: `mask_id UUID PK, profile_id, fragments JSONB [{fragment, position, detector_type}], original_text TEXT, request_id, created_at`
- Valkey-кэш: `mask:<mask_id> → JSON цепочки`, TTL конфигурируемый
- Write-through: при `/mask` пишем в PG → вскидываем в Valkey
- Read-through: при `/unmask` читаем Valkey first, PG fallback → обновляем кэш
- `/mask` endpoint: `POST /api/v1/shield/mask?mask_id=<uuid>` body: prompt → сканирование детекторами → template-замена → сохранение цепочки → ответ с `X-Mask-ID`
- `/unmask` endpoint: `POST /api/v1/shield/unmask?mask_ids=id1,id2` body: masked_text → резолв цепочек → замена template-ссылок → восстановленный текст
- mask_id — UUIDv7, text, уникален глобально. User может передать свой mask_id (если занят — 409). Привязка к профилю опционально (тогда уникальность `(profile_id, mask_id)`).
- Пакет `src/internal/domain/shield/mask/` — MaskEntry entity, MaskStorage interface, MaskUseCase
- Пакет `src/internal/adapters/repository/mask/` — PostgresMaskRepo + ValkeyMaskRepo

**Зависимости:** 21, 01

---

### 23-shield-reactions

**Цель:** Механизм реакций при обнаружении sensitive data: block, redact, mask, alert.

**Ключевые артефакты:**

- `src/internal/domain/shield/reaction/` — ReactionExecutor interface + реализации
  - `BlockReaction` — возвращает 403 с описанием причины
  - `RedactReaction` — заменяет PII на `***`
  - `MaskReaction` — показывает первые/последние символы (email: `j***@***.com`)
  - `AlertReaction` — логирует инцидент без блокировки
- Pipeline выбора реакции на основе PolicyEvaluator
- Тесты: каждый тип реакции изолированно

**Зависимости:** 20, 21

---

### 24-shield-dictionaries

**Цель:** Словари — именованные списки значений (ключ + список строк), привязанные к профилю через БД. Словарь существует ТОЛЬКО в контексте профиля. Покрывает кейсы: запрещённые имена, внутренние коды, конкурентные продукты, IP-адреса, домены.

**Ключевые артефакты:**

- `src/internal/domain/shield/dictionary/` — Dictionary entity
  - `Dictionary` — ValueObject: ProfileSlug (уникальный), Name, Entries []string, MatchMode (exact|contains|regex|fuzzy)
  - `MatchMode` — exact (точное совпадение через set/map), contains (подстрока), regex (каждый entry — регулярка), fuzzy (нечёткое с порогом)
  - `DictionaryRepository` interface — CRUD по ProfileSlug
- `DictionaryDetector` — реализует `Detector` interface, матчит текст против entries профиля
  - Exact match — HashSet (O(1) lookup)
  - Contains — Aho-Corasick automaton
  - Regex match — каждый entry компилируется в regex
- `WordlistMatcher` — утилита для быстрого мульти-паттерн поиска
- Словарь не имеет standalone API/UI — управляется через профиль (inline entries в форме профиля)
- Хранится в БД: таблица `dictionary_entries` (profile_slug → entries[])
- Профиль загружается целиком со словарями через ProfileRepository

**Зависимости:** 20, 23

---

### 25-shield-preprocessors

**Цель:** Препроцессоры CSV/JSON для структурированных данных внутри AI-запросов. Позволяют маскировать колонки CSV и JSON-поля по имени/пути до того, как данные попадут в детекторы. Препроцессор — inline-часть профиля, хранится в БД. Адаптация из RELAY.

**Ключевые артефакты:**

- `src/internal/domain/shield/preprocessor/` — Processor interface + factory
  - `Processor` interface: `Name() string`, `Process(data string, namespace string) *ProcessResult`
  - `ProcessResult` — `ModifiedText string`, `Replacements map[string]string`
  - `NewPreprocessor(preprocDef) (Processor, error)` — фабрика по типу
- `CSVProcessor` — обнаруживает CSV-блоки в тексте (строки с запятыми, заголовок + строки), маскирует указанные колонки
  - Rules: `[{columns: ["email","phone"], mask: "full"}]`
  - Mask modes: `full` (placeholder `{{csv.ns.N}}`), `surname` (только первое слово)
  - Поддержка кавычек и экранирования в CSV
- `JSONProcessor` — обнаруживает JSON-блоки (включая внутри markdown fences \`\`\`json), маскирует поля по JSONPath
  - Rules: `[{path: "user.email", mask: "full"}, {path: "items[*].secret", mask: "full"}]`
  - Поддержка вложенных объектов, массивов, wildcard `[*]`
  - JSONPath парсер: segments, index, wildcard
- `PreprocessorDef` — структура хранится в БД в JSONB-поле профиля (не YAML):
  ```json
  {
    "name": "mask-user-csv",
    "type": "csv",
    "rules": [{ "columns": ["email", "phone"], "mask": "full" }]
  }
  ```
- Профиль может содержать список препроцессоров в поле `preprocessors: []PreprocessorDef` (JSONB)
- Интеграция в Shield Engine pipeline: препроцессоры запускаются ДО детекторов

**Зависимости:** 20

---

## Группа 3: Persistence

### 30-shield-persistence

**Цель:** PostgreSQL repository для профилей, словарей и инцидентов. Миграции, CRUD, транзакции.

**Ключевые артефакты:**

- `src/internal/adapters/repository/postgres/migrations/` — goose/golang-migrate миграции
  - `001_profiles.sql` — profiles table (id, slug VARCHAR UNIQUE, name, tenant_id, preprocessors JSONB, status, version, created_at, updated_at)
  - `002_dictionary_entries.sql` — dictionary_entries table (id, profile_slug VARCHAR REFERENCES profiles(slug), entry_value TEXT, match_mode VARCHAR, created_at)
  - `003_incidents.sql` — incidents table (id, profile_slug, request_id, detector_type, entry_value, severity, action, raw_snippet, timestamp)
- Profile `slug` — уникальный идентификатор профиля (генерируется при создании), используется как внешний ключ для словарей и инцидентов
- Препроцессоры хранятся inline в `profiles.preprocessors` (JSONB), не отдельной таблицей
- Словари хранятся в `dictionary_entries` с привязкой по `profile_slug`
- `src/internal/adapters/repository/postgres/` — ProfileRepo (с полной загрузкой словарей + препроцессоров), DictionaryRepo, IncidentRepo
- Connection pool config, транзакционный репозиторий
- Unit-тесты (mock) + integration-тесты (testcontainers)

**Зависимости:** 20, 24, 25, 01

---

## Группа 4: Profiles API & UI

### 40-profiles-api

**Цель:** REST API для CRUD профилей (словари и препроцессоры — inline внутри профиля). Gin handlers, request validation, error handling.

**Ключевые артефакты:**

- `src/internal/api/handler/profile/` — CreateProfile, GetProfile, ListProfiles, UpdateProfile, DeleteProfile
- `src/internal/api/dto/profile.go` — request/response DTO со встроенными словарями и препроцессорами
- Валидация (go-playground/validator), проверка уникальности slug
- Error handling middleware (единый формат ошибок)
- `GET /api/v1/profiles` — list (slug, name, status)
- `POST /api/v1/profiles` — create (body включает dictionaries + preprocessors inline)
- `GET /api/v1/profiles/:slug` — get by slug (полная структура со словарями и препроцессорами)
- `PUT /api/v1/profiles/:slug` — update (перезапись dictionaries + preprocessors)
- `DELETE /api/v1/profiles/:slug` — delete (каскадное удаление словарей)
- `PATCH /api/v1/profiles/:slug/dictionary` — добавить/удалить entries словаря
- Нет отдельных endpoint'ов для dictionaries/preprocessors — управление только через профиль

**Зависимости:** 30, 10

---

### 41-profiles-ui

**Цель:** React (TypeScript + Vite) приложение для управления профилями со встроенными словарями и препроцессорами.

**Ключевые артефакты:**

- `ui/` — Vite + React + TypeScript
- `ui/src/pages/Profiles/` — ProfileList, ProfileDetail, ProfileForm
- `ui/src/components/` — DetectorConfigurator, ReactionSelector, PatternEditor, DictionaryEditor (inline entries manager), PreprocessorEditor (CSV/JSON rule builder)
- Routing (react-router): `/profiles`, `/profiles/new`, `/profiles/:slug`
- Словари и препроцессоры редактируются внутри формы профиля (inline), а не на отдельных страницах

**Зависимости:** 40

---

## Группа 5: Shield Runtime

### 50-shield-engine

**Цель:** Оркестратор сканирования. Принимает запрос (текст + профиль), прогоняет через препроцессоры → детекторы (включая словари) → реакции, возвращает результат.

**Ключевые артефакты:**

- `src/internal/app/usecase/shield/` — ScanUseCase, ApplyPolicyUseCase
- `ScanPipeline` — фабрика цепочки из конфига профиля:
  1. Препроцессоры (CSV/JSON — маскируют структурированные данные)
  2. Словари (wordlist matching — точные/частичные совпадения)
  3. Детекторы (PII, secrets, financial, PHI — regex)
  4. PolicyEvaluator (выбор реакции по приоритету)
  5. ReactionExecutor (block/redact/mask/alert)
- `ShieldEngine` — public API: `Scan(ctx, req ScanRequest) ScanResult`
- Placeholder-based masking: `{{csv.ns.N}}`, `{{json.ns.N}}`, `{{p.ns.N}}`, `{{dict.ns.N}}`
- Интеграционные тесты: полный pipeline (препроцессор → словарь → детектор → реакция → результат)

**Зависимости:** 20, 21, 22, 23, 24, 25, 30

---

### 51-shield-gateway-integration

**Цель:** Интеграция Shield Engine в gateway request lifecycle. Перехват входящих запросов, сканирование промпта, блокировка/редэкшн до отправки провайдеру.

**Ключевые артефакты:**

- Middleware `ShieldMiddleware` — перехват POST `/v1/chat/completions` etc.
- Profile resolution per-request (tenant + model lookup)
- Pre-request scan: чтение body, прогон через ShieldEngine, реакция
- Post-response scan (опционально): сканирование ответа от провайдера
- Headers: `X-Shield-Status`, `X-Shield-Incident-ID`
- Интеграционные тесты: полный цикл запрос → shield → block/allow

**Зависимости:** 50, 10

---

## Группа 6: Audit & Observability

### 60-audit-incidents

**Цель:** Хранение и просмотр инцидентов Content Shield. API + базовая UI-страница.

**Ключевые артефакты:**

- API: `GET /api/v1/incidents` (list, filter by severity/tenant/profile_slug), `GET /api/v1/incidents/:id`
- Инцидент: request_id, timestamp, tenant, profile_slug, detector_type, entry_value, severity, action, prompt snippet (redacted), response snippet
- UI: Incidents page — таблица с фильтрами, детальный просмотр
- Экспорт в CSV/JSON

**Зависимости:** 30, 40, 41

---

### 61-observability

**Цель:** OpenTelemetry, Prometheus metrics, structured logging (slog), distributed tracing.

**Ключевые артефакты:**

- `src/internal/infra/telemetry/` — OTel SDK init (traces + metrics)
- `src/internal/infra/metrics/` — Prometheus counters (requests, shield_blocks, shield_redacts, errors, latency)
- `src/internal/infra/logging/` — slog adapter, structured fields
- Gin middleware: OTel trace propagation, request duration metric
- Shield-специфичные метрики: scan_duration_ms, incidents_by_severity, profiles_evaluated
- `docker-compose` включает Prometheus + Grafana

**Зависимости:** 10, 50

---

## Группа 7: Routing & Egress

### 70-routing-engine

**Цель:** Провайдеры, модели, роутинг. Provider registry, model→provider mapping, fallback.

**Ключевые артефакты:**

- `src/internal/domain/routing/` — Provider, Model, Route, RoutingRule entities
- `src/internal/domain/routing/service/` — RouteSelector (select provider by model), FallbackHandler
- Config-based routing rules (YAML: `model: gpt-4 → provider: openai, fallback: azure-openai`)
- Health-aware routing: skip unhealthy providers
- API: `POST /v1/chat/completions` с routing resolution

**Зависимости:** 10

---

### 71-egress-streaming

**Цель:** HTTP/HTTPS egress с поддержкой outbound proxy, SSE streaming, retries, cancellation.

**Ключевые артефакты:**

- `src/internal/adapters/egress/` — HTTP client with proxy dialer (HTTP_PROXY/HTTPS_PROXY support)
- SSE streaming: chunk forwarding from provider → client
- Retry with exponential backoff + jitter
- Request cancellation propagation (context)
- Timeout per-provider (configurable)
- Connection pooling optimizations

**Зависимости:** 70

---

## Группа 8: Multi-Tenancy & Advanced

### 80-tenant-isolation

**Цель:** Multi-tenant поддержка. API key → tenant mapping, изоляция профилей, tenant-scoped routing.

**Ключевые артефакты:**

- `src/internal/domain/tenant/` — Tenant entity, APIKey value object
- API key authentication middleware
- Profile isolation: каждый tenant видит только свои профили
- Tenant-scoped config overrides
- `X-Tenant-ID` header propagation

**Зависимости:** 30, 40, 50

---

### 81-rate-limiting-budgets

**Цель:** Rate limiting и token budgets. Valkey-based counters.

**Ключевые артефакты:**

- `src/internal/domain/budget/` — TokenBudget, RateLimit entities
- Sliding window rate limiter (Valkey Sorted Set)
- Token budget tracking per-tenant, per-model
- Budget enforcement middleware
- Valkey repository для counters

**Зависимости:** 10, 80

---

### 82-profile-versioning

**Цель:** Версионирование профилей. История изменений, diff, rollback, draft → active workflow.

**Ключевые артефакты:**

- Profile версии: draft, active, archived
- `profile_versions` table + repository
- Diff API: `GET /api/v1/profiles/:id/versions/:v1/diff/:v2`
- Rollback: `POST /api/v1/profiles/:id/rollback/:version`
- UI: version history timeline, diff viewer, promote/demote

**Зависимости:** 40, 30

---

## Группа 9: Hardening & Production

### 90-production-hardening

**Цель:** Performance tuning, load testing, security audit, production runbook.

**Ключевые артефакты:**

- pprof endpoints, benchmark suite
- Connection pool tuning (PG, HTTP)
- Load testing script (k6/locust)
- Security checklist (TLS, secrets rotation, audit)
- Production docker-compose profile
- Runbook: startup, health check, debug, recovery

**Зависимости:** все предыдущие

---

## PostMVP (будущие фазы)

| Slug                 | Описание                                                                   |
| -------------------- | -------------------------------------------------------------------------- |
| `envoy-data-plane`   | Envoy ext_proc gRPC + xDS control plane. Build tag `envoy`.                |
| `k8s-operator`       | Kubernetes Operator: CRDs, reconciliation, Gateway API.                    |
| `advanced-detectors` | ML-based classifiers, context-aware PII, multi-language.                   |
| `policy-engine`      | OPA/Rego policies, WASM filters, declarative policy model.                 |
| `shield-benchmark`   | Performance benchmark suite для детекторов: throughput, latency, accuracy. |

---

## Порядок разработки

```
00 → 01 → 10 → 20 → 21 → 22 → 23 → 24 → 25
                                             ↓
30 → 40 → 41 → 50 → 51 → 60 → 61
                                    ↓
70 → 71 → 80 → 81 → 82 → 90
```

Каждая фаза — полный speckeep цикл:

1. `/spk.spec <slug>` — создание ветки `feature/<slug>` + spec-файл
2. `/spk.plan` — план реализации
3. `/spk.tasks` — декомпозиция на задачи
4. `/spk.implement` — реализация
5. `/spk.verify` — верификация (тесты + AC)
6. `speckeep archive <slug> .` — архив (после verify: pass)
