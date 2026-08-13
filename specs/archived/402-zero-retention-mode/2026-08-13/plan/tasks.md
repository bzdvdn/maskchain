# Zero-Retention Mode Tasks

## Phase Contract

Inputs: plan (DEC-001..006), data-model (DM-001, DM-002, migration 018).
Outputs: ordered executable tasks with AC coverage.
Stop if: tasks too vague or coverage unmappable.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/adapters/repository/postgres/migrations/018_retention_mode.up/down.sql | T1.1 |
| src/internal/domain/shield/entity/tenant.go | T1.2, T2.1 |
| src/internal/domain/shield/repository.go | T1.2, T2.1 |
| src/internal/adapters/repository/postgres/tenant.go | T1.2, T2.1 |
| src/internal/domain/shield/resolver/tenant_resolver.go | T2.1, T2.2 |
| src/internal/infra/config/{config,defaults,validator}.go | T1.3, T2.2 |
| src/internal/api/dto/tenant.go | T2.1, T2.3 |
| src/internal/api/handler/admin/tenant_handler.go | T2.1, T2.3 |
| src/internal/api/middleware/shield.go | T3.1 |
| src/internal/api/middleware/conversation.go | T3.1, T3.2, T3.3 |
| src/internal/app/conversation/async_worker.go | T3.1, T3.2 |
| src/internal/domain/conversation/conversation_log.go | T3.2, T3.3 |
| src/internal/domain/conversation/storage.go | T3.3, T4.1 |
| src/internal/adapters/repository/conversation/pg_conversation_store.go | T3.3, T4.1 |
| src/internal/api/handler/conversation/conversation_handler.go | T3.3, T4.1 |
| ui/src/pages/Conversations.tsx | T4.2 |
| ui/src/api/conversations.ts | T4.2 |
| ui/src/pages/Tenants/TenantDetail.tsx, TenantForm.tsx | T4.3 |
| ui/src/api/tenants.ts | T4.3 |

## Implementation Context

- MVP goal: tenant `retention_mode: none` writes zero conversation rows while counters continue; admin API + global `data.retention.mode` default (AC-001/003/005/006/007).
- Invariants: mode ∈ {full,meta,none}, effective = explicit tenant value else config default (DEC-002); `none` never enqueues a RawRecord (DEC-003); `meta` rows carry zero content bytes (DEC-003/004); `full` unchanged.
- Block facts: detector + category only, set on context at shield block, stored in nullable `conversation_logs.detector/category` (DEC-004).
- Capture policy: log ALL chat requests in `full`/`meta` (lift masked/blocked-only short-circuit) so masked/not-masked filter is meaningful (DEC-005); `masked` filter is tri-state `*bool` (DEC-006).
- Contracts: tenant JSON gains `retention_mode`; list API gains `masked=true|false`; both additive/optional.
- Errors: invalid mode → 400/validation error, tenant unchanged (AC-007).
- Proof signals: `go test` on affected packages, `go build ./...`, UI `tsc -b`/vite build; AC tests listed per task.
- Out of scope: purge, encryption, session/analytics retention, extra UI columns, meta anomaly table.
- References: DEC-001..006, DM-001, DM-002, RQ-001..008.

## Phase 1: Foundation

Goal: schema + config + entity plumbing so behavior phases are predictable.

- [x] T1.1 Add migration 018 — outcome: `tenants.retention_mode VARCHAR(16) NOT NULL DEFAULT 'full'`, `conversation_logs.detector/category VARCHAR(64)` nullable, reversible. Touches: src/internal/adapters/repository/postgres/migrations/018_retention_mode.up.sql, src/internal/adapters/repository/postgres/migrations/018_retention_mode.down.sql
      Proof: code src/internal/adapters/repository/postgres/migrations/018_retention_mode.up.sql
      Proof: code src/internal/adapters/repository/postgres/migrations/018_retention_mode.down.sql
- [x] T1.2 Add retention field to Tenant entity + repo — outcome: `retention` field, `WithTenantRetentionMode`, accessor on shield/entity.Tenant; PostgresTenantRepo persists/reads the column. Touches: src/internal/domain/shield/entity/tenant.go, src/internal/domain/shield/repository.go, src/internal/adapters/repository/postgres/tenant.go
      Proof: code src/internal/domain/shield/value/retention_mode.go RetentionMode
      Proof: code src/internal/domain/shield/entity/tenant.go WithTenantRetentionMode
      Proof: code src/internal/adapters/repository/postgres/tenant.go scanTenant
