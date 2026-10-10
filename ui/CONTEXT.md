# ui — Operator Console

## Purpose

The admin control-plane UI: a Vite + React + TypeScript single-page app that operators use to manage tenants, shield policy, virtual keys, budgets, routing/providers, analytics, compliance, and live operations. Built as a static bundle and embedded into the admin binary; it is an operator console, not an end-user chat surface.

## Responsibilities

- Owns: SPA routing, the operator console pages, the design system (tokens + `components/ui` primitives), API clients, and the shared attention inbox.
- Does not own: auth backend, DLP/masking, routing/budget enforcement, or any server-side state. It only calls the admin REST API under `/api/v1/*`.

## How it works

- `main.tsx` mounts `<App>` inside `BrowserRouter` + `ToastProvider`; `styles/*` supply the v2 tokens.
- `App.tsx` gates on an admin session (localStorage token + `GET /api/v1/admin/verify`), then renders `AttentionProvider` → `Layout` → lazy-loaded routes behind a per-page `ErrorBoundary`/`Suspense`.
- `Layout.tsx` is the sidebar IA (Overview/Traffic/Governance/Routing/Operations/System), header actions, workspace switcher, live-health pill, and the critical attention badge; it also hosts the command palette.
- `api/client.ts` (`apiFetch`) unwraps the `{data}` envelope, injects the bearer token, and on 401 clears the token and emits `maskchain:unauthorized`.
- `AttentionProvider` + `hooks/useAttention.ts` compute dashboard signals (budgets, providers, expiring keys, blocked, workspace-scoped compliance deviation), with ack/snooze persisted in localStorage; consumed by the dashboard inbox and the header badge.
- `hooks/useWorkspace.ts` broadcasts the selected tenant scope via the `maskchain:workspace` event; pages/analytics read it for scoping.
- Pages fetch through `ui/src/api/*` and render with the shared v2 primitives (`components/ui`) and `AsyncSection` loading/empty/error contract.
- `utils/routingStrategy.ts` computes route strategy (`single`/`priority`/`weighted`/`fallback` + weight distribution) for the Routing page.

## Key files

- `ui/src/App.tsx` — session gate + route table + lazy pages.
- `ui/src/components/Layout.tsx` — sidebar IA, header actions, health pill, attention badge.
- `ui/src/api/client.ts` — fetch wrapper (envelope unwrap, auth, 401 handling).
- `ui/src/hooks/useAttention.ts` — attention items + ack/snooze.
- `ui/src/components/ui/` — v2 primitives + `v2.css`.
- `ui/src/pages/Routing.tsx` — overrides/aliases + strategy view + weight editor.

## Public contracts

- Admin REST API under `/api/v1/*` (auth via `Authorization: Bearer <admin session token>`); 401 triggers forced re-login.
- Response envelope `{ data: ... }` (some endpoints paginate with `{ data, pagination }`).
- Cross-component events: `maskchain:workspace` (tenant scope), `maskchain:unauthorized`, `maskchain:command-open`.
- localStorage keys: `maskchain.workspace`, `maskchain.theme`, `maskchain.analytics.range`, `maskchain.attention.acked`, `maskchain.attention.snoozed`.

## Dependencies / Consumed by

- Depends on the admin binary serving `/api/v1/*` and embedding the built bundle (`ui/embed.go`).
- Consumed by: operators (browser); embedded by the admin/combined binaries.

## Gotchas

- The bundle is embedded at build time (`ui/embed.go`) — rebuild the UI after changes for the served console to reflect them.
- `AttentionProvider` must wrap consumers (Dashboard/Layout); `useAttentionState` throws outside it.
- Attention ack/snooze is per-browser (localStorage), not server state; corrupt entries fail open (item visible).
- Strategy labels mirror backend semantics: effective weight = `weight > 0 ? weight : 1`, healthy providers only.
- Legacy pages (Login, Sessions, Conversations, Budgets, AuditLog, Swagger, Tenants list/form) use older class names, not the v2 primitives.
- Not responsive and English-only; tests run with vitest (`npx vitest run`) + `tsc -b`.
