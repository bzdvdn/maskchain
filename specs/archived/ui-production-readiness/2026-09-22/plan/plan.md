# UI Production Readiness Plan

## Phase Contract

Inputs: `spec.md` + minimal repo context.
Outputs: `plan.md` (no data-model/contracts expansion).
Stop if: spec is vague — not the case; decisions pinned below.

## Goal

Harden and unify the operator console: route all authenticated calls through the shared client, contain shell crashes, harden the session cookie, and standardize the UI (one modal, one async-state contract, design-system primitives, server-side paging/search, global workspace scope with URL-persisted filters, and an honest live indicator). Delivered in increments: P0 correctness first, then consistency, then scale/UX, then CI gates.

## MVP Slice

- P0: shared client for all authenticated calls, dead-code removal, shell error boundary, cookie hardening.
- AC covered: AC-001, AC-002, AC-003, AC-004.

## First Validation Path

1. Open Conversations → requests carry the token; expire the session → any page redirects to login.
2. Force a shell render error → contained fallback with reset.
3. Login over HTTPS → cookie is `Secure` + `SameSite`; over HTTP it stays non-secure locally.

## Scope

- Frontend: API client adoption, shell boundary, shared modal, unified async state, primitives, pagination/search controls, workspace + URL filters, live indicator.
- Backend: pagination/search for tenants/keys/budgets; cookie hardening.
- CI: typecheck/lint/test/build gates for the UI.

## Out of Scope

- Visual redesign, new features, i18n, Storybook/visual regression, data-plane/DLP changes.

## Performance Budget

- Live indicator polls the status endpoint at a low cadence (>= 15s) and pauses when the tab is hidden. List endpoints must page (no full-table fetches).

## Implementation Surfaces

- `ui/src/api/conversations.ts` — migrate to `apiFetch`; delete `ui/src/api/profiles.ts`.
- `ui/src/components/DictionaryEditor.tsx`, `ui/src/components/PreprocessorEditor.tsx`, `ui/src/components/ProtectedRoute.tsx` — remove orphaned code.
- `ui/src/__tests__/api.test.ts` — drop removed-endpoint assertions.
- `ui/src/App.tsx`, `ui/src/components/ErrorBoundary.tsx` — shell-level boundary.
- `src/internal/api/handler/admin/admin_auth_handler.go` — cookie Secure/SameSite.
- `ui/src/components/ui/Modal.tsx` (new) + `index.ts`; migrate `ConfirmModal`, `DictionaryModal`, and page modals (Keys, Budgets, Routing, Providers, Models, Conversations, Tenants).
- `ui/src/hooks/useAsyncData.ts` + `ui/src/components/ui/AsyncSection.tsx` (new) — unified loading/empty/error+retry.
- `ui/src/components/ui/Badge.tsx` — remove after migrating call sites to `StatusPill`; unify progress on `ProgressBar`.
- `ui/src/hooks/useUrlFilters.ts` (new) — URL-persisted filters/ranges; pages adopt.
- `ui/src/components/Layout.tsx` — live indicator from status; auto-refresh wiring.
- `src/internal/api/handler/admin/{tenant_handler.go,virtual_key_handler.go,budget_handler.go}` + repositories — limit/offset/search + total.
- `ui/src/api/{tenants,keys,budgets}.ts` + list pages — paging/search params.
- `ui/package.json`, `.github/workflows/ci.yml` — `typecheck` script and UI CI job.

## Bootstrapping Surfaces

- none — client, primitives, hooks and handlers exist.

## Architecture Impact

- Local: one new modal primitive and one async-state component; hooks for URL filters.
- Integration: admin list endpoints gain paging/search params (additive; existing callers keep working with defaults).
- Compatibility: cookie attributes change only on HTTPS; no data-plane change.

## Acceptance Approach

- AC-001 → conversations use `apiFetch`; proof: api test + unauthorized-event test.
- AC-002 → remove orphan client/editors/route; proof: grep + build/tests.
- AC-003 → shell boundary; proof: boundary test with a throwing child + reset.
- AC-004 → cookie attributes; proof: handler test for HTTPS and HTTP.
- AC-005 → shared modal; proof: focus/Escape/aria tests + page migration.
- AC-006 → unified async; proof: page tests for loading/empty/error+retry.
- AC-007 → primitives + single status/progress; proof: removed duplicates + tests.
- AC-008 → paging/search; proof: handler tests (limit/offset/search/total) + UI controls.
- AC-009 → workspace + URL; proof: workspace-across-pages test + URL round-trip.
- AC-010 → live indicator + auto-refresh; proof: indicator test + refresh test.
- AC-011 → CI gates; proof: workflow steps and a local run.

