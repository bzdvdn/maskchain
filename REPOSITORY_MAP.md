# Repository Map

## Entry Points
- `src/cmd/gateway/main.go` — gateway binary entrypoint (binary: `bin/gateway`, data plane: proxy, shield, incident/tenant API for automation)
- `src/cmd/admin/main.go` — admin binary entrypoint (binary: `bin/admin`, control plane: UI, incident/tenant management)
- `src/cmd/all/main.go` — combined binary entrypoint (binary: `bin/all`, both profiles on separate ports)

## Top-Level Code
- `src/cmd/` — application entrypoints
- `src/internal/domain/` — domain entities, value objects, aggregates (DDD)
- `src/internal/app/` — application services, use cases, orchestration
- `src/internal/ports/` — inbound/outbound port interfaces (driven/driving)
- `src/internal/adapters/` — adapter implementations of ports (DB, HTTP, provider clients, egress)
- `src/internal/infra/` — infrastructure: config, logging, metrics, telemetry
- `src/internal/api/` — HTTP/gRPC handlers, middleware, request/response types
- `src/pkg/` — shared utilities (future)
- `ui/` — Vite + React + TypeScript frontend (profiles management, incidents viewer)
  - `ui/embed.go` — Go embed для встраивания статики в admin (не gateway)
  - `ui/src/pages/Profiles/` — ProfileList, ProfileDetail, ProfileForm
  - `ui/src/pages/Incidents/` — IncidentList, IncidentDetail
  - `ui/src/components/` — DictionaryEditor, PreprocessorEditor, ErrorBoundary
  - `ui/src/api/profiles.ts` — API client для `/api/v1/profiles/*`
  - `ui/src/api/incidents.ts` — API client для `/api/v1/incidents/*`
- `specs/active/` — active spec artifacts (speckeep-managed)
- `deployments/` — Docker, migrations, docker-compose configs

## Key Paths
- `src/cmd/gateway/main.go` — gateway binary entrypoint (DI wiring: proxy, shield, incident/tenant API for automation)
- `src/cmd/admin/main.go` — admin binary entrypoint (DI wiring: UI, incident/tenant handlers, metrics)
- `src/internal/domain/` — core business logic: Content Shield, tenants, policies, routing
  - `src/internal/domain/shield/mask/` — mask entry entity, storage interface, use case, UUIDv7
  - `src/internal/domain/shield/detector/` — detector interface, registry, composite detector
  - `src/internal/domain/routing/` — routing domain entities (Provider, Route, RoutingRule, HealthStatus)
  - `src/internal/domain/routing/service/` — routing services (ProviderRegistry, RouteSelector, FallbackHandler, HealthChecker)
- `src/internal/app/` — use case orchestration
  - `src/internal/app/usecase/shield/` — ShieldEngine, ScanUseCase, ScanPipelineFactory, ApplyPolicyUseCase
- `src/internal/ports/` — inbound (REST, gRPC) and outbound (repository, provider) interfaces
  - `src/internal/ports/provider.go` — ProviderClient port interface (outbound, LLM provider abstraction), ProviderChunk type, Stream() method
- `src/internal/adapters/` — repository impl (PostgreSQL), provider clients (OpenAI, Anthropic, etc.)
  - `src/internal/adapters/repository/mask/` — Postgres, Valkey, Cached mask repos
  - `src/internal/adapters/provider/` — provider adapter stubs (OpenAI, Anthropic, etc.)
  - `src/internal/adapters/egress/` — egress HTTP/HTTPS client with per-provider proxy dialer (HTTP/SOCKS5), SSE streaming, retry, connection pooling (implements ProviderClient with Call + Stream)
- `src/internal/api/` — HTTP handlers, middleware, request/response types
  - `src/internal/api/mask_handler.go` — POST /api/v1/shield/mask and /unmask handlers
  - `src/internal/api/provider_handler.go` — RoutingProxyHandler (proxy to LLM providers), legacy stubs
  - `src/internal/api/server.go` — gateway router setup, RegisterProxyRoute accepts RoutingProxyHandler
  - `src/internal/api/health/` — health check endpoints (liveness/readiness probes), service status aggregation
