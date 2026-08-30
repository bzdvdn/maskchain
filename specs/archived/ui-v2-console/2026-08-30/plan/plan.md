# UI v2 Console — Plan

## Phase Contract

Inputs: `spec.md` (approved), `inspect.md` (pass), constitution summary, minimal repo context.
Outputs: `plan.md`; no `data-model.md` (no persistence change); no `contracts/*` files (additive API params only).
Stop if: any AC would require inventing behavior beyond the spec — none found.

## Goal

Rebuild the admin UI as the operator **control-plane console**: ship a v2 design system (CSS tokens + React primitives, no new runtime deps), regroup navigation and contextual header actions, and redesign each page to consume existing endpoints. The only backend touches are two small additive, backward-compatible surfaces: an optional `tenant` query param on analytics endpoints (real workspace scoping) and a read-only `/api/v1/admin/status` endpoint for Settings. MVP = foundation + Operations HQ + Keys (AC-001..006); then Analytics/Routing/Tenant/Settings (AC-007..011).

## MVP Slice

- Foundation (design tokens v2 + primitives + `PageHeader` + shared format utils) → Dashboard Operations HQ → Keys product page.
- Must satisfy `AC-001`, `AC-002`, `AC-003`, `AC-004`, `AC-005`, `AC-006` before expanding scope.

## First Validation Path

1. `ui` dev server (`npm run dev` in `ui/`) against a local stack; open Dashboard → KPI deltas + Needs-attention panel render from seeded/mock analytics, budget, health, incident payloads; header shows page actions and workspace switcher.
2. Keys page → endpoint quickstart with copy, create-key drawer validates and reveals the key once, table shows model chips + spend progress + switch; search/filter narrow rows.
3. Grep assertions: migrated pages contain no inline `style=` and no legacy alias classes; `toLocaleString` with a locale arg absent.

## Scope

- `ui/src/styles/tokens.css`, `base.css`, `components.css` → v2 token model + removal of legacy aliases on migrated pages only.
- `ui/src/components/ui/*` → new primitives: `Card`, `PageHeader`, `StatusPill`/`StatusDot`, `Segmented`, `ChipTag` (+ tag-input), `Switch`, `ProgressBar`, `Spinner`, `TableSkeleton`, `EmptyState` with CTA.
- `ui/src/components/Layout.tsx` → new IA (Overview/Traffic/Governance/Operations/System), contextual header actions, workspace switcher, live-status pill.
- `ui/src/utils/format.ts` (new) → relative time + locale-neutral dates + money/token scaling; adopted by all migrated pages.
- Pages: `Dashboard` (HQ), `Keys`, `Analytics`, `Routing`, `Tenants/TenantDetail`, `Settings`.
- Backend (small, additive): optional `tenant` param on `/api/v1/analytics/tokens|cost|timeseries`; new read-only `/api/v1/admin/status`.
- Untouched: domain/store schema (no persistence change), auth/login, light-theme redesign, chat/playground, chart engine.

## Performance Budget

- Frontend: keep lazy routing (`React.lazy`) — Dashboard and Keys load within the current route-level budget; no new production runtime dependency for the MVP.
- Analytics: tenant-scoped series must not regress the aggregate path — default `tenant` absent path stays byte-for-byte identical; scoping reuses existing indexed tenant query patterns.
- `none` for other latency/memory targets (admin console, out of hot path).

## Implementation Surfaces

- `ui/src/styles/*` (existing) — token v2 + cleanup on migrated pages.
- `ui/src/components/ui/*` (existing dir, extends) — new primitives.
- `ui/src/components/Layout.tsx`, `CommandPalette.tsx` (existing) — IA, header, switcher.
- `ui/src/pages/{Dashboard,Keys,Analytics,Routing,Settings}.tsx`, `ui/src/pages/Tenants/TenantDetail.tsx` (existing) — redesigns.
- `ui/src/api/analytics.ts` (existing) — optional `tenant` param forwarding.
- `ui/src/api/admin.ts` (existing) — `getSystemStatus` client for the new endpoint.
- `src/internal/api/handler/analytics/analytics_handler.go` (existing) — optional `tenant` param (records via `QueryByTenant`, series via new tenant-scoped store call).
- `src/internal/domain/analytics/` + store implementation (existing) — tenant-scoped timeseries; resolves the in-code `TODO` for tenant filtering.
- `src/internal/api/admin.go` + small new handler (existing router) — read-only `/api/v1/admin/status`.

