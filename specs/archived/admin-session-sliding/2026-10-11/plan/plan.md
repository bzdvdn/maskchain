# Admin Session Sliding Expiry Plan

## Approach

Derive the original lifetime from `ExpiresAt - CreatedAt` (no schema change). On validate, if the session is past half its lifetime, extend `ExpiresAt` to `now + lifetime`, persist it best-effort, and refresh the browser cookie in the middleware. Expired sessions are still rejected; idle sessions still expire after one lifetime.

## Decisions

- DEC-001 Lifetime source: `ExpiresAt - CreatedAt`; avoids storing TTL or changing the schema.
- DEC-002 Renewal threshold: extend only when remaining < half the lifetime (limits DB writes to roughly one per half-TTL).
- DEC-003 Persistence: add `UpdateExpiry` to the store; a failure is non-fatal (log via the caller, request proceeds).
- DEC-004 Cookie refresh lives in the middleware (`AdminSessionAuth`, `AdminSessionOrTokenAuth`) so both cookie and Bearer paths stay consistent; only refreshed when a session was actually validated.
- DEC-005 No config change: default TTL and `admin.session_ttl` keep their meaning (idle timeout).

## Implementation Surfaces

- `src/internal/domain/admin_session/entity.go` — `Renew(now)`.
- `src/internal/domain/admin_session/storage.go` — `UpdateExpiry`.
- `src/internal/adapters/repository/postgres/admin_session_store.go` — `UpdateExpiry`.
- `src/internal/domain/admin_session/usecase.go` — renew in `Validate`.
- `src/internal/api/middleware/adminauth.go` — refresh cookie after validate.

## Data and Contracts

- No schema change; `admin_sessions.expires_at` is updated in place.
- Cookie `admin_token` re-issued with `Max-Age = remaining lifetime`, SameSite=Lax, Secure over HTTPS.

## Acceptance Approach

- AC-001/AC-003/AC-004 via `usecase_test.go` (renewed expiry, expired path, no-update-early path).
- AC-002 via `adminauth_test.go` asserting `Set-Cookie`.
- AC-005 via the combined-middleware test with a tenant/token fallback.

## Constitution Compliance

- Security: idle sessions still expire; no weakening of auth. Scope: no unrelated changes.
