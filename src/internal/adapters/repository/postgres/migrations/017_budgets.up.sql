-- @sk-task 301-budget-enforcement#T1.2: Create budgets, spend history, and daily aggregation tables (AC-002)
CREATE TABLE IF NOT EXISTS budgets (
    id             TEXT         PRIMARY KEY,
    tenant_id      TEXT         NOT NULL REFERENCES tenants(slug) ON DELETE CASCADE,
    virtual_key_id TEXT         REFERENCES virtual_keys(id) ON DELETE CASCADE,
    model          TEXT         NOT NULL DEFAULT '',
    scope          TEXT         NOT NULL,
    type           TEXT         NOT NULL,
    custom_days    INT          NOT NULL DEFAULT 0,
    soft_limit     NUMERIC(14,6),
    hard_limit     NUMERIC(14,6),
    currency       TEXT         NOT NULL DEFAULT 'USD',
    notify_at      JSONB        NOT NULL DEFAULT '[]'::JSONB,
    enabled        BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_budgets_tenant ON budgets (tenant_id);
CREATE INDEX IF NOT EXISTS idx_budgets_key ON budgets (virtual_key_id);

CREATE TABLE IF NOT EXISTS budget_spend (
    id             TEXT         PRIMARY KEY,
    budget_id      TEXT         NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    virtual_key_id TEXT         NOT NULL DEFAULT '',
    tenant_id      TEXT         NOT NULL DEFAULT '',
    model          TEXT         NOT NULL DEFAULT '',
    cost           NUMERIC(14,6) NOT NULL,
    tokens         BIGINT       NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_budget_spend_budget ON budget_spend (budget_id, created_at DESC);

CREATE TABLE IF NOT EXISTS budget_spend_daily (
    budget_id      TEXT         NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    virtual_key_id TEXT         NOT NULL DEFAULT '',
    tenant_id      TEXT         NOT NULL DEFAULT '',
    model          TEXT         NOT NULL DEFAULT '',
    day            DATE         NOT NULL,
    total_cost     NUMERIC(14,6) NOT NULL DEFAULT 0,
    total_tokens   BIGINT       NOT NULL DEFAULT 0,
    request_count  BIGINT       NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (budget_id, virtual_key_id, tenant_id, model, day)
);
