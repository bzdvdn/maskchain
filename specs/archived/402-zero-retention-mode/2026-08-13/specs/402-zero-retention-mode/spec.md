# Zero-Retention Mode (per-tenant `data.retention`)

## Scope Snapshot

- In scope: per-tenant retention mode `full|meta|none` that gates whether prompt/response/masking content is persisted to the conversation log, with an optional global `data.retention.mode` default for tenants that do not set one.
- Out of scope: retroactive purge of already-stored records when a tenant downgrades its mode, and content encryption/key rotation.

## Goal

Regulated customers ("don't store prompts at all") get a per-tenant switch: operators set a tenant's retention mode, and the gateway guarantees that in `none` mode no conversation log rows are written at all (only aggregated counters remain), while `meta` keeps metadata-only rows with blocked anomalies recorded as anonymized detector/category facts. Success is visible when a tenant in `none` mode shows zero conversation log rows over a full traffic window while its session/analytics counters keep incrementing.

## Main Scenario

1. Operator creates a tenant via the admin API; the tenant's retention mode is `full` by default (no behavior change to today).
2. Operator sets the tenant to `meta` or `none` (per-tenant field on create/update, or a global `data.retention.mode` default).
3. In `meta`: the conversation log is written as today (id, tenant, model, status, counters, timestamps) but the request, response, and masking payloads are stored empty; a blocked anomaly stores only the detector + category that fired.
4. In `none`: no conversation log row is created; session and analytics counters (tokens, masks, redirects) keep accumulating.
5. Fallback: an invalid mode value (anything outside `full|meta|none`) is rejected at validation time, and the tenant keeps its previous mode.

## User Stories

- P1 Story (MVP): an operator flips a tenant to `none` and observes, end-to-end, that prompts/responses are not persisted anywhere while counters continue.
- P2 Story: an operator flips a tenant to `meta` and still gets metadata-only traces (status, counts, detector/category on blocked anomalies) without content.

## MVP Slice

The smallest independently valuable slice is the `none` mode + global default config: no content rows written, counters intact, mode exposed through the admin tenant API. The slice must close AC-001, AC-003, AC-005, AC-006 first. `meta` mode (AC-002, AC-004), the masked/not-masked filter (AC-008, AC-009), and the UI mode control (AC-010) are the second slice in the same feature.

## First Deployable Outcome

After the first implementation pass an operator can: set a tenant to `none` (or a global default), run traffic through that tenant, and verify via the Conversations page that no rows appear for it while session/analytics counters grow. The retention-mode control and the masked/not-masked filter are part of the same UI (Tenant page + Conversations page).

## Scope

- A `retention` mode field on the Tenant entity plus create/update plumbing through the admin API (request DTO, response DTO, entity option).
- Global default `data.retention.mode` in config (`default: full`), applied when a tenant does not set its own mode; validated against the enum.
- Gating in the conversation capture path (middleware → async worker) so content is not captured/persisted for `meta`, and no row is captured for `none`.
- Metadata-only rows carry the same non-content fields as today (id, tenant, model, status, masked, streamed, timestamps, counters).
- Blocked anomalies in `meta` are represented only by detector + category; in `none` a blocked anomaly leaves no record (only aggregate counters); the existing session/analytics aggregated counters are untouched.
- A `masked` (true/false) filter on the conversation log list query, available in every retention mode (the `Masked` field already exists on rows).
- UI: retention-mode control on the Tenant page, and a masked/not-masked filter on the existing Conversations request-log page (the LiteLLM-style request log already shows id/tenant/model/status/masked/streamed/created).

## Context

- Today `ConversationMiddleware` captures the exchange, ships a `RawRecord` to `AsyncWorker.encrypt`, which encrypts request/response/masking and persists via `ConversationStore.SaveBatch`. Retention gating belongs at the capture boundary so nothing sensitive is ever handed to the worker in `none`.
- `ConversationLog` currently requires a non-empty request and holds encrypted `Request`/`Response`/`Masking`; `meta` needs that invariant relaxed or a metadata-only variant so a row can exist without content.
- Tenant per-tenant config already follows the entity-option pattern (`WithTenantPIIConfig`); a `WithTenantRetentionMode` option is the consistent extension.
- Global config sections (e.g. `ConversationsConfig`, `DataConfig.Cache`) use `mapstructure`/`yaml` tags and are validated in `validator.go`; `data.retention` follows the same pattern.
- Sessions already carry only counters (no content), so they remain valid in all modes.
- The admin UI already has a LiteLLM-style request-log page (`Conversations.tsx`) listing id/tenant/model/status/masked/streamed/created with tenant/status/model filters and a content detail view — the masked filter slots into the same filter bar and tenant page hosts the mode control.

## Dependencies

- `src/internal/api/middleware/conversation.go` (capture path) and `src/internal/app/conversation/async_worker.go` (encrypt/persist).
- `src/internal/domain/tenant` / `shield/entity` for the mode field, and `src/internal/infra/config` for `data.retention.mode`.
- `src/internal/domain/conversation` for the metadata-only row representation and the `Masked` list filter field (extending `ConversationFilter`).
- `src/internal/api/handler/conversation` for the list handler wiring of the `masked` query param.
- `ui/src/pages/Conversations.tsx` for the masked/not-masked filter UI and `ui/src/api/conversations.ts` for the filter field.
- `ui/src/pages/Tenants/` (TenantDetail/TenantForm) for the retention-mode control, wired to the tenant DTO.
- `none` — no new external service or library.

## Requirements

