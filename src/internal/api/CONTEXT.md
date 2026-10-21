# src/internal/api — HTTP Data Plane and Admin Control Plane

## Purpose

The HTTP edge: a gateway `Server` for the OpenAI-compatible data plane and an `AdminServer` for the operator API, plus shared middleware, DTOs, health probes, and embedded Swagger UI.

## Responsibilities

- Owns: routing/mounting, request/response shaping (`dto` envelope), auth, shield/budget/cache/usage/rate-limit middleware, media/responses/agent handlers, admin CRUD handlers.
- Does not own: business rules (domain), persistence (adapters), or worker scheduling (app/cmd).

## How it works

- `New(...)` builds the gateway gin engine and installs the global chain: OTel → request ID → logger → recovery → CORS, then per-route sessions, shield, model access, budget, cache, usage, conversation, and export middleware.
- Gateway routes under `/api/v1/*`: chat/completions, responses, media, sessions, self-service, MCP/A2A agents, mask; health at `/health|/ready|/live`, metrics at `/metrics`.
- `NewAdminServer(...)` builds the control plane: `/api/v1/admin/*` (login/logout/verify, status), `/api/v1/tenants`, `/api/v1/keys`, `/api/v1/budgets`, `/api/v1/providers|models|routing`, `/api/v1/compliance`, `/api/v1/audit`, analytics, plus static UI and Swagger.
- Middleware lives in `api/middleware` (49 files): `AdminSessionOrTokenAuth`, `VirtualKeyAuth`, shield/session/budget/cache/usage/ratelimit, envelope, CORS, recovery.
- `api/dto` defines the `{data, error}` envelope, pagination, and per-resource DTOs; `api/health` aggregates probes (database/valkey/egress); `api/swagger` embeds the spec/UI.

## Key files

- `src/internal/api/server.go` — gateway composition + global middleware.
- `src/internal/api/admin.go` — control-plane composition and route registration.
- `src/internal/api/middleware/adminauth.go` — admin session/token auth + cookie refresh.
- `src/internal/api/middleware/virtualkey_auth.go` — tenant resolution by key hash.
- `src/internal/api/dto/envelope.go` — response envelope and error mapping.
- `src/internal/api/handler/admin/virtual_key_handler.go` — key CRUD/rotate/reveal/purge.

## Public contracts

- Wire protocol: OpenAI-compatible `/api/v1/chat/completions`, `/responses`, media; SSE streaming.
- Admin REST under `/api/v1/*`; auth via admin session (`Authorization: Bearer` or `admin_token` cookie) or tenant key.
- Envelope `{ "data": ..., "error": { "code", "message" } }`; keys `sk-mc_*`; admin session TTL + sliding renewal.
- Health: aggregated `status` ∈ `ok|degraded|down|unknown`.

## Dependencies / Consumed by

- Uses: domain, app services, adapters, infra config/metrics/telemetry, health.
- Used by: `cmd/gateway`, `cmd/admin`, `cmd/all`; the UI consumes its API.

## Gotchas

- Middleware order is load-bearing; shielding runs before provider calls, usage/budget after.
- Any 401 from an admin endpoint clears the UI session — keep auth failures precise.
- `VirtualKeyAuth` only runs when tenants are loaded; it aborts on auth-header mismatch.
- Do not add SQL or business rules to handlers; call the app/domain layer.
- Keep the OpenAPI spec in `api/swagger/openapi.yaml` in sync with new admin routes.
