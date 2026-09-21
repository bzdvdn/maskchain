# UI Production Readiness Tasks

## Phase Contract

Inputs: `plan.md` + `spec.md` (decisions pinned).
Outputs: ordered executable tasks with coverage mapping.
Stop if: acceptance coverage cannot be mapped — not the case.

## Surface Map

| Surface | Tasks |
|---------|-------|
| ui/src/api/conversations.ts | T1.1 |
| ui/src/api/profiles.ts | T1.1 |
| ui/src/components/DictionaryEditor.tsx | T1.1 |
| ui/src/components/PreprocessorEditor.tsx | T1.1 |
| ui/src/components/ProtectedRoute.tsx | T1.1 |
| ui/src/__tests__/api.test.ts | T1.1, T5.3 |
| ui/src/App.tsx | T1.2 |
| ui/src/components/ErrorBoundary.tsx | T1.2, T5.3 |
| src/internal/api/handler/admin/admin_auth_handler.go | T1.3 |
| ui/src/components/ui/Modal.tsx | T2.1, T5.3 |
| ui/src/components/ui/index.ts | T2.1 |
| ui/src/components/ConfirmModal.tsx | T2.1 |
| ui/src/components/DictionaryModal.tsx | T2.1 |
| ui/src/hooks/useAsyncData.ts | T2.2 |
| ui/src/components/ui/AsyncSection.tsx | T2.2, T5.3 |
| ui/src/components/ui/Badge.tsx | T2.3 |
| src/internal/api/handler/admin/tenant_handler.go | T3.1, T5.2 |
| src/internal/api/handler/admin/virtual_key_handler.go | T3.1, T5.2 |
| src/internal/api/handler/admin/budget_handler.go | T3.1, T5.2 |
| src/internal/repository | T3.1 |
| ui/src/api/tenants.ts | T3.2 |
| ui/src/api/keys.ts | T3.2 |
| ui/src/api/budgets.ts | T3.2 |
| ui/src/hooks/useUrlFilters.ts | T4.1, T5.3 |
| ui/src/hooks/useWorkspace.ts | T4.1 |
| ui/src/components/Layout.tsx | T4.2, T5.3 |
| ui/src/api/admin.ts | T4.2 |
| ui/package.json | T5.1, T5.4 |
| .github/workflows/ci.yml | T5.1 |
| src/internal/api/handler/admin/admin_auth_handler_test.go | T5.2 |
| src/internal/api/handler/admin/tenant_handler_test.go | T5.2 |
| src/internal/api/handler/admin/virtual_key_handler_test.go | T5.2 |
| src/internal/api/handler/admin/budget_handler_test.go | T5.2 |
| ui/src/pages | T1.1, T2.1, T2.2, T2.3, T3.2, T4.1, T4.2, T5.3 |

## Implementation Context

- MVP Goal: authenticated console traffic uses one client, a shell crash is contained, and the admin cookie is hardened.
- Acceptance Boundaries: AC-001..AC-011.
- Key Rules: migrate remaining authenticated calls to `apiFetch` and delete orphaned code — DEC-001; app-level boundary in addition to per-page — DEC-002; cookie `Secure` on HTTPS with an explicit `SameSite`, HTTP still works locally — DEC-003; one `Modal` primitive with focus trap and a11y, migrate every modal — DEC-004; one async-state contract via `useAsyncData` error/retry plus `AsyncSection` — DEC-005; one status and one progress implementation, adopt `Card`/`PageHeader`/`EmptyState` — DEC-006; additive `limit`/`offset`/`search` plus `total` on tenants/keys/budgets lists — DEC-007; workspace scope on every tenant-scoped page and URL-persisted filters — DEC-008; live indicator from the status endpoint with low-cadence polling paused when hidden — DEC-009; UI CI gates for typecheck/lint/test/build — DEC-010.
- Invariants: no visual redesign; no data-plane/DLP change; no data-model or migration change; existing callers of the list endpoints keep working with defaults; local HTTP development keeps a non-secure cookie; login/logout and the bootstrap verify call may keep direct fetch.
- Contracts/Protocols: tenants/keys/budgets list endpoints accept optional `limit`/`offset`/`search` and return a `total` in pagination; modal exposes an accessible name via `aria-labelledby`; status endpoint drives the live indicator.
- Scope Boundaries: do not add new product screens or third-party state/caching libraries; do not rewrite the charting stack; do not add i18n or Storybook.
- Proof Signals: Go handler tests for cookie attributes and list pagination/search; UI tests for shared modal, async states, primitives, paging controls, workspace/URL round-trip and live indicator; `go test`, `vitest`, lint, typecheck and build green.
- References: DEC-001..DEC-010 (plan.md), RQ-001..RQ-011 (spec.md).

