-- @sk-task key-secret-reveal#T1.1: Store a retrievable (encrypted) virtual key secret (AC-001)
ALTER TABLE virtual_keys ADD COLUMN IF NOT EXISTS secret_encrypted TEXT NOT NULL DEFAULT '';