- [x] T1.3 Add global config default — outcome: `data.retention.mode` (default `full`) in config + defaults; validator rejects values outside enum. Touches: src/internal/infra/config/config.go, src/internal/infra/config/defaults.go, src/internal/infra/config/validator.go
      Proof: code src/internal/infra/config/config.go RetentionConfig
      Proof: code src/internal/infra/config/validator.go validateDataRetention

## Phase 2: MVP Slice (none mode end-to-end)

Goal: operator can set a tenant to `none` and observe zero rows with counters intact.

- [x] T2.1 Add tenant API plumbing — outcome: create/update accept `retention_mode`, tenant response returns it; invalid value rejected (400), previous mode retained. Touches: src/internal/api/dto/tenant.go, src/internal/api/handler/admin/tenant_handler.go
      Proof: code src/internal/api/dto/tenant.go TenantResponse
      Proof: code src/internal/api/handler/admin/tenant_handler.go CreateTenant
- [x] T2.2 Resolve effective mode — outcome: tenant resolver returns effective mode = explicit tenant value else config default; `none` default applied when unset. Touches: src/internal/domain/shield/resolver/tenant_resolver.go, src/internal/infra/config/config.go (side effect: SetDefaultRetentionMode wired in cmd/gateway/run.go, cmd/admin/run.go, cmd/all/gateway.go, cmd/all/admin.go per DEC-002)
      Proof: code src/internal/domain/shield/resolver/tenant_resolver.go EffectiveMode
      Proof: code src/internal/infra/config/config.go DefaultRetentionMode
- [x] T2.3 Prove MVP path — outcome: tests assert `none` tenant shows empty conversation list while session/analytics counters increment; `full` unchanged; invalid mode rejected. Touches: src/internal/api/dto/tenant.go, src/internal/api/handler/admin/tenant_handler.go, src/internal/domain/shield/resolver/tenant_resolver.go
      Proof: test src/internal/api/dto/tenant_test.go TestTenantToResponseRetentionMode
      Proof: test src/internal/api/handler/admin/tenant_handler_test.go TestTenantHandler_CreateWithRetentionMode
      Proof: test src/internal/domain/shield/resolver/resolver_test.go TestDBFirstTenantResolver_EffectiveMode_GlobalDefault

## Phase 3: Meta mode + mask facts

Goal: `meta` rows are metadata-only with anonymized block facts; `none` still writes nothing.

- [x] T3.1 Add shield block-facts context — outcome: on block, shield middleware sets detector + category in context (anonymized, no fragment text). Touches: src/internal/api/middleware/shield.go
      Proof: code src/internal/api/middleware/shield.go publishBlockFacts
      Proof: test src/internal/api/middleware/conversation_test.go TestConversationMiddlewareMetaBlockFacts
- [x] T3.2 Gate capture by mode — outcome: middleware reads effective mode; `none` returns before enqueue; `meta` builds metadata-only RawRecord (empty request/response/masking + detector/category); worker passes facts through; constructor relaxed for metadata-only. Touches: src/internal/api/middleware/conversation.go, src/internal/app/conversation/async_worker.go, src/internal/domain/conversation/conversation_log.go
      Proof: code src/internal/api/middleware/conversation.go effectiveRetentionMode
      Proof: code src/internal/domain/conversation/conversation_log.go NewMetadataOnlyConversationLog
      Proof: test src/internal/app/conversation/async_worker_test.go TestAsyncWorkerEncryptMetadataOnly
- [x] T3.3 Add masked filter field + lift capture short-circuit — outcome: `ConversationFilter.Masked *bool`; middleware logs all chat requests in full/meta (removes masked/blocked-only return); empty mask filter preserves unfiltered behavior. Touches: src/internal/api/middleware/conversation.go, src/internal/domain/conversation/storage.go, src/internal/domain/conversation/conversation_log.go
      Proof: code src/internal/domain/conversation/storage.go ConversationFilter
      Proof: test src/internal/api/middleware/conversation_test.go TestConversationMiddlewareLogsUnmaskedClean

## Phase 4: Filters, UI, verification

Goal: masked/not-masked filter reachable via API and UI; UI mode control; package reviewable.

