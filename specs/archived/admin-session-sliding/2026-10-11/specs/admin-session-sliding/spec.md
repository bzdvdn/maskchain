# Admin Session Sliding Expiry

## Scope Snapshot

- In scope: renew an active admin session on use so operators are not logged out mid-work, while idle sessions still expire.
- Out of scope: changing the session TTL semantics, adding remember-me, or refresh tokens.

## Goal

The admin session has a fixed TTL (`admin.session_ttl`, default 30m) and is never renewed, so an operator is kicked out 30 minutes after login regardless of activity; the UI hard-logs-out on the resulting 401. This feature makes the session slide: on each authenticated request, if the session is past half its lifetime it is extended back to the full TTL, the expiry is persisted, and the browser cookie is refreshed. Active operators stay signed in; idle operators still expire after one TTL.

## Primary User Flow

1. Starting point: an operator logs in and works in the console.
2. Main interaction: requests keep flowing (polling, navigation, actions).
3. Outcome: the session is extended before it expires and the cookie stays valid; no logout while active.
4. Failure/fallback path: after a full TTL of inactivity the session expires and the operator is asked to log in again.

## MVP Slice

- Renew on validate + persist + cookie refresh (AC-001, AC-002, AC-003).

## First Deployable Outcome

- Stay logged in past the original TTL while making requests; still get logged out after inactivity.

## Scope

- `admin_session` domain: a `Renew` rule and a store method to persist the new expiry.
- Postgres store: `UpdateExpiry`.
- Middleware: refresh the `admin_token` cookie after a validated session.

## Context

- `AdminSessionUseCase.Validate` only checks `now > ExpiresAt` (`usecase.go:31-41`) — no sliding, no refresh endpoint.
- The cookie is set at login with `maxAge = ttl` (`admin_auth_handler.go`); the server must refresh it or the browser drops it even if the DB expiry is extended.
- The UI clears the token on any 401 (`api/client.ts`), so the fix must prevent the 401 rather than only improve the message.

## Dependencies

- none (reuses the existing session store and cookie).

## Requirements

- RQ-001 On a successful validate, if the session's remaining lifetime is below half of its original lifetime, extend `ExpiresAt` to `now + originalLifetime`.
- RQ-002 Persist the extended expiry; a persistence failure MUST NOT fail the request (log/ignore) — the session stays valid with its prior expiry.
- RQ-003 Refresh the `admin_token` cookie on each validated session so the browser keeps it alive.
- RQ-004 Expired sessions MUST still be rejected; idle sessions expire after one full TTL.
- RQ-005 No behavior change for non-session auth (X-Admin-Token / tenant key fallback).

## Non-Goals

- Changing the default TTL or adding a config knob.
- Server-side session invalidation on password change.
- Client-side keep-alive timers.

## Acceptance Criteria

### AC-001 Active session is extended

- Why this matters: operators stop being kicked out mid-work.
- **Given** a session past half of its lifetime
- **When** it is validated
- **Then** `ExpiresAt` is moved to `now + originalLifetime` and persisted.
- Evidence: a usecase test asserts the renewed expiry and the store call.

### AC-002 Cookie is refreshed

- Why this matters: the browser must keep the cookie.
- **Given** a validated session
- **When** a protected request completes the middleware
- **Then** a fresh `admin_token` cookie is set with the remaining lifetime.
- Evidence: a middleware test asserts the `Set-Cookie` header.

### AC-003 Idle sessions still expire

- Why this matters: security is not weakened.
- **Given** a session whose `ExpiresAt` has passed
- **When** it is validated
- **Then** validation fails and no extension happens.
- Evidence: a usecase test asserts the expired path returns an error.

### AC-004 Early requests do not churn the store

- Why this matters: avoid a DB write per request.
- **Given** a session with more than half its lifetime remaining
- **When** it is validated
- **Then** no update is persisted.
- Evidence: a usecase test asserts the store update is not called.

### AC-005 Non-session auth is unaffected

- Why this matters: no regression for token/tenant fallbacks.
- **Given** a request authenticated by X-Admin-Token or a tenant key
- **When** it passes the combined middleware
- **Then** no session cookie is refreshed and the request succeeds.
- Evidence: a middleware test asserts the request passes without a session cookie.

## Assumptions

- The original lifetime is `ExpiresAt - CreatedAt` (no schema change needed).
- Refreshing the cookie on every validated request is acceptable overhead.
- Half-lifetime is a reasonable renewal threshold.

## Success Criteria

- SC-001 Continuous requests keep a session alive beyond its original TTL.

## Edge Cases

- `ExpiresAt - CreatedAt <= 0` → treat as expired, never extend.
- Clock skew → extension uses server time consistently.
- Store update error → request still succeeds with the old expiry.

## Open Questions

- none
