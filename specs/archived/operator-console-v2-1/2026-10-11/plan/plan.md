# Operator Console v2.1 Plan

## Phase Contract

Inputs: `specs/active/operator-console-v2-1/spec.md` and targeted repo context.
Outputs: `plan.md` (data model noted inline).
Stop if: the spec is too vague to plan safely.

## Goal

Frontend-only upgrade of the operator console: (1) the dashboard "Needs attention" panel becomes a shared, actionable inbox — stable item keys, deep links, real one-click actions where an API exists, snooze/ack, and a header critical-count badge; (2) the Routing page computes and shows each route's strategy (`single`/`priority`/`weighted`/`fallback`) with a weighted distribution preview and inline weight editing.

## MVP Slice

- A shared attention source consumed by the Dashboard panel and the header badge, with deep links, snooze/ack, and one real action (re-apply compliance pack). Must satisfy `AC-001`–`AC-004`.

## First Validation Path

1. Unit-test the strategy function (single/priority/weighted/fallback) and the attention store (ack/snooze).
2. Render tests: the header badge shows the un-acked critical count; clicking an item navigates; "Re-apply pack" calls the API.
3. Manual: change a weight on the Routing page and see the preview/strategy update.

## Scope

- Dashboard attention items: add `key`, `critical`, optional `action`.
- A shared `AttentionProvider` + `useAttention` hook (app root) so the panel and header share state and ack/snooze persistence.
- Header critical badge.
- Routing strategy view + weighted distribution preview + inline weight edit.
- Untouched: Go backend, APIs, design tokens.

## Performance Budget

- Attention computation stays at the current dashboard load cost; the compliance source is bounded to the active workspace tenant and best-effort.
- No new blocking calls added to the header; the badge reads shared state.

## Implementation Surfaces

- `ui/src/hooks/useAttention.ts` (new) — compute items, ack/snooze persistence, critical count.
- `ui/src/components/AttentionProvider.tsx` (new) — app-level context.
- `ui/src/App.tsx` — wrap the app in the provider.
- `ui/src/pages/Dashboard.tsx` — consume the shared items; render action buttons.
- `ui/src/components/Layout.tsx` — header critical badge.
- `ui/src/utils/routingStrategy.ts` (new) — pure strategy computation.
- `ui/src/pages/Routing.tsx` — strategy column/marker, distribution preview, inline weight edit.
- `ui/src/api/routing.ts` — reuse `listProviders`/`listRoutes`/`upsertProvider`.
- Tests: `routingStrategy.test.ts`, `useAttention.test.ts` (new), `Dashboard.test.tsx`, `Routing.test.tsx`, `Layout` (via existing component tests).

## Bootstrapping Surfaces

- none — the design system and API clients already exist.

## Architecture Impact

- Local: one hook + one context + one util; two pages and the header consume them.
- No backend/API/schema change.

## Acceptance Approach

- AC-001 → item→route mapping; surfaces `useAttention.ts`, `Dashboard.tsx`; proof: unit test on the mapping + render test navigation.
- AC-002 → compliance item action calls `applyCompliancePack`; surfaces `useAttention.ts`, `Dashboard.tsx`; proof: API mock records the call.
- AC-003 → ack/snooze persistence; surfaces `useAttention.ts`; proof: unit test with a fake clock/localStorage.
- AC-004 → header badge from shared state; surfaces `Layout.tsx`, `AttentionProvider.tsx`; proof: render test shows/hides the count.
- AC-005 → strategy function; surfaces `routingStrategy.ts`; proof: unit test for single/priority/weighted/fallback.
- AC-006 → distribution preview; surfaces `routingStrategy.ts`, `Routing.tsx`; proof: unit test 3:1 → 75/25.
- AC-007 → inline weight edit via `upsertProvider`; surfaces `Routing.tsx`, `api/routing.ts`; proof: API mock records the new weight; preview updates.
- AC-008 → no regression/no new endpoint; surfaces all; proof: `npx vitest run` + `npm run typecheck`; call-surface review.

## Data and Contracts

- Data model: no change — ack/snooze live in `localStorage`; no server state.
- Contracts: `AttentionItem` gains `key`, `critical`, optional `action`/`actionLabel`; a new `strategy` computed value (UI-only).
- Reuse: `listProviders`, `listRoutes`, `upsertProvider`, `applyCompliancePack`.

## Implementation Strategy

