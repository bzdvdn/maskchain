# src/internal/domain — Bounded Contexts

## Purpose

The business core: entities, value objects, rules, and repository *ports* for each bounded context. Pure Go with no HTTP, SQL, or vendor SDK imports; adapters implement the ports.

## Responsibilities

- Owns: invariants and domain logic for tenants, virtual keys, sessions, shield (detection/masking), routing, budgets, analytics, cache, conversations, compliance, log export, admin sessions.
- Does not own: persistence, HTTP, config loading, or background scheduling (those live in adapters/api/app).

## How it works

- `tenant` — tenant profile, auth header, retention mode, PII config, dictionaries; `TenantRepository` port.
- `virtualkey` — hashed-key entity (`sk-mc` prefix), scopes/budgets/expiry, `KeyHash`, `NewSecret` (64 hex), `VirtualKeyRepository`.
- `session` / `admin_session` — live chat sessions and admin sessions with separate stores/use cases; admin sessions carry sliding renewal.
- `shield` — subpackages `detector`, `dictionary`, `entity`, `mask`, `preprocessor`, `reaction`, `resolver`, `service`, `value`; the masking pipeline and detector catalog.
- `routing` (+ `service`) — providers, routes, aliases, health status, weighted selection.
- `budget` — budget entity, keys, rate limits, token budgets, `Repository` port.
- `analytics` — usage records, token usage, cost rates, aggregation, `UsageStore`.
- `cache` — semantic-cache entity/keys/repository port.
- `conversation` — encrypted conversation log entity + storage port.
- `compliance` — pack loader and pack model; `logexport` — record + sink resolver.

## Key files

- `src/internal/domain/virtualkey/entity.go` — key generation/hash/scopes.
- `src/internal/domain/shield/repository.go` — tenant dictionary contract.
- `src/internal/domain/routing/service/selector.go` — weighted provider selection.
- `src/internal/domain/admin_session/usecase.go` — session create/validate (sliding).
- `src/internal/domain/budget/entity.go` — budget scope/limits.

## Public contracts

- Repository ports per context (`TenantRepository`, `VirtualKeyRepository`, `UsageStore`, `Repository` for budgets/cache, `Storage` for sessions/conversations, `AdminSessionStore`).
- `virtualkey.KeyHash`, `NewSecret`, `NewKeyID`, `VirtualKey.Valid/AllowsModel`.
- `logexport.Sink` and `Resolver`; `routing.HealthStatus`.

## Dependencies / Consumed by

- Uses: stdlib and small shared value types; no infra imports.
- Used by: `internal/app/*` (use cases), `internal/api/*`, `internal/adapters/*`.

## Gotchas

- Only the SHA-256 hash is stored for keys; the plaintext secret is returned once (plus the separately encrypted reveal copy).
- `match_mode` for dictionaries is limited to `exact|contains|regex|fuzzy`.
- Routing effective weight is `weight > 0 ? weight : 1`, healthy providers only.
- Keep I/O out of this package; add a port instead of importing a driver.
