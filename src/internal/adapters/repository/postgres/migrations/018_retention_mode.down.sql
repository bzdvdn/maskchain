-- zero-retention-mode#T1.1: Drop retention_mode and detector/category on rollback (DM-001, DM-002)
ALTER TABLE conversation_logs DROP COLUMN IF EXISTS detector;
ALTER TABLE conversation_logs DROP COLUMN IF EXISTS category;

ALTER TABLE tenants DROP COLUMN IF EXISTS retention_mode;