# Zero-Retention Mode Implementation Plan

## Phase Contract

Inputs: spec, inspect (concerns, findings resolved), minimal repo context (conversation middleware/store, shield entity/resolver, config).
Outputs: plan.md, data-model.md.
Stop if: spec intent not preservable.

## Goal

Deliver per-tenant retention `full|meta|none` gating conversation-log persistence, a global `data.retention.mode` default, and a masked/not-masked filter on the request-log API + UI, with the Conversations page columns kept exactly `id/tenant/model/status/masked/streamed/created`.

## MVP Slice

- `none` mode + global default + admin tenant API field: no rows written for a `none` tenant, counters intact. Closes AC-001, AC-003, AC-005, AC-006, AC-007.
- Second slice: `meta` mode (AC-002, AC-004) + masked/not-masked filter API+UI (AC-008, AC-009, AC-010).

## First Validation Path

Scripted: create tenant `bob` with `retention_mode: none` (admin API), send chat traffic, assert `GET /api/v1/conversations?tenant_id=bob` returns empty while session/analytics counters grow. Then flip a second tenant to `meta`, send one blocked request, `GET` the row and confirm detector/category present with no content.

## Scope

- Capture-path gating in `src/internal/api/middleware/conversation.go` (none → skip enqueue; meta → metadata-only RawRecord; full → current behavior).
- Lifting the masked/blocked-only capture short-circuit so all chat requests are logged in `full`/`meta` (required for a meaningful masked/not-masked filter).
- Tenant entity retention field + admin DTO/handler plumbing + pg persistence (new `retention_mode` column).
- Global config `data.retention.mode` (`full|meta|none`, default `full`) + validation.
- `meta` block-anomaly facts: detector + category persisted in new nullable `conversation_logs` columns.
- Anonymized-block context captured in the shield middleware and carried through RawRecord.
- `ConversationFilter.Masked` + list handler `masked` query param + pg WHERE + UI filter select.
- UI: masked filter on Conversations page (columns unchanged); retention-mode control on Tenant detail/form.
- Explicitly out: retroactive purge, encryption changes, retention at session/analytics layer, extra UI columns, meta-anomaly table normalization.

## Performance Budget

- Retention gating adds one branch on an already-resolved tenant; target: no change to SLO, p99 delta < 100 ms vs baseline (SC-002).
- `meta`/`none` reduce DB writes; `full` logging-all requests increases write volume and storage — size discipline: reuse `maxContentSize=1MB` cap; no per-request allocations on the capture path beyond the existing `RawRecord`.

## Implementation Surfaces

- `src/internal/domain/conversation/conversation_log.go` — relax `NewConversationLog` empty-request invariant (or add metadata-only constructor) for `meta`; add `Detector`/`Category` fields.
- `src/internal/domain/conversation/storage.go` — add `Masked *bool` to `ConversationFilter`.
- `src/internal/api/middleware/conversation.go` — mode gate; metadata-only record; remove masked/blocked-only short-circuit (exists today, violates "log all").
- `src/internal/api/middleware/shield.go` — set detector/category context key on block.
- `src/internal/app/conversation/async_worker.go` — pass detector/category through `RawRecord` → `ConversationLog`.
- `src/internal/adapters/repository/conversation/pg_conversation_store.go` — masked WHERE, detector/category in insert/list/get.
- `src/internal/domain/shield/entity/tenant.go` — `retention` field + `WithTenantRetentionMode` option + accessor.
- `src/internal/domain/shield/repository.go` + `src/internal/adapters/repository/postgres/tenant.go` — persist/read retention mode.
- `src/internal/api/handler/admin/tenant_handler.go` + `src/internal/api/dto/tenant.go` — request/response `retention_mode`.
- `src/internal/api/handler/conversation/conversation_handler.go` — `masked` query param; `src/internal/domain/shield/resolver/tenant_resolver.go` — effective mode resolution.
- `src/internal/infra/config/{config,defaults}.go` + `validator.go` — `data.retention.mode`.
- `ui/src/pages/Conversations.tsx` + `ui/src/api/conversations.ts` — masked filter (columns unchanged).
- `ui/src/pages/Tenants/TenantDetail.tsx`/`TenantForm.tsx` + `ui/src/api/tenants.ts` + `ui/src/api/client.ts` types — mode control.

## Bootstrapping Surfaces

- Migration `018_retention_mode.up/down.sql` (tenants.retention_mode + conversation_logs.detector/category) — must exist before behavior; runner already in `bootstrap`.
- No other new structure; entity/repo/middleware patterns already exist.

## Architecture Impact

- Behavior change to conversation-logging: capture from "masked/blocked only" to "all requests" in `full`/`meta` — increases volume; documented as intentional product direction (replaces T6.1 noise-reduction short-circuit).
- Local only; no external integrations. Rollout: additive nullable columns + default `full`; no migration for existing rows.

## Acceptance Approach

- AC-001 (default full) → no gate change default path; test: tenant without override shows content rows. Surfaces: middleware, worker.
- AC-002 (meta metadata-only) → metadata-only RawRecord + relaxed constructor; test: meta row empty content via `Get`. Surfaces: middleware, conversation_log.go, worker.
- AC-003 (none no rows) → capture returns before enqueue; test: empty list + counters increase. Surfaces: middleware, session/analytics untouched.
- AC-004 (blocked anonymized) → shield block context → RawRecord.Detector/Category → columns; test: blocked meta row has facts, no content. Surfaces: shield.go, worker, pg store.
- AC-005 (per-tenant) → mode read from tenant-in-context; test: alice full vs bob none with identical traffic differ. Surfaces: resolver, middleware.
- AC-006 (global default) → effective mode = explicit tenant value else config default; test: config `none` + unset tenant behaves none. Surfaces: resolver, config.
- AC-007 (invalid rejected) → enum validation in config + tenant DTO; test: `redacted` rejected, mode unchanged. Surfaces: validator, tenant_handler.
- AC-008 (masked filter) → `ConversationFilter.Masked` + handler `masked=true/false` + pg WHERE; test: row counts match. Surfaces: storage, handler, pg store.
- AC-009 (UI filter) → select in filter bar, page re-requests with `masked`; test: rendered rows match. Surfaces: Conversations.tsx, conversations.ts.
- AC-010 (UI mode) → control on Tenant pages, persists via tenant API; test: mode round-trips, none tenant shows zero rows. Surfaces: TenantDetail/Form, tenants.ts.