## Bootstrapping Surfaces

- `ui/src/components/ui/` primitives and `ui/src/utils/format.ts` MUST land before any page redesign — pages depend on them (AC-001, AC-011).
- Analytics `tenant` param and tenant-scoped series MUST land before Dashboard/Analytics scoping work.
- Existing router/health/analytics structure is otherwise sufficient — no new top-level packages required.

## Architecture Impact

- Frontend: single shared visual/format layer replaces per-page inline styling; legacy pages keep rendering until migrated (incremental, no blank-page window).
- Backend: analytics handler gains an optional, backward-compatible filter; store gains a tenant-scoped series query that leaves the default path unchanged.
- One new additive admin endpoint; router wiring only (no middleware/data-model change), audited like the other admin handlers.

## Acceptance Approach

- AC-001 (tokens/primitives) → token v2 + primitives first; migrated pages use only them → proof: grep no inline `style=`/legacy aliases in migrated files; diff review.
- AC-002 (IA + header actions) → Layout rework + per-route action map → proof: route walkthrough; header actions differ per page per spec.
- AC-003 (HQ deltas + Needs-attention) → Dashboard KPI cards + attention panel over analytics/budget/health/incident APIs → proof: unit test on mocked payloads (budget warn + provider down render items).
- AC-004 (metric toggle) → segmented control re-issues `/analytics/timeseries` with same from/to → proof: test asserting same-range request on toggle; no reload.
- AC-005 (Keys quickstart + create) → endpoint card + drawer + one-time reveal; `CreateKeyResponse.key` already exists → proof: flow test; invalid submit keeps drawer state.
- AC-006 (search/filter/spend) → toolbar filters + `spent`/`budget_cap` progress from existing DTO → proof: filter unit test + threshold color test.
- AC-007 (Analytics persist/compare) → localStorage range + compare badges + breakdown tabs → proof: persistence unit test; compare values from mocked records.
- AC-008 (Routing health) → provider cards + rules table from existing routing API → proof: component test for three health states.
- AC-009 (Tenant tabs + deviation) → tabs + banner using existing `getComplianceReport`/`applyCompliancePack` → proof: banner render + action-call test.
- AC-010 (Settings live + diff) → health aggregation + new `/api/v1/admin/status` + config diff view → proof: zero-hardcode grep; render from mocked status payload.
- AC-011 (format utils) → shared formatters adopted everywhere → proof: util unit tests + grep for locale-parameterized `toLocaleString` = 0.

## Data and Contracts

- `Data model: no change` — no persistence/schema change for this feature.
- Analytics API: optional `tenant=<slug>` query param on `/api/v1/analytics/tokens`, `/cost`, `/timeseries` (absent = all tenants, current behavior). Backward compatible; response schema unchanged.
- New additive contract: `GET /api/v1/admin/status` (read-only) → version, uptime, key-at-rest configured flag, aggregated store health. No existing route modified.
- `contracts/api.md` not created — both API changes are small, additive, and documented in this plan's DEC-003/DEC-004.

## Implementation Strategy

- DEC-001 Token v2 + CSS primitives, no UI framework
  Why: reuses the existing `prefers-color-scheme`/`data-theme` token mechanism, zero new runtime dependency; prototype validated in `.new-design/`.
  Tradeoff: hand-rolled primitives require our own a11y discipline.
  Affects: `ui/src/styles/*`, `ui/src/components/ui/*`.
  Validation: vitest primitives; grep no inline styles on migrated pages.
- DEC-002 Keep Recharts for all charts
  Why: already in `dependencies`; sufficient for HQ trend, Analytics bars, status bars.
  Not: custom SVG chart lib or d3 (new dependency surface, re-charting all views).
  Affects: `Dashboard`, `Analytics`, `TimeSeriesChart`.
  Validation: existing charts render in v2 pages; bundle unchanged.
- DEC-003 Real workspace scoping → optional `tenant` param on analytics endpoints
  Why: decision W2 = real data scoping; timeseries is pre-aggregated across tenants, so client-side partition is impossible; `fetchRecords` already has a tenant path, and the store has a `TODO` for tenant-scoped series — this feature realizes it.
  Not: visual-only filter (rejected), client-side partition (infeasible for series).
  Tradeoff: small backend touch + one store query variant; default path unchanged.
  Affects: `analytics_handler.go`, analytics store, `analytics.ts`.
  Validation: handler unit tests for scoped + unscoped; Dashboard/Analytics scoping check.
