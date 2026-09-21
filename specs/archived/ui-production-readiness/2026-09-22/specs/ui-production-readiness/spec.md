# UI Production Readiness

## Scope Snapshot

- In scope: make the operator console production-grade — every authenticated call goes through the shared client, the app shell is crash-contained, the admin session cookie is hardened, and the UI becomes consistent (shared modal, unified async states, design-system primitives, server-side pagination/search, global workspace scope with shareable URLs, and an honest live indicator).
- Out of scope: new product features, visual redesign, i18n, Storybook/visual regression, and any change to the data plane or DLP.

## Goal

The console is functional but reads as unfinished: some requests bypass the shared client (no auth header, no global 401), a page crash in the shell is uncaught, the session cookie is not hardened for TLS, modals lack focus management, loading/empty/error states differ per page, half the pages hand-roll markup instead of the design system, lists load everything client-side, the workspace scope applies to only some pages, filters are lost on navigation, and the "Live" pill is decorative. This feature makes the console trustworthy and consistent end to end, so an operator can use it all day and a security reviewer can sign off.

## Primary User Flow

1. Starting point: an operator logs in and opens the console.
2. Main interaction: they move between pages, filter/search/paginate lists, change the workspace, open and close modals, and leave a page open.
3. Outcome: every page behaves the same way (same loading, empty, error+retry, modal behavior), the workspace and filters persist and are shareable via the URL, lists page on the server, and the live indicator tells the truth.
4. Failure/fallback path: an expired session sends the operator to login from any page; a crashing page shows a contained error with a retry, not a blank app.

## User Stories

- P1 Story: as an operator, an expired session never leaves me on a broken page — I am sent to login.
- P2 Story: as an operator, every list supports search and paging, and my filters survive navigation and are shareable.
- P3 Story: as an operator, the workspace I pick scopes every page, and the "Live" indicator is real.
- P4 Story: as a security reviewer, the admin session cookie is hardened and all authenticated traffic uses one client.

## MVP Slice

- P0 (shared client for all authenticated calls, dead code removal, shell error containment, cookie hardening).
- AC covered: AC-001, AC-002, AC-003, AC-004.

## First Deployable Outcome

- After the first pass, conversations use the shared client, an expired session redirects to login from any page, a shell error is contained, and the admin cookie is Secure/SameSite on HTTPS.

## Scope

- Frontend: shared API client adoption, shell error boundary, shared modal primitive, unified async state, design-system adoption, pagination/search controls, workspace scope + URL-persisted filters, live indicator.
- Backend: pagination/search for the admin list endpoints that lack it (tenants, keys, budgets), and cookie hardening on login.
- CI: a typecheck script and lint/test/build gates for the UI.

## Out of Scope

- Visual redesign or new screens; dark/light token changes.
- New backend analytics/metrics.
- i18n, Storybook, visual regression, a11y certification beyond the checks listed here.
- Any data-plane, DLP, or routing behavior change.

## Context

- `ui/src/api/client.ts` centralizes auth headers and global 401 handling, but `ui/src/api/conversations.ts` still uses raw `fetch` (no auth header, no 401 handling) and `ui/src/api/profiles.ts` targets a removed endpoint.
- The error boundary wraps only route content (`ui/src/App.tsx`), so a crash in the shell is uncaught.
- The admin login handler sets the session cookie with `secure=false` and no explicit `SameSite`.
- Modals are hand-rolled per page; only `ConfirmModal`/`DictionaryModal` handle Escape, and none trap focus or set `aria-labelledby`.
- Pages mix `Spinner`, plain "Loading…" text, and silent catches; `Card`/`PageHeader` are effectively unused while `Badge` and `StatusPill` (and two progress implementations) coexist.
- Admin list endpoints vary: tenants/keys/budgets return everything; audit and conversations support paging. The workspace switcher is consumed by only some pages, and filters/ranges are not in the URL.
- The header "Live" pill is static markup, and only Sessions auto-refreshes.

## Dependencies

- Existing `apiFetch` client and `useAsyncData` hook.
- Existing design-system primitives (`ui/src/components/ui/`).
- Admin list handlers and the audit/conversations paging pattern as the reference.
- No new external dependency.

## Requirements

- RQ-001 Every authenticated admin UI request MUST go through the shared API client so it carries the session token and participates in global 401 handling.
- RQ-002 Orphaned API clients and UI code that target removed endpoints MUST be deleted.
- RQ-003 A failure in the application shell (layout, header, command palette) MUST be contained by an error boundary with a recover action, not render a blank app.
- RQ-004 The admin session cookie MUST set `Secure` when the request is HTTPS and MUST set an explicit `SameSite` policy.
- RQ-005 A single shared modal primitive MUST provide focus trap, initial focus, Escape-to-close, `aria-labelledby`, and background scroll lock; every modal MUST use it.
- RQ-006 Every data page MUST render the same async states — skeleton while loading, a meaningful empty state, and an error state with retry — and MUST NOT swallow errors silently.
- RQ-007 Pages MUST use the design-system primitives (Card, PageHeader, EmptyState, StatusPill, Button, ProgressBar) instead of hand-rolled equivalents, and there MUST be a single status and a single progress implementation.
- RQ-008 The tenant, key, budget, conversation, and audit lists MUST support server-side pagination and search with a total count, and the UI MUST expose paging and search controls.
- RQ-009 The selected workspace MUST scope every tenant-scoped page, and list filters/ranges MUST persist in the URL so a view is shareable and survives navigation.
- RQ-010 The header live indicator MUST reflect real backend health (or be removed), and the pages where freshness matters MUST refresh automatically.
- RQ-011 The UI MUST have CI gates: a typecheck script, lint, tests, and a production build MUST pass in CI.

