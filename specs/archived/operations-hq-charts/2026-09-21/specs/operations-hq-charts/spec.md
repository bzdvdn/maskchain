# Operations HQ Charts Clarity

## Scope Snapshot

- In scope: make the Operations HQ dashboard truthful and readable — each KPI card and chart shows exactly what its label claims, with labeled axes, units, hover values, and correct period/tenant scoping.
- Out of scope: new backend analytics metrics, new pages, and changing the charting library.

## Goal

Operators open Operations HQ to answer "what is happening right now" and cannot: the trend chart has no axes or tooltip, all four KPI sparklines render the same series regardless of what the card measures, card labels claim a fixed window that does not match the selected range, and the pass-rate tile ignores the selected workspace. This feature makes every chart self-describing and every number correctly scoped, so an operator can read the page without guessing.

## Primary User Flow

1. Starting point: an operator opens Operations HQ with data present.
2. Main interaction: they read the KPI cards and the trend chart, switch the trend metric, and change the time range/workspace.
3. Outcome: each card's sparkline matches its own metric, labels reflect the chosen range and scope, and the trend chart shows time/value axes plus a hover readout.
4. Failure/fallback path: with no data, a single point, or all-zero values, the page shows an explicit empty state instead of a flat or misleading line.

## User Stories

- P1 Story: as an operator, I can trust that the "Spend" card shows spend and the "Requests" card shows requests.
- P2 Story: as an operator, I can see what the trend chart's line means (metric, period, units, per-point value).
- P3 Story: as an operator, switching workspace scopes every tile on the page, including pass rate.

## MVP Slice

- AC-001 (per-card metric), AC-002 (range-accurate labels), AC-003 (axes + tooltip). These remove the two misleading behaviors and make the chart legible.

## First Deployable Outcome

- After the first pass, the four KPI cards show four different, correctly labeled sparklines for the selected range, and the trend chart has a labeled time axis, a value axis with units, and a hover tooltip.

## Scope

- `ui/src/pages/Dashboard.tsx` — KPI cards, sparklines, trend chart wiring, pass-rate scoping.
- `ui/src/components/TimeSeriesChart.tsx` — generalized to accept a metric and value formatter so it can back the dashboard trend chart.
- `ui/src/utils/format.ts` — a range-label helper when needed.
- `ui/src/pages/Dashboard.test.tsx` — coverage for the corrected behavior.

## Out of Scope

- Any backend/analytics change (for example a range-accurate blocked-request metric).
- New dashboard widgets, layout redesign, or navigation changes.
- Replacing the charting library.
- Per-model or per-tenant breakdowns on Operations HQ (that stays on the Analytics page).

## Context

- The dashboard already fetches current and previous period totals and a timeseries, so correct deltas and range labels need no new API.
- A reusable `TimeSeriesChart` (axes, ticks, tooltip, legend, previous-period comparison) already exists and backs the Analytics page; the dashboard currently ships a second, less capable chart implementation.
- The time range picker offers Today, Yesterday, 7d, 30d, All, and custom, so any hardcoded "last 24h" label is wrong for most selections.
- Conversation listing supports a tenant filter but not a server-side date filter, so the pass-rate tile can be tenant-scoped but its window is "the last N requests" rather than the selected range.

## Dependencies

- Existing analytics endpoints (`tokens`, `cost`, `timeseries`) and conversation listing.
- Existing `TimeSeriesChart` component and `Segmented` primitive.
- No new dependency.

## Requirements

- RQ-001 Each KPI card's sparkline MUST plot the metric that card represents, independent of the trend metric selector.
- RQ-002 KPI card labels MUST state the selected time range and the tenant scope, not a fixed "last 24h".
- RQ-003 The trend chart MUST show a labeled time axis, a value axis with units for the selected metric (tokens, currency, or counts), and a hover tooltip giving the value and the bucket time.
- RQ-004 The trend chart header MUST name the selected metric and range and show the current total and delta, so the metric selector's effect is explicit.
- RQ-005 The Requests KPI MUST show a delta computed from the previous period, consistent with Spend and Tokens.
- RQ-006 The pass-rate tile MUST respect the selected workspace and be labeled with its actual window and scope.
- RQ-007 The page MUST state the meaning and units of the displayed values (tokens are input plus output; cost currency; requests source) and the trend chart MUST be accessible to assistive technology.
- RQ-008 With no data, a single bucket, or all-zero values, the page MUST show an explicit empty/neutral state rather than a flat or misleading line.

