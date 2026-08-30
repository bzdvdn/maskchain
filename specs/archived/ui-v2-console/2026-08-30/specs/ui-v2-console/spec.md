# UI v2 Console

## Scope Snapshot

- In scope: rebuild the MaskChain admin UI into a product-grade, dark-first operator console (LiteLLM-class) — new design system, information architecture, and full-page redesigns, consuming existing data plane APIs.
- Out of scope: new backend features, analytics/telemetry pipelines, auth/SSO, rewriting the existing UI pages that are not covered by the v2 slices.

## Goal

Platform operators need an admin console that lets them monitor, understand, and act on LLM traffic at a glance. Today the UI reads as a functional admin panel: flat navigation, static header, raw counters without deltas, hardcoded values, and no guidance for first-run users. UI v2 rewrites the React surface around five principles — each page answers *what is happening now, what changed, needs-attention*; operator persona over end-user persona; one source of truth (data comes from the API, never hardcoded); a coherent status system; consistent primitives instead of ad-hoc inline styling. Success is visible when an operator can triage budget/health/compliance anomalies from the Dashboard, issue a key with model/budget constraints in a few actions, and trust that every number on screen reflects live state.

## Primary User Flow

1. Starting point: operator signs in and lands on **Operations HQ**, the default dashboard.
2. Main interaction: HQ surfaces KPI deltas, a trend chart with a metric toggle, and a "Needs attention" panel fed by real signals (budget alerts, provider health, incidents, expiring keys); the operator drills into Analytics, Routing, or Tenant profiles.
3. Outcome: any anomaly or degradation is visible within one screen and actionable with one click (create key, re-apply compliance pack, inspect provider fallback).
4. Failure/fallback path: during migration, legacy pages keep rendering on the current token set until they are migrated to v2 primitives; broken API calls surface inline error banners with retry rather than a blank page.

## User Stories

- P1 Story: "As an operator I want to see budget/provider/compliance anomalies without opening five screens, so I can react before a tenant is blocked."
- P2 Story: "As an operator I want to issue a tenant-scoped key with model and budget constraints and hand the tenant a curl snippet, so the tenant integrates in minutes."

## MVP Slice

Foundation (design tokens v2 + component primitives + `PageHeader`) **plus Dashboard HQ** **plus Keys page**. This slice delivers observable value first — the design system validates the migration path, Dashboard proves the "monitor/act" pattern, Keys proves the "product" pattern. MVP must satisfy `AC-001`, `AC-002`, `AC-003`, `AC-004`, `AC-005`, `AC-006`.

## First Deployable Outcome

After the first implement pass, serving `ui/` shows the v2 sidebar/header, Dashboard HQ with live KPIs and the Needs-attention panel, and the Keys page with endpoint quickstart and create-key drawer — while Analytics, Routing, Tenants, Settings and the remaining pages continue to render with the legacy styling until migrated. The feature is independently deployable because it is a pure frontend change over existing endpoints.

## Scope

- `ui/src/styles/` → design tokens v2 (canvas/surface/border/text layers, semantic status colors, shape scale) and removal of legacy aliases on migrated pages.
- `ui/src/components/ui/` → v2 primitives: `Card`, `PageHeader`, `StatusPill`/`StatusDot`, `Segmented`, `ChipTag` (+ tag-input), `Switch`, `ProgressBar`, `Spinner`/`Skeleton`, `EmptyState` with CTA.
- `ui/src/components/Layout.tsx` → new IA (nav regrouped by job-to-be-done), header with contextual actions and workspace/tenant switcher, live-status indicator.
- `ui/src/pages/Dashboard.tsx` → Operations HQ (KPI + delta + sparkline, trend chart with segmented metric toggle, Needs-attention panel, quick actions, activity feed).
- `ui/src/pages/Keys.tsx` → endpoint quickstart, toolbar with search/filters, table with model chips + spend progress + status switch, create-key drawer, one-time-key reveal, revoke confirm.
- `ui/src/pages/Analytics.tsx`, `Routing.tsx`, `Tenants/TenantDetail.tsx`, `Settings.tsx` → v2 page designs; `Settings` rebinds to live data (no hardcoded values).
- Onboarding/empty states for first-run Dashboard and Keys.
- `src/internal/api/` → minimal read-only server/status endpoint (uptime, version, key-at-rest config) as the sole backend addition powering Settings (RQ-009); no other backend surface.
- Shared formatting utilities (`relative time`, locale-neutral dates, money/token format) replacing ad-hoc `toLocaleString('ru-RU')`.
- Constitution follow-up: the "React UI is only for tenant management and logs" line is aligned to the control-plane console decision via a constitution-phase change.

