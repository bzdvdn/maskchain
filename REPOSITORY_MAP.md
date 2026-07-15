# Repository Map

## Entry Points
- `src/cmd/gateway/main.go` — gateway binary entrypoint (binary: `bin/gateway`, data plane: proxy, shield, incident/tenant API for automation)
- `src/cmd/admin/main.go` — admin binary entrypoint (binary: `bin/admin`, control plane: UI, incident/tenant management)

## Top-Level Code
- `src/cmd/` — application entrypoints
- `src/internal/domain/` — domain entities, value objects, aggregates (DDD)
- `src/internal/app/` — application services, use cases, orchestration
- `src/internal/ports/` — inbound/outbound port interfaces (driven/driving)
- `src/internal/adapters/` — adapter implementations of ports (DB, HTTP, etc.)
- `src/internal/infra/` — infrastructure: config, logging, metrics, egress dialers
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
  - `src/internal/adapters/egress/` — egress HTTP/HTTPS client with proxy dialer, SSE streaming, retry, connection pooling (implements ProviderClient with Call + Stream)
- `src/internal/api/` — HTTP handlers, middleware, request/response types
  - `src/internal/api/mask_handler.go` — POST /api/v1/shield/mask and /unmask handlers
  - `src/internal/api/provider_handler.go` — RoutingProxyHandler (proxy to LLM providers), legacy stubs
  - `src/internal/api/server.go` — gateway router setup, RegisterProxyRoute accepts RoutingProxyHandler
  - `src/internal/api/health/` — health check endpoints (liveness/readiness probes), service status aggregation
- `src/internal/api/admin.go` — admin router setup (AdminServer), static files, incident/tenant handlers
  - `src/internal/api/handler/incident/` — Incident read/export handlers (list, get, export CSV/JSON)
  - `src/internal/api/handler/admin/` — Tenant CRUD handlers
  - `src/internal/api/dto/` — request/response DTOs (IncidentResponse, TenantResponse, PaginatedResponse)
- `src/internal/infra/config/` — cobra/viper config loading, validation, defaults (RoutingConfig, ProviderConfig, RouteConfig, RuleConfig)
- `src/internal/infra/telemetry/` — OTel SDK init, TracerProvider, MeterProvider, OTLP exporters
- `src/internal/infra/metrics/` — Prometheus metric definitions (HTTP, shield), /metrics handler
- `src/internal/infra/logging/` — slog adapter with OTel trace_id/span_id enrichment
- `src/internal/adapters/repository/postgres/migrations/` — SQL migrations (profiles, dictionary_entries, incidents, tenants, mask_entries)
- `specs/active/22-shield-mask-storage/` — mask storage phase: spec, plan, tasks
- `specs/active/41-profiles-ui/` — profiles UI phase: spec, plan, tasks, inspect
- `specs/active/50-shield-engine/` — shield engine orchestration: spec, plan, tasks, inspect
- `specs/active/60-audit-incidents/` — audit incidents viewer: spec, plan, tasks, inspect, data-model
- `specs/active/61-observability/` — observability phase: spec, plan, tasks, inspect, data-model
- `specs/active/100-admin-control-plane/` — admin control plane phase: spec, plan, tasks, inspect, data-model
- `specs/active/cleanup-profile-repository/` — cleanup deprecated ProfileRepository: spec, plan, tasks, data-model
- `deployments/docker-compose/` — local dev environment (PostgreSQL, Valkey)
- `Dockerfile` — symlink → `Dockerfile.admin` (обратная совместимость)
- `Dockerfile.gateway` — gateway Dockerfile (Go build → distroless, без node)
- `Dockerfile.admin` — admin Dockerfile (node build → Go build → distroless)
- `Makefile` — build, test, lint, docker-build, clean, check-structure targets
- `.golangci.yml` — linter configuration (gofmt, govet, staticcheck, errcheck, unused)
- `.editorconfig` — editor formatting rules
- `ROADMAP.md` — phased development roadmap (speckeep-verified specs)

## Where To Edit
- New domain logic — `src/internal/domain/`
- New use case / feature — `src/internal/app/` + `src/internal/ports/` + `src/internal/adapters/`
- New API endpoint — `src/internal/api/` + `src/internal/ports/` (inbound interface)
- Health check changes — `src/internal/api/health/`
- Routing domain changes — `src/internal/domain/routing/` + `src/internal/domain/routing/service/`
- New provider adapter — `src/internal/adapters/provider/` + `src/internal/ports/provider.go`
- Configuration changes — `src/internal/infra/config/`
- Observability/telemetry changes — `src/internal/infra/telemetry/`, `src/internal/infra/metrics/`, `src/internal/infra/logging/`
- Frontend changes — `ui/` (when implemented)
- Spec/plan/tasks changes — `specs/active/<slug>/`
- Admin binary changes — `src/cmd/admin/` + `src/internal/api/admin.go`
- Build/CI changes — `Makefile`, `Dockerfile.gateway`, `Dockerfile.admin`, `.golangci.yml`
- Deployment changes — `deployments/docker-compose/`

## Excluded
- `.speckeep/**` — excluded from indexing (agent workflow config)
- `specs/archived/**` — excluded from indexing (completed specs)
- `.git/**` — excluded from indexing
- `bin/**` — excluded from indexing (build artifacts)
- `demo/**` — excluded from indexing
- `docs/**` — excluded from indexing
- `node_modules/**` — excluded from indexing
- `vendor/**` — excluded from indexing
- `dist/**`, `build/**`, `coverage/**` — excluded from indexing
- `.opencode/**` — excluded from indexing (IDE config)