## Phase 1: P0 Correctness and Security

Goal: one authenticated client, no dead code, contained shell failures, hardened cookie.

- [x] T1.1 Route remaining authenticated calls through the shared client and remove orphaned code — outcome: conversations use `apiFetch` (token plus global 401), the orphaned profiles client and its unused components are deleted, and the API test no longer references removed endpoints. Touches: ui/src/api/conversations.ts, ui/src/api/profiles.ts, ui/src/components/DictionaryEditor.tsx, ui/src/components/PreprocessorEditor.tsx, ui/src/components/ProtectedRoute.tsx, ui/src/__tests__/api.test.ts
      Proof: test ui/src/__tests__/api.test.ts listConversations
- [x] T1.2 Contain shell failures with an app-level error boundary — outcome: a throw in the shell renders a contained fallback with a reset action while the per-page boundary still covers route content. Touches: ui/src/App.tsx, ui/src/components/ErrorBoundary.tsx
      Proof: code ui/src/App.tsx ErrorBoundary
- [x] T1.3 Harden the admin session cookie — outcome: login and logout set an explicit `SameSite` and `Secure` on HTTPS (via `r.TLS` or `X-Forwarded-Proto`) while plain HTTP stays non-secure. Touches: src/internal/api/handler/admin/admin_auth_handler.go
      Proof: code src/internal/api/handler/admin/admin_auth_handler.go setSessionCookie

## Phase 2: Consistency

Goal: one modal, one async-state contract, one design system.

- [x] T2.1 Add a shared Modal primitive and migrate every modal — outcome: `Modal` provides focus trap, initial focus, Escape, `aria-labelledby` and scroll lock, and ConfirmModal, DictionaryModal and the page modals use it. Touches: ui/src/components/ui/Modal.tsx, ui/src/components/ui/index.ts, ui/src/components/ConfirmModal.tsx, ui/src/components/DictionaryModal.tsx, ui/src/pages
      Proof: code ui/src/components/ui/Modal.tsx focusable
- [x] T2.2 Unify loading, empty, and error states — outcome: `useAsyncData` exposes the error and a retry, `AsyncSection` renders skeleton/empty/error+retry, and every data page adopts it with no silent catches. Touches: ui/src/hooks/useAsyncData.ts, ui/src/components/ui/AsyncSection.tsx, ui/src/pages
      Proof: code ui/src/components/ui/AsyncSection.tsx onRetry
- [x] T2.3 Consolidate on the design system — outcome: `Badge` is removed in favor of `StatusPill`, a single progress implementation remains, and pages use `Card`/`PageHeader`/`EmptyState` instead of hand-rolled markup. Touches: ui/src/components/ui/Badge.tsx, ui/src/pages
      Proof: code ui/src/components/ui/Status.tsx statusTone

## Phase 3: Scale

Goal: lists page and search on the server.

- [x] T3.1 Add server-side pagination and search to tenants, keys, and budgets — outcome: the three list endpoints and their repositories accept optional `limit`/`offset`/`search` and return a `total`, with unchanged behavior when nothing is passed. Touches: src/internal/api/handler/admin/tenant_handler.go, src/internal/api/handler/admin/virtual_key_handler.go, src/internal/api/handler/admin/budget_handler.go, src/internal/repository
      Proof: code src/internal/api/handler/admin/list_query.go parseListQuery
- [x] T3.2 Add paging and search controls to list pages — outcome: the tenants, keys and budgets clients pass `limit`/`offset`/`search` and the pages show paging and search with page and total. Touches: ui/src/api/tenants.ts, ui/src/api/keys.ts, ui/src/api/budgets.ts, ui/src/pages
      Proof: code ui/src/pages/Keys.tsx onSearch

## Phase 4: Operator Experience

Goal: workspace applies everywhere, filters are shareable, the live indicator is honest.

- [x] T4.1 Apply workspace scope everywhere and persist filters in the URL — outcome: every tenant-scoped page honors the selected workspace and list filters/ranges round-trip through the URL. Touches: ui/src/hooks/useUrlFilters.ts, ui/src/hooks/useWorkspace.ts, ui/src/pages
      Proof: code ui/src/hooks/useUrlFilters.ts set
- [x] T4.2 Make the live indicator honest and auto-refresh — outcome: the header indicator reflects the aggregated status (unknown when unreachable), polls at a low cadence paused when hidden, and freshness-critical pages refresh automatically. Touches: ui/src/components/Layout.tsx, ui/src/api/admin.ts, ui/src/pages
      Proof: code ui/src/components/Layout.tsx health

## Phase 5: Validation and CI

Goal: prove behavior and gate regressions.