## Context

- Existing UI already spans tenant/log management; constitution line "React UI is only for tenant management and logs" predates the current admin surface. Product decision: the UI is the operator **control-plane console**; the constitution line is aligned via a constitution-phase change (tracked as a follow-up of this feature).
- Existing endpoints supply the data: `/api/v1/analytics/*` (tokens/cost/series, CSV export), `/api/v1/keys/*`, `/api/v1/budgets/*`, `/api/v1/tenants/*` (incl. compliance pack apply/report), `/api/v1/sessions`, `/api/v1/incidents/*`, health service aggregation, config serialization/diff watcher.
- The existing UI already ships dark/light theme tokens, lazy-loaded routes, `ErrorBoundary`/`Suspense`, `useSort`, `ConfirmModal`, `CommandPalette`, `Toast` — v2 reuses these foundations and replaces visual/inline-style inconsistencies.
- Prototypes in `.new-design/` (approved by the product owner) are the visual reference; the spec does not restate every pixel, only the behaviors and invariants.
- Assumption: migration is incremental — legacy pages remain functional until migrated; no page is left in a broken intermediate state at deploy time.

## Dependencies

- Existing analytics/cost/series aggregation and CSV export endpoints (`/api/v1/analytics/...`) — no new pipelines.
- Existing health/status aggregation (`src/internal/api/health/`) for Settings and Routing liveness.
- Existing budget `RecordSpend`/alert signals for the Needs-attention panel (budget threshold + hard-limit `429`).
- Virtual-key and tenant APIs incl. one-time key reveal and compliance pack apply/report.
- Library dependency already present: `react-router-dom@7`, `recharts`, `lucide-react` — no new runtime dependencies anticipated for the MVP; visual primitives are CSS-based.

## Requirements

- RQ-001 The UI MUST provide a coherent dark-first design system (canvas/surface/border/text layers, semantic status colors, radius/shadow scale) behind reusable primitives; migrated pages MUST NOT carry ad-hoc inline styles or legacy alias classes.
- RQ-002 The layout MUST regroup navigation by job-to-be-done (Overview / Traffic / Governance / Operations / System) and MUST render page-relevant actions (Create key, Export, Apply pack) in the header contextually.
- RQ-003 The header MUST expose a workspace/tenant scope switcher that scopes Dashboard, Analytics, and Budgets views to the selected workspace's **actual data** (real filtering of the underlying queries, not a presentational overlay); the mechanism (tenant query parameter vs client-side partition of tenant-typed responses) is chosen in plan.
- RQ-004 Operations HQ MUST render KPI cards (spend, requests, tokens, pass rate) with a delta vs the previous period and a sparkline, a trend chart with a Tokens/Cost/Requests metric toggle, and a Needs-attention panel driven by live signals (budget warn/exhaust, provider down, incidents, expiring keys, compliance deviation), each actionable.
- RQ-005 The Keys page MUST offer an endpoint quickstart (base URL + copy + curl), a create-key flow with tenant, label, model allow/deny tag-input chips, budget/limit presets and expiry presets, a table with model chips, spend/budget progress and an enable/disable switch, plus search/filter toolbar and one-time-reveal on create.
- RQ-006 Analytics MUST persist the selected time range, offer a "vs previous period" compare toggle with delta badges, and provide day/model/tenant breakdown tabs over the existing aggregation endpoints.
- RQ-007 Routing MUST present providers as health cards (status dot, p50 latency, 24h ok-rate, routes, active fallback chain) plus a routing-rules table, driven by the existing health/routing state.
- RQ-008 The Tenant profile MUST use tabs (Overview / Policies & Shield / Compliance / Activity), a compliance-deviation banner with Review-diff and Re-apply actions when the report detects deviation.
- RQ-009 Settings MUST render live system status (database/valkey/key-at-rest/TLS/telemetry health, server info) from the API and MUST render config-vs-live diff with apply; to power this it MAY use health aggregation plus one minimal read-only server/status endpoint (uptime, version, key-at-rest config) allowed as the sole backend addition. It MUST NOT contain hardcoded display values.
- RQ-010 All timestamps MUST render as locale-neutral relative time ("2m ago") with tooltip absolute value; all numeric/metric formatting MUST use shared formatters (money, `1.2K`/`1.2M` tokens) — no locale-parameterized ad-hoc formatting.
- RQ-011 First-run empty states (no keys, no traffic) MUST render an onboarding guidance card with a connect-your-app curl snippet instead of a bare empty table.