- DEC-001 One app-level `AttentionProvider` shared by the panel and the header
  Why: the header badge and the dashboard must read the same items and ack state without double-fetching. Not: fetching attention separately in Layout.
  Tradeoff: one more provider at the app root.
  Affects: `App.tsx`, `AttentionProvider.tsx`, `useAttention.ts`, `Dashboard.tsx`, `Layout.tsx`.
  Validation: render tests for both consumers.
- DEC-002 Stable item keys + client-side ack/snooze in localStorage
  Why: no backend state; ack/snooze must survive reloads and be scoped to one browser. Not: a server-side acknowledgement store.
  Tradeoff: not shared across devices; corrupted data is treated as expired (fail-open).
  Affects: `useAttention.ts`.
  Validation: unit test with faked time/storage.
- DEC-003 Compliance-deviation attention is bounded to the active workspace tenant
  Why: there is no global deviation endpoint; fanning out over all tenants would be N×packs calls. Not: querying every tenant.
  Tradeoff: deviation alerts are hidden when no workspace is selected.
  Affects: `useAttention.ts`.
  Validation: AC-002 test; degrade gracefully on error.
- DEC-004 Pure `routingStrategy` function
  Why: the strategy is deterministic from providers (priority/weight/health) and testable without rendering. Not: inferring it inline in JSX.
  Tradeoff: the label taxonomy must be documented.
  Affects: `routingStrategy.ts`, `Routing.tsx`.
  Validation: AC-005 unit test.
- DEC-005 Effective-weight distribution matches the selector
  Why: the preview must not lie about gateway behavior (`weight>0?weight:1`, min priority tier, healthy only). Not: naive raw weights.
  Tradeoff: mirrors backend semantics in the UI (a small duplication).
  Affects: `routingStrategy.ts`.
  Validation: AC-006 unit test.
- DEC-006 Inline weight edit reuses `upsertProvider` with the listed (masked) provider
  Why: the backend preserves masked secrets on upsert, so re-submitting the listed provider with a new weight is safe. Not: a new weight-only endpoint.
  Tradeoff: depends on the masked-secret preservation behavior.
  Affects: `Routing.tsx`, `api/routing.ts`.
  Validation: AC-007 test; the provider write path is already covered by backend tests.
- DEC-007 Header badge counts un-acked critical items only
  Why: acknowledged items must stop shouting; the badge is hidden at zero. Not: total attention count.
  Tradeoff: a critical item that is merely snoozed still hides the badge (by design).
  Affects: `AttentionProvider.tsx`, `Layout.tsx`.
  Validation: AC-004 test.

## Incremental Delivery

### MVP (First Value)

- `useAttention` + provider + Dashboard consumption with deep links, snooze/ack, header badge, and the re-apply compliance action.
- MVP readiness: `AC-001`–`AC-004` pass.

### Iterative Expansion

- `routingStrategy` + Routing strategy view + distribution preview → `AC-005`, `AC-006`.
- Inline weight editing → `AC-007`.
- Regression/no-endpoint verification → `AC-008`.

## Sequencing Notes

- `useAttention` + provider before Dashboard/Layout consumption.
- `routingStrategy` util before the Routing view.
- Keep the existing attention computations; only extend their shape and add the compliance source.

## Risks

- Compliance fan-out latency on the dashboard.
  Mitigation: bounded to the active tenant, best-effort, never blocks the panel.
- Provider upsert re-submitting masked keys.
  Mitigation: relies on the shipped `preserveMaskedSecrets`; covered by backend tests; the UI sends the listed provider unchanged except the weight.
- Strategy label ambiguity (priority vs fallback).
  Mitigation: documented taxonomy in `routingStrategy.ts`; unit-tested.
- localStorage cross-tab staleness.
  Mitigation: re-read on focus; acceptable for the MVP.

## Rollout and Compatibility

- Additive UI; no migration or flag. Existing Dashboard/Routing behavior preserved; the "Needs attention" list keeps its current items and gains actions.

## Validation

- Automated: `npx vitest run` and `npm run typecheck` in `ui/`.
- New tests: strategy function, attention store (ack/snooze with fake clock), Dashboard action/navigation, header badge, Routing weight edit.
- Proves: `AC-001`–`AC-008`; `DEC-001`–`DEC-007`.

## Constitution Compliance

- no conflicts. Operator control-plane UI only; no end-user/chat surface; no backend or DLP change.
