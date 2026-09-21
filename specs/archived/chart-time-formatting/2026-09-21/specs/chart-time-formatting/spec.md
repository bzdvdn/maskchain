# Chart Time Buckets and Labels

## Scope Snapshot

- In scope: give time-series charts meaningful intra-day resolution for ranges up to 7 days and format their axis/tooltip labels from the range span, so ticks never show a fake `00:00` or mix date-only and date+time formats.
- Out of scope: hourly resolution for 30d/All, API or schema changes, and other charts/pages.

## Goal

Operators reading a 7-day trend currently see daily buckets labelled inconsistently: some ticks render `d.m HH:MM` with a meaningless `00:00` and others render `d.m`, because the formatter decides from "now minus bucket" rather than from the range. This feature makes the backend serve hourly buckets for ranges up to 7 days and makes the chart choose its label format from the actual range span, so a short range reads as clock time, a week reads as date plus time, and longer ranges read as clean dates — always with a full date+time tooltip.

## Primary User Flow

1. Starting point: an operator opens a chart on Operations HQ or Analytics.
2. Main interaction: they pick a range (Today, Yesterday, 7d, 30d, All, custom) and hover the line.
3. Outcome: the axis shows clock times for short ranges, date+time for up to a week, and dates beyond that; the tooltip always shows the full date and time.
4. Failure/fallback path: a range that returns a single bucket still shows one clear label; missing/invalid bucket timestamps fall back to a safe label.

## User Stories

- P1 Story: as an operator, I can see hourly movement over the last week, not just daily dots.
- P2 Story: as an operator, I never see a fake `00:00` or a mixture of label formats on one axis.
- P3 Story: as an operator, hovering a point always tells me the exact date and time.

## MVP Slice

- AC-001 (hourly up to 7d) and AC-002 (span-driven axis labels). Together they deliver the visible improvement.

## First Deployable Outcome

- After the first pass, a 7-day chart has hourly buckets and reads as `d.m HH:MM`, while a 30-day chart stays daily with `d.m` labels and the tooltip shows full date+time.

## Scope

- Backend: the time-series bucket granularity selection.
- Frontend: `TimeSeriesChart` axis tick and tooltip label formatting, driven by the span of the data it renders.
- Tests: bucket selection and label formatting.

## Out of Scope

- Hourly buckets for 30d/All (would be hundreds of points).
- Changes to analytics endpoints, DTOs, database schema, or aggregation.
- Other charts, pages, or the metric selector.

## Context

- `resolveBucket(from, to)` in the usage store currently returns `hour` only when the span is under 48 hours and `day` otherwise, and both time-series queries share it.
- `TimeSeriesChart` is shared by Operations HQ and Analytics and currently decides tick formatting from `now - bucket`, which produces mixed formats and fake midnight times on multi-day ranges.
- Buckets are absolute timestamps, so the chart can derive the range span from its first and last points without a new prop.
- The chart already sets `minTickGap`/`interval`, so denser hourly data will not produce overlapping labels automatically.

## Dependencies

- Existing `resolveBucket` helper and both time-series queries.
- Existing `TimeSeriesChart` component and its callers.
- No new dependency.

## Requirements

- RQ-001 The time-series bucket selection MUST return hourly buckets for ranges up to and including 7 days, and daily buckets beyond that.
- RQ-002 The chart axis MUST format tick labels from the requested range span supplied by its caller (falling back to the span of the rendered data when not supplied): clock time for spans up to 48 hours, date plus time for spans up to 7 days, and date only for longer spans.
- RQ-003 The chart MUST NOT render a `00:00` time component for daily buckets and MUST NOT mix date-only and date+time formats within one axis.
- RQ-004 The tooltip MUST always show the full date and time for the hovered bucket, including the year when the span crosses more than one year.
- RQ-005 Denser hourly data for up to 7 days MUST remain readable (no overlapping tick labels).
- RQ-006 The shared chart's other consumer (Analytics) MUST keep working, and existing tests MUST stay green.

## Non-Goals

- Sub-hourly buckets or real-time streaming resolution.
- Changing the bucket unit for the daily aggregation used by reports.
- Adding zoom/pan or a granularity selector.

## Acceptance Criteria

### AC-001 Hourly buckets up to seven days

- Why this matters: a week of daily points hides intra-day movement.
- **Given** a time-series query whose range spans 7 days
- **When** the bucket unit is resolved
- **Then** it is hourly; and for a range beyond 7 days it is daily.
- Evidence: unit tests asserting the bucket unit at the 48h, 7d and 7d+ boundaries.

### AC-002 Axis labels follow the range span

- Why this matters: the current labels are inconsistent and misleading.
- **Given** chart data spanning up to 48 hours, up to 7 days, and more than 7 days
- **When** the axis renders
- **Then** the tick labels are clock time, date plus time, and date only respectively.
- Evidence: formatter tests for each span.

### AC-003 No fake midnight and no mixed formats

- Why this matters: `00:00` on a daily bucket and mixed formats read as data errors.
- **Given** daily buckets across a multi-day range
- **When** the axis renders
- **Then** no label contains a `00:00` time component and all labels use the same format.
- Evidence: a test asserting daily-span labels contain only dates.

### AC-004 Tooltip shows the full timestamp

- Why this matters: the tooltip is the exact-value readout.
- **Given** a hovered bucket
- **When** the tooltip renders
- **Then** it shows the full date and time, including the year for spans over a year.
- Evidence: formatter tests for the tooltip label, including the multi-year case.

### AC-005 Dense hourly data stays readable

- Why this matters: 168 hourly points must not collide on the axis.
- **Given** hourly data across 7 days
- **When** the axis renders
- **Then** tick labels do not overlap and remain legible.
- Evidence: a test or assertion that the axis keeps a minimum tick gap/interval configuration.

### AC-006 Shared chart regression is avoided

- Why this matters: Analytics uses the same component.
- **Given** the Analytics page renders the chart
- **When** it renders with its existing data
- **Then** it still displays correctly and its tests pass.
- Evidence: the existing Analytics test suite stays green plus any updated chart tests.

## Assumptions

- "Up to 7 days" is inclusive; a span of exactly 7 days uses hourly buckets.
- Callers pass the requested range span to the chart; when it is absent the chart falls back to the span of the data it renders. No API or DTO change is required.
- Using the requested span (rather than only the data span) prevents a sparse long range, whose buckets are daily, from being labelled as if it were a short hourly range.
- A single-bucket series is treated as a short span and labelled with clock time.
- Weekday names are not required; `D.M` / `D.MM` numeric labels are acceptable for this iteration.

## Success Criteria

- SC-001 A 7-day chart shows hourly resolution without overlapping labels.
- SC-002 No axis in the app displays a `00:00` on daily buckets or mixed formats.
- SC-003 Existing Operations HQ and Analytics tests remain green.

## Edge Cases

- Single bucket: one clear label, clock time.
- Invalid/missing bucket timestamp: fall back to a safe placeholder rather than `NaN`.
- Custom range under 48 hours but crossing midnight: labels remain clock time; the tooltip carries the date.
- Span just over 7 days: switches to daily labels with no midnight time.
- Very long (All) range: date labels only; tooltip includes the year.

## Open Questions

- none
