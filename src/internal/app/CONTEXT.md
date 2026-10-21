# src/internal/app — Use Cases and Workers

## Purpose

Application services that orchestrate domain rules and ports: background workers, pipelines, and use cases. This layer decides *when* to call the domain and *which* ports to use, but contains no SQL, HTTP, or provider SDK calls.

## Responsibilities

- Owns: usage analytics pipeline, budget aggregation/notifications, semantic-cache service, compliance apply/report, conversation logging pipeline, log-export pipeline, shield scan/apply use cases, session cleanup.
- Does not own: transport, persistence details, config parsing, or routing/selection logic (domain).

## How it works

- `analytics` — `AsyncWorker` buffers `TokenUsage` and flushes to `UsageStore`; `AggregationWorker` rolls usage up; `CleanupWorker` enforces retention.
- `budget` — `AggregationWorker` rolls spend; `notifier` fires webhook alerts for soft limits / thresholds.
- `cache` — `SemanticCacheService` with an embedder (+ fallback), context handling, and a guard deciding hit eligibility.
- `compliance` — `ApplyPackService` applies a pack to a tenant; `ComplianceReportService` computes active/deviated/missing rules.
- `conversation` — `AsyncWorker` batches encrypted logs to the conversation store; `CleanupWorker` expires old logs.
- `logexport` — `Pipeline` batches records and fans out to domain `Sink`s with per-sink isolation.
- `usecase/shield` — `ShieldEngine`, `PipelineFactory`, `ScanUseCase`, `ApplyPolicyUseCase`.
- `worker` — `CleanupWorker` for expired tenant sessions.

## Key files

- `src/internal/app/cache/service.go` — semantic-cache orchestration.
- `src/internal/app/logexport/pipeline.go` — batching/fan-out with retries.
- `src/internal/app/compliance/apply.go` — pack application.
- `src/internal/app/analytics/async_worker.go` — usage buffering.
- `src/internal/app/usecase/shield/shield_engine.go` — masking pipeline entrypoint.

## Public contracts

- Services exposed to handlers/bootstrap: `SemanticCacheService`, `ApplyPackService`, `ComplianceReportService`, `Pipeline`, shield use cases.
- Worker constructors + `Run(ctx)` started by `cmd/*` bootstrap.

## Dependencies / Consumed by

- Uses: `internal/domain/*` ports, `internal/ports`, `internal/infra/*`.
- Used by: `cmd/*` bootstrap (workers/services) and `internal/api/*` handlers/middleware.

## Gotchas

- Workers are top-level goroutines started at boot; they must honor `ctx` cancellation and be idempotent on restart.
- A failing log-export sink must not block other sinks.
- Tenant scope matters: most services take a tenant/workspace and must not leak across tenants.
- No direct DB/SDK calls — go through the domain repository ports.
