-- zero-retention-mode#T1.1: Add retention_mode to tenants + detector/category to conversation_logs (DM-001, DM-002)
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS retention_mode VARCHAR(16) NOT NULL DEFAULT 'full';

ALTER TABLE conversation_logs ADD COLUMN IF NOT EXISTS detector VARCHAR(64);
ALTER TABLE conversation_logs ADD COLUMN IF NOT EXISTS category VARCHAR(64);