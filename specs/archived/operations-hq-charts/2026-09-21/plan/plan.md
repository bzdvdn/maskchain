# Operations HQ Charts Clarity Plan

## Phase Contract

Inputs: `spec.md` + minimal repo context.
Outputs: `plan.md` (no data-model/contracts expansion).
Stop if: spec is vague — not the case; decisions pinned below.

## Goal

Make Operations HQ self-describing: give each KPI card its own metric and a range/scope-accurate label, replace the bespoke unlabeled trend chart with the existing axis/tooltip-capable `TimeSeriesChart` generalized over metrics, and handle empty/zero/single-point data explicitly. No backend changes.

## MVP Slice

- Per-card metric + range/scope labels + generalized chart with axes/tooltip.
- AC covered: AC-001, AC-002, AC-003.

## First Validation Path

1. Open Operations HQ with data.
2. Each of the four cards shows a differently shaped sparkline and a label like "Spend · 7d".
3. Switch the trend metric and range: the chart header and axes update; hover shows value + time.
4. Select a workspace: pass-rate and labels reflect it.

## Scope

- `ui/src/components/TimeSeriesChart.tsx` — add a `metric` dimension (tokens/cost/requests) with per-metric series, formatting and tooltip, keeping the tokens default behavior.
- `ui/src/pages/Dashboard.tsx` — card descriptors with their own metric, range/scope labels, requests delta, tenant-scoped pass rate, trend header, semantics caption, empty/zero handling; remove the bespoke `TrendChart`.
- `ui/src/utils/format.ts` — a range/scope label helper.
- `ui/src/pages/Dashboard.test.tsx`, `ui/src/components/TimeSeriesChart.test.tsx` — coverage.

## Out of Scope

- Backend analytics changes (range-accurate block rate), new pages, chart-library changes, layout redesign.

## Performance Budget

- UI-only; no new network calls beyond reusing the already-fetched previous-period cost totals. Rendering stays within the existing recharts budget.

## Implementation Surfaces

- `ui/src/components/TimeSeriesChart.tsx` — existing; generalize with an optional `metric` and a value formatter; keep `data`/`compare`/`height` props and the tokens behavior as default.
- `ui/src/pages/Dashboard.tsx` — existing; card model, labels, delta, pass-rate scoping, chart header, semantics, empty states.
- `ui/src/utils/format.ts` — existing; add a range/scope label helper.
- `ui/src/components/TimeSeriesChart.test.tsx` — new; metric rendering.
- `ui/src/pages/Dashboard.test.tsx` — existing; corrected behavior.
- `README.md` — optional one-line note only if behavior is user-visible in docs (not required).

## Bootstrapping Surfaces

- none — all components and APIs exist.

## Architecture Impact

- Local: one shared chart component gains a metric dimension; the dashboard drops its bespoke chart.
- Integration: Analytics keeps using `TimeSeriesChart` with the default metric, so its rendering is unchanged.
- No backend, no data model, no config.

## Acceptance Approach

- AC-001 → a card descriptor carries its own metric; `Sparkline` receives that metric; proof: Dashboard test asserting distinct series per card.
- AC-002 → `rangeLabel(mode, from, to)` + workspace suffix feed card labels; proof: Dashboard test changing range/workspace.
- AC-003 → `TimeSeriesChart` axes/tooltip with per-metric formatter; proof: chart test asserting tick formatting and tooltip content.
- AC-004 → trend header derives metric label + range + total/delta from existing totals; proof: Dashboard test on metric switch.
- AC-005 → fetch previous-period `request_count` and compute the delta; proof: Dashboard test with previous data.
- AC-006 → pass workspace into the conversation list and label window/scope; proof: Dashboard test asserting the tenant filter and label.
- AC-007 → semantics caption + `role="img"`/`aria-label` on the chart; proof: Dashboard/chart test asserting text and accessible name.
- AC-008 → explicit empty/zero/single handling before rendering; proof: Dashboard tests for each case.

## Data and Contracts

- Data model: no change.
- API contract: no change (reuse tokens/cost/timeseries and conversations).
- Config contract: none.
- No `contracts/*` or `data-model.md` needed.

## Implementation Strategy

- DEC-001 Generalize `TimeSeriesChart` instead of a second chart
  Why: one implementation, reuses axes/ticks/tooltip/compare, consistent with Analytics.
  Tradeoff: a shared component is touched, so a regression in Analytics is possible.
  Affects: TimeSeriesChart.tsx.
  Validation: default `metric='tokens'` preserves current output; chart tests.
- DEC-002 Card descriptors carry their own metric
  Why: removes the root cause of the misleading sparklines.
  Tradeoff: cards are data-driven rather than hardcoded JSX.
  Affects: Dashboard.tsx.
  Validation: Dashboard tests.
- DEC-003 Range/scope label helper in `format.ts`
  Why: one source of truth for "Today/7d/30d/custom + workspace" labels.
  Tradeoff: a small utility addition.
  Affects: format.ts, Dashboard.tsx.
  Validation: unit tests.
- DEC-004 Pass rate is tenant-scoped with an honest window label; range-accurate block rate deferred
  Why: conversations have no server-side date filter, so range accuracy would need a backend metric (out of scope).
  Tradeoff: the tile's window is "recent requests", stated in the label.
  Affects: Dashboard.tsx.
  Validation: Dashboard test.
- DEC-005 Keep the small bespoke `Sparkline` for cards
  Why: recharts is heavier than needed for a 30px sparkline; the main chart uses recharts.
  Tradeoff: two chart primitives remain (accepted, with the shared axis/tooltip logic only in the main chart).
  Affects: Dashboard.tsx.
  Validation: Dashboard tests.
- DEC-006 Requests delta from the already-fetched previous cost totals
  Why: no new endpoint; `request_count` is already in the previous cost response.
  Tradeoff: none.
  Affects: Dashboard.tsx.
  Validation: Dashboard test.

## Incremental Delivery

### MVP (First Value)

- Card metrics + labels + generalized chart with axes/tooltip.
- Ready when AC-001/002/003 pass.

### Iterative Expansion

- AC-004/005 (header + requests delta), AC-006 (pass-rate scoping), AC-007 (units/a11y), AC-008 (empty/zero/single).
- Each validated independently.

## Sequencing Notes

- Generalize `TimeSeriesChart` first (dashboard depends on it) while keeping the tokens default.
- Then rework the dashboard cards/labels/header.
- Then pass-rate scoping, units/a11y and empty-state handling.

## Risks

- Analytics regression from touching `TimeSeriesChart` → mitigated by the default metric and chart tests.
- Pass-rate accuracy expectation → mitigated by an explicit window/scope label (DEC-004).
- Zero/single-point rendering edge cases → explicit handling and tests (AC-008).
- Range label drift for custom ranges → covered by the label helper tests.

## Rollout and Compatibility

- Frontend-only, no migration; Analytics behavior unchanged by the default metric.
- No feature flag needed.

## Validation

- Unit: `rangeLabel` helper; `TimeSeriesChart` metric rendering/tooltip.
- Component: Dashboard per-card metrics, labels, requests delta, pass-rate scoping, empty/zero/single.
- Acceptance IDs covered: AC-001..AC-008. Decisions: DEC-001..DEC-006.

## Constitution Compliance

- no conflicts — operator console only, no DLP or data-plane change, no new dependency.
