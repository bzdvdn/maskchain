# Virtual Key Permanent Delete Tasks

## Phase Contract

Inputs: `specs/active/key-permanent-delete/spec.md` (express lane).
Outputs: ordered executable tasks with coverage mapping.
Stop if: tasks would be vague or acceptance coverage cannot be mapped.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/domain/virtualkey/repository.go | T1.1 |
| src/internal/adapters/repository/postgres/virtual_key_repo.go | T1.1 |
| src/internal/api/handler/admin/virtual_key_handler.go | T2.1 |
| src/internal/api/admin.go | T2.1 |
| src/internal/api/middleware/*_fake_test.go, src/internal/api/integration_test.go, src/internal/api/handler/admin/virtual_key_handler_test.go | T1.2 |
| src/internal/api/handler/admin/virtual_key_handler_test.go | T3.1 |
| ui/src/api/keys.ts | T2.2 |
| ui/src/pages/Keys.tsx | T2.3 |
| ui/src/pages/Keys.test.tsx | T3.2 |
| src/, ui/ | T3.3 |

## Implementation Context

- MVP Goal: permanently delete a virtual key from the console.
- Acceptance Boundaries: AC-001..AC-005.
- Key Rules: purge is a real row delete; revoke stays a soft delete; invalidate cache + audit on purge.
- Data/Domain Invariants: `budgets.virtual_key_id` cascades on delete; `key_hash` uniqueness means a purged key cannot authenticate.
- Contracts/Protocols: `DELETE /api/v1/keys/:id/purge` → `200 {"ok":true}`; `404` unknown.
- Proof Signals: handler + UI tests; `go test ./src/...`; `npx vitest run` + `npm run typecheck`.
- Data model: no change.
- Out of Scope: bulk/tenant purge, restore, changing revoke.

## Phase 1: Repository

Goal: a hard-delete operation exists and all implementers compile.

- [x] T1.1 Add `Purge(ctx, id)` to the repository port and implement it in Postgres (`DELETE ... WHERE id = $1`, `ErrNotFound` on zero rows). Touches: src/internal/domain/virtualkey/repository.go, src/internal/adapters/repository/postgres/virtual_key_repo.go
  Proof: code src/internal/adapters/repository/postgres/virtual_key_repo.go Purge
- [x] T1.2 Update the in-memory test fakes to satisfy the new interface method. Touches: src/internal/api/middleware/virtualkey_fake_test.go, src/internal/api/middleware/budget_fake_test.go, src/internal/api/integration_test.go, src/internal/api/handler/admin/virtual_key_handler_test.go
  Proof: test src/internal/api/handler/admin/virtual_key_handler_test.go fakeVirtualKeyRepo

## Phase 2: API and UI

Goal: the endpoint and the console action exist.

- [x] T2.1 Add the `Purge` handler (delete, invalidate cache, audit `purge_key`, `404`/`500`) and register `DELETE /api/v1/keys/:id/purge` (AC-001, AC-002, AC-003, AC-004). Touches: src/internal/api/handler/admin/virtual_key_handler.go, src/internal/api/admin.go
  Proof: code src/internal/api/handler/admin/virtual_key_handler.go Purge
- [x] T2.2 Add `purgeKey(id)` to the keys API client. Touches: ui/src/api/keys.ts
  Proof: code ui/src/api/keys.ts purgeKey
- [x] T2.3 Add a Delete action (distinct from Revoke) with an irreversible confirm that calls `purgeKey` (AC-005). Touches: ui/src/pages/Keys.tsx
  Proof: code ui/src/pages/Keys.tsx doPurge

## Phase 3: Validation

Goal: prove behavior and leave the package reviewable.

- [x] T3.1 Handler tests: purge removes the key and invalidates the cache (AC-001, AC-003), 404 unknown (AC-002), audit (AC-004). Touches: src/internal/api/handler/admin/virtual_key_handler_test.go
  Proof: test src/internal/api/handler/admin/virtual_key_handler_test.go TestVirtualKeyHandlerPurge
- [x] T3.2 UI test: the Delete action calls `purgeKey` after confirmation (AC-005). Touches: ui/src/pages/Keys.test.tsx
  Proof: test ui/src/pages/Keys.test.tsx
- [x] T3.3 Run `go build ./...` + `go test ./src/...` and `npx vitest run` + `npm run typecheck`; fix regressions. Touches: src/, ui/
  Proof: test src/internal/api/handler/admin/virtual_key_handler_test.go

## Acceptance Coverage

- AC-001 -> T1.1, T2.1, T3.1
- AC-002 -> T2.1, T3.1
- AC-003 -> T2.1, T3.1
- AC-004 -> T2.1, T3.1
- AC-005 -> T2.2, T2.3, T3.2

## Notes

- Purge is destructive and irreversible; the UI confirm must say so and be distinct from Revoke.
- Revoke (`DELETE /:id`) is unchanged.
- Every `[x]` task needs a `Proof:` line on the next line; no proof means not done.
