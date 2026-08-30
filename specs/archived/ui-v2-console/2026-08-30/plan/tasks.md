# UI v2 Console — Tasks

## Phase Contract

Inputs: `plan.md` (approved), `spec.md`, constitution summary.
Outputs: ordered tasks with `Touches:` per task, Surface Map, Implementation Context, AC coverage.
Stop if: any AC cannot be mapped to executable work — none; all 11 covered.

## Surface Map

| Surface | Tasks |
|---------|-------|
| ui/src/styles/tokens.css | T1.1 |
| ui/src/components/ui/*, ui/src/components/ui/index.ts | T1.2, T1.4 |
| ui/src/utils/format.ts, ui/src/utils/format.test.ts | T1.3, T1.4 |
| src/internal/api/dto/analytics.go | T2.1 |
| src/internal/domain/analytics/usage_store.go | T2.1 |
| src/internal/adapters/repository/analytics/pg_usage_store.go | T2.1 |
| src/internal/api/handler/analytics/analytics_handler.go | T2.1 |
| src/internal/api/handler/analytics/analytics_handler_test.go | T2.1 |
| src/internal/api/handler/admin/status_handler.go (new) | T2.2 |
| src/internal/api/handler/admin/status_handler_test.go (new) | T2.2 |
| src/internal/api/admin.go | T2.2 |
| ui/src/components/Layout.tsx | T3.1 |
| ui/src/pages/Dashboard.tsx | T3.2, T5.1 |
| ui/src/api/analytics.ts | T3.2, T4.1 |
| ui/src/pages/Keys.tsx | T3.3, T5.1 |
| ui/src/api/keys.ts | T3.3 |
| ui/src/components/TimeRangePicker.tsx, TimeSeriesChart.tsx | T4.1 |
| ui/src/pages/Analytics.tsx | T4.1, T5.1 |
| ui/src/pages/Routing.tsx | T4.2, T5.1 |
| ui/src/api/routing.ts | T4.2 |
| ui/src/pages/Tenants/TenantDetail.tsx | T4.3, T5.1 |
| ui/src/api/tenants.ts | T4.3 |
| ui/src/pages/Settings.tsx | T4.4, T5.1 |
| ui/src/api/admin.ts | T4.4 |
| ui/src/pages/{Dashboard,Keys,Analytics,Routing,Tenants/TenantDetail,Settings}.test.tsx | T3.4, T4.5 |
| repo-wide (ui/ greps) | T5.2 |

## Implementation Context

- MVP Goal: land tokens v2 + primitives, Layout/header with real workspace scoping, Operations HQ and the Keys product page so an operator can triage anomalies and issue a constrained key (AC-001..006).
- Acceptance Boundaries: AC-001..AC-011. Backend confined to two additive surfaces: optional `tenant` param on analytics endpoints (DEC-003) and read-only `/api/v1/admin/status` incl. `config_diff` (DEC-004).
- Key Rules: UI is the control-plane console (constitution follow-up tracked in archive checklist); migrated pages use only v2 tokens/primitives — no inline `style=`, no legacy alias classes (DEC-001, DEC-005); scoping uses an explicit `tenant` query param, never ambient context (DEC-003).
- Contracts: `GET /api/v1/analytics/tokens|cost|timeseries?tenant=<slug>` — absent param = all tenants, response shape unchanged; `GET /api/v1/admin/status` — additive read-only (version, uptime, key_at_rest, stores, config_diff). Keys DTO already exposes `spent`/`budget_cap` and one-time `key` on create.
- Invariants: default (unscoped) analytics path stays byte-identical; `config_diff` reuses `src/internal/infra/config` serialize/diff; no persistence/data-model change.
- Proof Signals: grep gates (no inline styles / legacy aliases on migrated pages; zero locale-arg `toLocaleString`; zero hardcoded Settings literals), vitest + `go test ./...`, per-AC checks from tasks.
- Out of Scope: no chart library change, no light-theme redesign, no auth/SSO, no new analytics pipelines, no playground UI.

## Phase 1: Foundation

Goal: v2 design system + shared formatting so every page pass reuses stable primitives.

- [x] T1.1 Tokens v2 — outcome: `tokens.css` exposes v2 layers (canvas/surface/surface-2/surface-3, border/text tiers, status colors, radius 14/11/8/6, shadows, tabular-nums base) and keeps the light mapping; no consumer breaks. Touches: ui/src/styles/tokens.css
      Proof: code ui/src/styles/tokens.css
- [x] T1.2 UI primitives — outcome: `Card`, `PageHeader`, `StatusDot`/`StatusPill`, `Segmented`, `ChipTag` (+ tag-input), `Switch`, `ProgressBar`, `Spinner`, `Skeleton`, `EmptyState` with CTA built in `ui/src/components/ui/` and exported via index; each keyed to v2 tokens. Touches: ui/src/components/ui/index.ts, ui/src/components/ui/Card.tsx, ui/src/components/ui/PageHeader.tsx, ui/src/components/ui/Status.tsx, ui/src/components/ui/Segmented.tsx, ui/src/components/ui/ChipTag.tsx, ui/src/components/ui/Switch.tsx, ui/src/components/ui/ProgressBar.tsx, ui/src/components/ui/EmptyState.tsx
      Proof: code ui/src/components/ui/index.ts
- [x] T1.3 Format utilities — outcome: `ui/src/utils/format.ts` ships `relativeTime(iso)`/`absDate(iso)` (locale-neutral), `money()`, `fmtTokens()` (1.2K/12.4M), tabular-nums defaults; no locale parameters. Touches: ui/src/utils/format.ts
      Proof: code ui/src/utils/format.ts
- [x] T1.4 Primitive + format tests — outcome: vitest green for all primitives (toggle/segmented/chips) and formatter cases (relative vs absolute, tz-neutral, scaling). Touches: ui/src/utils/format.test.ts, ui/src/components/ui/*.test.tsx
      Proof: test ui/src/utils/format.test.ts relativeTime

## Phase 2: Backend Additive Surfaces

Goal: the two approved additive endpoints land first so UI scoping (AC-003/004/007) and Settings (AC-010) have their data sources.

- [x] T2.1 Analytics tenant scope — outcome: handlers accept optional `tenant=<slug>`; records route via existing `QueryByTenant`, timeseries via a new tenant-scoped store call (closes the existing in-code tenant-filter gap); response shape unchanged; absent param behaves as today; handler tests for scoped + unscoped pass. Touches: src/internal/api/handler/analytics/analytics_handler.go, src/internal/api/handler/analytics/analytics_handler_test.go, src/internal/api/dto/analytics.go, src/internal/domain/analytics/usage_store.go, src/internal/adapters/repository/analytics/pg_usage_store.go
      Proof: test src/internal/api/handler/analytics/analytics_handler_test.go TestAnalyticsHandler_Tokens_TenantParam
- [x] T2.2 Read-only status endpoint — outcome: `GET /api/v1/admin/status` returns version, uptime, key_at_rest configured flag, aggregated store health, and `config_diff` computed from `infra/config` serialize/diff; wired in the admin router under admin auth; handler unit test passes and existing admin tests stay green. Touches: src/internal/api/handler/admin/status_handler.go, src/internal/api/handler/admin/status_handler_test.go, src/internal/api/admin.go
      Proof: test src/internal/api/handler/admin/status_handler_test.go TestStatusHandler_ReportsVersionHealthAndKeyAtRest

## Phase 3: MVP Pages

Goal: deliver Layout/header, Operations HQ and the Keys product page on the new system (AC-002..006).

- [x] T3.1 Layout v2 + header — outcome: nav regrouped by job (Overview/Traffic/Governance/Operations/System); header renders per-route primary actions, workspace switcher (real scoping via `tenant` param), live-status pill; all existing routes still render. Touches: ui/src/components/Layout.tsx
      Proof: code ui/src/components/Layout.tsx
- [x] T3.2 Dashboard HQ — outcome: KPI cards (spend, requests, tokens, pass rate) with delta vs previous period + sparkline; trend chart with Tokens/Cost/Requests segmented that re-issues the same-range tenant-scoped series; Needs-attention panel sourced from analytics/budget/health/incidents payloads with drill links. Touches: ui/src/pages/Dashboard.tsx, ui/src/api/analytics.ts
      Proof: code ui/src/pages/Dashboard.tsx
- [x] T3.3 Keys page — outcome: endpoint quickstart (base URL + copy + curl), create-key drawer (tenant, label, model allow/deny chips, budget, expiry presets) with inline validation and one-time reveal, table with model chips + spend/budget progress (spent/budget_cap) + enable/disable switch + search/tenant/status filters + copy/revoke. Touches: ui/src/pages/Keys.tsx, ui/src/api/keys.ts
      Proof: code ui/src/pages/Keys.tsx
- [x] T3.4 MVP tests — outcome: Dashboard attention-mapping + metric-toggle-keeps-range, Keys create-validation + filter + threshold-color tests green. Touches: ui/src/pages/Dashboard.test.tsx, ui/src/pages/Keys.test.tsx
      Proof: test ui/src/pages/Dashboard.test.tsx

## Phase 4: Expansion Pages

Goal: extend the console to Analytics, Routing, Tenant profile and Settings (AC-007..010).

- [x] T4.1 Analytics v2 — outcome: persisted range (localStorage), compare-vs-previous badges on KPIs, day/model/tenant breakdown tabs, CSV export honoring active range + scope. Touches: ui/src/pages/Analytics.tsx, ui/src/api/analytics.ts, ui/src/components/TimeRangePicker.tsx, ui/src/components/TimeSeriesChart.tsx
      Proof: code ui/src/pages/Analytics.tsx
- [x] T4.2 Routing v2 — outcome: provider cards (status dot/pill, p50 latency, 24h ok-rate, routes, active fallback chain) + routing-rules table with switches, rendered from the existing routing API. Touches: ui/src/pages/Routing.tsx, ui/src/api/routing.ts
      Proof: code ui/src/pages/Routing.tsx
- [x] T4.3 Tenant profile v2 — outcome: Overview / Policies & Shield / Compliance / Activity tabs; compliance-deviation banner with Review-diff and Re-apply wired to `getComplianceReport`/`applyCompliancePack`. Touches: ui/src/pages/Tenants/TenantDetail.tsx, ui/src/api/tenants.ts
      Proof: code ui/src/pages/Tenants/TenantDetail.tsx
- [x] T4.4 Settings live — outcome: system status cards, server info and config diff rendered from `/api/v1/admin/status` via a new `getSystemStatus` client; apply/discard wired to the documented endpoint semantics; zero hardcoded literals. Touches: ui/src/pages/Settings.tsx, ui/src/api/admin.ts
      Proof: code ui/src/api/admin.ts
- [x] T4.5 Expansion tests — outcome: Analytics persistence/compare, Routing three-state rendering, Tenant banner + action calls, Settings render-from-status tests green. Touches: ui/src/pages/Analytics.test.tsx, ui/src/pages/Routing.test.tsx, ui/src/pages/Tenants/TenantDetail.test.tsx, ui/src/pages/Settings.test.tsx
      Proof: test ui/src/pages/Tenants/TenantDetail.test.tsx

## Phase 5: Validation & Polish

Goal: onboarding, format-utils adoption everywhere, and global gates before review (AC-011 + whole-feature audit).

- [x] T5.1 Onboarding + format adoption — outcome: first-run empty states on Dashboard and Keys render an onboarding CTA with connect-your-app curl; all migrated pages use `format.ts` for dates/numbers. Touches: ui/src/pages/Dashboard.tsx, ui/src/pages/Keys.tsx, ui/src/pages/Analytics.tsx, ui/src/pages/Routing.tsx, ui/src/pages/Tenants/TenantDetail.tsx, ui/src/pages/Settings.tsx
      Proof: code ui/src/pages/Keys.tsx
- [x] T5.2 Global gates + walkthrough — outcome: greps pass (no inline `style=` / legacy aliases on migrated pages; zero locale-arg `toLocaleString`; zero Settings hardcoded literals); `vitest run` and `go test ./...` green; manual walkthrough of all 11 routes shows no broken page. Touches: repo-wide grep over ui/src, specs/active/ui-v2-console/tasks.md
      Proof: chore specs/active/ui-v2-console/tasks.md

## Acceptance Coverage

- AC-001 -> T1.1, T1.2, T1.4
- AC-002 -> T3.1
- AC-003 -> T2.1, T3.1, T3.2, T3.4
- AC-004 -> T2.1, T3.2, T3.4
- AC-005 -> T3.3, T3.4
- AC-006 -> T3.3, T3.4
- AC-007 -> T2.1, T4.1, T4.5
- AC-008 -> T4.2, T4.5
- AC-009 -> T4.3, T4.5
- AC-010 -> T2.2, T4.4, T4.5
- AC-011 -> T1.3, T1.4, T5.1, T5.2

## Notes

- Task IDs follow `T<phase>.<index>`; all phases present — none intentionally omitted.
- Each completed task must carry `Proof: <kind> <repo-root-relative-path> [<anchor>]` on the following line; without it the task is not done.
- Layout (`T3.1`) depends on primitives (`T1.2`); Dashboard scoping depends on `T2.1`; Settings on `T2.2`.