## Non-Goals

- Changing what the analytics endpoints return or adding new metrics.
- Redesigning the dashboard layout or other pages.
- Adding interactive drill-downs from the chart.

## Acceptance Criteria

### AC-001 Per-card sparklines use the card's own metric

- Why this matters: a "Spend" tile that plots tokens is actively misleading.
- **Given** the dashboard has timeseries data and a trend metric selected
- **When** the KPI cards render
- **Then** the Spend, Requests, and Tokens sparklines each plot their own metric, independent of the trend selector.
- Evidence: a UI test that renders the dashboard with distinct series and asserts each card's sparkline values derive from its own metric.

### AC-002 Card labels reflect the selected range and scope

- Why this matters: the totals already follow the range, so the label must too.
- **Given** the operator selects the 7d range (and optionally a workspace)
- **When** the KPI cards render
- **Then** each label names that range and scope instead of "last 24h".
- Evidence: a UI test asserting the card labels change when the range/workspace changes.

### AC-003 Trend chart has axes and a hover readout

- Why this matters: the chart is currently unreadable.
- **Given** the trend chart has points
- **When** it renders
- **Then** it shows time labels on the horizontal axis, formatted values with units on the vertical axis, and a tooltip with the value and bucket time on hover.
- Evidence: a UI test asserting axis tick formatting and that a tooltip content renders for a point.

### AC-004 Trend header states metric, range, total and delta

- Why this matters: the metric selector must be self-explanatory.
- **Given** the operator switches the trend metric
- **When** the header renders
- **Then** it names the metric and range and shows the matching total and delta.
- Evidence: a UI test asserting the header text updates with the metric and range.

### AC-005 Requests delta is computed

- Why this matters: an absent delta next to peers looks broken.
- **Given** a previous period with a different request count
- **When** the Requests card renders
- **Then** it shows a delta consistent with the Spend and Tokens cards.
- Evidence: a UI test asserting the Requests card renders a delta when previous data exists.

### AC-006 Pass rate respects the workspace and is labeled honestly

- Why this matters: a pass rate that silently ignores the workspace is wrong.
- **Given** a workspace is selected
- **When** the pass-rate tile renders
- **Then** it is computed from that workspace's recent requests and labeled with its window and scope.
- Evidence: a UI test asserting the conversation request is tenant-scoped and the label states the window/scope.

### AC-007 Units and accessibility are explicit

- Why this matters: values without units cannot be interpreted or announced.
- **Given** the dashboard renders
- **When** a user or assistive technology reads the chart and cards
- **Then** the token/cost/request semantics and units are stated and the chart exposes an accessible name/description.
- Evidence: a UI test asserting the descriptive text and the chart's accessible role/title.

### AC-008 Empty, single-point, and zero data are explicit

- Why this matters: a flat line at zero reads as "traffic exists" when it does not.
- **Given** an empty series, a single bucket, or all-zero values
- **When** the chart and cards render
- **Then** an explicit empty/neutral state is shown instead of a misleading line.
- Evidence: UI tests for each case asserting the empty state and absence of a drawn trend.

## Assumptions

- Reusing and generalizing the existing `TimeSeriesChart` is acceptable (it already provides axes, ticks, tooltip and comparison), and the dashboard's bespoke chart is removed.
- The pass-rate window stays "the most recent requests" (client-side) because no server-side date filter exists for conversations; only tenant scoping and an honest label are required now.
- Cost currency is not exposed per request on the dashboard; the label states that cost is in the configured currency rather than adding a currency endpoint.
- The existing `Segmented` control remains the metric selector; it is relabeled/scoped to the chart.

## Success Criteria

- SC-001 No KPI tile plots a metric other than the one it names.
- SC-002 The trend chart conveys metric, period, units, and per-point value without opening another page.
- SC-003 Changing range or workspace updates every range/scope-dependent label and value on the page.

## Edge Cases

- A single bucket: show the point (or a neutral marker) with its value rather than a degenerate line.
- All-zero values: show a zero baseline with an explicit "no activity" note, not a filled area.
- Series load failure: surface a non-blocking error state for the chart section instead of an empty chart.
- Very large values: axis ticks and tooltip use compact formatting (K/M) without losing the unit.
- Custom range: the label reflects the custom window rather than a preset name.

## Open Questions

- none
