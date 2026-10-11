# Key Secret Reveal Plan

## Approach

Store each virtual key's plaintext secret as AES-256-GCM ciphertext in a dedicated column, sealed with the existing at-rest key, and expose it through a new admin-only reveal endpoint. Keep the domain and DTOs free of plaintext: the handler seals at create/rotate and opens only in the reveal handler.

## Decisions

- DEC-001 Storage: add `virtual_keys.secret_encrypted TEXT NOT NULL DEFAULT ''`; store base64 of `nonce(12)||ciphertext`; empty means "not revealable". Rationale: mirrors the existing at-rest pattern and keeps columns nullable-safe.
- DEC-002 Encryption: reuse `crypto.Encryptor` + `config.KeysKeyEnvVar` (`MASKCHAIN_KEYS_KEY`); do not introduce a new key or algorithm.
- DEC-003 Boundary: the domain `VirtualKey` carries an opaque `SecretEncrypted` string; the handler owns seal/open; DTOs never map it. Rationale: keeps plaintext out of the domain and API responses.
- DEC-004 Unavailable vs error: missing ciphertext, nil encryptor, or decryption failure all return `409 SECRET_UNAVAILABLE` with actionable wording. Rationale: a clear, non-fatal state the UI can act on; avoids 500s leaking internals.
- DEC-005 Wiring: add `NewPostgresVirtualKeyRepositoryWithEncryptor(pool, enc)` (keep the existing constructor); the handler gains a `*crypto.Encryptor` argument. Wire it in `cmd/admin/run.go` and `cmd/all/admin.go`; the gateway repo keeps the no-encryptor constructor (auth uses hashes only).
- DEC-006 Audit: reveal writes `reveal_key_secret` with the key id; no secret in the audit payload.
- DEC-007 UI: a Reveal action (eye icon) per key row reuses the existing one-time secret panel; on `SECRET_UNAVAILABLE` show a hint + Rotate.

## Rollout / Migration

- Migration 024 adds the column; existing rows get `''` → reveal returns 409 until the key is rotated.
- No backfill possible (plaintext was never stored).
- The gateway is unaffected: authentication still matches `key_hash`.

## Risks / Mitigations

- Recoverable secrets raise impact if an operator session is compromised → admin-only endpoint, audited, and no exposure in list responses.
- Wrong encryption key after rotation → decryption failure maps to 409, not a crash.
- Encryptor not wired in one binary → feature degrades to 409; covered by wiring tasks + AC-005.

## Data model

See `data-model.md`.

## Implementation Surfaces

- Storage: migration 024, `domain/virtualkey/entity.go`, `postgres/virtual_key_repo.go`.
- API: `api/middleware/errors.go`, `api/dto/virtual_key.go`, `api/handler/admin/virtual_key_handler.go`, `api/admin.go`.
- Wiring: `cmd/admin/run.go`, `cmd/all/admin.go`.
- UI: `ui/src/api/keys.ts`, `ui/src/pages/Keys.tsx`.

## Constitution Compliance

- Security: secrets are encrypted at rest (no plaintext), reveal is admin-only and audited.
- Scope discipline: no change to the gateway auth path; the gateway repo keeps the no-encryptor constructor.

## Data and Contracts

- New column `virtual_keys.secret_encrypted TEXT NOT NULL DEFAULT ''`.
- `GET /api/v1/keys/:id/secret` → `200 {"key":"sk-mc_..."}`; `409 {"error":{"code":"SECRET_UNAVAILABLE"}}`; `404` unknown id.
- List/detail/create/rotate responses: unchanged shape; never include `secret_encrypted`.

## Acceptance Approach

- AC-001/AC-003/AC-004/AC-005/AC-006 via handler tests against the in-memory `fakeVirtualKeyRepo` (create+reveal, rotate+reveal, empty ciphertext, nil encryptor, audit capture).
- AC-002 via a DTO test asserting the response type exposes no secret/ciphertext field.
- UI: `Keys.test.tsx` covers the Reveal action and the `SECRET_UNAVAILABLE` fallback.
