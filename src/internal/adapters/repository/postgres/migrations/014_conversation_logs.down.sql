-- @sk-task conversation-logging#T1.4: Drop conversation_logs table (AC-001)
-- @sk-task conversation-logging#T2.4: Drop streamed column (RQ-009, DEC-007, AC-003)
-- @sk-task conversation-logging#T5.1: Drop mask_id column (AC-005)
ALTER TABLE conversation_logs DROP COLUMN IF EXISTS streamed;
ALTER TABLE conversation_logs DROP COLUMN IF EXISTS mask_id;
DROP INDEX IF EXISTS idx_conversation_logs_tenant_created_at;
DROP TABLE IF EXISTS conversation_logs;