## Non-Goals

- No backend/domain feature work: no new analytics pipelines, no new key/budget storage, no health-check changes. The single exception is the minimal read-only server/status endpoint required by RQ-009/SC-002.
- No authentication/SSO/MFA/role-scoped UI; login stays as today.
- Light theme parity is not redesigned in v2 (tokens preserve the existing light mappings; visual polish of light mode is deferred).
- No charting engine replacement or custom SVG chart library authoring beyond Recharts composition.
- No playground/chat UI for end users.
- No visual polish pass on legacy pages outside their migration to v2 primitives.

## Acceptance Criteria

### AC-001 Design tokens v2 and primitives are the shared baseline

- Why this matters: one visual language so future pages converge instead of diverging.
- **Given** the v2 token set and primitives are merged into `ui/src/styles/` and `ui/src/components/ui/`
- **When** a v2-migrated page is opened alongside a legacy page
- **Then** migrated pages render from the shared token/primitive set and contain no inline `style=` props nor legacy alias classes; a reviewer can flag any violation from the diff.
- Evidence: lint/grep pass showing no `className` legacy aliases or inline styles on migrated pages; visual parity across migrated pages.

### AC-002 Navigation and header regroup by job-to-be-done with contextual actions

- Why this matters: operators reach the right surface with fewer stops.
- **Given** a logged-in operator on any v2 page
- **When** the sidebar and header render
- **Then** nav is grouped (Overview/Traffic/Governance/Operations/System) and the header shows page-specific primary actions (e.g., "Create key" on Keys, "Export" on Analytics) plus the workspace switcher.
- Evidence: screenshot/manual walk through all routes; header action set differs per route per the spec.

### AC-003 Operations HQ surfaces KPI deltas and a Needs-attention panel

- Why this matters: anomalies are visible before the operator opens per-page screens.
- **Given** a tenant with a budget above the warn threshold and a provider reported down by health checks
- **When** the operator opens Operations HQ
- **Then** the spend KPI shows `86%`-style usage with a delta vs the previous period, and the Needs-attention panel lists the budget-warn and provider-down items with one-click drill targets.
- Evidence: component/unit test asserting panel items derive from the mocked budget/health/incident payloads; manual check with seeded data.

### AC-004 Trend chart toggles metric without losing the time range

- Why this matters: analysts compare tokens, cost and volume in one view.
- **Given** a selected time range on Operations HQ
- **When** the operator switches the segmented control from Tokens to Cost
- **Then** the chart redraws using the corresponding `/api/v1/analytics/series` payload while the range and compare delta remain unchanged.
- Evidence: test capturing that switching the metric issues a request with the same from/to and renders the new series; no full-page reload.

### AC-005 Keys page provides endpoint quickstart and a constrained create flow

- Why this matters: tenants self-serve integration; operators constrain by model and budget.
- **Given** the operator opens Keys
- **When** the page renders and the operator submits a valid create-key form with tenant, label, model chips, budget and expiry
- **Then** an endpoint card shows the base URL with copy and a curl snippet; the drawer validates inputs and after creation reveals the key exactly once with a copy action.
- Evidence: e2e/manual flow creating a key and observing the one-time reveal; invalid submit shows inline field errors without losing the drawer state.

### AC-006 Keys table supports search, filters and spend context

- Why this matters: operators find and reason about keys at scale.
- **Given** a populated virtual-keys list
- **When** the operator types in search, applies tenant/status filters, or inspects a row
- **Then** rows narrow accordingly, and each row shows model chips, a budget/spend progress bar with threshold coloring (warn/danger at 100%), enable/disable switch and copy/revoke actions.
- Evidence: unit test on filter logic and rendering of threshold-colored progress; manual search/filter walkthrough.

### AC-007 Analytics persists range and compares periods

- Why this matters: before/after context makes cost/token deltas meaningful.
- **Given** the operator sets 30d and enables "vs previous period"
- **When** the same user later returns to Analytics
- **Then** the range is restored and the period-over-period delta badges render next to KPIs; breakdown tabs (day/model/tenant) render from existing aggregation endpoints and CSV export honors the active range and filters.
- Evidence: test of range persistence (localStorage) and compare badge values from mocked records.

### AC-008 Routing renders provider health with fallback chains

- Why this matters: routing health is actionable at a glance.
- **Given** one healthy, one degraded, and one down provider
- **When** Routing renders
- **Then** each provider card shows the correct status dot/pill, p50 latency, 24h ok-rate, routes, and an active fallback chain where applicable, plus the rules table with switch toggles.
- Evidence: component test rendering the three states from mocked provider health payloads.

