---
report_type: verify
slug: log-export
status: pass
docs_language: en
generated_at: 2026-09-21
---

# Verify Report: log-export

## Scope

- snapshot: a per-tenant pipeline ships masked records + metadata to webhook/S3/Langfuse sinks, retention-gated, asynchronous and failure-isolated, never exporting originals or the mask mapping.
- verification_mode: default
- artifacts:
  - CONSTITUTION.md
  - specs/active/log-export/spec.md
  - specs/active/log-export/plan.md
  - specs/active/log-export/tasks.md
- inspected_surfaces:
  - src/internal/domain/logexport/{record,routing}.go (+ routing test)
  - src/internal/app/logexport/pipeline.go (+ test)
  - src/internal/adapters/logexport/{wire,webhook,s3,langfuse}.go (+ tests)
  - src/internal/api/middleware/export.go (+ test)
  - src/internal/api/server.go
  - src/cmd/gateway/run.go, src/cmd/all/gateway.go, src/cmd/internal/bootstrap/export.go
  - src/internal/infra/config/{config,defaults}.go, src/internal/infra/metrics/metrics.go
  - go.mod, README.md, examples/config.yaml

## Verdict

- status: pass
- archive_readiness: safe
- summary: all 15 tasks carry `Proof:` lines, 14 export tests pass, every AC maps to passing evidence, and the pipeline is inert unless a tenant is explicitly routed.

## Checks

- task_state: completed=15, open=0 (verify-task-state.sh: `OK: all tasks are marked complete`; `PROOFS_MISSING=0`).
- acceptance_evidence:
  - AC-001 -> `TestPipelineUnroutedTenantDrops`, `TestResolverNil`, config default `enabled: false`.
  - AC-002 -> `TestExportMiddlewareCapturesMaskedContent` (masked request/response captured), `TestWebhookSinkSignsAndDelivers` (no original in payload), S3/Langfuse masked assertions.
  - AC-003 -> `TestResolverSinksFor`, `TestPipelineFanOutByTenant` (1 record per tenant sink).
  - AC-004 -> `TestExportMiddlewareMetaOnly` (no content), `TestExportMiddlewareNoneRetention` (no records).
  - AC-005 -> `TestWebhookSinkSignsAndDelivers` (HMAC verified, event id + timestamp), `TestWebhookSinkErrorOnNon2xx`.
  - AC-006 -> `TestS3SinkWritesPerTenant` (one object per tenant, `logs/<tenant>/<YYYY>/<MM>/<DD>/<event>.jsonl`, NDJSON body).
  - AC-007 -> `TestLangfuseSinkPostsMasked` (`/api/public/ingestion`, basic auth, masked input/output, `trace-create`).
  - AC-008 -> `TestPipelineFailureIsolation` (healthy sink still delivered, failure metric), `TestPipelineQueueOverflowDrops`.
- implementation_alignment:
  - `ExportMiddleware` runs after the shield/budget stages and before the provider handler, so it sees masked request bytes and the provider's masked response; it never wraps/rewrites the response.
  - `Pipeline` fans out by tenant, retries per sink, and drops with a metric instead of blocking; sink failures are counted and never surface to the client.
  - `BuildExportSinks` builds webhook/S3/Langfuse sinks and skips incomplete definitions; S3 client supports custom endpoints (path-style).

## Errors

- none

## Warnings

- none

## Questions

- none

## Not Verified

- Live sink endpoints: webhook/Langfuse are verified with `httptest`, and S3 with a fake client; no real network delivery or real S3/Langfuse instance was exercised.
- Streaming content: v1 captures whatever the response writer emits (best-effort); there is no dedicated streaming-export test.
- `BuildExportSinks` is covered by adapter/constructor tests, but the config-driven construction (including real S3 client build) was not exercised end-to-end.
- The dependency upgrade to `aws-sdk-go-v2 v1.47.0` / `service/s3 v1.113.1` builds and vets clean, but was not runtime-verified against a live object store.

## Next Step

- safe to archive
