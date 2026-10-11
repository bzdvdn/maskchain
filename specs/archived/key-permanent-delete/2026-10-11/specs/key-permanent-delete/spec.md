# Virtual Key Permanent Delete

## Scope Snapshot

- In scope: permanently delete a virtual key (hard delete) from the console, in addition to the existing revoke.
- Out of scope: bulk delete, tenant-wide purge, and changing revoke semantics.

## Goal

Today a virtual key can only be revoked: `DELETE /api/v1/keys/:id` sets `enabled = FALSE` (soft delete), so revoked keys stay in the list forever. Operators need to remove a key entirely. This feature adds a hard-delete path: a repository `Purge`, an admin endpoint, cache invalidation and audit, and a distinct "Delete" action in the Keys page with an irreversible confirmation.

## Primary User Flow

1. Starting point: an operator opens Keys and finds a key they want gone.
2. Main interaction: they choose "Delete" (not "Revoke") and confirm the irreversible action.
3. Outcome: the key row is removed from the database and disappears from the list; it no longer authenticates.
4. Failure/fallback path: an unknown id returns 404; "Revoke" still works unchanged for a reversible stop.

## User Stories

- P1 Story: as an operator, I can permanently remove a key so stale keys do not accumulate.

## MVP Slice

- Repo `Purge` + endpoint + UI Delete action (AC-001, AC-005).

## First Deployable Outcome

- A key deleted from the console is gone from the database and the list.

## Scope

- `virtualkey.VirtualKeyRepository.Purge` and the Postgres implementation.
- Admin endpoint `DELETE /api/v1/keys/:id/purge` with cache invalidation and audit.
- Keys page: a Delete action with a strong confirm; `revoke` stays.

## Context

- `Delete` is a soft delete (`UPDATE ... SET enabled = FALSE`), and the list shows all rows, so revoked keys never leave the UI.
- `budgets.virtual_key_id REFERENCES virtual_keys(id) ON DELETE CASCADE`, so a hard delete also removes that key's budgets without an FK error.
- `key_hash` is unique; a purged key's secret can no longer authenticate.

## Dependencies

- none (reuses the existing key repository, cache and audit).

## Requirements

- RQ-001 Add `Purge(ctx, id)` to the repository port; it deletes the row and returns `ErrNotFound` when nothing matched.
- RQ-002 Add admin endpoint `DELETE /api/v1/keys/:id/purge` returning `200 {"ok":true}`; `404` for an unknown id.
- RQ-003 A purge MUST invalidate the in-process auth cache and write an audit entry (`purge_key`).
- RQ-004 The Keys page MUST expose a Delete action distinct from Revoke, with an irreversible confirmation.
- RQ-005 Revoke (soft delete) behavior MUST stay unchanged.

## Non-Goals

- Bulk or tenant-scoped purge.
- Undo / restore of a purged key.
- Changing the `DELETE /:id` (revoke) route.

## Acceptance Criteria

### AC-001 Purge removes the key

- Why this matters: this is the requested capability.
- **Given** an existing key
- **When** it is purged
- **Then** it is no longer retrievable and no longer authenticates.
- Evidence: a handler test asserts the key is gone from the repository after purge.

### AC-002 Unknown id returns 404

- Why this matters: correct API semantics.
- **Given** a non-existent id
- **When** purge is called
- **Then** the API returns `404`.
- Evidence: a handler test asserts the 404.

### AC-003 Cache is invalidated

- Why this matters: in-process auth must stop immediately.
- **Given** a warmed auth cache containing the key
- **When** it is purged
- **Then** the key is removed from the cache.
- Evidence: a handler test asserts the cache no longer holds the key.

### AC-004 Purge is audited

- Why this matters: traceability of a destructive action.
- **Given** a successful purge
- **When** it completes
- **Then** an audit entry records `purge_key` with the id.
- Evidence: a handler test asserts the audit logger received `purge_key`.

### AC-005 UI Delete action

- Why this matters: the operator can trigger it.
- **Given** a key row
- **When** the operator confirms Delete
- **Then** the client calls the purge endpoint.
- Evidence: a UI test asserts `purgeKey` is called after confirmation.

## Assumptions

- Cascading delete of the key's budgets is acceptable (FK is ON DELETE CASCADE).
- Only an authenticated admin session can purge (existing middleware).
- Permanent delete is intentional and needs no grace period.

## Success Criteria

- SC-001 A purged key does not appear in `GET /api/v1/keys`.

## Edge Cases

- Purge an already-revoked key → allowed (row still exists).
- Purge a key with budgets → budgets cascade.
- Purge while the key is in the auth cache → cache invalidated.

## Open Questions

- none
