# UI Visual & Responsive Production Pass Plan

## Phase Contract

Inputs: `specs/active/ui-visual-polish/spec.md`.
Outputs: `plan.md` (data model noted inline).
Stop if: the spec is too vague to plan safely.

## Goal

Turn the v2 console into a coherent production product through a token-first design layer and a responsive shell: extend the design tokens (spacing, elevation, semantic color, focus ring, layout, density), make the shell responsive (icon rail ≥640px, overlay drawer <640px), complete the light theme with no first-paint flash, and apply global table/state/motion/accessibility polish that benefits every page without per-page rewrites.

## MVP Slice

- Tokens + responsive shell + complete light theme. Must satisfy `AC-001`, `AC-002`, `AC-003`.

## First Validation Path

1. Render at 375/768/1440 in both themes; assert no page-level horizontal scroll and a collapsed sidebar under the breakpoint.
2. Toggle theme + density and reload to confirm persistence and no flash.
3. `npx vitest run` + `npm run typecheck`.

## Scope

- Token layer: spacing/elevation/semantic color/focus/layout/density + real light theme.
- Responsive shell in `base.css` + `Layout.tsx` (rail/drawer, hamburger, breadcrumb, dense header on phone).
- Global table polish, state contracts, motion, and focus in `components.css`.
- Page shell (`PageHeader`) adoption and legacy-page migration (page-by-page).

## Performance Budget

- CSS-only responsive/layout changes; no added render cost. Density/theme applied without re-render churn (body attributes).

## Implementation Surfaces

- `ui/src/styles/tokens.css` — token additions + light theme completion + density tokens.
- `ui/src/styles/base.css` — responsive shell, page/header/content layout, breadcrumb, hamburger.
- `ui/src/styles/components.css` — sticky tables, row hover, numeric alignment, buttons/status polish, focus ring, motion, reduced-motion.
- `ui/src/hooks/useDensity.ts` (new) — persisted density toggle.
- `ui/src/components/Layout.tsx` — sidebar collapse state, hamburger, density + theme toggles, breadcrumb.
- `ui/src/components/ui/` — `PageHeader`, `Card`, `Status` used consistently (adopt).
- Pages — `PageHeader` adoption + legacy class migration.
- `ui/index.html` — pre-paint theme/density script (no flash).
- Tests: layout/responsive smoke, `useDensity`, token assertions.

## Bootstrapping Surfaces

- none — tokens, primitives, and the shell already exist.

## Architecture Impact

- Local: styles + shell component + one hook; page shells updated page-by-page.
- No backend/API change.

## Acceptance Approach

- AC-001 → rail/drawer shell + hamburger state; surfaces `base.css`, `Layout.tsx`; proof: render test asserts collapsed class + `aria-expanded`.
- AC-002 → reflow + container scroll; surfaces `base.css`, `components.css`; proof: smoke test `scrollWidth<=clientWidth` at 768/375 for core pages.
- AC-003 → light theme tokens + pre-paint; surfaces `tokens.css`, `index.html`; proof: token assertions + manual check.
- AC-004 → `PageHeader` adoption; surfaces pages + `components/ui`; proof: header renders per route.
- AC-005 → focus ring token on interactive elements; surfaces `tokens.css`, `components.css`; proof: CSS assertion + manual tab.
- AC-006 → `AsyncSection` on all data surfaces; surfaces pages; proof: state render test; legacy pages migrated.
- AC-007 → sticky headers; surfaces `components.css`; proof: CSS assertion on `.tbl thead`.
- AC-008 → reduced-motion; surfaces `components.css`; proof: media-rule assertion.
- AC-009 → tokens only; surfaces styles; proof: grep check.
- AC-010 → suites; surfaces all; proof: vitest + typecheck.

## Data and Contracts

- Data model: no change. `body[data-density]` and `body[data-theme]` are the only new surface state (localStorage-backed).

## Implementation Strategy

- DEC-001 Token-first design layer
  Why: one source for color/space/elevation/density keeps every page consistent and themable; no literals. Not: per-component values.
  Tradeoff: an initial token sweep.
  Affects: `tokens.css` (+ consumers).
  Validation: AC-003/AC-009.
- DEC-002 Responsive shell via CSS + a small JS collapse state
  Why: CSS handles layout (rail/drawer) while JS only toggles a class/`aria-expanded`, so it stays testable. Not: a JS layout engine.
  Tradeoff: breakpoints live in CSS (documented in tokens as comments).
  Affects: `base.css`, `Layout.tsx`.
  Validation: AC-001/AC-002.
- DEC-003 Density as `body[data-density]` + `--density-*` tokens
  Why: a single attribute flips row/card padding without per-component props. Not: a React context threading density.
  Tradeoff: components must read density tokens for spacing.
  Affects: `tokens.css`, `components.css`, `useDensity.ts`, `Layout.tsx`.
  Validation: `useDensity` test + manual.
- DEC-004 Pre-paint theme/density to avoid flash
  Why: setting `data-theme`/`data-density` from localStorage in an inline script before the bundle loads prevents FOUC. Not: applying in React after mount.
  Tradeoff: a tiny inline script in `index.html`.
  Affects: `index.html`.
  Validation: AC-003/AC-010.
- DEC-005 Global table polish in CSS
  Why: `.tbl` is shared, so sticky header + hover + numeric alignment land everywhere at once. Not: editing every table.
  Tradeoff: relies on existing `.tbl` class usage (legacy pages included).
  Affects: `components.css`.
  Validation: AC-007.
- DEC-006 Centralized motion + focus with reduced-motion
  Why: consistent feel and accessibility from one place. Not: per-component transitions.
  Tradeoff: none material.
  Affects: `tokens.css`, `components.css`.
  Validation: AC-005/AC-008.
- DEC-007 Page shell + legacy migration is page-by-page
  Why: `PageHeader` adoption cannot be done by CSS; do the core pages first, then legacy. Not: a big-bang rewrite.
  Tradeoff: legacy pages may lag one increment.
  Affects: pages, `components/ui/PageHeader.tsx`.
  Validation: AC-004/AC-006.

## Incremental Delivery

### MVP (First Value)

- Tokens + light theme + responsive shell + pre-paint. `AC-001`–`AC-003`.

### Iterative Expansion

- Global tables/focus/motion/states. `AC-005`, `AC-007`, `AC-008`, `AC-009`.
- Page shell adoption + legacy migration. `AC-004`, `AC-006`.

## Sequencing Notes

- Tokens before shell before page adoption.
- Pre-paint script with the theme work (same commit).
- Density tokens before the toggle.

## Risks

- Responsive changes could break existing layout tests.
  Mitigation: CSS scoped to breakpoints; run the suite.
- Legacy pages without `PageHeader` remain inconsistent.
  Mitigation: migrate in a bounded follow-up within the same feature.
- Over-animation.
  Mitigation: only hover/expand; reduced-motion off.

## Rollout and Compatibility

- Additive CSS + a body attribute; no migration/flag. Default density stays comfortable; dark stays default.

## Validation

- Automated: `npx vitest run` + `npm run typecheck`; a responsive smoke test; `useDensity` test.
- Manual: 375/768/1440 in both themes.
- Proves: `AC-001`–`AC-010`; `DEC-001`–`DEC-007`.

## Constitution Compliance

- no conflicts. Operator control-plane UI only; no backend/DLP change.
