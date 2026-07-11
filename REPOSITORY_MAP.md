# Repository Map

## Entry Points
- `src/cmd/gateway/main.go` — runtime entrypoint (binary: `bin/gateway`)

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
  - `ui/embed.go` — Go embed для встраивания статики в gateway
  - `ui/src/pages/Profiles/` — ProfileList, ProfileDetail, ProfileForm
  - `ui/src/pages/Incidents/` — IncidentList, IncidentDetail
  - `ui/src/components/` — DictionaryEditor, PreprocessorEditor, ErrorBoundary
  - `ui/src/api/profiles.ts` — API client для `/api/v1/profiles/*`
  - `ui/src/api/incidents.ts` — API client для `/api/v1/incidents/*`
- `specs/active/` — active spec artifacts (speckeep-managed)
- `deployments/` — Docker, migrations, docker-compose configs

## Key Paths
- `src/cmd/gateway/main.go` — gateway binary entrypoint (DI wiring for all components)
- `src/internal/domain/` — core business logic: Content Shield, profiles, policies
  - `src/internal/domain/shield/mask/` — mask entry entity, storage interface, use case, UUIDv7
  - `src/internal/domain/shield/detector/` — detector interface, registry, composite detector
- `src/internal/app/` — use case orchestration
  - `src/internal/app/usecase/shield/` — ShieldEngine, ScanUseCase, ScanPipelineFactory, ApplyPolicyUseCase
- `src/internal/ports/` — inbound (REST, gRPC) and outbound (repository, provider) interfaces
- `src/internal/adapters/` — repository impl (PostgreSQL), provider clients (OpenAI, Anthropic, etc.)
  - `src/internal/adapters/repository/mask/` — Postgres, Valkey, Cached mask repos
- `src/internal/api/` — HTTP handlers, middleware, request/response types
  - `src/internal/api/mask_handler.go` — POST /api/v1/shield/mask and /unmask handlers
   - `src/internal/api/handler/profile/` — Profile CRUD handlers (list, get, create, update, delete, patch dictionary)
   - `src/internal/api/handler/incident/` — Incident read/export handlers (list, get, export CSV/JSON)
   - `src/internal/api/dto/` — request/response DTOs (ProfileResponse, PaginatedResponse, DictionaryDTO, IncidentResponse)
- `src/internal/infra/config/` — cobra/viper config loading, validation, defaults
- `src/internal/infra/telemetry/` — OTel SDK init, TracerProvider, MeterProvider, OTLP exporters
- `src/internal/infra/metrics/` — Prometheus metric definitions (HTTP, shield), /metrics handler
- `src/internal/infra/logging/` — slog adapter with OTel trace_id/span_id enrichment
- `src/internal/infra/migrations/` — SQL migrations (001_mask_entries.sql, 002_incidents.sql)
- `specs/active/22-shield-mask-storage/` — mask storage phase: spec, plan, tasks
- `specs/active/41-profiles-ui/` — profiles UI phase: spec, plan, tasks, inspect
- `specs/active/50-shield-engine/` — shield engine orchestration: spec, plan, tasks, inspect
- `specs/active/60-audit-incidents/` — audit incidents viewer: spec, plan, tasks, inspect, data-model
- `specs/active/61-observability/` — observability phase: spec, plan, tasks, inspect, data-model
- `deployments/docker-compose/` — local dev environment (PostgreSQL, Valkey)
- `Dockerfile` — multi-stage Docker build (node → golang → distroless)
- `Dockerfile` — multistage Docker build (golang:1.26-alpine → distroless)
- `Makefile` — build, test, lint, docker-build, clean, check-structure targets
- `.golangci.yml` — linter configuration (gofmt, govet, staticcheck, errcheck, unused)
- `.editorconfig` — editor formatting rules
- `ROADMAP.md` — phased development roadmap (speckeep-verified specs)

## Where To Edit
- New domain logic — `src/internal/domain/`
- New use case / feature — `src/internal/app/` + `src/internal/ports/` + `src/internal/adapters/`
- New API endpoint — `src/internal/api/` + `src/internal/ports/` (inbound interface)
- Configuration changes — `src/internal/infra/config/`
- Observability/telemetry changes — `src/internal/infra/telemetry/`, `src/internal/infra/metrics/`, `src/internal/infra/logging/`
- Frontend changes — `ui/` (when implemented)
- Spec/plan/tasks changes — `specs/active/<slug>/`
- Build/CI changes — `Makefile`, `Dockerfile`, `.golangci.yml`
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
