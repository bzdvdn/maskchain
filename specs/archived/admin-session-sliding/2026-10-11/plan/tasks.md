# Admin Session Sliding Expiry Tasks

## Phase Contract

Inputs: `specs/active/admin-session-sliding/{spec.md,plan.md}`.
Outputs: ordered executable tasks with coverage mapping.
Stop if: tasks would be vague or acceptance coverage cannot be mapped.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/domain/admin_session/entity.go | T1.1 |
| src/internal/domain/admin_session/storage.go | T1.2 |
| src/internal/adapters/repository/postgres/admin_session_store.go | T1.2 |
| src/internal/domain/admin_session/usecase.go | T2.1 |
| src/internal/api/middleware/adminauth.go | T2.2 |
| src/internal/domain/admin_session/usecase_test.go | T3.1 |
| src/internal/api/middleware/adminauth_test.go | T3.2 |
| src/ | T3.3 |

## Implementation Context

- MVP Goal: keep active admin sessions alive while idle sessions still expire.
- Acceptance Boundaries: AC-001..AC-005.
- Key Rules: derive lifetime from `ExpiresAt - CreatedAt`; extend only past half-lifetime; persistence is best-effort; no config/schema change.
- Data/Domain Invariants: expired sessions are never extended; no update when more than half remains; cookie refreshed only for validated sessions.
- Contracts/Protocols: `admin_sessions.expires_at` updated in place; cookie `admin_token` re-issued with remaining `Max-Age`.
- Proof Signals: usecase + middleware tests; `go test ./src/...` + `go build ./...`.
- Data model: no change.
- Out of Scope: TTL defaults, refresh tokens, client keep-alive.

## Phase 1: Domain and store

Goal: the renewal rule and its persistence exist.

- [x] T1.1 Add `AdminSession.Renew(now)` that extends `ExpiresAt` to `now + (ExpiresAt - CreatedAt)` only when the remaining lifetime is below half; return whether it extended (AC-001, AC-003). Touches: src/internal/domain/admin_session/entity.go
  Proof: code src/internal/domain/admin_session/entity.go Renew
- [x] T1.2 Add `UpdateExpiry(ctx, id, expiresAt)` to the store interface and the Postgres implementation, and update the in-package test mocks. Touches: src/internal/domain/admin_session/storage.go, src/internal/adapters/repository/postgres/admin_session_store.go
  Proof: code src/internal/adapters/repository/postgres/admin_session_store.go UpdateExpiry

## Phase 2: Use and refresh

Goal: validate renews, middleware keeps the cookie alive.

- [x] T2.1 In `Validate`, call `Renew` after the expiry check and persist via `UpdateExpiry` (best-effort, non-fatal) (AC-001, AC-004). Touches: src/internal/domain/admin_session/usecase.go
  Proof: code src/internal/domain/admin_session/usecase.go Validate
- [x] T2.2 Refresh the `admin_token` cookie after a validated session in both session middlewares, with `Max-Age = remaining` (AC-002, AC-005). Touches: src/internal/api/middleware/adminauth.go
  Proof: code src/internal/api/middleware/adminauth.go refreshAdminCookie

## Phase 3: Validation

Goal: prove behavior and leave the package reviewable.

- [x] T3.1 Usecase tests: renews and persists past half-lifetime (AC-001), does not update when fresh (AC-004), rejects expired (AC-003). Touches: src/internal/domain/admin_session/usecase_test.go
  Proof: test src/internal/domain/admin_session/usecase_test.go TestAdminSessionUseCaseValidateRenews
- [x] T3.2 Middleware test: a validated session sets a fresh `admin_token` cookie; the token/tenant fallback path does not (AC-002, AC-005). Touches: src/internal/api/middleware/adminauth_test.go
  Proof: test src/internal/api/middleware/adminauth_test.go TestAdminSessionAuthRefreshesCookie
- [x] T3.3 Run `go build ./...` + `go test ./src/...`; fix regressions. Touches: src/
  Proof: test src/internal/domain/admin_session/usecase_test.go

## Acceptance Coverage

- AC-001 -> T1.1, T2.1, T3.1
- AC-002 -> T2.2, T3.2
- AC-003 -> T1.1, T2.1, T3.1
- AC-004 -> T2.1, T3.1
- AC-005 -> T2.2, T3.2

## Notes

- Renewal threshold is half the original lifetime, so writes happen at most about once per half-TTL per session.
- The cookie must be refreshed or the browser drops it even when the DB expiry is extended.
- Every `[x]` task needs a `Proof:` line on the next line; no proof means not done.
