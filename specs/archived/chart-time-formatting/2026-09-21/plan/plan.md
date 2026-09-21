# Chart Time Buckets and Labels Plan

## Phase Contract

Inputs: `spec.md` + minimal repo context.
Outputs: `plan.md` (no data-model/contracts expansion).
Stop if: spec is vague — not the case; decisions pinned below.

## Goal

Serve hourly buckets for ranges up to seven days, and make the shared `TimeSeriesChart` choose its axis label format from the requested range span (passed by the caller) so a week reads as date+time, short ranges read as clock time, and longer ranges read as clean dates — never a fake `00:00` or a mix of formats. Tooltips always show the full date and time.

## MVP Slice

- Backend bucket threshold + span-aware axis labels.
- AC covered: AC-001, AC-002.

## First Validation Path

1. Open a 7d chart: buckets are hourly and labels read `d.m HH:MM`.
2. Switch to 30d: labels read `d.m` with no `00:00`.
3. Switch to Today: labels read `HH:MM`.
4. Hover any point: tooltip shows the full date and time.

## Scope

- `resolveBucket` threshold in the usage store.
- Time formatters moved to `utils/format.ts` (exported, unit-tested) and used by `TimeSeriesChart`.
- `TimeSeriesChart` gains an optional span prop; both callers pass the selected range span.
- Tests for bucket selection and label formatting.

## Out of Scope

- Hourly buckets for 30d/All, API/DTO/schema changes, other charts, zoom/pan, granularity selector.

## Performance Budget

- 7d hourly = up to 168 points per series; the chart already renders two areas plus an optional compare line. No new per-point work beyond label formatting. Query cost grows with rows per bucket group, not with total rows.

## Implementation Surfaces

- `src/internal/adapters/repository/analytics/pg_usage_store.go` — existing: `resolveBucket` threshold (both time-series queries share it).
- `src/internal/adapters/repository/analytics/pg_usage_store_test.go` — existing: bucket-boundary tests.
- `ui/src/utils/format.ts` — existing: add exported `chartTickLabel(iso, spanMs)` and `chartTooltipLabel(iso)`.
- `ui/src/utils/format.test.ts` — existing: formatter tests.
- `ui/src/components/TimeSeriesChart.tsx` — existing: optional `spanMs` prop, use the formatters, keep tick-gap config.
- `ui/src/pages/Dashboard.tsx` — existing: pass the selected range span.
- `ui/src/pages/Analytics.tsx` — existing: pass the selected range span.
- `ui/src/components/TimeSeriesChart.test.tsx` — existing: adjust if the default formatting changes.

## Bootstrapping Surfaces

- none — all files exist.

## Architecture Impact

- Local: one backend helper threshold, one shared formatter pair, one optional prop threaded through two callers.
- Integration: Analytics and Operations HQ both benefit; Analytics labels change for short ranges (intended).
- No API, DTO, schema, or config change.

## Acceptance Approach

- AC-001 → `resolveBucket` returns `hour` for spans ≤ 7d and `day` beyond; proof: unit test at 48h, 7d, 7d+.
- AC-002 → `chartTickLabel(iso, spanMs)` branches on the span; the chart receives `spanMs` from callers; proof: formatter tests + caller wiring.
- AC-003 → daily spans never emit a time component; proof: formatter test asserting date-only output for a >7d span.
- AC-004 → `chartTooltipLabel` always includes date, time and year; proof: formatter test.
- AC-005 → keep `minTickGap`/`interval="preserveStartEnd"`; proof: chart test asserts the axis config is unchanged and rendering 168 points does not throw.
- AC-006 → Analytics tests stay green with the new default labels; proof: existing Analytics test suite.

## Data and Contracts

- Data model: no change.
- API contract: no change; buckets remain absolute timestamps, only their unit changes for ≤7d ranges.
- Config contract: none.
- No `contracts/*` or `data-model.md` needed.

## Implementation Strategy

- DEC-001 Hourly buckets for spans up to and including 7 days
  Why: a week of daily points hides intra-day movement, which the operator explicitly wants to see.
  Tradeoff: up to 168 points per series; heavier than daily but bounded and acceptable.
  Affects: pg_usage_store.go.
  Validation: `resolveBucket` boundary tests.
- DEC-002 The axis span comes from the caller's selected range, with a data-span fallback
  Why: deriving the span only from data mislabels sparse long ranges (daily buckets labelled as a short hourly span would reintroduce `00:00`).
  Tradeoff: the chart gains an optional prop and both callers must pass it.
  Affects: TimeSeriesChart.tsx, Dashboard.tsx, Analytics.tsx.
  Validation: formatter tests + a chart test with an explicit span.
- DEC-003 Time formatters live in `utils/format.ts`
  Why: they can be unit-tested without rendering recharts, and both the axis and tooltip share one source.
  Tradeoff: `format.ts` grows by two small functions.
  Affects: format.ts, format.test.ts, TimeSeriesChart.tsx.
  Validation: formatter tests.
- DEC-004 The tooltip always shows full date and time (including the year)
  Why: the tooltip is the exact readout; an unambiguous timestamp beats a shorter one.
  Tradeoff: a slightly longer tooltip string.
  Affects: format.ts.
  Validation: formatter test including a multi-year span.
- DEC-005 Keep the existing tick-gap configuration; add no custom overlap logic
  Why: recharts already skips ticks via `minTickGap`/`interval`, and denser data does not need new layout code.
  Tradeoff: relies on the existing config staying in place.
  Affects: TimeSeriesChart.tsx.
  Validation: chart test asserting the axis config and rendering 168 points.

## Incremental Delivery

### MVP (First Value)

- Bucket threshold + span-aware tick labels.
- Ready when AC-001/002 pass.

### Iterative Expansion

- AC-003 (no midnight/mixed), AC-004 (tooltip), AC-005 (density), AC-006 (Analytics regression).
- Each validated independently.

## Sequencing Notes

- Formatters first (chart depends on them), then the chart prop, then the two callers, then the backend threshold, then tests.

## Risks

- Sparse long ranges mislabelled → mitigated by DEC-002 (requested span wins).
- Density/overlap on 7d hourly → mitigated by existing tick-gap config plus a rendering test (AC-005).
- Analytics label change → intended and covered by AC-006.
- Boundary ambiguity at exactly 7 days → resolved as inclusive in DEC-001 and tested.

## Rollout and Compatibility

- Backend: bucket unit changes for ≤7d ranges only; no migration, no schema change.
- Frontend: labels change for short ranges; no API change.
- No feature flag required.

## Validation

- Unit (Go): `resolveBucket` at 48h, 7d, 7d+.
- Unit (TS): `chartTickLabel` for ≤48h / ≤7d / >7d and `chartTooltipLabel`.
- Component: `TimeSeriesChart` with an explicit span renders span-correct labels and 168 points without error.
- Regression: existing Dashboard/Analytics suites green.
- Acceptance IDs covered: AC-001..AC-006. Decisions: DEC-001..DEC-005.

## Constitution Compliance

- no conflicts — UI/analytics only; no DLP or data-plane change; no new dependency.
