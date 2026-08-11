-- @sk-task 300-virtual-keys#T1.1: Create virtual_keys table (AC-001)
CREATE TABLE IF NOT EXISTS virtual_keys (
    id             TEXT         PRIMARY KEY,
    key_hash       TEXT         NOT NULL UNIQUE,
    tenant_id      TEXT         NOT NULL REFERENCES tenants(slug) ON DELETE CASCADE,
    label          TEXT         NOT NULL DEFAULT '',
    allowed_models JSONB        NOT NULL DEFAULT '[]'::JSONB,
    blocked_models JSONB        NOT NULL DEFAULT '[]'::JSONB,
    budget_cap     NUMERIC(14,6),
    spent          NUMERIC(14,6) NOT NULL DEFAULT 0,
    expires_at     TIMESTAMPTZ,
    metadata       JSONB        NOT NULL DEFAULT '{}'::JSONB,
    enabled        BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_virtual_keys_tenant ON virtual_keys (tenant_id);
CREATE INDEX IF NOT EXISTS idx_virtual_keys_hash ON virtual_keys (key_hash);