# Log Export Pipeline Plan

## Phase Contract

Inputs: `spec.md` + minimal repo context.
Outputs: `plan.md` (no data-model/contracts expansion).
Stop if: spec is vague — not the case; decisions pinned below.

## Goal

Add a per-tenant export pipeline that captures the **masked** request and provider response on the proxy path, fans the record out to the sinks a tenant is allowed to use (webhook, S3-compatible, Langfuse), and never exports originals or the mask mapping. Delivery is asynchronous, bounded and failure-isolated; enablement and credentials are server-side.

## MVP Slice

- Capture middleware + async pipeline + generic webhook sink + masked payload + per-tenant routing + failure isolation.
- AC covered: AC-001, AC-002, AC-003, AC-005, AC-008.

## First Validation Path

1. Configure a webhook sink and enable it for one tenant in server config.
2. Send a request whose content is masked by the shield.
3. Observe a signed, masked record at the webhook; a tenant without the sink produces nothing.
4. Point the webhook at a failing endpoint: the client response is unchanged and the failure metric increments.

## Scope

- New export domain (record, sink interface, per-tenant routing) and an async worker.
- Three outbound adapters behind the sink interface: webhook, S3-compatible, Langfuse.
- New capture middleware placed after the shield and before the provider handler.
- Server config section for sinks/credentials/queue plus per-tenant routing.
- Metrics for delivered/failed/dropped records.
- Untouched: shield, conversation storage, OTel, budget alerts.

## Performance Budget

- Delivery is off the request path; SC-001 requires no measurable added latency. Capture does one bounded in-memory copy of the masked request/response.
- Bounded queue; on overflow records are dropped and counted rather than blocking (SC/edge case).

## Implementation Surfaces

- `src/internal/domain/logexport/record.go` — new: `Record` (masked content + metadata) and the `Sink` interface.
- `src/internal/domain/logexport/routing.go` — new: per-tenant routing/config resolution.
- `src/internal/app/logexport/pipeline.go` — new: bounded async worker, fan-out, retry/backoff, drop metric.
- `src/internal/adapters/logexport/webhook.go`, `s3.go`, `langfuse.go` — new: sink adapters.
- `src/internal/api/middleware/export.go` — new: capture masked request/response + enqueue (no writer mutation).
- `src/internal/infra/config/config.go` + `defaults.go` — new `log_export` section.
- `src/internal/infra/metrics/metrics.go` — new export counters.
- `src/internal/api/server.go` — register the export middleware in the proxy chain when configured.
- `src/cmd/gateway/run.go`, `src/cmd/all/gateway.go` — build the pipeline and sinks, register the middleware.
- Tests: `src/internal/app/logexport/*_test.go`, `src/internal/adapters/logexport/*_test.go`, `src/internal/api/middleware/export_test.go`, `src/internal/api/server_test.go`.
- `README.md`, `examples/config.yaml` — document the section and a webhook example.

## Bootstrapping Surfaces

- none — middleware, worker, config and metrics patterns already exist.

## Architecture Impact

- Local: one domain package, one app package, three adapters, one middleware, config + metrics.
- Integration: the proxy chain gains a capture middleware; no change to client-visible behavior.
- No DB migration: per-tenant enablement and credentials are server-side config (DEC-001).
- Streaming content is best-effort in v1 (metadata always; assembled masked text when available).

## Acceptance Approach

- AC-001 → pipeline is inert without routing config; proof: recording sink sees zero deliveries.
- AC-002 → capture reads masked request body and masked provider bytes; proof: delivered payload has placeholders and no originals/mapping.
- AC-003 → routing resolves sinks per tenant; proof: two recording sinks, delivery counts per tenant.
- AC-004 → retention mode gates content at enqueue time; proof: `meta` payload has no content, `none` produces no delivery.
- AC-005 → webhook adapter signs body with HMAC + event id/timestamp; proof: receiver recomputes and matches.
- AC-006 → S3 adapter batches and writes deterministic keys; proof: fake object-store client asserts key/partition/body.
- AC-007 → Langfuse adapter posts masked input/output; proof: captured request body assertion.
- AC-008 → worker isolates per-sink failures; proof: failing + healthy sink test with failure metric.

