# UI Visual & Responsive Production Pass Tasks

## Phase Contract

Inputs: `specs/active/ui-visual-polish/plan.md`, `spec.md`.
Outputs: ordered executable tasks with coverage mapping.
Stop if: tasks would be vague or acceptance coverage cannot be mapped.

## Surface Map

| Surface | Tasks |
|---------|-------|
| ui/src/styles/tokens.css | T1.1, T3.2 |
| ui/index.html | T1.2 |
| ui/src/hooks/useTheme.ts | T1.2 |
| ui/src/hooks/useDensity.ts | T1.2, T3.3 |
| ui/src/styles/tokens.test.ts | T1.3 |
| ui/src/styles/base.css | T2.1 |
| ui/src/components/Layout.tsx | T2.2 |
| ui/src/components/Layout.test.tsx | T2.3 |
| ui/src/styles/components.css | T3.1, T3.2 |
| ui/src/hooks/useDensity.test.ts | T3.3 |
| ui/src/components/ui/PageHeader.tsx | T4.1 |
| ui/src/pages/* | T4.1, T4.2 |
| ui/ | T4.3 |

## Implementation Context

- MVP goal: token-first design layer + responsive shell + complete light theme (AC-001–AC-003).
- Invariants/semantics: colors/space/elevation/density come from `tokens.css` only (no literals); `body[data-theme]` and `body[data-density]` are set before first paint from localStorage; the shell collapses to an icon rail at ≤1024px and an overlay drawer at ≤640px (hamburger toggles a class + `aria-expanded`); wide tables scroll within their container so the page never scrolls horizontally; motion only on hover/expand and disabled under `prefers-reduced-motion`; `:focus-visible` uses a token ring.
- Errors/codes: none (presentation only).
- Contracts/protocols: localStorage `maskchain.theme` (existing) + `maskchain.density` (new); body attributes `data-theme` / `data-density`; CSS classes `.sidebar.collapsed`, `.sidebar.drawer-open`, `.page-shell`, `.breadcrumbs`.
- Scope boundaries: no backend/API change; no new product features (saved views, bulk actions, command-palette actions, onboarding); no i18n; no card-per-row mobile tables.
- Proof signals: token/theme assertions; a responsive smoke assertion (`scrollWidth<=clientWidth`); `useDensity` persistence test; `npx vitest run` + `npm run typecheck` clean.
- References: DEC-001..DEC-007, RQ-001..RQ-010 (no mandatory re-read).

## Phase 1: Tokens, theme, pre-paint

Goal: the design layer and a complete light theme with no flash.

- [x] T1.1 Extend `tokens.css` with semantic color, elevation, spacing, radius, focus-ring, layout, and density tokens; complete the light theme (surfaces/text/border/chart parity) (DEC-001, DEC-004, AC-003, AC-009). Touches: ui/src/styles/tokens.css
  Proof: code ui/src/styles/tokens.css#--focus-ring
- [x] T1.2 Add a pre-paint theme+density script to `index.html` and a persisted `useDensity` hook; align `useTheme` with the body attribute (DEC-003, DEC-004, AC-003). Touches: ui/index.html, ui/src/hooks/useTheme.ts, ui/src/hooks/useDensity.ts
  Proof: code ui/src/hooks/useDensity.ts
- [x] T1.3 Add token assertions (semantic tokens + light block + no literals outside tokens) (AC-003, AC-009). Touches: ui/src/styles/tokens.test.ts
  Proof: test ui/src/styles/tokens.test.ts

## Phase 2: Responsive shell

Goal: a shell that works at 375/768/1440.

- [x] T2.1 Make `base.css` responsive: icon rail ≤1024px, overlay drawer ≤640px, hamburger, content max-width, page shell + breadcrumb, dense header on phone (DEC-002, AC-001, AC-002). Touches: ui/src/styles/base.css
  Proof: code ui/src/styles/base.css
- [x] T2.2 Add the sidebar collapse/drawer state, hamburger, density + theme toggles, and breadcrumb to `Layout.tsx` with `aria-expanded`/`aria-current` (DEC-002, DEC-003, AC-001, AC-005). Touches: ui/src/components/Layout.tsx
  Proof: code ui/src/components/Layout.tsx
- [x] T2.3 Add a responsive smoke test (collapsed class + `aria-expanded`; no page-level horizontal overflow) (AC-001, AC-002). Touches: ui/src/components/Layout.test.tsx
  Proof: test ui/src/components/Layout.test.tsx

## Phase 3: Global polish (tables, states, focus, motion)

Goal: consistent polish on every page via shared CSS.

- [x] T3.1 Global table polish: sticky headers, row hover, numeric right-alignment; centralize focus ring + hover/active on buttons/links/rows; add motion + `prefers-reduced-motion` (DEC-005, DEC-006, AC-005, AC-007, AC-008). Touches: ui/src/styles/components.css
  Proof: code ui/src/styles/components.css
- [x] T3.2 Wire density tokens into table/card padding so `body[data-density=compact]` tightens rows and cards (DEC-003, AC-002). Touches: ui/src/styles/components.css, ui/src/styles/tokens.css
  Proof: code ui/src/styles/components.css
- [x] T3.3 Add a `useDensity` test (persist + body attribute) (AC-003). Touches: ui/src/hooks/useDensity.test.ts
  Proof: test ui/src/hooks/useDensity.test.ts

## Phase 4: Page shell and validation

Goal: consistent shells across pages and a clean suite.

- [x] T4.1 Adopt the shared `PageHeader` shell on the core pages (Dashboard/Analytics/Keys/Routing/Providers/Models/Budgets/Compliance/Settings) (AC-004). Touches: ui/src/pages/
  Proof: code ui/src/pages/Dashboard.tsx
- [x] T4.2 Migrate the legacy data pages (Sessions, Conversations, AuditLog) to the shared shell on top of their existing `AsyncSection` states (AC-004, AC-006). Touches: ui/src/pages/
  Proof: code ui/src/pages/Sessions.tsx
- [x] T4.3 Add `PageHeader` shell adoption for the remaining routes (Tenants list/form, Swagger) (AC-004; Login is pre-auth and intentionally outside the shell). Touches: ui/src/pages/Tenants/, ui/src/pages/Swagger.tsx
  Proof: code ui/src/pages/Tenants/TenantList.tsx
- [x] T4.4 Run `npx vitest run` + `npm run typecheck`; fix regressions (AC-010). Touches: ui/
  Proof: test ui/src/pages/Dashboard.test.tsx

## Acceptance Coverage

- AC-001 -> T2.1, T2.2, T2.3
- AC-002 -> T2.1, T2.3, T3.2
- AC-003 -> T1.1, T1.2, T1.3
- AC-004 -> T4.1, T4.2, T4.3
- AC-005 -> T2.2, T3.1
- AC-006 -> T4.2
- AC-007 -> T3.1
- AC-008 -> T3.1
- AC-009 -> T1.1, T1.3
- AC-010 -> T4.4

## Notes

- MVP is Phase 1–2; Phase 3 is global (benefits all pages); Phase 4 is page-by-page.
- The `.tbl`/card classes are shared, so Phase 3 lands everywhere without per-page edits.
- Evidence for T4.4: `npx vitest run` → 30 files / 141 tests passed; `tsc -b` clean.
- Login is a pre-auth full-screen route and stays outside the app shell.
- Every `[x]` task needs a `Proof:` line on the next line; no proof means not done.
