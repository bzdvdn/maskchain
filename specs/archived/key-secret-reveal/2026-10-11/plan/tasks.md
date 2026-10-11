# Key Secret Reveal Tasks

## Phase Contract

Inputs: `specs/active/key-secret-reveal/{spec.md,plan.md,data-model.md}`.
Outputs: ordered executable tasks with coverage mapping.
Stop if: tasks would be vague or acceptance coverage cannot be mapped.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/adapters/repository/postgres/migrations/024_virtual_key_secret.{up,down}.sql | T1.1 |
| src/internal/domain/virtualkey/entity.go | T1.2 |
| src/internal/adapters/repository/postgres/virtual_key_repo.go | T1.2 |
| src/internal/api/middleware/errors.go | T2.1 |
| src/internal/api/dto/virtual_key.go | T2.1 |
| src/internal/api/handler/admin/virtual_key_handler.go | T2.1, T2.2 |
| src/internal/api/admin.go | T2.1 |
| src/cmd/admin/run.go, src/cmd/all/admin.go | T2.3 |
| ui/src/api/keys.ts | T3.1 |
| ui/src/pages/Keys.tsx | T3.2 |
| src/internal/api/handler/admin/virtual_key_handler_test.go | T4.1 |
| src/internal/api/dto/virtual_key_test.go | T4.2 |
| ui/src/pages/Keys.test.tsx | T4.3 |
| src/, ui/ | T4.4 |

## Implementation Context

- MVP Goal: reveal a key's secret again from the console, with the secret stored encrypted at rest.
- Acceptance Boundaries: AC-001..AC-006.
- Key Rules: reuse `crypto.Encryptor` + `config.KeysKeyEnvVar`; admin-only reveal; audit every reveal; no plaintext at rest.
- Data/Domain Invariants: `key_hash` stays the only auth source; `secret_encrypted` is opaque base64(`nonce||ct`) and never leaves the server except as revealed plaintext; empty ⇒ unavailable.
- Contracts/Protocols: `GET /api/v1/keys/:id/secret` → `200 {"key":"sk-mc_..."}`; unavailable → `409 {"error":{"code":"SECRET_UNAVAILABLE"}}`.
- Proof Signals: handler/DTO/UI tests; `go test ./src/...`; `npx vitest run` + `npm run typecheck`.
- Data model: see `data-model.md` (new column `secret_encrypted`).
- Out of Scope: gateway reveal, plaintext fallback, bulk reveal, pre-existing keys.

## Phase 1: Storage

Goal: persist the encrypted secret without touching existing behavior.

- [x] T1.1 Add migration 024 up/down adding `virtual_keys.secret_encrypted TEXT NOT NULL DEFAULT ''`. Touches: src/internal/adapters/repository/postgres/migrations/024_virtual_key_secret.up.sql, src/internal/adapters/repository/postgres/migrations/024_virtual_key_secret.down.sql
  Proof: code src/internal/adapters/repository/postgres/migrations/024_virtual_key_secret.up.sql
- [x] T1.2 Add the opaque `SecretEncrypted` field to the domain entity and read/write the column in the repository (`SELECT`, scan, `INSERT`, `UPDATE`). Touches: src/internal/domain/virtualkey/entity.go, src/internal/adapters/repository/postgres/virtual_key_repo.go
  Proof: code src/internal/adapters/repository/postgres/virtual_key_repo.go scanVirtualKey

## Phase 2: API

Goal: seal at create/rotate, reveal via a new admin endpoint, and wire the encryptor.

- [x] T2.1 Add the `SECRET_UNAVAILABLE` error code, a reveal response DTO, the `GET /keys/:id/secret` route, and handler logic: seal the secret on create/rotate when an encryptor is present, and open it on reveal (missing/nil/decrypt-failure → 409). Touches: src/internal/api/middleware/errors.go, src/internal/api/dto/virtual_key.go, src/internal/api/handler/admin/virtual_key_handler.go, src/internal/api/admin.go
  Proof: code src/internal/api/handler/admin/virtual_key_handler.go Reveal
- [x] T2.2 Audit every reveal (`reveal_key_secret`, key id only; never the secret). Touches: src/internal/api/handler/admin/virtual_key_handler.go
  Proof: code src/internal/api/handler/admin/virtual_key_handler.go writeKeyAudit
- [x] T2.3 Wire the encryptor into the virtual key handler/repo for the admin binaries (keep the gateway repo's no-encryptor constructor). Touches: src/cmd/admin/run.go, src/cmd/all/admin.go
  Proof: code src/cmd/admin/run.go

## Phase 3: UI

Goal: let an operator reveal a stored secret, with a rotate fallback.

- [x] T3.1 Add `revealKey(id)` to the keys API client returning the plaintext secret. Touches: ui/src/api/keys.ts
  Proof: code ui/src/api/keys.ts revealKey
- [x] T3.2 Add a Reveal action per key row that shows the secret in the existing one-time panel with copy, and on `SECRET_UNAVAILABLE` shows guidance + offers Rotate. Touches: ui/src/pages/Keys.tsx
  Proof: code ui/src/pages/Keys.tsx doReveal

## Phase 4: Validation

Goal: prove behavior and leave the package reviewable.

- [x] T4.1 Handler tests: reveal returns the created secret (AC-001), rotate then reveal returns the new secret (AC-003), empty ciphertext → 409 (AC-004), nil encryptor → 409 and no plaintext stored (AC-005), reveal is audited (AC-006). Touches: src/internal/api/handler/admin/virtual_key_handler_test.go
  Proof: test src/internal/api/handler/admin/virtual_key_handler_test.go TestVirtualKeyHandlerRevealStoredSecret
- [x] T4.2 DTO test asserting the key response exposes neither plaintext nor ciphertext (AC-002). Touches: src/internal/api/dto/virtual_key_test.go
  Proof: test src/internal/api/dto/virtual_key_test.go TestVirtualKeyResponseDoesNotLeakSecret
- [x] T4.3 UI test for the Reveal action and the `SECRET_UNAVAILABLE` fallback (AC-001, AC-004). Touches: ui/src/pages/Keys.test.tsx
  Proof: test ui/src/pages/Keys.test.tsx
- [x] T4.4 Run `go build ./...` + `go test ./src/...` and `npx vitest run` + `npm run typecheck`; fix regressions. Touches: src/, ui/
  Proof: test src/internal/api/handler/admin/virtual_key_handler_test.go

## Acceptance Coverage

- AC-001 -> T2.1, T3.1, T3.2, T4.1, T4.3
- AC-002 -> T2.1, T4.2
- AC-003 -> T2.1, T4.1
- AC-004 -> T2.1, T3.2, T4.1, T4.3
- AC-005 -> T2.1, T2.3, T4.1
- AC-006 -> T2.2, T4.1

## Notes

- Migration number 024 follows 023_models_endpoint.
- Cipher format: base64(nonce(12)||ciphertext). Empty string means not revealable.
- The gateway auth path is unchanged: it still matches `key_hash` only.
- Every `[x]` task needs a `Proof:` line on the next line; no proof means not done.
