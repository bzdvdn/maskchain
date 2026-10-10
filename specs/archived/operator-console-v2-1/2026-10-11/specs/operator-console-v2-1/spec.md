# Operator Console v2.1

## Scope Snapshot

- In scope: turn the dashboard's passive "Needs attention" panel into an actionable inbox with deep links, available one-click actions, snooze/ack and a header critical badge; surface the shipped routing/weighted-LB as a computed strategy view on the Routing page (with inline weight editing).
- Out of scope: any new backend endpoint; an Incidents page; blocked-by-category breakdown; provider ok-rate/rerouted/retry; real-time push; responsive/i18n; RPM/TPM or SSO/MFA UI.

## Goal

Operators get two upgrades over the current console (and over the static mockups): (1) every "Needs attention" signal becomes a one-click path to its fix, with acknowledgement so a known issue stops shouting, and a critical count in the header; (2) the routing table shows what the gateway will actually do — single / priority-tier / weighted (with a distribution preview) / fallback — and lets the operator adjust weights inline. Everything is frontend-only and uses existing admin APIs.

## Primary User Flow

1. Starting point: an operator opens Operations HQ and sees the Needs-attention inbox with a critical badge in the header.
2. Main interaction: they click an item to jump to the exact page (and tenant/filter), or use its action (re-apply a compliance pack, open the budget editor, open the expiring keys); they can acknowledge or snooze items.
3. Outcome: the item is resolved or acknowledged; the critical badge count updates; the inbox only shows live signals.
4. Routing: on the Routing page the operator sees each route's strategy; for a weighted route they see the distribution and can change a provider weight inline; the strategy preview updates.

## User Stories

- P1 Story: as an operator, an alert takes me straight to where I fix it, not to a page I then have to search.
- P2 Story: as an operator, I can acknowledge a known issue so it stops counting as critical.
- P3 Story: as an operator, I can see and tune how traffic is split across a weighted route.
- P4 Story: as an operator, the header tells me at a glance whether anything is critical.

## MVP Slice

- Actionable Needs-attention inbox (deep links, snooze/ack, header critical badge) and the Routing strategy view with inline weight editing. Must satisfy `AC-001`–`AC-008`.

## First Deployable Outcome

- Clicking a dashboard alert navigates to the right page; "Re-apply pack" resolves a deviation; the header badge drops when items are acknowledged.
- The Routing page shows a strategy badge and a 70/30 preview for a weighted route, and editing a weight updates both.

## Scope

- Dashboard `Needs attention`: each item gets a deep link and, when an API exists, a primary action; add snooze and acknowledge (client-side persistence) and exclude acknowledged/snoozed items from counts.
- Header: a critical-count badge fed by the same attention source.
- Routing page: computed per-route strategy (`single`, `priority`, `weighted`, `fallback`) from provider priority/weight/health; a weighted distribution preview; inline weight editing persisted through the provider API.
- Reuse existing components (`Card`, `StatusPill`, `Segmented`, buttons) and existing API clients.

## Context

- The Dashboard already computes attention items (budget ≥70%/100%, provider down/degraded, expiring keys, compliance deviation) but they are passive text.
- Compliance apply (`POST /api/v1/tenants/:slug/compliance/apply`) and provider upsert (`PUT /api/v1/routing/providers`) exist; there is **no** incident API, no blocked-category aggregation, and no provider retry/ok-rate endpoint.
- The routing/weighted-LB feature is shipped: provider `weight` and `priority` are persisted; the selector picks the minimum priority tier then weighted-random (effective weight = `weight>0 ? weight : 1`; unweighted tier → first declared).
- Routes are read from `GET /api/v1/routing/routes`; providers from `GET /api/v1/routing/providers`.

## Dependencies

- Existing admin APIs (analytics, budgets, keys, routing/providers, tenants/compliance) and the v2 UI primitives. No backend change.

## Requirements

- RQ-001 Each Needs-attention item MUST expose a deep link that navigates to the exact page (and, where applicable, the tenant/filter) that resolves it.
- RQ-002 An item MUST expose a primary action only when an existing API can perform it (re-apply compliance pack; open the budget editor; open expiring keys); items without a backend action MUST NOT show a fake action.
- RQ-003 The operator MUST be able to snooze an item for a bounded duration and to acknowledge it; snoozed/acknowledged items MUST be hidden until the snooze expires and MUST NOT count toward critical totals.
- RQ-004 The header MUST show a critical-count badge whenever at least one un-acknowledged critical item exists.
- RQ-005 The Routing page MUST show a computed strategy per route: `single`, `priority`, `weighted`, or `fallback`, derived from the route's providers (priority, weight, health).
- RQ-006 A `weighted` route MUST show a distribution preview whose shares match the effective-weight semantics used by the selector.
- RQ-007 A provider weight MUST be editable inline from the Routing strategy view and persist via the provider API; the preview MUST update after a successful save.
- RQ-008 The changes MUST use only existing admin endpoints and MUST NOT regress existing Dashboard/Routing behavior.

