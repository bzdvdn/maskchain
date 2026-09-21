# Log Export Pipeline

## Scope Snapshot

- In scope: a per-tenant pipeline that ships masked request/response records and metadata to pluggable sinks (generic webhook, S3-compatible object store, Langfuse), respecting tenant retention mode, without ever exporting originals or the mask mapping.
- Out of scope: durable delivery/replay, Datadog/GCS-native sinks, OTel log export (tracing/metrics already exist), and any change to conversation storage or the shield itself.

## Goal

Compliance and platform teams need LLM traffic in their own observability and storage systems, but today MaskChain only exposes OTel metrics/traces and keeps conversation logs inside its own Postgres. This feature lets an operator route each tenant's traffic to the sinks that tenant is allowed to use — a webhook, an S3-compatible bucket, or Langfuse — while guaranteeing that only masked content and metadata ever leave the gateway. Success is visible when a tenant's request appears in its configured sink with placeholders intact and no original values.

## Primary User Flow

1. Starting point: an operator configures sinks server-side (endpoints and credentials) and enables one or more sinks for a tenant.
2. Main interaction: a client sends a proxied request that completes.
3. Outcome: a masked record (or metadata-only, per retention mode) is delivered asynchronously to each sink enabled for that tenant, and the client response is unaffected.
4. Failure/fallback path: when a sink is down or slow, the request still succeeds, other sinks still receive the record, and a failure metric is incremented.

## User Stories

- P1 Story: as a compliance engineer, I can ship a tenant's masked traffic to our own storage without raw PII leaving the network.
- P2 Story: as a platform engineer, I can send one tenant to Langfuse and another to S3 without cross-tenant leakage.
- P3 Story: as a security engineer, I can prove that exported payloads never contain originals or the mask mapping.

## MVP Slice

- Pipeline + generic webhook sink + masked payload + per-tenant routing + failure isolation.
- AC covered: AC-001, AC-002, AC-003, AC-005, AC-008.

## First Deployable Outcome

- After the first pass, a tenant with a webhook sink enabled produces a signed, masked record at the webhook for each completed request, and disabling the sink stops delivery.

## Scope

- An export pipeline fed from the same masked capture point as conversation logging, independent of whether conversation logging is enabled.
- Pluggable sinks behind one interface: generic webhook (HMAC-signed), S3-compatible object store, Langfuse ingestion.
- Per-tenant enablement/routing of sinks, with retention mode gating content.
- Asynchronous, non-blocking delivery with retry/backoff, failure isolation and metrics.
- Server-side sink credentials, never exposed through the tenant API.

## Out of Scope

- Durable delivery guarantees, dead-letter persistence, or replay tooling.
- Native Datadog, GCS, Azure Blob, or Splunk sinks (a generic webhook can cover them initially).
- Changing what is captured or how conversation logs are stored/encrypted.
- Per-user or per-key export routing (tenant scope only in this feature).
- Exporting raw (unmasked) content or the mask mapping under any configuration.

## Context

- Conversation logging already captures request/response at the proxy and stores it encrypted; the export pipeline must be able to run without conversation logging being enabled.
- Retention mode is per tenant (`full`, `meta`, `none`) and is already exposed on the tenant API; it is the natural gate for whether content may be exported.
- Budget alerts already use an HTTP webhook notifier, which is a precedent for outbound delivery and retry behavior.
- The shield middleware publishes the mask id and masked content on the request path, so export must consume the masked form, not the provider-bound original.
- The gateway already uses a bounded async worker pattern (analytics and conversation pipelines) that the export pipeline should follow.

## Dependencies

- Existing tenant model (retention mode) and tenant configuration storage.
- Existing proxy middleware chain (a capture point for completed requests).
- Existing outbound HTTP client patterns and the analytics/conversation async worker pattern.
- External: the operator's webhook endpoint, an S3-compatible store, and a Langfuse instance (all optional and operator-provided).
- No new runtime dependency beyond a standard S3-compatible client library.

## Requirements

- RQ-001 Export MUST be disabled by default; a tenant with no enabled sink produces no outbound export traffic.
- RQ-002 Exported records MUST contain only masked request/response content plus metadata (tenant, model, token counts, cost, status, mask id, timestamp); original values and the mask mapping MUST never be exported.
- RQ-003 Retention mode MUST gate content: `full` exports masked content plus metadata, `meta` exports metadata only, and `none` exports nothing.
- RQ-004 A record MUST be delivered only to the sinks enabled for that tenant; tenants MUST NOT receive each other's records.
- RQ-005 The webhook sink MUST sign each delivery (HMAC) and include an event id and timestamp so the receiver can verify authenticity and deduplicate.
- RQ-006 The S3 sink MUST write batched records as objects under a deterministic key scheme that includes the tenant and a time partition.
- RQ-007 The Langfuse sink MUST submit masked input/output plus metadata to the Langfuse ingestion API.
- RQ-008 Delivery MUST be asynchronous and non-blocking: sink failures MUST NOT affect the client response, MUST NOT block delivery to other sinks, and MUST increment a failure metric with a structured log.
- RQ-009 Sink endpoints and credentials MUST be configured server-side and MUST NOT be exposed or modifiable through the tenant API.

