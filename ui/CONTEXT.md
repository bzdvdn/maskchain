# ui — Operator Console

## Purpose

The admin control-plane UI: a Vite + React + TypeScript single-page app that operators use to manage tenants, shield policy, virtual keys, budgets, routing/providers, analytics, compliance, and live operations. Built as a static bundle and embedded into the admin binary; it is an operator console, not an end-user chat surface.

## Responsibilities

- Owns: SPA routing, operator pages, the design system (tokens, `components/ui` primitives, responsive shell), API clients, and the shared attention inbox.
- Does not own: auth backend, DLP/masking, routing/budget enforcement, or server state. It only calls the admin REST API under `/api/v1/*`.

## How it works

- `main.tsx` mounts `<App>` inside `BrowserRouter` + `ToastProvider`; `styles/*` supply the tokens/base/components layers.
- `App.tsx` gates on an admin session (`admin_token` in localStorage + `GET /api/v1/admin/verify`), then renders `AttentionProvider` → `Layout` → lazy routes behind a per-page `ErrorBoundary`/`Suspense`.
- `Layout.tsx` is the sidebar IA (Overview/Traffic/Governance/Routing/Operations/System), header actions, workspace switcher, live-health pill (`ok|degraded|down|unknown`), breadcrumbs, density + theme toggles, and the command palette. It collapses to an icon rail at ≤1024px and an overlay drawer at ≤640px (hamburger, closes on navigation).
- Theme and density are applied pre-paint from `index.html` (`body[data-theme]`, `body[data-density]`), managed by `hooks/useTheme.ts` and `hooks/useDensity.ts`.
- `api/client.ts` (`apiFetch`) returns `body.data ?? body`, injects the bearer token, and on 401 clears the token and emits `maskchain:unauthorized` (forced re-login).
- `AttentionProvider` + `hooks/useAttention.ts` compute dashboard signals (budgets, providers, expiring keys, blocked, workspace-scoped compliance deviation), with ack/snooze persisted in localStorage; consumed by the dashboard inbox and header badge.
- `hooks/useWorkspace.ts` broadcasts the selected tenant scope via `maskchain:workspace`; pages/analytics read it for scoping.
- Pages fetch through `ui/src/api/*` and render with the v2 primitives and the `AsyncSection` loading/empty/error contract.
- Dashboard onboarding shows only for a keyless install (stable signal, not live usage). Tenant form edits PII rules and dictionaries; Keys supports create/rotate/reveal/purge.

## Key files

- `ui/src/App.tsx` — session gate + route table + lazy pages.
- `ui/src/components/Layout.tsx` — sidebar IA, responsive shell, header actions, health pill, toggles.
- `ui/src/api/client.ts` — fetch wrapper (envelope, auth, 401 handling).
- `ui/src/hooks/useAttention.ts` — attention items + ack/snooze.
- `ui/src/components/DictionaryEditor.tsx` — tenant dictionary editor (entries round-trip).
- `ui/src/styles/tokens.css` — design tokens, light/dark, density/layout vars.

## Public contracts

- Admin REST API under `/api/v1/*` (auth via `Authorization: Bearer <admin session token>`; server sessions slide on activity); 401 triggers forced re-login.
- Response envelope `{ data: ... , error? }` (some endpoints paginate with `{ data, pagination }`).
- Cross-component events: `maskchain:workspace`, `maskchain:unauthorized`, `maskchain:command-open`.
- localStorage keys: `admin_token`, `maskchain.workspace`, `maskchain-theme`, `maskchain-density`, `maskchain.analytics.range`, `maskchain.attention.acked`, `maskchain.attention.snoozed`.
- Theme/density contract: `body[data-theme=light|dark]`, `body[data-density=comfortable|compact]`.

## Dependencies / Consumed by

- Depends on the admin binary serving `/api/v1/*` and embedding the built bundle (`ui/embed.go`).
- Consumed by: operators (browser); embedded by the admin/combined binaries.

## Gotchas

- The bundle is embedded at build time (`ui/embed.go`) — rebuild the binary after UI changes; `npm run build` runs `tsc -b` + vite.
- `AttentionProvider` must wrap consumers (Dashboard/Layout); `useAttentionState` throws outside it.
- Attention ack/snooze is per-browser (localStorage), not server state; corrupt entries fail open (item visible).
- Routing strategy labels mirror backend semantics: effective weight = `weight > 0 ? weight : 1`, healthy providers only.
- Some older pages still use legacy class names alongside `PageHeader`; Login is intentionally outside the app shell.
- Responsive by design now (rail/drawer/density); keep new pages on the shared primitives and `PageHeader`.
- Tests: `npx vitest run` (or `npm test`); also `npm run typecheck` and `npm run lint`.
