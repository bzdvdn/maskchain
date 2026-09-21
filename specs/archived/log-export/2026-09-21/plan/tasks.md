# Log Export Pipeline Tasks

## Phase Contract

Inputs: `plan.md` + `spec.md` (decisions pinned).
Outputs: ordered executable tasks with coverage mapping.
Stop if: acceptance coverage cannot be mapped — not the case.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/domain/logexport/record.go | T1.1 |
| src/internal/domain/logexport/routing.go | T1.1 |
| src/internal/domain/logexport/routing_test.go | T4.1 |
| src/internal/infra/config/config.go | T1.2 |
| src/internal/infra/config/defaults.go | T1.2 |
| src/internal/infra/metrics/metrics.go | T1.3 |
| src/internal/app/logexport/pipeline.go | T2.1 |
| src/internal/app/logexport/pipeline_test.go | T4.2 |
| src/internal/api/middleware/export.go | T2.2 |
| src/internal/api/middleware/export_test.go | T4.4 |
| src/internal/adapters/logexport/webhook.go | T2.3 |
| src/internal/adapters/logexport/s3.go | T3.1 |
| src/internal/adapters/logexport/langfuse.go | T3.2 |
| src/internal/adapters/logexport/webhook_test.go | T4.3 |
| src/internal/adapters/logexport/s3_test.go | T4.3 |
| src/internal/adapters/logexport/langfuse_test.go | T4.3 |
| src/internal/api/server.go | T2.4 |
| src/internal/api/server_test.go | T4.4 |
| src/cmd/gateway/run.go | T2.5 |
| src/cmd/all/gateway.go | T2.5 |
| src/cmd/internal/bootstrap/export.go | T2.5, T3.1, T3.2 |
| README.md | T4.5 |
| examples/config.yaml | T4.5 |

## Implementation Context

- MVP Goal: a tenant with a webhook sink enabled emits a signed, masked record per completed request; delivery is async and failure-isolated; disabled by default.
- Acceptance Boundaries: AC-001..AC-008.
- Key Rules: capture after the shield so request is masked and the response writer sees masked provider bytes (DEC-002); config-based per-tenant routing and server-side creds (DEC-001); bounded async worker with retry + drop metric (DEC-003); sink interface + adapters (DEC-004); masked-only, retention-gated (DEC-005).
- Invariants: never export originals or the mask mapping; a record goes only to sinks enabled for its tenant; client response is never affected by sink failures; `none` retention exports nothing.
- Errors/Codes: export is best-effort — sink errors are logged and counted, never returned to the client; queue overflow drops with a metric; missing/invalid creds treat the sink as disabled with a warning.
- Contracts/Protocols: new `log_export` config section (sinks with creds, queue/batch sizes, per-tenant routes); outbound payload = `{event_id, timestamp, tenant, model, status, tokens, cost, mask_id, content{masked_request, masked_response}}`; webhook carries `X-MaskChain-Signature` (HMAC-SHA256) and event id; S3 key scheme includes tenant + date partition; Langfuse posts to its ingestion API.
- Scope Boundaries: do not export on the tenant API or expose credentials; do not export raw content or the mapping; do not implement durable delivery/replay or Datadog/GCS-native sinks.
- Proof Signals: routing/domain tests, pipeline fan-out/retry/drop/isolation tests, webhook HMAC test, S3 key/body test, Langfuse payload test, middleware masked-capture + retention test, end-to-end enqueue test, `go test` green.
- References: DEC-001..DEC-005 (plan.md), RQ-001..RQ-009 (spec.md).

## Phase 1: Foundation

Goal: the export record, routing, config and metrics exist.

- [x] T1.1 Define the export record, sink interface and per-tenant routing — outcome: a `Record` carrying masked content + metadata, a `Sink` interface, and a resolver mapping a tenant to its enabled sinks. Touches: src/internal/domain/logexport/record.go, src/internal/domain/logexport/routing.go
      Proof: code src/internal/domain/logexport/record.go Record
- [x] T1.2 Add the `log_export` config section — outcome: sinks with credentials, queue/batch sizes and per-tenant routes parse with safe defaults (disabled). Touches: src/internal/infra/config/config.go, src/internal/infra/config/defaults.go
      Proof: code src/internal/infra/config/config.go LogExportConfig
- [x] T1.3 Add export metrics — outcome: delivered/failed/dropped counters registered and visible at `/metrics`. Touches: src/internal/infra/metrics/metrics.go
      Proof: code src/internal/infra/metrics/metrics.go LogExportDeliveredTotal

## Phase 2: MVP Slice

Goal: masked records reach a webhook, asynchronously and per tenant.

