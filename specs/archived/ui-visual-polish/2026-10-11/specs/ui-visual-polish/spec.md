# UI Visual & Responsive Production Pass

## Scope Snapshot

- In scope: a design-quality pass across the operator console — responsive layout, a complete light theme, consistent interaction states, official page shells, polished loading/empty/error states, sticky tables, and accessible, motion-aware micro-interactions.
- Out of scope: new product features (saved views, bulk/inline actions, command-palette actions, onboarding checklist), i18n, and new pages.

## Goal

The console should look and feel production-grade on any viewport and in both themes: it collapses gracefully to tablet/mobile, the light theme is complete and readable, every page shares the same shell and state treatments, and interactions (focus, hover, loading) feel deliberate rather than incidental. A reviewer opening the console at 1440px, 768px, and 375px, in dark and light, should see a coherent product with no broken layout, unreadable text, or unstyled ad‑hoc screens.

## Primary User Flow

1. Starting point: an operator opens the console on a laptop, a tablet, or a phone, in dark or light.
2. Main interaction: they navigate via the (collapsing) sidebar, skim tables and cards, open drawers/modals, and switch theme.
3. Outcome: layout reflows cleanly, every screen uses the same shell and states, and interactive elements are clearly focused/hovered.
4. Failure/fallback path: loading shows skeletons, empty shows guidance, errors show a retry — never a blank or broken screen.

## User Stories

- P1 Story: as an operator on a smaller screen, I can still use the console without a horizontal scroll of the page.
- P2 Story: as an operator who prefers light mode, everything is readable and consistent.
- P3 Story: as a reviewer, the console looks like a finished product on every page, not a collection of screens.
- P4 Story: as a keyboard-only user, I can see where focus is and navigate reliably.

## MVP Slice

- Responsive shell (collapsing sidebar + reflowing content) and a complete light theme across the core pages. Must satisfy `AC-001`, `AC-002`, `AC-003`.

## First Deployable Outcome

- At 375px the sidebar collapses to a drawer/rail and the page has no horizontal body scroll; the light theme is readable on Dashboard, Keys, Routing, and Settings.

## Scope

- A shared breakpoint/token system and a responsive app shell (sidebar collapse, header adaptation, content reflow).
- Complete light theme: surfaces, chips, tables, code blocks, charts, modals, focus rings.
- Official page shell (PageHeader title/subtitle/actions) applied to every page, including legacy ones.
- Interaction states: focus-visible rings, hover/active, disabled; motion with `prefers-reduced-motion`.
- State treatment: every data surface uses the shared loading/empty/error pattern (migrate legacy pages).
- Table polish: sticky headers, row hover, responsive overflow, numeric alignment.
- Accessibility pass: landmarks, roles/labels, keyboard order, contrast in both themes.

## Context

- The v2 design system exists (`ui/src/styles/tokens.css`, `base.css`, `components.css`; `ui/src/components/ui/` primitives) and colors are already tokenized (no hardcoded hex in components).
- The only `@media` rule today is the color-scheme toggle; there is no responsive layout and no breakpoint scale.
- `Layout.tsx` uses a fixed sidebar and `body { overflow: hidden; height: 100vh }`, so small screens clip content.
- Several pages are "legacy" (Login, Sessions, Conversations, Budgets, AuditLog, Swagger, Tenants list/form) using older class names rather than the v2 shell/primitives.
- Light theme is partial (tokens exist but not all surfaces were validated).

## Dependencies

- The v2 tokens/primitives and existing `AsyncSection`/`Modal`/`Table` components. No backend change.

## Requirements

- RQ-001 A breakpoint scale MUST be defined in tokens (e.g. `sm`/`md`/`lg`) and used consistently instead of ad-hoc widths.
- RQ-002 The app shell MUST collapse the sidebar below the tablet breakpoint (drawer or icon rail) with a visible toggle, and the header actions MUST adapt to a compact layout.
- RQ-003 Content MUST reflow so the page itself never scrolls horizontally at 768px and 375px; wide tables/plots MUST scroll within their container.
- RQ-004 The light theme MUST be complete and readable on every page (surfaces, text, chips, tables, code, charts, modals) with no dark-only literal.
- RQ-005 Every page MUST render the shared page shell (title, optional subtitle, actions) and use the v2 primitives; legacy pages MUST be migrated.
- RQ-006 Every data surface MUST use the shared loading/empty/error treatment (skeleton, guided empty state, retryable error).
- RQ-007 Interactive elements MUST show a visible `:focus-visible` ring, hover/active feedback, and a disabled style.
- RQ-008 Non-essential animation MUST be disabled under `prefers-reduced-motion`.
- RQ-009 Every table MUST keep its header visible while scrolling its body and use row hover + numeric alignment.
- RQ-010 The theme toggle MUST persist and the chosen theme MUST apply before first paint (no flash of the wrong theme).

## Non-Goals

- New product/UX features (saved views, bulk selection, command-palette actions, onboarding checklist, breadcrumbs).
- Internationalization and RTL.
- New pages or backend changes.
- A native mobile app or a PWA.

## Acceptance Criteria

### AC-001 Sidebar collapses responsively