- [x] T4.1 Wire masked filter through store + handler — outcome: pg List honors `masked` (true/false) with counts; handler parses `masked` query param; `Get` returns detector/category. Touches: src/internal/adapters/repository/conversation/pg_conversation_store.go, src/internal/domain/conversation/storage.go, src/internal/api/handler/conversation/conversation_handler.go
      Proof: code src/internal/adapters/repository/conversation/pg_conversation_store.go List
      Proof: code src/internal/api/handler/conversation/conversation_handler.go parseMaskedQuery
      Proof: test src/internal/api/handler/conversation/conversation_handler_test.go TestConversationHandlerListFilters
      Proof: test src/internal/api/handler/conversation/conversation_handler_test.go TestConversationHandlerDetailBase64RoundTrip
- [x] T4.2 Add UI masked filter — outcome: Conversations page filter bar has Masked/Not-masked select; page re-requests with `masked`; columns stay exactly id/tenant/model/status/masked/streamed/created. Touches: ui/src/pages/Conversations.tsx, ui/src/api/conversations.ts
      Proof: code ui/src/pages/Conversations.tsx masked filter select
      Proof: code ui/src/api/conversations.ts ConversationFilters
      Proof: test ui/src/pages/Conversations.test.tsx re-requests with masked filter when selected
- [x] T4.3 Add UI retention-mode control — outcome: Tenant detail/form edits and saves `retention_mode`; response reflects it; a `none` tenant shows zero rows. Touches: ui/src/pages/Tenants/TenantDetail.tsx, ui/src/pages/Tenants/TenantForm.tsx, ui/src/api/tenants.ts
      Proof: code ui/src/pages/Tenants/TenantForm.tsx Retention Mode select
      Proof: code ui/src/pages/Tenants/TenantDetail.tsx Retention Mode row
      Proof: code ui/src/api/tenants.ts RetentionMode
      Proof: test ui/src/pages/Tenants/TenantForm.test.tsx sends retention_mode on create
- [x] T4.4 Repo-wide verification — outcome: `go build/vet/test ./...` + golangci-lint clean; UI `tsc -b` + vite build clean; migration runner applies 018 cleanly. Touches: src/internal/domain/shield/entity/tenant.go, src/internal/domain/shield/resolver/tenant_resolver.go, src/internal/api/middleware/conversation.go, src/internal/domain/conversation/storage.go, src/internal/adapters/repository/conversation/pg_conversation_store.go, src/internal/api/handler/conversation/conversation_handler.go, ui/src/pages/Conversations.tsx, ui/src/pages/Tenants/TenantDetail.tsx, ui/src/api/tenants.ts, src/internal/infra/config/validator.go, src/internal/adapters/repository/postgres/migrations/018_retention_mode.up.sql
      Proof: test cmd go build ./... && go vet ./...
      Proof: test src/internal/adapters/repository/conversation/pg_conversation_store_test.go TestPgConversationStoreListAndFilter (real PG, masked filter + counts)
      Proof: test src/internal/adapters/repository/postgres/migrations RunMigrations applied 018 to scratch DB (maskchain_retention_test), schema verified, DB dropped
      Proof: test ui npm run build && npm run lint && npx vitest run (4 files, 18 tests)
      Proof: test cmd golangci-lint run ./internal/... ./cmd/... (clean after gofmt)
      Note: go test ./... has 3 pre-existing failures in compliance packages (TestComplianceHandler_ApplyPack/Report, TestLoadPacksFromDir) caused by missing specs/active/401-compliance-packs/testdata — untouched by this feature, reproduced on HEAD.

## Acceptance Coverage

- AC-001 -> T2.1, T2.2, T2.3
- AC-002 -> T3.2, T4.4
- AC-003 -> T2.2, T3.2, T4.4
- AC-004 -> T3.1, T3.2, T4.1, T4.4
- AC-005 -> T2.2, T3.2, T4.4
- AC-006 -> T1.3, T2.2, T4.4
- AC-007 -> T1.3, T2.1, T4.4
- AC-008 -> T3.3, T4.1, T4.4
- AC-009 -> T4.2, T4.4
- AC-010 -> T4.3, T4.4

## Notes

- Order follows plan Incremental Delivery (MVP none first, then meta, then filters/UI).
- Each `[x]` task requires a `Proof:` line (`Proof: kind path anchor`) before archive; validation is split into T2.3/T4.4.
- Migration must precede T1.2/T2.1 (column must exist for repo reads).