- [x] T2.1 Implement the async export pipeline — outcome: bounded queue, fan-out to a tenant's sinks, per-sink retry/backoff, failure isolation and drop-with-metric. Touches: src/internal/app/logexport/pipeline.go
      Proof: code src/internal/app/logexport/pipeline.go Pipeline
- [x] T2.2 Implement the capture middleware — outcome: reads the masked request body and masked provider response, applies retention-mode gating, and enqueues a record without altering the client response. Touches: src/internal/api/middleware/export.go
      Proof: code src/internal/api/middleware/export.go ExportMiddleware
- [x] T2.3 Implement the webhook sink — outcome: delivers JSON with an HMAC-SHA256 signature, event id and timestamp. Touches: src/internal/adapters/logexport/webhook.go
      Proof: code src/internal/adapters/logexport/webhook.go WebhookSink
- [x] T2.4 Register the export middleware in the proxy chain — outcome: the middleware runs after the shield/budget stages and before the provider handler when export is configured. Touches: src/internal/api/server.go
      Proof: code src/internal/api/server.go RegisterExportMiddleware
- [x] T2.5 Wire the pipeline and sinks in both binaries — outcome: gateway and combined build sinks/pipeline and register the middleware. Touches: src/cmd/gateway/run.go, src/cmd/all/gateway.go, src/cmd/internal/bootstrap/export.go
      Proof: code src/cmd/gateway/run.go RegisterExportMiddleware

## Phase 3: Core Implementation

Goal: the remaining sinks behind the same interface.

- [x] T3.1 Implement the S3-compatible sink — outcome: batches records and writes objects under a tenant + date-partitioned key. Touches: src/internal/adapters/logexport/s3.go, src/cmd/internal/bootstrap/export.go
      Proof: code src/internal/adapters/logexport/s3.go S3Sink
- [x] T3.2 Implement the Langfuse sink — outcome: submits masked input/output plus metadata to the Langfuse ingestion API. Touches: src/internal/adapters/logexport/langfuse.go, src/cmd/internal/bootstrap/export.go
      Proof: code src/internal/adapters/logexport/langfuse.go LangfuseSink

## Phase 4: Validation

Goal: prove behavior and leave the package reviewable.

- [x] T4.1 Add routing/domain coverage — outcome: tests assert tenant-to-sink resolution and that a tenant with no route resolves to no sinks (AC-003). Touches: src/internal/domain/logexport/routing_test.go
      Proof: test src/internal/domain/logexport/routing_test.go TestResolverSinksFor
- [x] T4.2 Add pipeline coverage — outcome: tests assert fan-out, per-sink failure isolation with the failure metric, and drop-on-overflow (AC-003, AC-008). Touches: src/internal/app/logexport/pipeline_test.go
      Proof: test src/internal/app/logexport/pipeline_test.go TestPipelineFanOutByTenant
- [x] T4.3 Add adapter coverage — outcome: tests assert webhook HMAC verification, S3 key/partition/body, and Langfuse masked payload (AC-005, AC-006, AC-007). Touches: src/internal/adapters/logexport/webhook_test.go, src/internal/adapters/logexport/s3_test.go, src/internal/adapters/logexport/langfuse_test.go
      Proof: test src/internal/adapters/logexport/webhook_test.go TestWebhookSinkSignsAndDelivers
- [x] T4.4 Add middleware and end-to-end coverage — outcome: tests assert masked request/response capture, retention gating, no delivery when unconfigured, and enqueue through the proxy chain (AC-001, AC-002, AC-004). Touches: src/internal/api/middleware/export_test.go, src/internal/api/server_test.go
      Proof: test src/internal/api/middleware/export_test.go TestExportMiddlewareCapturesMaskedContent
- [x] T4.5 Document the section and a webhook example — outcome: README describes log export and `examples/config.yaml` shows a webhook sink with a tenant route. Touches: README.md, examples/config.yaml
      Proof: docs README.md

## Acceptance Coverage

- AC-001 -> T1.1, T2.1, T2.4, T2.5, T4.4
- AC-002 -> T1.1, T2.2, T4.4
- AC-003 -> T1.1, T2.1, T4.1, T4.2
- AC-004 -> T2.2, T4.4
- AC-005 -> T2.3, T4.3
- AC-006 -> T3.1, T4.3
- AC-007 -> T3.2, T4.3
- AC-008 -> T2.1, T4.2

## Notes

- Ordering is dependency-driven: domain/config/metrics → pipeline+capture+webhook → S3/Langfuse → tests/docs.
- No DB migration; enablement and credentials are server config.
- Streaming content is best-effort in v1 (metadata always); note this in T2.2.
- T4.5 is documentation only and must not change runtime behavior.
