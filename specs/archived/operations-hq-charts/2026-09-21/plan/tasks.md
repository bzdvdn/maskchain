# Operations HQ Charts Clarity Tasks

## Phase Contract

Inputs: `plan.md` + `spec.md` (decisions pinned).
Outputs: ordered executable tasks with coverage mapping.
Stop if: acceptance coverage cannot be mapped — not the case.

## Surface Map

| Surface | Tasks |
|---------|-------|
| ui/src/components/TimeSeriesChart.tsx | T1.1, T3.1 |
| ui/src/components/TimeSeriesChart.test.tsx | T4.2 |
| ui/src/utils/format.ts | T1.2 |
| ui/src/utils/format.test.ts | T4.1 |
| ui/src/pages/Dashboard.tsx | T2.1, T2.2, T2.3, T3.1 |
| ui/src/pages/Dashboard.test.tsx | T4.3 |

## Implementation Context

- MVP Goal: each KPI card plots its own metric with a range/scope-accurate label, and the trend chart has axes, units and a hover tooltip.
- Acceptance Boundaries: AC-001..AC-008.
- Key Rules: generalize the shared `TimeSeriesChart` with a `metric` prop and keep `metric='tokens'` as the default so Analytics is unchanged (DEC-001); cards are data-driven descriptors (DEC-002); one range/scope label helper (DEC-003); pass rate is tenant-scoped with an honest window label, range-accuracy deferred (DEC-004); keep the small bespoke `Sparkline` (DEC-005); requests delta from the already-fetched previous cost totals (DEC-006).
- Invariants: the trend metric selector affects the chart only, never the cards; labels always match the selected range and workspace; no backend/API change; Analytics rendering unchanged.
- Contracts/Protocols: `TimeSeriesChart` props stay `data`, `compare`, `height` and gain optional `metric: 'tokens' | 'cost' | 'requests'`; `SeriesPoint` already carries `input_tokens`, `output_tokens`, `cost`, `requests`; `rangeLabel(mode, from, to)` returns a short label such as `Today`, `7d`, `30d`, `All`, or a custom date span.
- Scope Boundaries: do not touch backend analytics or add endpoints; do not replace recharts; do not add drill-downs or per-model breakdowns.
- Proof Signals: `vitest` green for `format`, `TimeSeriesChart`, `Dashboard`; assertions on per-card series, labels, delta, tenant-scoped conversations, axes/tooltip formatting, accessible name, and empty/zero/single states.
- References: DEC-001..DEC-006 (plan.md), RQ-001..RQ-008 (spec.md).

## Phase 1: Foundation

Goal: the shared chart and the label helper can express a metric.

- [x] T1.1 Generalize `TimeSeriesChart` over metrics — outcome: an optional `metric` prop renders tokens (Input/Output split), cost (single series, money formatter) or requests (single series, count formatter), with matching tooltip/legend/axis formatting; the default keeps today's tokens output. Touches: ui/src/components/TimeSeriesChart.tsx
      Proof: code ui/src/components/TimeSeriesChart.tsx TimeSeriesChart
- [x] T1.2 Add a range/scope label helper — outcome: `rangeLabel(mode, from, to)` returns a short human label for Today/Yesterday/7d/30d/All/custom, and the dashboard can append the workspace scope. Touches: ui/src/utils/format.ts
      Proof: code ui/src/utils/format.ts rangeLabel

## Phase 2: MVP Slice

Goal: the dashboard cards and chart are correct and legible.

- [x] T2.1 Give each KPI card its own metric and range/scope label — outcome: Spend/Requests/Tokens cards plot their own metric in the sparkline and their labels reflect the selected range and workspace, independent of the trend selector (AC-001, AC-002). Touches: ui/src/pages/Dashboard.tsx
      Proof: code ui/src/pages/Dashboard.tsx Sparkline
- [x] T2.2 Use the generalized chart and state the trend header — outcome: the bespoke `TrendChart` is removed, the shared chart renders the selected metric with axes/tooltip, and the header names metric + range + total + delta (AC-003, AC-004). Touches: ui/src/pages/Dashboard.tsx
      Proof: code ui/src/pages/Dashboard.tsx TimeSeriesChart
- [x] T2.3 Add the requests delta and tenant-scoped pass rate — outcome: Requests shows a delta from the previous period, and the pass-rate tile queries the selected workspace and states its window/scope (AC-005, AC-006). Touches: ui/src/pages/Dashboard.tsx
      Proof: code ui/src/pages/Dashboard.tsx requestsDelta

## Phase 3: Core Implementation

Goal: semantics, accessibility and edge cases.

- [x] T3.1 Add semantics, accessibility and empty/zero/single handling — outcome: the page states token/cost/request meaning and units, the chart exposes an accessible name, and empty/all-zero/single-point series show an explicit neutral state instead of a misleading line (AC-007, AC-008). Touches: ui/src/pages/Dashboard.tsx, ui/src/components/TimeSeriesChart.tsx
      Proof: code ui/src/components/TimeSeriesChart.tsx METRIC_LABEL

## Phase 4: Validation

Goal: prove behavior and leave the package reviewable.

- [x] T4.1 Add `rangeLabel` coverage — outcome: tests assert preset and custom labels (AC-002). Touches: ui/src/utils/format.test.ts
      Proof: test ui/src/utils/format.test.ts rangeLabel
- [x] T4.2 Add `TimeSeriesChart` metric coverage — outcome: tests assert tokens split, cost/requests single-series rendering, per-metric tick/tooltip formatting, the accessible name, and the empty/zero state (AC-003, AC-007, AC-008). Touches: ui/src/components/TimeSeriesChart.test.tsx
      Proof: test ui/src/components/TimeSeriesChart.test.tsx TimeSeriesChart
- [x] T4.3 Add dashboard coverage — outcome: tests assert per-card metrics, range/scope labels, requests delta, tenant-scoped pass rate, and empty/zero/single states (AC-001, AC-002, AC-004, AC-005, AC-006, AC-007, AC-008). Touches: ui/src/pages/Dashboard.test.tsx
      Proof: test ui/src/pages/Dashboard.test.tsx Dashboard

## Acceptance Coverage

- AC-001 -> T2.1, T4.3
- AC-002 -> T1.2, T2.1, T4.1, T4.3
- AC-003 -> T1.1, T2.2, T4.2
- AC-004 -> T2.2, T4.3
- AC-005 -> T2.3, T4.3
- AC-006 -> T2.3, T4.3
- AC-007 -> T3.1, T4.2, T4.3
- AC-008 -> T3.1, T4.2, T4.3

## Notes

- Frontend-only; no data model, API or config change.
- Keep `TimeSeriesChart` backward compatible: Analytics must keep rendering identically with the default metric.
- Ordering: chart + helper → dashboard MVP → semantics/edge → tests.