## Non-Goals

- Guaranteeing exactly-once or at-least-once delivery (best-effort with retry in this feature).
- Replacing the existing OTel tracing/metrics or the audit log.
- Exporting to a sink on behalf of a tenant that has retention mode `none`.
- Adding UI screens beyond what is needed to enable a sink for a tenant.

## Acceptance Criteria

### AC-001 Export is disabled by default

- Why this matters: no tenant leaks traffic without explicit configuration.
- **Given** a deployment with no sink enabled for a tenant
- **When** that tenant's request completes
- **Then** no outbound delivery to any sink occurs.
- Evidence: a test with a recording sink asserting zero deliveries when export is unconfigured.

### AC-002 Only masked content and metadata leave the gateway

- Why this matters: this is the core privacy guarantee.
- **Given** a request whose content was masked by the shield
- **When** the record is delivered to a sink
- **Then** the payload contains the placeholders and metadata, and contains neither the original values nor the mask mapping.
- Evidence: a test capturing the delivered payload and asserting placeholders are present and originals/mapping are absent.

### AC-003 Per-tenant routing is enforced

- Why this matters: tenants must not receive each other's traffic.
- **Given** tenant A enabled on sink X and tenant B enabled on sink Y
- **When** a request from A completes
- **Then** only X receives A's record and Y receives nothing for that request.
- Evidence: a test with two recording sinks asserting delivery counts per tenant.

### AC-004 Retention mode gates content

- Why this matters: zero-retention tenants must not have content exported.
- **Given** a tenant with retention mode `meta` and another with `none`
- **When** their requests complete
- **Then** the `meta` tenant's record has metadata but no content, and the `none` tenant produces no delivery.
- Evidence: tests asserting the payload shape for `meta` and zero deliveries for `none`.

### AC-005 Webhook deliveries are signed

- Why this matters: receivers must be able to verify authenticity.
- **Given** a webhook sink configured with a signing secret
- **When** a record is delivered
- **Then** the request carries an HMAC signature, an event id and a timestamp, and the signature verifies against the body with the secret.
- Evidence: a test receiver that recomputes the HMAC and asserts it matches.

### AC-006 S3 sink writes deterministic objects

- Why this matters: operators need predictable, partitioned storage.
- **Given** an S3 sink enabled for a tenant
- **When** records are flushed
- **Then** an object is written whose key includes the tenant and a time partition, containing the batched records.
- Evidence: a test using a fake object-store client asserting the key pattern and object contents.

### AC-007 Langfuse sink receives masked input/output

- Why this matters: LLM observability must not require unmasked data.
- **Given** a Langfuse sink enabled for a tenant
- **When** a request completes
- **Then** the ingestion payload contains the masked input and output plus the model/tenant metadata.
- Evidence: a test capturing the Langfuse request body and asserting masked fields.

### AC-008 Sink failures are isolated and observable

- Why this matters: a broken sink must not break the gateway or other sinks.
- **Given** one sink returning errors and another healthy sink, both enabled for a tenant
- **When** a request completes
- **Then** the client response is unchanged, the healthy sink still receives the record, and the failure metric increments for the failing sink.
- Evidence: a test asserting the response status, healthy-sink delivery, and the failure metric value.

## Assumptions

- Sink definitions and credentials live in server configuration; per-tenant enablement/routing lives with tenant settings. The exact storage is a plan decision.
- A bounded in-memory queue with retry/backoff is acceptable for this feature; durable delivery is deferred.
- The masked request/response and mask id are available at a capture point on the proxy path (the same data conversation logging uses).
- Langfuse ingestion is its batch HTTP endpoint; S3 is any S3-compatible store (AWS, MinIO, GCS via interoperability).
- Export volume per tenant is bounded by the existing request rate; the queue drops records with a metric rather than blocking when full.

## Success Criteria

- SC-001 Enabling export adds no measurable latency to the client response (delivery is off the request path).
- SC-002 No original value or mask mapping is present in any exported payload across all sinks (verified by tests).
- SC-003 A failing sink produces no client-visible errors and does not reduce delivery to healthy sinks.

## Edge Cases

- Request fails before masking (for example a routing error): only metadata is exported, never partial content.
- Streamed responses: the exported output is the final assembled masked text (or metadata only for `meta`).
- Very large payloads: records exceeding a size limit are dropped with a metric rather than blocking the queue.
- Sink credentials missing or invalid: the sink is treated as disabled for delivery and a configuration warning is logged.
- Tenant retention mode changes: subsequent requests honor the new mode; already-queued records keep their captured mode.

## Open Questions

- none