## Non-Goals

- Any backend endpoint (incidents, blocked-category aggregation, provider ok-rate/rerouted/retry).
- An Incidents page or real-time SSE updates.
- Responsive layout, density toggle, light-theme rework, i18n.
- RPM/TPM or SSO/MFA UI.

## Acceptance Criteria

### AC-001 Attention items deep-link to the fix

- Why this matters: an alert leads straight to the fix.
- **Given** a budget, provider, key-expiry, or compliance-deviation item
- **When** the operator clicks it
- **Then** the app navigates to the page that resolves it (budgets, providers, keys, or the tenant compliance tab).
- Evidence: navigation target matches the item kind (unit test on the item→route mapping; manual click).

### AC-002 Available actions perform a real call

- Why this matters: no fake actions.
- **Given** a compliance-deviation item on tenant `t`
- **When** the operator clicks "Re-apply pack"
- **Then** `applyCompliancePack(t, pack)` is called and the inbox refreshes.
- Evidence: the API mock records the call; the item updates after reload.

### AC-003 Snooze and acknowledge

- Why this matters: a known issue stops shouting.
- **Given** an attention item
- **When** the operator snoozes it (bounded duration) or acknowledges it
- **Then** it is hidden and excluded from counts, and reappears after the snooze expires.
- Evidence: persistence in localStorage; the hidden item returns when the stored snooze time passes (test with a fake clock).

### AC-004 Header critical badge

- Why this matters: at-a-glance severity.
- **Given** N un-acknowledged critical items
- **When** any page is rendered
- **Then** the header shows a badge with count N (hidden when N is 0).
- Evidence: header test renders the count and hides at 0.

### AC-005 Routing strategy is computed correctly

- Why this matters: the table must reflect real gateway behavior.
- **Given** routes with (a) one provider, (b) two same-priority weighted providers, (c) providers in different priority tiers, (d) a primary plus a lower-tier provider
- **When** the Routing page renders
- **Then** each route is labeled `single`, `weighted`, `priority`, or `fallback` respectively.
- Evidence: a unit test on the strategy function covers all four cases.

### AC-006 Weighted distribution preview

- Why this matters: operators must see the split.
- **Given** same-tier providers with weights 3 and 1
- **When** the strategy view renders
- **Then** the preview shows 75% / 25%.
- Evidence: a unit test asserts the computed shares from effective weights.

### AC-007 Inline weight edit persists

- Why this matters: tuning from one screen.
- **Given** a weighted route
- **When** the operator changes a provider weight and saves
- **Then** the provider is upserted and the preview/strategy update.
- Evidence: the provider API mock records the new weight; the rendered preview matches.

### AC-008 No regression / backend-only-reuse

- Why this matters: keep the console stable.
- **Given** the existing Dashboard and Routing pages
- **When** the changes ship
- **Then** existing tests pass and no new backend endpoint is called.
- Evidence: the UI test suite passes; a review of the call surface shows only existing endpoints.

## Assumptions

- Attention items are the ones already computed by the Dashboard.
- Snooze/ack persistence is client-side (localStorage) — no backend state.
- Effective weight = `weight > 0 ? weight : 1`; providers in the same minimum priority tier are weighted; a single tier with unset weights keeps declaration order.
- Provider health (`status`) is available from the providers API for the strategy label.

## Success Criteria

- SC-001 The attention inbox renders without extra blocking API calls beyond the current dashboard loads.
- SC-002 No layout regression on Dashboard/Routing at the common desktop widths; light/dark both render.

## Edge Cases

- All attention items acknowledged/snoozed: badge hidden, inbox shows an "all clear" state.
- A route references a provider that no longer exists: strategy degrades gracefully (label as `unknown`/`broken`) without crashing.
- A weighted route where an unhealthy provider is excluded: preview reflects only healthy providers.
- Snooze stored with a corrupted timestamp: treated as expired (fail-open to visible).
- Provider save fails: the inline editor shows an error and reverts the optimistic value.

## Open Questions

- Default snooze durations (e.g., 1h / 24h / 7d) and whether snooze/ack should be per-tenant-scoped.
- Should the strategy view also address aliases (aliased routes show the target's strategy)?
- Follow-up specs: backend for blocked-category breakdown, provider ok-rate/`Retry now`, and an Incidents view (mockup parity) — dependencies for the fuller mockups.