## Non-Goals

- Changing the visual language, adding new product capabilities, or rewriting the charting stack.
- Replacing `useAsyncData` with a third-party data-caching library.
- Adding client-side state management beyond URL and existing hooks.

## Acceptance Criteria

### AC-001 All authenticated calls use the shared client

- Why this matters: a call without the token silently fails and bypasses logout.
- **Given** the operator console
- **When** any authenticated resource is fetched (including conversations)
- **Then** the request carries the session token and a 401 triggers the global re-login path.
- Evidence: a test asserting conversations use the shared client and that a 401 from it dispatches the unauthorized event.

### AC-002 Dead code targeting removed endpoints is gone

- Why this matters: orphaned clients and screens mislead maintainers and can 404 at runtime.
- **Given** the repository
- **When** the orphaned API client and its unused UI are removed
- **Then** no code references the removed endpoints and lint/tests pass.
- Evidence: grep shows no references to the removed endpoints; build and tests pass.

### AC-003 Shell failures are contained

- Why this matters: a shell crash must not blank the app.
- **Given** a component inside the shell throws during render
- **When** the app renders
- **Then** an error boundary shows a recover action and the app remains usable after reset.
- Evidence: a test rendering a throwing child under the shell boundary and asserting the fallback plus reset.

### AC-004 Session cookie is hardened

- **Given** an HTTPS admin login
- **When** the session cookie is set
- **Then** it is `Secure` with an explicit `SameSite` policy; over plain HTTP it stays non-secure so local development still works.
- Evidence: a handler test asserting cookie attributes for HTTPS and HTTP.

### AC-005 Shared modal behavior

- Why this matters: keyboard users must be able to operate every dialog.
- **Given** any modal in the console
- **When** it opens and the operator presses Escape or tabs
- **Then** focus moves into the dialog, is trapped inside it, Escape closes it, and it exposes an accessible name.
- Evidence: UI tests on the shared modal (initial focus, Escape, aria-labelledby) and a check that pages use it.

### AC-006 Unified async states

- Why this matters: inconsistent loading/error handling looks unfinished and hides failures.
- **Given** any data page
- **When** its data is loading, empty, or fails
- **Then** it shows a skeleton, a meaningful empty state, or an error with a retry action respectively.
- Evidence: page tests covering loading, empty, and error+retry.

### AC-007 Design-system adoption

- Why this matters: duplicated implementations drift.
- **Given** the pages
- **When** they render cards, headers, empty states, statuses, and progress
- **Then** they use the shared primitives and only one status and one progress implementation remain.
- Evidence: a check that the duplicate implementations are removed and pages import the primitives; UI tests pass.

### AC-008 Server-side pagination and search

- Why this matters: loading everything client-side breaks at scale.
- **Given** more records than a page
- **When** the operator searches or pages a tenant/key/budget/conversation/audit list
- **Then** the server returns only that page with a total, and the UI reflects page and total.
- Evidence: handler tests for limit/offset/search and UI tests for paging/search controls.

### AC-009 Global workspace scope and shareable filters

- Why this matters: a workspace switch must not be silently ignored, and a filtered view must be shareable.
- **Given** a workspace is selected and a page has filters
- **When** the operator navigates away and back or shares the URL
- **Then** the same workspace and filters are applied, and every tenant-scoped page honors the workspace.
- Evidence: UI tests for workspace application across pages and URL round-trip of filters.

### AC-010 Honest live indicator and refresh

- Why this matters: a decorative "Live" pill is misleading.
- **Given** the header indicator
- **When** backend health changes
- **Then** the indicator reflects the aggregated status, and pages where freshness matters refresh automatically.
- Evidence: a test asserting the indicator reflects status and an auto-refresh test.

### AC-011 UI CI gates

- Why this matters: regressions must be caught before merge.
- **Given** a pull request
- **When** CI runs
- **Then** typecheck, lint, tests, and the production build must pass.
- Evidence: CI workflow includes the UI steps and they pass locally.

## Assumptions

- "Authenticated" means admin-session-scoped resources; login/logout and the app bootstrap verify call may keep direct fetch.
- Server-side search is a substring match on the natural key (slug/name/label/model) unless an endpoint already defines otherwise.
- Pagination defaults follow the existing audit/conversations pattern (limit/offset or page/per_page with a total).
- URL persistence uses query parameters and the existing `useWorkspace`/range helpers.
- Cookie hardening keeps local HTTP development working (Secure only on HTTPS).

## Success Criteria

- SC-001 No authenticated request bypasses the shared client.
- SC-002 Every page presents the same loading/empty/error contract and uses the shared modal.
- SC-003 Lists remain responsive with thousands of records (server-side paging).
- SC-004 An expired session always lands on login, from any page.

## Edge Cases

- 401 during a background refresh: redirect to login without a crash loop.
- Empty search result vs empty dataset: distinct messages.
- Modal opened from a drawer/overlay: focus returns to the trigger on close.
- Workspace with no data: page shows an empty state scoped to that workspace.
- Health unknown (status endpoint unreachable): indicator shows unknown, not "Live".
- Very large page size: clamped by the server to a maximum.

## Open Questions

- none