## Data and Contracts

- Data model: no change.
- API contract: tenants/keys/budgets list endpoints gain optional `limit`/`offset`/`search` and a `total` in the envelope pagination; additive and backward compatible.
- Config contract: none.
- No `contracts/*` or `data-model.md` needed.

## Implementation Strategy

- DEC-001 Migrate remaining authenticated calls to `apiFetch` and delete orphaned code
  Why: one auth/401 path; removed endpoints must not linger.
  Tradeoff: deleting files and test assertions.
  Affects: conversations.ts, profiles.ts, orphan editors/route, api.test.ts.
  Validation: build/tests + grep.
- DEC-002 App-level error boundary in addition to per-page
  Why: a shell crash currently blanks the app.
  Tradeoff: two boundaries to reason about.
  Affects: App.tsx, ErrorBoundary.tsx.
  Validation: boundary test.
- DEC-003 Cookie hardening via `SetSameSite` + HTTPS detection (`r.TLS` or `X-Forwarded-Proto`)
  Why: security; keep local HTTP working.
  Tradeoff: proxy-aware detection.
  Affects: admin_auth_handler.go.
  Validation: handler test.
- DEC-004 One shared `Modal` primitive with focus trap and a11y
  Why: keyboard accessibility and consistency.
  Tradeoff: migrate every modal.
  Affects: ui/components/ui/Modal.tsx + pages.
  Validation: modal tests.
- DEC-005 Unified async state via `useAsyncData` error/retry + an `AsyncSection` component
  Why: one loading/empty/error contract; no silent catches.
  Tradeoff: page refactor.
  Affects: useAsyncData.ts, AsyncSection.tsx, pages.
  Validation: page tests.
- DEC-006 Consolidate design system (remove `Badge`, single progress, adopt Card/PageHeader/EmptyState)
  Why: duplicated implementations drift.
  Tradeoff: broad but mechanical edits.
  Affects: Badge.tsx, pages.
  Validation: tests + import checks.
- DEC-007 Server-side pagination/search on tenants/keys/budgets
  Why: client-side full loads do not scale.
  Tradeoff: backend + repo + UI work; must stay backward compatible.
  Affects: handlers, repos, UI clients/pages.
  Validation: handler tests + UI controls.
- DEC-008 Global workspace scope + URL-persisted filters via hooks
  Why: a workspace switch must not be ignored; views must be shareable.
  Tradeoff: pages adopt a shared hook.
  Affects: useUrlFilters.ts, pages.
  Validation: workspace + URL tests.
- DEC-009 Live indicator from the status endpoint with low-cadence polling
  Why: an honest indicator; decorative UI is misleading.
  Tradeoff: background polling (paused when hidden).
  Affects: Layout.tsx, admin status client.
  Validation: indicator + refresh tests.
- DEC-010 UI CI gates (typecheck/lint/test/build)
  Why: catch regressions before merge.
  Tradeoff: CI time.
  Affects: ui/package.json, ci.yml.
  Validation: CI run.

## Incremental Delivery

### MVP (First Value)

- P0 items (AC-001..004). Ready when P0 tests pass.

### Iterative Expansion

- Consistency (AC-005/006/007) → scale (AC-008) → workspace/live (AC-009/010) → CI (AC-011). Each increment independently verifiable.

## Sequencing Notes

- P0 first (client, dead code, boundary, cookie).
- Then the shared Modal and async-state component (prerequisites for page migrations).
- Then primitive adoption; then pagination (backend first, then UI); then workspace/URL; then live; CI last.

## Risks

- Broad UI refactor regressing pages → mitigated by incremental increments and page tests.
- Cookie `Secure` behind a TLS-terminating proxy → mitigated by `X-Forwarded-Proto` detection and an HTTP fallback test.
- Pagination API change breaking callers → mitigated by optional params and defaults; existing callers unaffected.
- Removing `Badge`/duplicate progress touching tests → mitigated by mechanical migration and running the suite.
- Polling load → low cadence + visibility pause.

## Rollout and Compatibility

- Frontend-only behavior changes plus additive API params; no migration.
- Cookie attributes tighten only on HTTPS.
- No feature flag required.

## Validation

- Go: cookie attributes; tenants/keys/budgets pagination/search/total.
- TS: shared modal, async states, primitives, pagination controls, workspace/URL, live indicator; existing suites green.
- CI: UI typecheck/lint/test/build job passes.
- Acceptance IDs covered: AC-001..AC-011. Decisions: DEC-001..DEC-010.

## Constitution Compliance

- no conflicts — operator console only; no DLP or data-plane change; no new dependency.