- DEC-004 Read-only `/api/v1/admin/status` for Settings
  Why: health aggregation covers stores/TLS but not version/uptime/key-at-rest; decision W3 permits it; Settings must be hardcode-free (AC-010).
  Not: extending config-editing endpoints in this feature.
  Affects: `src/internal/api/admin.go`, new small handler, `ui/src/api/admin.ts`, `Settings`.
  Validation: handler unit test; Settings grep for stale literals = 0.
- DEC-005 Incremental page migration, legacy untouched until their pass
  Why: every deploy keeps all 11 routes rendering; reduces blast radius and review surface.
  Not: big-bang rewrite of all pages.
  Affects: all current pages.
  Validation: CI green at each step; manual walkthrough shows no blank/broken page.
- DEC-006 Single format module (`ui/src/utils/format.ts`)
  Why: AC-011; removes the current `toLocaleString('ru-RU')` chart-tooltip defect in one place.
  Affects: `ui/src/utils/`, migrated pages.
  Validation: formatter unit tests; grep for locale-arg `toLocaleString` in migrated pages = 0.

## Incremental Delivery

### MVP (First Value)

- Tasks: tokens v2 + primitives + format utils → Layout/IA/header → Dashboard HQ → Keys page. Backend: analytics `tenant` param (needed by HQ scoping).
- MVP readiness: AC-001..006 pass; validation path from "First Validation Path".
- Proof per task via `Proof:` lines in `tasks.md` (code/test) and greps (`parse`/`no-hardcode`).

### Iterative Expansion

- Step 2 (Analytics): persist range + compare + breakdown → AC-007; proof: tests + visual compare badges.
- Step 3 (Routing + Tenant): provider health cards + rules table (AC-008), tenant tabs + deviation banner (AC-009), tenant-scoped analytics to power scoping across pages.
- Step 4 (Settings): read-only status endpoint + live Settings + config diff (AC-010).
- Step 5 (Polish): formatter adoption everywhere + onboarding empty states (AC-011) + final grep audit.

## Sequencing Notes

- Primitives + format utils first (AC-001, AC-011 depend on them).
- Analytics `tenant` param before Dashboard/Analytics scoping; default path must stay identical.
- Settings status endpoint independent and parallelizable with Analytics/Routing work.
- Keys page reuses the existing create-response reveal — no backend sequencing constraint.
- Migration per page (DEC-005) means routing/tenant/settings passes can be picked up in any order after MVP.

## Risks

- Backend surface growth (analytics filter, status endpoint)
  Mitigation: additive + optional params, existing tests kept green, new handler/store unit tests; default paths unchanged.
- Series aggregation perf with tenant filter (TODO in code)
  Mitigation: reuse existing per-tenant query/indexes; scoped path exercised only when a tenant param is present; unscoped default untouched.
- Migration drift — legacy pages keep old styles next to v2
  Mitigation: DEC-005 incremental + per-page grep gate (no inline styles/aliases in migrated files) enforced in tasks proof.
- Workspace switcher semantics leaks into other endpoints if context-based
  Mitigation: scoping uses an explicit `tenant` query param, not ambient context, so no cross-request state leakage.

## Rollout and Compatibility

- No DB migration or feature flag needed. Analytics `tenant` param and `/api/v1/admin/status` are additive; old clients unaffected.
- Contributing pages change visually only when their migration lands; each landing is independently reviewable.
- Constitution alignment ("React UI = control-plane console") is a tracked follow-up via `/spk.constitution`; carry it in the archive checklist — non-blocking for this feature's rollout.

## Validation

- Unit: primitives + formatters (vitest); Dashboard attention-panel mapping; Keys create/filter; Analytics persistence/compare; Routing states; Tenant banner; Settings status render; analytics handler scoped/unscoped; status endpoint handler (Go `go test`).
- Grep gates: no inline `style=`/legacy aliases on migrated pages; no locale-arg `toLocaleString`; no hardcoded Settings literals.
- Manual: validation path in "First Validation Path"; walk through all 11 routes confirming none break.
- Proves: AC-001..AC-011, DEC-001..DEC-006.

## Constitution Compliance

- Non-conflict on structure/language/DoD. One divergence surfaced in spec + inspect: constitution line "React UI is only for tenant management and logs" vs the control-plane console decision — recorded as a resolved Open Question (OQ-1) and scheduled as a `/spk.constitution` follow-up in the archive checklist; does not block planning/implementation of this feature.