- RQ-001 The system MUST keep `full` as the default retention mode, so existing tenants behave exactly as today unless explicitly reconfigured.
- RQ-002 The Tenant entity MUST carry a retention mode (`full|meta|none`) configurable through the admin tenant create/update API and returned in the tenant response.
- RQ-003 The global config MUST support `data.retention.mode` with values `full|meta|none` (default `full`) applied to tenants that do not set their own mode.
- RQ-004 In `meta` mode the system MUST persist conversation log rows without request, response, or masking content.
- RQ-005 In `meta` mode a blocked anomaly MUST be recorded only with the firing detector and category (no prompt text or other payload facts).
- RQ-006 In `none` mode the system MUST NOT create conversation log rows; session and analytics aggregated counters MUST continue to accumulate.
- RQ-007 The config and tenant field MUST reject any value outside `full|meta|none` at validation time.
- RQ-008 The conversation log list MUST support filtering by `masked` (`true`/`false`), so operators can split masked vs not-masked requests in any retention mode; the filter MUST apply to the metadata-only rows of `meta` mode as well.

## Out of Scope

- Retroactive deletion/purge of conversation log rows when a tenant's mode is downgraded from `full`.
- Content encryption key rotation or changes to the existing encryption pipeline.
- Applying retention at the analytics/session layer (aggregates are explicitly kept in all modes).
- UI for a tenant's retention-mode control is in scope; richer log analytics (cost, latency graphs) on the Conversations page is not.

## Acceptance Criteria

### AC-001 Default is full

Given a tenant created without a retention override, When traffic flows through it with conversation logging enabled, Then request/response/masking are encrypted and persisted exactly as today (observable: the existing conversation list shows rows with content for that tenant).

### AC-002 Meta persists metadata only

Given a tenant with mode `meta`, When a prompt passes through it, Then a conversation log row is created with tenant/model/status/counters/timestamps but empty request, response, and masking fields (observable: `List` shows the row; reading it yields no content).

### AC-003 None writes no rows

Given a tenant with mode `none`, When traffic flows through it over a full window, Then zero conversation log rows exist for that tenant while its session/analytics counters increment (observable: conversation list is empty for the tenant, counters increase).

### AC-004 Blocked anomaly is anonymized (meta)

Given a tenant with mode `meta`, When a request is blocked by a shield rule, Then only the blocking detector and category are recorded and no prompt text, masking mapping, or payload appears (observable: the record contains detector/category fields and no content). In `none` mode such an anomaly leaves no record at all (see AC-003).

### AC-005 Mode is per-tenant

Given two tenants `alice` (full) and `bob` (none), When both process identical traffic, Then `alice` accumulates conversation rows and `bob` does not (observable: per-tenant filtering in the conversation list).

### AC-006 Global default applies

Given `data.retention.mode: none` at config level and a tenant created without an explicit mode, When that tenant processes traffic, Then it behaves as `none` (observable: no conversation rows for the tenant).

### AC-007 Invalid mode rejected

Given an operator sends a tenant update with retention mode `redacted`, When validation runs, Then the update is rejected and the tenant retains its previous mode (observable: API error, mode unchanged in subsequent GET).

### AC-008 Masked/not-masked filter

Given a conversation list that contains both masked and not-masked requests for a tenant (in any retention mode), When the operator queries the list with `masked=true` and `masked=false`, Then each query returns only matching rows (observable: row counts differ and every row's masked flag matches the filter; empty/missing filter keeps today's unfiltered behavior).

### AC-009 UI shows the request log with the masked filter

Given the admin UI Conversations page, When the operator selects "Masked" / "Not masked" in the filter bar, Then only matching requests are shown in the table and the page reflects the filter in its request (observable: the page re-requests with `masked=<value>` and the rendered rows all match).

### AC-010 UI exposes the tenant retention mode

Given a tenant in the admin UI, When the operator sets its retention mode to `meta` or `none` and saves, Then the tenant detail shows the saved mode and subsequent traffic obeys it (observable: the mode persists in the tenant response; a `none` tenant shows zero rows in the Conversations page).

## Assumptions

- A tenant's mode is evaluated at capture time using the tenant currently in memory; config/API changes are picked up on the tenant reload cycle like other tenant settings.
- Existing stored records are left in place when a tenant downgrades from `full` (purge is explicitly out of scope).
- `meta` metadata-only rows keep the same ID/status semantics so existing consumers (list, export, cleanup) keep working.
- The masked placeholder→original mapping in `meta`/`none` is never needed because content is not stored.

## Success Criteria

- SC-001 In `none` mode the conversation write path emits zero database rows for the tenant (measurable: table row count is flat over the window).
- SC-002 Retention gating adds no observable latency to the hot path beyond the existing tenant lookup (measurable: p99 proxy latency delta < 100 ms or within ±5% of the pre-change baseline).

## Edge Cases

- Tenant with no explicit mode and no global default → `full` (existing behavior).
- Response body absent (e.g. upstream error) in `meta` → row created with empty content, consistent with today's empty-response handling.
- In `none`, even a blocked anomaly leaves no conversation record (only aggregate counters).
- Mode changed mid-window → rows take the mode in effect at capture time.
- Worker buffer full in `none` → no rows to drop; counters unaffected.

## Open Questions

- Should `meta` rows be further normalized (e.g. separate anonymized-anomaly table) if reporting grows — deferred, not needed for this feature.
- ~~Whether the Conversations page needs additional LiteLLM-style columns~~ — resolved by product: columns stay exactly `id/tenant/model/status/masked/streamed/created` (the current set); no latency/token columns are added.