## Data and Contracts

- Data model: no change (config-based routing).
- Config contract: new `log_export` section (sinks with credentials, queue size, batch size, per-tenant routes). No tenant API change.
- API contract: none; export is outbound only.
- No `contracts/*` or `data-model.md` needed.

## Implementation Strategy

- DEC-001 Config-based per-tenant routing, no DB migration
  Why: fastest to ship, keeps credentials server-side (RQ-009), avoids a migration and API/UI work.
  Tradeoff: routing changes need a restart or the existing config reload path; no self-service UI yet.
  Affects: config.go, defaults.go, routing.go, cmd wiring.
  Validation: routing tests + config parse tests.
- DEC-002 Capture after the shield, before the provider handler
  Why: there the request body is already masked and the response writer sees masked provider bytes (the unmask writer sits upstream), which is exactly what may be exported.
  Tradeoff: a dedicated middleware plus best-effort streaming assembly; conversation middleware cannot be reused because it captures originals.
  Affects: middleware/export.go, server.go.
  Validation: middleware tests asserting masked request and response capture.
- DEC-003 Bounded async worker with per-sink retry and drop-with-metric
  Why: non-blocking, failure-isolated, matches the analytics/conversation worker pattern.
  Tradeoff: records can be dropped under sustained overload; durable delivery is deferred.
  Affects: app/logexport/pipeline.go.
  Validation: failure-isolation and overflow tests.
- DEC-004 Sink interface with three adapters
  Why: one pluggable seam; webhook first, S3 and Langfuse reuse it.
  Tradeoff: more files; shared retry lives in the worker, not each sink.
  Affects: domain/logexport/record.go, adapters/logexport/*.
  Validation: per-adapter tests.
- DEC-005 Masked-only, retention-gated payloads
  Why: the privacy guarantee is the product; raw export is forbidden.
  Tradeoff: no debug mode to export originals.
  Affects: record.go, routing.go, middleware/export.go.
  Validation: payload-assertion tests (AC-002, AC-004).

## Incremental Delivery

### MVP (First Value)

- Domain + pipeline + capture middleware + webhook sink + routing + failure isolation.
- Ready when AC-001/002/003/005/008 pass.

### Iterative Expansion

- S3 adapter (AC-006), then Langfuse adapter (AC-007), each behind the same sink interface.
- Retention gating (AC-004) can land with the pipeline; validated independently.

## Sequencing Notes

- Domain record + sink interface first (all adapters depend on it).
- Pipeline + capture middleware + config/metrics next (MVP).
- Adapters: webhook → S3 → Langfuse.
- The capture middleware must be appended to the chain after budget and before the handler.

## Risks

- Capture ordering vs the shield unmask writer → mitigated by DEC-002 and explicit chain placement tests.
- Streaming assembly complexity → v1 exports metadata for all responses and assembled masked text for non-streaming; streaming content is best-effort.
- S3 dependency → reuse the already-present aws-sdk-go-v2 stack (add the s3 client package).
- Overload dropping records → bounded queue with a drop metric; operators size the queue via config.
- Config-only routing needs reload for changes → documented; existing config watcher can cover it later.

## Rollout and Compatibility

- Additive and disabled by default; no client-visible change and no migration.
- Operators opt in per tenant; watch delivered/failed/dropped metrics after enabling.
- Existing OTel, conversation logs and budget webhooks are unaffected.

## Validation

- Unit: record/routing resolution, pipeline fan-out/retry/drop, webhook signature, S3 key/body, Langfuse payload.
- Middleware: masked request/response capture and retention gating.
- Route/integration: end-to-end enqueue via the proxy chain with a recording sink.
- Acceptance IDs covered: AC-001..AC-008. Decisions: DEC-001..DEC-005.

## Constitution Compliance

- no conflicts — Content Shield stays the source of masking; export consumes masked data only, adds no new runtime dependency beyond the existing AWS SDK stack, and does not weaken DLP or tenant isolation.
