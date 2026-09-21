# Chart Time Buckets and Labels Tasks

## Phase Contract

Inputs: `plan.md` + `spec.md` (decisions pinned).
Outputs: ordered executable tasks with coverage mapping.
Stop if: acceptance coverage cannot be mapped — not the case.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/adapters/repository/analytics/pg_usage_store.go | T1.1 |
| src/internal/adapters/repository/analytics/pg_usage_store_test.go | T3.1 |
| ui/src/utils/format.ts | T1.2, T2.2 |
| ui/src/utils/format.test.ts | T3.2 |
| ui/src/components/TimeSeriesChart.tsx | T2.1 |
| ui/src/components/TimeSeriesChart.test.tsx | T3.3 |
| ui/src/pages/Dashboard.tsx | T2.2 |
| ui/src/pages/Analytics.tsx | T2.2 |

## Implementation Context

- MVP Goal: hourly buckets for ranges up to 7 days, and axis labels chosen from the requested range span so a week reads as date+time, short ranges as clock time, longer ranges as dates only.
- Acceptance Boundaries: AC-001..AC-006.
- Key Rules: hourly threshold is 7 days inclusive (DEC-001); the axis span comes from the caller's selected range with a data-span fallback (DEC-002); time formatters live in `utils/format.ts` and are unit-tested (DEC-003); the tooltip always shows full date+time including the year (DEC-004); keep the existing tick-gap config, no custom overlap logic (DEC-005).
- Invariants: daily buckets never render a `00:00` time component; one axis never mixes date-only and date+time formats; Analytics and Operations HQ share one chart implementation; no API/DTO/schema change.
- Contracts/Protocols: `resolveBucket(from, to)` returns `"hour"` for spans ≤ 7 days, else `"day"`; `chartTickLabel(iso, spanMs)` → `HH:MM` (≤48h) / `D.M HH:MM` (≤7d) / `D.M` (>7d); `chartTooltipLabel(iso)` → full `D.M.YYYY HH:MM`; `TimeSeriesChart` gains an optional `spanMs` prop.
- Scope Boundaries: do not add hourly buckets for 30d/All; do not change analytics endpoints, DTOs, or the schema; do not touch other charts.
- Proof Signals: Go tests for bucket boundaries; TS tests for both formatters across spans; a chart test with an explicit span and 168 points; existing Dashboard/Analytics suites green.
- References: DEC-001..DEC-005 (plan.md), RQ-001..RQ-006 (spec.md).

## Phase 1: Foundation

Goal: the bucket unit and the label formatters exist.

- [x] T1.1 Serve hourly buckets up to seven days — outcome: `resolveBucket` returns `hour` for spans up to and including 7 days and `day` beyond, for both time-series queries. Touches: src/internal/adapters/repository/analytics/pg_usage_store.go
      Proof: code src/internal/adapters/repository/analytics/pg_usage_store.go resolveBucket
- [x] T1.2 Add span-aware chart time formatters — outcome: exported `chartTickLabel(iso, spanMs)` and `chartTooltipLabel(iso)` return the span-correct labels and a full date+time tooltip. Touches: ui/src/utils/format.ts
      Proof: code ui/src/utils/format.ts chartTickLabel

## Phase 2: MVP Slice

Goal: the shared chart renders span-correct labels.

- [x] T2.1 Use the formatters in `TimeSeriesChart` with an optional span prop — outcome: the axis and tooltip use the new formatters, `spanMs` overrides the data-derived span, and the existing tick-gap config is retained (AC-002, AC-005). Touches: ui/src/components/TimeSeriesChart.tsx
      Proof: code ui/src/components/TimeSeriesChart.tsx dataSpanMs
- [x] T2.2 Pass the selected range span from both callers — outcome: Operations HQ and Analytics pass the span of their selected range so sparse long ranges are labelled correctly (AC-002, AC-006). Touches: ui/src/pages/Dashboard.tsx, ui/src/pages/Analytics.tsx, ui/src/utils/format.ts
      Proof: code ui/src/pages/Dashboard.tsx rangeSpanMs

## Phase 3: Validation

Goal: prove behavior and leave the package reviewable.

- [x] T3.1 Add bucket-boundary coverage — outcome: tests assert `hour` at ≤48h and 7d and `day` beyond 7 days (AC-001). Touches: src/internal/adapters/repository/analytics/pg_usage_store_test.go
      Proof: test src/internal/adapters/repository/analytics/pg_usage_store_test.go TestResolveBucket
- [x] T3.2 Add formatter coverage — outcome: tests assert `HH:MM`, `D.M HH:MM`, date-only for daily spans (no `00:00`), and a full tooltip label including the year (AC-002, AC-003, AC-004). Touches: ui/src/utils/format.test.ts
      Proof: test ui/src/utils/format.test.ts chartTickLabel
- [x] T3.3 Add chart span/density coverage — outcome: a chart test with an explicit span renders span-correct labels and 168 points without error, and the existing Analytics suite stays green (AC-005, AC-006). Touches: ui/src/components/TimeSeriesChart.test.tsx
      Proof: test ui/src/components/TimeSeriesChart.test.tsx TimeSeriesChart

## Acceptance Coverage

- AC-001 -> T1.1, T3.1
- AC-002 -> T1.2, T2.1, T2.2, T3.2
- AC-003 -> T1.2, T3.2
- AC-004 -> T1.2, T3.2
- AC-005 -> T2.1, T3.3
- AC-006 -> T2.2, T3.3

## Notes

- Ordering: formatters and the bucket helper first, then the chart and its callers, then tests.
- No data model, API, schema, or config change; the bucket unit for >7d stays daily.
- Keep `TimeSeriesChart` backward compatible: without `spanMs` it falls back to the span of the rendered data.