- `src/internal/api/admin.go` — admin router setup (AdminServer), static files, incident/tenant handlers
  - `src/internal/api/handler/incident/` — Incident read/export handlers (list, get, export CSV/JSON)
  - `src/internal/api/handler/admin/` — Tenant CRUD handlers
  - `src/internal/api/dto/` — request/response DTOs (IncidentResponse, TenantResponse, PaginatedResponse)
- `src/internal/infra/config/` — cobra/viper config loading, validation, defaults (RoutingConfig, ProviderConfig with ProxyURL, RouteConfig, RuleConfig), serialize/diff/watcher
- `src/internal/infra/telemetry/` — OTel SDK init, TracerProvider, MeterProvider, OTLP exporters
- `src/internal/infra/metrics/` — Prometheus metric definitions (HTTP, shield), /metrics handler
- `src/internal/infra/logging/` — slog adapter with OTel trace_id/span_id enrichment
- `src/internal/adapters/repository/postgres/migrations/` — SQL migrations (dictionary_entries, incidents, tenants, mask_entries)
- `deployments/` — Docker, Helm, docker-compose, migrations
  - `deployments/helm/maskchain/` — Helm chart for Kubernetes (Bitnami subcharts, ConfigMap, Ingress/GatewayAPI, ServiceMonitor)
  - `deployments/docker-compose/` — local dev / production compose stacks
  - `deployments/migrations/` — SQL migration files
- `Dockerfile` — combined binary Dockerfile (all-in-one image → `bzdvdn/maskchain`)
- `Dockerfile.gateway` — gateway-only Dockerfile (→ `bzdvdn/maskchain-gateway`)
- `Dockerfile.admin` — admin-only Dockerfile (→ `bzdvdn/maskchain-admin`, with node build stage)
- `Makefile` — build, test, lint, security-check, docker-build, helm-lint, ci targets
- `.golangci.yml` — linter configuration (14 linters: govet, staticcheck, errcheck, gosec, gosimple, bodyclose, noctx, thelper, prealloc, misspell, exportloopref, ineffassign, unused, gofmt)
- `.editorconfig` — editor formatting rules
- `.dockerignore` — hardened ignore list (30 entries)
- `ROADMAP.md` — phased development roadmap

## Where To Edit
- New domain logic — `src/internal/domain/`
- New use case / feature — `src/internal/app/` + `src/internal/ports/` + `src/internal/adapters/`
- New API endpoint — `src/internal/api/` + `src/internal/ports/` (inbound interface)
- Health check changes — `src/internal/api/health/`
- Egress/proxy changes — `src/internal/adapters/egress/` (proxy dialer, pool, retry, CB)
- Routing domain changes — `src/internal/domain/routing/` + `src/internal/domain/routing/service/`
- New provider adapter — `src/internal/adapters/provider/` + `src/internal/ports/provider.go`
- Configuration changes — `src/internal/infra/config/`
- Observability/telemetry changes — `src/internal/infra/telemetry/`, `src/internal/infra/metrics/`, `src/internal/infra/logging/`
- Frontend changes — `ui/` (when implemented)
- Spec/plan/tasks changes — `specs/active/<slug>/`
- Admin binary changes — `src/cmd/admin/` + `src/internal/api/admin.go`
- Build/CI changes — `Makefile`, `Dockerfile*`, `.golangci.yml`, `.github/workflows/ci.yml`, `.github/dependabot.yml`
- Deployment changes — `deployments/docker-compose/`, `deployments/helm/maskchain/`, `deployments/migrations/`

## Excluded
- `.speckeep/**` — excluded from indexing (agent workflow config)
- `specs/archived/**` — excluded from indexing (completed specs)
- `.github/**` — excluded from indexing (CI/CD workflow config)
- `.git/**` — excluded from indexing
- `bin/**` — excluded from indexing (build artifacts)
- `demo/**` — excluded from indexing
- `docs/**` — excluded from indexing
- `node_modules/**` — excluded from indexing
- `vendor/**` — excluded from indexing
- `dist/**`, `build/**`, `coverage/**` — excluded from indexing
- `.opencode/**` — excluded from indexing (IDE config)
