# src/cmd — Binaries and Composition Root

## Purpose

Entrypoints and wiring. Each binary parses config, initializes shared infrastructure, constructs repositories/services/middleware, starts servers and background workers, and handles graceful shutdown. This is the only place that knows the full dependency graph.

## Responsibilities

- Owns: process lifecycle, flag/config parsing, bootstrap (PG/Valkey/health/logger), migrations, secret re-encryption, service/repo construction, worker startup, HTTP server start/stop.
- Does not own: business logic, SQL, HTTP handler bodies, or config parsing rules (see `internal/*`).

## How it works

- `gateway/` — data plane only: builds the `api.Server`, provider registry/selector, shield pipeline, session/cache/budget/usage/export middleware, health checker; no admin API or UI.
- `admin/` — control plane only: builds `api.AdminServer`, admin session auth, tenants/keys/budgets/routing/compliance/audit handlers, status, Swagger, and serves the embedded UI (`ui.DistFiles`).
- `all/` — combined process: starts **both** servers (gateway port + admin port) in one binary, sharing one DB/Valkey/health service and a single `vkCache`; `all/admin.go` and `all/gateway.go` build each side.
- `debug/` — dev-only CLI that exercises a provider adapter (Mistral) directly; not a deployment binary.
- `internal/bootstrap/` — shared helpers: `InitBootstrap` (PG, Valkey, health, logger), `RunMigrations`, `BackfillVirtualKeys`, provider-secret re-encryption, routing/cost-rate loading, cache/budget/compliance/export builders.
- Startup order in every server binary: load config → bootstrap → run migrations (DB) → seal legacy provider secrets → build deps → register routes/middleware → start workers → serve → shutdown on signal.

## Key files

- `src/cmd/gateway/run.go` — data-plane wiring.
- `src/cmd/admin/run.go` — control-plane wiring + embedded UI.
- `src/cmd/all/run.go` — combined lifecycle (two servers, one shutdown).
- `src/cmd/all/admin.go` / `src/cmd/all/gateway.go` — combined-side builders.
- `src/cmd/internal/bootstrap/bootstrap.go` — `InitBootstrap`, `HealthCheck`, logger.

## Public contracts

- Cobra roots: `config.NewGatewayRootCmd()`, `config.NewAdminRootCmd()`; `all` also supports `health`.
- `bootstrap.InitBootstrap(ctx, cfg, log, serviceName) (*Bootstrap, error)` returning `PGPool`, `ValkeyClient`, `HealthSvc`, `PromRegistry`, `OTelShutdown`.
- Graceful shutdown on SIGINT/SIGTERM with a bounded timeout.

## Dependencies / Consumed by

- Uses: `internal/api`, `internal/app`, `internal/adapters`, `internal/domain`, `internal/infra`, `ui` (embedded).
- Used by: container/compose/Helm entrypoints (`deployments/`).

## Gotchas

- Migrations run on boot; a failed migration aborts startup.
- When DB routing is enabled, a missing/invalid `MASKCHAIN_KEYS_KEY` fails closed.
- `all` must shut both servers down; do not leak goroutines on one-sided failure.
- Workers (analytics, conversations, budget, session cleanup, log export, health, vk-cache) are started here and must honor context cancellation.
- The UI is embedded at build time — rebuild the binary to ship UI changes.
- `debug/` is a developer utility, not a production command.
