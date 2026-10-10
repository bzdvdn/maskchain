# Operator Console v2.1 Tasks

## Phase Contract

Inputs: `specs/active/operator-console-v2-1/plan.md`, `spec.md`.
Outputs: ordered executable tasks with coverage mapping.
Stop if: tasks would be vague or acceptance coverage cannot be mapped.

## Surface Map

| Surface | Tasks |
|---------|-------|
| ui/src/hooks/useAttention.ts | T1.1 |
| ui/src/components/AttentionProvider.tsx | T1.2 |
| ui/src/App.tsx | T1.2 |
| ui/src/hooks/useAttention.test.ts | T1.3 |
| ui/src/pages/Dashboard.tsx | T2.1 |
| ui/src/components/Layout.tsx | T2.2 |
| ui/src/pages/Dashboard.test.tsx | T2.3 |
| ui/src/components/Layout.test.tsx | T2.3 |
| ui/src/utils/routingStrategy.ts | T3.1 |
| ui/src/utils/routingStrategy.test.ts | T3.2 |
| ui/src/pages/Routing.tsx | T3.3 |
| ui/src/pages/Routing.test.tsx | T3.4 |
| ui/ (vitest + typecheck) | T4.1 |

## Implementation Context

- MVP goal: a shared attention inbox with deep links, snooze/ack, a re-apply-pack action, and a header critical badge (AC-001–AC-004).
- Invariants/semantics: `AttentionItem = { key, tone, critical, title, detail, to, actionLabel?, action? }` with a stable `key` (kind + target); ack/snooze persisted in `localStorage` keyed by the item key and fail-open on corrupt data; the header badge counts un-acked critical items and hides at 0; the compliance-deviation source is bounded to the active workspace tenant (skipped when no workspace); strategy taxonomy = `single` (1 provider), `priority` (same tier, unweighted, ≥2), `weighted` (same min tier, any weight > 0), `fallback` (providers span > 1 priority tier); effective weight = `weight > 0 ? weight : 1`, healthy providers only.
- Errors/codes: provider save failure reverts the optimistic weight and shows an error; corrupt snooze timestamp is treated as expired (item visible).
- Contracts/protocols: no new endpoints; reuse `listProviders`/`listRoutes`/`upsertProvider`/`applyCompliancePack`; inline weight edit re-submits the listed provider (masked secrets preserved by the backend).
- Scope boundaries: no backend/API/schema changes; do not add Incidents, blocked-category breakdown, provider ok-rate/rerouted/retry, real-time, responsive, i18n, RPM/TPM, or SSO/MFA UI.
- Proof signals: unit tests for `routingStrategy` (4 cases + distribution) and `useAttention` (ack/snooze, compliance bound); render tests for Dashboard navigation/action and the header badge; `npx vitest run` + `npm run typecheck` clean.
- References: DEC-001..DEC-007, RQ-001..RQ-008 (no mandatory re-read).

## Phase 1: Attention store and provider

Goal: one source of attention items with persistence, shared app-wide.

- [x] T1.1 Implement `useAttention`: compute items (budgets, providers, expiring keys, workspace-bounded compliance deviation), assign stable `key`/`critical`/optional `action`, and persist ack/snooze in localStorage (DEC-002, DEC-003, AC-001, AC-002, AC-003, AC-004). Touches: ui/src/hooks/useAttention.ts
      Proof: code ui/src/hooks/useAttention.ts useAttention
- [x] T1.2 Add `AttentionProvider` and wrap the app so the panel and header share state (DEC-001, AC-004). Touches: ui/src/components/AttentionProvider.tsx, ui/src/App.tsx
      Proof: code ui/src/components/AttentionProvider.tsx AttentionProvider
- [x] T1.3 Add store tests: ack hides an item, snooze reappears after expiry (fake clock), corrupt snooze fails open, compliance deviation is bounded to the workspace (AC-002, AC-003). Touches: ui/src/hooks/useAttention.test.ts
      Proof: test ui/src/hooks/useAttention.test.ts

## Phase 2: Dashboard inbox and header badge (MVP)

Goal: the actionable inbox and the critical badge.

- [x] T2.1 Consume the shared items in Dashboard: render the action button when present, deep links on click, and drop the local attention state (AC-001, AC-002). Touches: ui/src/pages/Dashboard.tsx
      Proof: code ui/src/pages/Dashboard.tsx
- [x] T2.2 Show the un-acked critical count in the header, hidden at zero (DEC-007, AC-004). Touches: ui/src/components/Layout.tsx
      Proof: code ui/src/components/Layout.tsx
- [x] T2.3 Add render tests: clicking an item navigates to its target; "Re-apply pack" calls the API; the header badge reflects the count (AC-001, AC-002, AC-004). Touches: ui/src/pages/Dashboard.test.tsx, ui/src/components/Layout.test.tsx
      Proof: test ui/src/components/Layout.test.tsx

## Phase 3: Routing strategy view

Goal: surface the computed routing strategy and allow inline weight editing.

- [x] T3.1 Implement the pure `routingStrategy` util (taxonomy + weighted distribution from effective weights, healthy-only) (DEC-004, DEC-005, AC-005, AC-006). Touches: ui/src/utils/routingStrategy.ts
      Proof: code ui/src/utils/routingStrategy.ts computeRouteStrategy
- [x] T3.2 Add unit tests for single/priority/weighted/fallback and the 3:1 → 75/25 distribution (AC-005, AC-006). Touches: ui/src/utils/routingStrategy.test.ts
      Proof: test ui/src/utils/routingStrategy.test.ts
- [x] T3.3 Render the strategy (label + distribution preview) on the Routing page and add inline weight editing that persists via `upsertProvider` and updates the preview (DEC-005, DEC-006, AC-006, AC-007). Touches: ui/src/pages/Routing.tsx
      Proof: code ui/src/pages/Routing.tsx
- [x] T3.4 Add Routing tests: strategy/preview render, weight edit persists (API mock) and reverts on failure (AC-007). Touches: ui/src/pages/Routing.test.tsx
      Proof: test ui/src/pages/Routing.test.tsx

## Phase 4: Validation

Goal: prove the feature end to end and leave the package reviewable.

- [x] T4.1 Run the UI suite (`npx vitest run`) and `npm run typecheck`; fix regressions and confirm no new backend endpoint is called (AC-008). Touches: ui/
      Proof: chore ui/package.json typecheck

## Acceptance Coverage

- AC-001 -> T1.1, T2.1, T2.3
- AC-002 -> T1.1, T1.3, T2.1, T2.3
- AC-003 -> T1.1, T1.3
- AC-004 -> T1.2, T2.2, T2.3
- AC-005 -> T3.1, T3.2
- AC-006 -> T3.1, T3.2, T3.3
- AC-007 -> T3.3, T3.4
- AC-008 -> T4.1

## Notes

- Frontend-only: no Go changes. Validation is `npx vitest run` + `npm run typecheck` in `ui/`.
- MVP is Phase 2; Phase 1 is its prerequisite, Phase 3 is independent polish.
- Migrate the Dashboard's existing attention computation into the hook rather than duplicating it.
- Every `[x]` task needs a `Proof:` line on the next line; no proof means not done.
