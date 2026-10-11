# Key Secret Reveal — Data Model

## Change

Add one column to `virtual_keys`.

| Column | Type | Null | Default | Notes |
|--------|------|------|---------|-------|
| `secret_encrypted` | `TEXT` | no | `''` | base64 of AES-256-GCM `nonce(12)||ciphertext`; `''` means "not revealable" |

## Migration 024

- Up: `ALTER TABLE virtual_keys ADD COLUMN IF NOT EXISTS secret_encrypted TEXT NOT NULL DEFAULT '';`
- Down: `ALTER TABLE virtual_keys DROP COLUMN IF EXISTS secret_encrypted;`

## Invariants

- `key_hash` remains the only authentication source; `secret_encrypted` is never used by the gateway auth path.
- `secret_encrypted` is never returned by any list/detail DTO.
- Empty `secret_encrypted` (legacy or no encryptor) ⇒ reveal returns `409 SECRET_UNAVAILABLE`.
