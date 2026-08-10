-- @sk-task conversation-logging#T1.4: Add conversation_logs table (AC-001, AC-004, AC-006, AC-007)
CREATE TABLE IF NOT EXISTS conversation_logs (
    id            UUID        PRIMARY KEY,
    tenant_id     VARCHAR(255) NOT NULL,
    model         VARCHAR(255) NOT NULL,
    status        VARCHAR(32)  NOT NULL,
    masked        BOOLEAN      NOT NULL DEFAULT false,
    masking       BYTEA,
    request       BYTEA        NOT NULL,
    response      BYTEA,
    request_len   INTEGER,
    response_len  INTEGER,
    created_at    TIMESTAMPTZ  NOT NULL
);

-- @sk-task conversation-logging#T2.4: Add streamed column (RQ-009, DEC-007, AC-003)
ALTER TABLE conversation_logs ADD COLUMN IF NOT EXISTS streamed BOOLEAN NOT NULL DEFAULT false;

-- @sk-task conversation-logging#T5.1: Add mask_id column to link log with shield mask (AC-005, AC-006)
ALTER TABLE conversation_logs ADD COLUMN IF NOT EXISTS mask_id VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_conversation_logs_tenant_created_at ON conversation_logs (tenant_id, created_at);