- [x] T5.1 Add UI CI gates — outcome: `ui/package.json` has a `typecheck` script and CI runs install, typecheck, lint, tests and the production build for the UI. Touches: ui/package.json, .github/workflows/ci.yml
      Proof: code .github/workflows/ci.yml ui
- [x] T5.2 Add Go tests for cookie hardening and pagination/search — outcome: tests assert cookie attributes over HTTPS and HTTP and limit/offset/search with a correct total for tenants, keys and budgets (AC-004, AC-008). Touches: src/internal/api/handler/admin/admin_auth_handler_test.go, src/internal/api/handler/admin/tenant_handler_test.go, src/internal/api/handler/admin/virtual_key_handler_test.go, src/internal/api/handler/admin/budget_handler_test.go
      Proof: test src/internal/api/handler/admin/tenant_handler_test.go TestListTenantsPagination
- [x] T5.3 Add UI tests for the new primitives and behaviors — outcome: tests cover the shared modal, async states, primitives, paging/search controls, workspace with URL round-trip and the live indicator with auto-refresh (AC-005..AC-010). Touches: ui/src/components/ui/Modal.test.tsx, ui/src/components/ui/AsyncSection.test.tsx, ui/src/components/ErrorBoundary.test.tsx, ui/src/pages
      Proof: test ui/src/components/ui/Modal.test.tsx Modal
- [x] T5.4 Run the full validation suite — outcome: Go tests, UI typecheck, lint, tests and the production build all pass with every AC covered and no regressions (AC-001..AC-011). Touches: ui/package.json, Makefile
      Proof: chore ui/package.json typecheck

## Acceptance Coverage

- AC-001 -> T1.1, T5.4
- AC-002 -> T1.1, T5.4
- AC-003 -> T1.2, T5.3
- AC-004 -> T1.3, T5.2
- AC-005 -> T2.1, T5.3
- AC-006 -> T2.2, T5.3, T6.1
- AC-007 -> T2.3, T5.3
- AC-008 -> T3.1, T3.2, T5.2, T5.3, T7.1
- AC-009 -> T4.1, T5.3, T6.2
- AC-010 -> T4.2, T5.3, T6.3
- AC-011 -> T5.1, T5.4

## Notes

- No data model, migration, or config change; list endpoints gain only optional query params and a `total`.
- Deliver in increments: P0 (T1.x) first, then consistency (T2.x), then scale (T3.x), then UX (T4.x), then validation and CI (T5.x).
- Removing `Badge` and the orphaned editors must happen together with their call-site migrations so the build stays green.

## Converge Follow-ups

- [x] T6.1 Finish the shared async-state contract on the remaining data pages — outcome: Analytics, Compliance and Dashboard render skeleton/empty/error+retry through `AsyncSection`, and Analytics no longer swallows its load error. Touches: ui/src/pages/Analytics.tsx, ui/src/pages/Compliance.tsx, ui/src/pages/Dashboard.tsx
      Proof: code ui/src/pages/Analytics.tsx AsyncSection
- [x] T6.2 Persist Keys and Budgets list filters in the URL — outcome: the search text and page round-trip through the query string for both lists, matching Conversations and Audit. Touches: ui/src/pages/Keys.tsx, ui/src/pages/Budgets.tsx
      Proof: code ui/src/pages/Budgets.tsx changePage
- [x] T6.3 Auto-refresh the freshness-critical pages — outcome: Dashboard, Conversations and Routing re-fetch on a low cadence that pauses while the tab is hidden, via a shared hook. Touches: ui/src/hooks/useAutoRefresh.ts, ui/src/pages/Dashboard.tsx, ui/src/pages/Conversations.tsx, ui/src/pages/Routing.tsx
      Proof: code ui/src/hooks/useAutoRefresh.ts interval

## Converge Follow-ups (round 2)

- [x] T7.1 Push pagination and search down to the repositories — outcome: `TenantRepository`, `VirtualKeyRepository` and `BudgetRepository` expose `ListPaged(limit, offset, search)` implemented with SQL `LIMIT`/`OFFSET`/`ILIKE` and a `COUNT(*)`, the list handlers page through it when `limit`/`offset`/`search` are present, and all fakes/decorators implement it. Touches: src/internal/domain/shield/repository.go, src/internal/domain/virtualkey/repository.go, src/internal/domain/budget/repository.go, src/internal/adapters/repository/postgres/tenant.go, src/internal/adapters/repository/postgres/virtual_key_repo.go, src/internal/adapters/repository/postgres/budget_repo.go, src/internal/api/handler/admin/tenant_handler.go, src/internal/api/handler/admin/virtual_key_handler.go, src/internal/api/handler/admin/budget_handler.go, src/internal/api/handler/admin/list_query.go
      Proof: code src/internal/adapters/repository/postgres/tenant.go ListPaged
