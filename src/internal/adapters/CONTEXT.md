# src/internal/adapters — Infrastructure Adapters

## Purpose

Concrete implementations of the application's ports: outbound provider HTTP clients, egress transport, log-export sinks, and PostgreSQL/Valkey repositories. Domain and app layers stay storage/transport-agnostic; everything here touches the network or a datastore.

## Responsibilities

- Owns: provider adapters, the egress HTTP client (proxy/pool/retry/circuit breaker/SSE), log-export sinks, and all SQL/Valkey persistence.
- Does not own: business rules, routing/budget decisions, HTTP API handlers, or config loading — it implements the interfaces those layers define.

## How it works

- `provider.NewProviderClient` selects an adapter by `api_type` (`openai`, `anthropic`, `ollama`, `proxy`, `gemini`, `bedrock`), builds a per-provider egress transport (proxy, timeout, circuit breaker), and returns a `ports.ProviderClient`.
- `egress.Client` performs `Call` and `Stream` (SSE), with retries, connection pooling, per-provider proxy dialers, and an optional circuit breaker.
- `provider/upstream.go` resolves upstream URLs and enforces the passthrough path allowlist plus `/v1` de-duplication.
- `repository/postgres` holds pgx-backed stores (tenants, virtual keys, budgets, routing registry, admin sessions, audit log) plus the pool and `TransactionManager` (tx passed via context).
- `repository/<backend>` subpackages add Valkey-backed counters/caches and cached composites: analytics, budget, cache, conversation, dictionary, mask, session.
- `logexport` ships batches to webhook, Langfuse, and S3 sinks; only masked content and metadata leave the process.

## Key files

- `src/internal/adapters/provider/factory.go` — provider factory, transport/timeout/CB wiring.
- `src/internal/adapters/provider/upstream.go` — upstream URL resolution, path allowlist, `/v1` de-dup.
- `src/internal/adapters/egress/client.go` — `Call`/`Stream`, retry, timeout.
- `src/internal/adapters/repository/postgres/pool.go` — pgxpool construction and migrations bootstrap.
- `src/internal/adapters/repository/postgres/transaction.go` — `TransactionManager` / ctx-scoped querier.
- `src/internal/adapters/logexport/wire.go` — exported record shape and event ids.

## Public contracts

- `ports.ProviderClient` (`Call`, `Stream`) — implemented by `egress.Client` and provider adapters.
- `domain/logexport.Sink` (`Name`, `Send`) — implemented by webhook/Langfuse/S3 sinks.
- Domain repository ports: `mask.MaskStorage`, cache store, budget counters, `shield.TenantRepository`, `virtualkey.VirtualKeyRepository`, `admin_session.AdminSessionStore`.
- `postgres.TransactionManager.RunInTx` — tx is passed through the context to `getQuerier`.

## Dependencies / Consumed by

- Uses: `internal/ports`, `internal/domain/*`, `internal/infra/config`, pgx, valkey-go, AWS S3 SDK.
- Used by: `cmd/*` bootstrap/wiring, `internal/app/*` services, `internal/api/*` handlers and middleware.

## Gotchas

- Migrations are embedded (`//go:embed migrations/*.sql`) and run on startup; new SQL goes there, never ad-hoc DDL.
- Postgres stores own all SQL; keep handlers/services free of queries.
- Hard-deleting a virtual key cascades its budgets (`budgets.virtual_key_id ... ON DELETE CASCADE`).
- Virtual key secrets are sealed with `MASKCHAIN_KEYS_KEY`; an empty ciphertext means "not revealable" and must degrade to `SECRET_UNAVAILABLE`.
- Egress clients are per-provider (own transport/pool); do not share one global client across providers.
- Log export must never carry unmasked content.
