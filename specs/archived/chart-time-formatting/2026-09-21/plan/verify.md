---
report_type: verify
slug: chart-time-formatting
status: pass
docs_language: en
generated_at: 2026-09-21
---

# Verify Report: chart-time-formatting

## Scope

- snapshot: charts serve hourly buckets for ranges up to 7 days and format axis/tooltip labels from the requested range span, removing fake `00:00` and mixed label formats.
- verification_mode: default
- artifacts:
  - CONSTITUTION.md
  - specs/active/chart-time-formatting/spec.md
  - specs/active/chart-time-formatting/plan.md
  - specs/active/chart-time-formatting/tasks.md
- inspected_surfaces:
  - src/internal/adapters/repository/analytics/pg_usage_store.go (+ test)
  - ui/src/utils/format.ts (+ test)
  - ui/src/components/TimeSeriesChart.tsx (+ test)
  - ui/src/pages/Dashboard.tsx, ui/src/pages/Analytics.tsx

## Verdict

- status: pass
- archive_readiness: safe
- summary: all 7 tasks carry `Proof:` lines, the bucket-boundary and formatter tests pass, both callers pass the range span, and the Analytics/Dashboard suites stay green.

## Checks

- task_state: completed=7, open=0 (verify-task-state.sh: `OK: all tasks are marked complete`; `PROOFS_MISSING=0`).
- acceptance_evidence:
  - AC-001 -> `TestResolveBucket` (hour at 1h/48h/7d, day at 7d+1m/30d).
  - AC-002 -> `chartTickLabel` tests (`14:05` / `21.09 14:05` / `21.09`); `TimeSeriesChart` passes `spanMs` from `rangeSpanMs` in both Dashboard and Analytics.
  - AC-003 -> `chartTickLabel` date-only assertion for a 30-day and an infinite span (no time component).
  - AC-004 -> `chartTooltipLabel` test (`21.09.2026 14:05`).
  - AC-005 -> `TimeSeriesChart span and density` renders 168 hourly points with an explicit week span without error; the axis keeps `minTickGap`/`interval="preserveStartEnd"` (code inspection).
  - AC-006 -> Analytics (2 tests) and Dashboard (11 tests) suites pass unchanged.
- implementation_alignment:
  - `resolveBucket` threshold is 7 days inclusive and is shared by both time-series queries.
  - The chart formats ticks/tooltips through the shared `format.ts` helpers and falls back to the data span when no span is supplied.
  - Callers pass `rangeSpanMs(range.mode, from, to)`, so presets without explicit bounds still yield the correct span.

## Errors

- none

## Warnings

- none

## Questions

- none

## Not Verified

- Rendered SVG axis tick text is not asserted in jsdom (recharts internals); formatting is verified by unit tests plus the caller wiring.
- Visual tick spacing on a real 7-day hourly chart was not checked in a browser; only the no-throw rendering of 168 points is asserted.
- 30d/All intentionally remain daily (out of scope), so they are not exercised for hourly behaviour.

## Next Step

- safe to archive