## Data & Contracts

- Data model changes (see data-model.md): `tenants.retention_mode` (default `full`), `conversation_logs.detector`/`category` (nullable). No API-contract breakage: additive optional fields.
- Validation rule `full|meta|none` shared by config and tenant DTO.

## Implementation Strategy

- DEC-001 Retention mode lives on the Tenant entity (shield/entity) with `WithTenantRetentionMode` + persisted via PostgresTenantRepo + admin DTO.
  Why: matches existing per-tenant option pattern (`WithTenantPIIConfig`) and keeps one source of truth; not a separate settings table.
  Tradeoff: entity + repo + migration surface across three layers for one field.
  Affects: shield/entity, shield/repository, pg tenant repo, admin handler/DTO, migration.
  Validation: tenant create/update round-trip via API; unit test on entity option.
- DEC-002 Effective mode = explicit tenant value, else global `data.retention.mode` default, resolved in the tenant resolver.
  Why: single resolution point the capture path can trust; avoids spreading config lookups into middleware.
  Tradeoff: resolver must know the config default at construction.
  Affects: tenant_resolver, config.
  Validation: AC-006 test (`config none + unset tenant`).
- DEC-003 Gating at the capture boundary (middleware) so in `none` no RawRecord is ever produced and in `meta` only metadata (+ detector/category) is.
  Why: content never leaves middleware memory; worker/persistence stay unchanged for `full`.
  Tradeoff: middleware needs the resolved mode; a hot-path branch.
  Affects: middleware/conversation.go, worker (pass-through only).
  Validation: AC-003/AC-002/AC-004 tests.
- DEC-004 Blocked-anomaly facts = detector type + category, set on context in shield middleware on block and persisted in two nullable columns.
  Why: satisfies "nothing more"; detaches from fragment text which must never be stored.
  Tradeoff: two new columns; facts require shield middleware cooperation.
  Affects: shield.go, conversation_log.go, pg store, migration.
  Validation: AC-004 test.
- DEC-005 Lift the masked/blocked-only capture short-circuit to log all chat requests in `full`/`meta`.
  Why: the masked/not-masked filter (AC-008/009) is meaningless when non-masked clean requests are never logged; matches explicit product requirement "log all requests".
  Tradeoff: increased write volume/storage; supersedes the earlier T6.1 noise-reduction decision.
  Affects: middleware/conversation.go.
  Validation: AC-008 test (both populations present in list).
- DEC-006 Masked filter as a nullable tri-state (`*bool`), not a defaulted bool.
  Why: empty/missing param must preserve current unfiltered behavior (AC-008); a non-nil pointer distinguishes "filter by true/false" from "no filter".
  Tradeoff: slight ergonomics cost in handler/store.
  Affects: ConversationFilter, handler, pg store, UI select.
  Validation: AC-008/AC-009 tests.

## Incremental Delivery

### MVP
- Migration + tenant field/DTO/repo + config default + middleware `none` gate + resolver effective-mode. Closes AC-001, AC-003, AC-006, AC-007, AC-005.
- Readiness: scripted validation path above.

### Iterative Expansion
- `meta` mode (AC-002, AC-004): metadata-only record + shield facts.
- Masked filter + UI (AC-008, AC-009): filter field → handler → store → Conversations select; lift capture short-circuit (pairs with AC-008).
- UI mode control (AC-010): Tenant pages + tenants.ts.

## Implementation Order

- Migration first (additive columns), then config default + entity/repo, then middleware gate + shield facts, then filter field → handler → store, then UI. Parallel-safe: config + filter surfaces are independent of middleware gate; UI depends on API.

## Risks

- Volume increase from "log all" in `full` → Mitigation: reuse 1MB cap; note for ops (storage); `none`/`meta` unaffected.
- Relaxing `NewConversationLog` may allow accidentally-empty content in `full` → Mitigation: keep constructor strict for `full` via explicit metadata-only constructor/variant used only by `meta`.
- Facts correctness (detector/category) depends on shield middleware setting context → Mitigation: DEFERRED context-set only on block; unit tests on shield block path + capture test.
- Config/tenant enum drift → Mitigation: single shared validation constant set; AC-007 test.

## Rollout & Compatibility

- Additive migration (nullable columns, default `full`) — backward compatible; no data backfill. Existing tenants behave as `full`.
- Operations note: after rollout, `full` tenants see more request rows (all requests); masked filter in UI documents this.
- No feature flag needed; behavior gated per-tenant with safe default `full`.

## Verification

- Automated: middleware capture tests (none/meta/full/blocked-facts), pg store masked-filter test, tenant DTO round-trip, config validation, resolver effective-mode, shield block-context test.
- Manual: scripted validation path; `curl` masked=true/false list; UI filter select renders matching rows.
- Confirms: AC-001..AC-010; DEC-001..DEC-006.

## Constitution Compliance

- No conflicts. Per-tenant settings stored in PostgreSQL (retention_mode column) aligns; UI is admin/tenant management + request logs (allowed); docs stay en.