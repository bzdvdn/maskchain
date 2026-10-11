-- @sk-task key-secret-reveal#T1.1: Drop the encrypted virtual key secret column
ALTER TABLE virtual_keys DROP COLUMN IF EXISTS secret_encrypted;
