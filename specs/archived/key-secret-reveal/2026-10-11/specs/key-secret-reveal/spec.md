# Key Secret Reveal

## Scope Snapshot

- In scope: store each virtual key's secret encrypted at rest so an admin can **reveal it again** from the console, plus the reveal endpoint and UI action.
- Out of scope: changing how keys authenticate, exposing secrets to the gateway path, and revealing secrets for keys created before this feature (they must be rotated/re-created).

## Goal

Operators currently cannot re-read a virtual key secret: only its SHA-256 hash is stored, so a lost secret must be rotated. This feature stores the secret encrypted with the existing at-rest key (`MASKCHAIN_KEYS_KEY`, AES-256-GCM) so an admin session can reveal it on demand. Success is visible as a "Reveal" action on the Keys page that shows the secret again with copy — while the security posture stays explicit: the secret is recoverable only by an authenticated admin, and every reveal is audited.

## Primary User Flow

1. Starting point: an admin opens Keys and needs a key they created earlier.
2. Main interaction: they click "Reveal" on the key row and confirm.
3. Outcome: the plaintext secret is returned by the API and shown in a copy panel; the reveal is written to the audit log.
4. Failure/fallback path: for legacy keys (created before this feature) or when at-rest encryption is not configured, reveal returns `409 SECRET_UNAVAILABLE` and the UI offers Rotate instead.

## User Stories

- P1 Story: as an operator, I can reveal an existing key secret so I do not have to rotate a key just to see it.

## MVP Slice

- Encrypted storage at create/rotate + `GET /api/v1/keys/:id/secret` + UI Reveal (AC-001, AC-002, AC-003).

## First Deployable Outcome

- Create a key, then reveal the same secret from the Keys page.

## Scope

- A `secret_encrypted` column on `virtual_keys` and the migration that adds it.
- Sealing the secret at create and rotate; revealing via a new admin endpoint; audit logging.
- UI Reveal action with copy and a rotate fallback for unavailable secrets.

## Context

- `crypto.Encryptor` (AES-256-GCM, `MASKCHAIN_KEYS_KEY`) already exists and seals provider secrets at rest; this feature reuses it.
- The config validator already requires `MASKCHAIN_KEYS_KEY` when database-backed routing/tenancy is enabled, so the key is present in DB deployments.
- The gateway authenticates by hashing the presented key and matching `key_hash`; reveal must not change that path.

## Dependencies

- Reuses `crypto.Encryptor` (`src/internal/infra/crypto`) and `config.KeysKeyEnvVar`.

## Requirements

- RQ-001 Add a nullable `secret_encrypted` column to `virtual_keys` via migration 024.
- RQ-002 At create and rotate, when an encryptor is configured, persist the AES-256-GCM ciphertext of the raw secret.
- RQ-003 Add admin endpoint `GET /api/v1/keys/:id/secret` returning the plaintext secret for an authenticated admin.
- RQ-004 List/detail/CREATE responses MUST NOT include the secret beyond the existing one-time `key` field, and MUST NOT expose the ciphertext.
- RQ-005 Return `409 SECRET_UNAVAILABLE` when the key has no stored secret or no encryptor is configured.
- RQ-006 Write an audit entry on every reveal.
- RQ-007 UI: a Reveal action per key shows the secret in a copy panel; on `SECRET_UNAVAILABLE` show guidance and offer Rotate.
- RQ-008 The secret MUST never be stored in plaintext on the server.

## Non-Goals

- Revealing secrets from the gateway or by tenant keys.
- Changing authentication to compare against the stored secret.
- Bulk reveal or export of secrets.
- Revealing secrets for pre-existing keys (only post-rotate/create).

## Acceptance Criteria

### AC-001 Reveal a stored secret

- Why this matters: the core operator need.
- **Given** a key created with at-rest encryption configured
- **When** an admin requests the key secret
- **Then** the API returns the same plaintext secret that was issued
- Evidence: a handler test asserts `GET /keys/:id/secret` returns the original secret.

### AC-002 Secrets are never leaked

- Why this matters: no over-exposure.
- **Given** any list or detail response
- **When** it serializes keys
- **Then** neither the plaintext secret nor the ciphertext appears
- Evidence: a DTO test asserts the response struct has no secret/ciphertext field.

### AC-003 Rotate refreshes the stored secret

- Why this matters: reveal must stay correct after rotation.
- **Given** a key with a stored secret
- **When** the admin rotates it
- **Then** revealing returns the new secret and the old one no longer authenticates
- Evidence: a test asserts reveal after rotate returns the new secret.

### AC-004 Legacy keys degrade to a clear error

- Why this matters: existing keys have no stored secret.
- **Given** a key whose `secret_encrypted` is empty
- **When** the admin requests its secret
- **Then** the API returns `409 SECRET_UNAVAILABLE`
- Evidence: a handler test asserts the 409 code and message.

### AC-005 Missing encryptor degrades safely

- Why this matters: no plaintext fallback.
- **Given** a deployment with no configured key encryption
- **When** a key is created and revealed
- **Then** create still succeeds, nothing is stored in plaintext, and reveal returns `409 SECRET_UNAVAILABLE`
- Evidence: a test constructs the handler with a nil encryptor and asserts the behavior.

### AC-006 Reveal is audited

- Why this matters: sensitive action traceability.
- **Given** a successful reveal
- **When** it completes
- **Then** an audit entry records the action and key id
- Evidence: a test asserts the audit logger received a `reveal_key_secret` entry.

## Assumptions

- `MASKCHAIN_KEYS_KEY` is present in DB deployments (enforced by the config validator), so the encryptor is normally non-nil.
- Column `secret_encrypted` stores base64 of `nonce(12)||ciphertext`; empty string means "unavailable".
- Admin session auth (existing) is the authorization boundary; no extra re-auth is added in this slice.

## Success Criteria

- SC-001 A key created and then revealed returns a secret equal to the one shown at creation.

## Edge Cases

- Empty/legacy `secret_encrypted` → `409 SECRET_UNAVAILABLE`.
- Nil encryptor → no ciphertext stored, reveal unavailable, no plaintext fallback.
- Decryption failure (wrong/rotated encryption key) → `409 SECRET_UNAVAILABLE` (treated as unavailable, not 500).
- Revealing a disabled or expired key is still allowed (admin maintenance).

## Open Questions

- none
