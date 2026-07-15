CREATE TABLE IF NOT EXISTS sessions (
    id                UUID        PRIMARY KEY,
    tenant_id         TEXT        NOT NULL,
    model             TEXT        NOT NULL,
    token_count       BIGINT      NOT NULL DEFAULT 0,
    message_count     INT         NOT NULL DEFAULT 0,
    total_masks       INT         NOT NULL DEFAULT 0,
    dict_mask_count   INT         NOT NULL DEFAULT 0,
    pii_mask_count    INT         NOT NULL DEFAULT 0,
    preprocessor_count INT        NOT NULL DEFAULT 0,
    status            TEXT        NOT NULL DEFAULT 'active',
    ttl               INTERVAL    NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at        TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_tenant_id ON sessions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions (status);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions (expires_at);