- Why this matters: small screens must stay usable.
- **Given** a viewport below the tablet breakpoint
- **When** the app loads
- **Then** the sidebar is collapsed (drawer or icon rail) with a toggle, and the main content uses the full width.
- Evidence: a render/DOM test at the breakpoint asserts the collapsed class/`aria-expanded` and that a toggle reveals the nav.

### AC-002 No page-level horizontal scroll

- Why this matters: clipped, side-scrolling pages feel broken.
- **Given** the Dashboard, Keys, Routing, and Settings pages
- **When** rendered at 768px and 375px widths
- **Then** the document does not scroll horizontally; any wide table/plot scrolls within its own container.
- Evidence: a layout test asserts `scrollWidth <= clientWidth` on the page root for those pages at both widths.

### AC-003 Light theme is complete

- Why this matters: operators using light mode need parity.
- **Given** the theme set to light
- **When** each core page renders
- **Then** text and surfaces meet contrast and no element keeps a dark-only background.
- Evidence: a token test asserts a light value per surface token; a visual/manual check on the core pages.

### AC-004 Shared page shell on every page

- Why this matters: consistency is the product feel.
- **Given** any route
- **When** it renders
- **Then** it shows the shared header shell (title + optional subtitle + actions) and v2 primitives.
- Evidence: a test asserts the page header renders on each route; a review confirms no legacy class-only page remains.

### AC-005 Interaction states are visible

- Why this matters: feedback and accessibility.
- **Given** a keyboard-only user
- **When** they tab through a page
- **Then** every interactive element shows a visible focus ring, and hover/disabled states are defined.
- Evidence: a CSS/unit check that `:focus-visible` styles exist for buttons/links/rows; manual tab test.

### AC-006 Consistent loading/empty/error states

- Why this matters: no blank or broken screens.
- **Given** a slow, empty, or failing request
- **When** a data surface renders
- **Then** it shows a skeleton, a guided empty state, or an error with retry — not a bare spinner or blank.
- Evidence: a test renders each state via the shared component on a representative page; legacy pages migrated.

### AC-007 Sticky table headers

- Why this matters: scanning long tables.
- **Given** a table taller than the viewport
- **When** the operator scrolls the table body
- **Then** the header stays visible.
- Evidence: a CSS/token assertion for the sticky header class on `.tbl`.

### AC-008 Reduced motion is respected

- Why this matters: accessibility.
- **Given** `prefers-reduced-motion: reduce`
- **When** the console renders
- **Then** non-essential transitions/animations are disabled.
- Evidence: a `@media (prefers-reduced-motion: reduce)` rule exists and is asserted.

### AC-009 No ad-hoc color literals

- Why this matters: theming stays complete.
- **Given** the styles and components
- **When** reviewed
- **Then** colors come from tokens only (no hex/rgb literals outside `tokens.css`).
- Evidence: a grep-based check in CI/manual over `styles/**` and `components/**`.

### AC-010 No regressions

- Why this matters: keep the console stable.
- **Given** the existing UI test suite
- **When** the pass ships
- **Then** all tests pass and `npm run typecheck` is clean.
- Evidence: `npx vitest run` + `npm run typecheck`.

## Assumptions

- Breakpoints are `640` (phone) / `1024` (tablet) / `1440` (wide); the shell collapses at tablet and becomes a drawer at phone.
- Mobile tables keep horizontal scroll inside the card (no card-per-row fallback in this pass).
- Mobile is "usable and clean", not a fully bespoke mobile product.
- Charts (recharts) are made responsive via container sizing.
- The design tokens are the single source of color; components must not introduce literals.

## Open Questions

- Follow-up spec: the productivity/interaction pass (command-palette actions, saved views, bulk/inline actions, onboarding, breadcrumbs).


## Success Criteria

- SC-001 No cumulative layout shift is introduced by the theme applying before paint (theme set pre-render).
- SC-002 Core pages render without horizontal overflow at 768px and 375px.

## Edge Cases

- Very long table cell values: truncate with ellipsis inside the responsive container.
- Sidebar open on mobile then navigating: the drawer closes on route change.
- Both themes with charts: series colors remain distinguishable.
- Modal on small screens: full-height scrollable body, footer actions reachable.
- Zoomed text / large font: layout still reflows without clipping.

## Design Language

- Quiet density: an operator tool, not marketing — dense data, restrained color, accent reserved for the primary action and the active state.
- Three text levels (`text` / `text-2` / `text-muted`) and one header per card (title + optional description + right-aligned actions).
- Semantic color tokens (`success`/`warning`/`danger`/`info` with `-hover`/`-border`/`-bg`) instead of ad-hoc green/amber/red usage.
- Elevation scale: `shadow-sm` for cards, `shadow` for modals, `shadow-lg` for drawers.
- Type scale + tabular numerals for all numeric data (already present); spacing from the token scale.
- Motion: 120–160ms `ease-out` for hover/expand only, disabled under `prefers-reduced-motion`.
- Focus: a token ring (`--focus-ring`) on buttons/links/rows/chips; `aria-current` on the active nav item.

### Decisions (resolved)

- Small-screen sidebar: icon rail below the tablet breakpoint (≥640px) and an overlay drawer below the phone breakpoint (<640px), closing on navigation.
- Density: comfortable by default with a persisted compact toggle.
- Accent: keep the indigo accent; light theme is a real theme (not a dark inversion).