### AC-009 Tenant profile is tabbed and surfaces compliance deviation

- Why this matters: compliance drift is acted on in the tenant context.
- **Given** a tenant whose compliance report detects outdated rules
- **When** the tenant profile renders
- **Then** it shows Overview / Policies & Shield / Compliance / Activity tabs and a deviation banner with "Review diff" and "Re-apply pack" actions wired to the existing compliance APIs.
- Evidence: test rendering the banner from a deviating report and asserting the actions call `getComplianceReport`/`applyCompliancePack`.

### AC-010 Settings renders live state and config diff, with no hardcoded values

- Why this matters: operators trust the console only if numbers are real.
- **Given** the Settings page is mounted
- **When** it fetches health/status and config data
- **Then** system status cards (PostgreSQL/Valkey/key-at-rest/TLS/telemetry) and server info render values returned by the API/health aggregation, and the config section renders the live-vs-file diff with apply/discard controls.
- Evidence: Settings page shows zero hardcoded literals (lint/grep over migrated component); rendering is driven by mocked API payloads in tests.

### AC-011 Time and number formatting is consistent and locale-neutral

- Why this matters: shared formatting removes drift and locale bugs (e.g., current `toLocaleString('ru-RU')` in chart tooltips).
- **Given** a datetime or metric value encountered anywhere in migrated pages
- **When** the UI formats it
- **Then** it uses the shared formatters (relative time with absolute tooltip, tabular numbers, `$`/token scaling) and no component calls locale-parameterized `toLocaleString`.
- Evidence: grep over migrated pages for `toLocaleString` with a locale argument returns zero matches; format utility unit tests.

## Assumptions

- The `.new-design/` prototypes are the approved visual reference; the implementation may deviate only where data availability forces it, and such deviations are recorded in `plan`.
- Existing `/api/v1/*` endpoints provide sufficient data for every migrated view; where a field is missing (e.g., server uptime/version), the UI uses the closest existing data or the field is listed in Non-Goals until an endpoint exists.
- Migration keeps every legacy page rendering until its v2 pass lands; no deploy step transitions a page to a blank/broken state.
- Dark theme is the primary product theme; existing light mapping persists via tokens but is not redesigned.
- All new reusable UI ships through `ui/src/components/ui/` and all formatting through shared utilities — no page-local style blocks.

## Success Criteria

- SC-001 After the MVP slice, the top navigation and at least Dashboard + Keys render entirely from v2 tokens/primitives with zero inline `style=` props and zero legacy alias classes in those files.
- SC-002 Settings renders exclusively from API/live data: grep for hardcoded `8081`/`admin`/`postgres:5432`-style literals in the migrated Settings component returns zero matches.
- SC-003 Dashboard and Keys load as part of the existing lazy route setup within the current bundle budget (no new production runtime dependency for the MVP).

## Edge Cases

- Empty first-run: no tenants/keys/traffic → Dashboard and Keys render onboarding CTA cards, not blank tables.
- Budget at 100% or hard-limit `429`: progress renders danger color and the Needs-attention item offers the tenant/budget drill target.
- All providers down: Routing renders each card in `down` state and marks fallback chains inactive with a global degraded banner.
- Model chip overflow: long/`+N` chip truncation for selected model lists in keys and tenant profiles.
- No data for selected range: Analytics/HQ charts show a "No data for this period" empty state with range hints instead of an empty axis.
- Timezone/locale edge: relative-time and absolute labels stay locale-neutral and use the operator's browser timezone via the shared formatter.
- Key reveal at create: if the user closes the reveal before copying, the raw key is not discoverable again anywhere in the UI (matches existing server-only-once behavior).

## Open Questions

- ~~Constitution line~~ **Resolved**: the UI is the operator control-plane console; the constitution text is aligned via a constitution-phase change tracked as a follow-up of this feature.
- ~~Backend additions~~ **Resolved**: yes — one minimal read-only server/status endpoint (uptime, version, key-at-rest) is allowed for Settings (RQ-009); everything else stays frontend-only over existing endpoints.
- ~~Workspace switcher semantics~~ **Resolved**: real scoping of Dashboard/Analytics/Budgets to the selected workspace's actual data; mechanism chosen in plan (tenant param vs client-side partition of tenant-typed responses).
- ~~Light theme~~ **Resolved**: dark-first for the v2 pass; existing light token mappings preserved but not redesigned (matching the Assumptions below).