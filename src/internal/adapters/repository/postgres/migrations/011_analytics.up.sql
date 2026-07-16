-- @sk-task 131-analytics-pipeline#T1.1: Create analytics tables (AC-002, AC-004, AC-008)
CREATE TABLE IF NOT EXISTS usage_raw (
    id                UUID         PRIMARY KEY,
    tenant_id         VARCHAR(255) NOT NULL,
    model             VARCHAR(255) NOT NULL,
    input_tokens      BIGINT       NOT NULL CHECK (input_tokens >= 0),
    output_tokens     BIGINT       NOT NULL CHECK (output_tokens >= 0),
    cost              NUMERIC(12,6) NOT NULL CHECK (cost >= 0),
    recorded_at       TIMESTAMPTZ  NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_usage_raw_tenant_recorded ON usage_raw (tenant_id, recorded_at);
CREATE INDEX IF NOT EXISTS idx_usage_raw_recorded_at ON usage_raw (recorded_at);

CREATE TABLE IF NOT EXISTS usage_agg_hourly (
    tenant_id          VARCHAR(255) NOT NULL,
    model              VARCHAR(255) NOT NULL,
    hour               TIMESTAMPTZ  NOT NULL,
    total_input_tokens  BIGINT       NOT NULL CHECK (total_input_tokens >= 0),
    total_output_tokens BIGINT       NOT NULL CHECK (total_output_tokens >= 0),
    total_cost          NUMERIC(14,6) NOT NULL CHECK (total_cost >= 0),
    request_count       BIGINT       NOT NULL CHECK (request_count >= 0),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, model, hour)
);

CREATE INDEX IF NOT EXISTS idx_usage_agg_hourly_tenant ON usage_agg_hourly (tenant_id, hour);
CREATE INDEX IF NOT EXISTS idx_usage_agg_hourly_model ON usage_agg_hourly (model, hour);

CREATE TABLE IF NOT EXISTS usage_agg_daily (
    tenant_id          VARCHAR(255) NOT NULL,
    model              VARCHAR(255) NOT NULL,
    day                DATE         NOT NULL,
    total_input_tokens  BIGINT       NOT NULL CHECK (total_input_tokens >= 0),
    total_output_tokens BIGINT       NOT NULL CHECK (total_output_tokens >= 0),
    total_cost          NUMERIC(14,6) NOT NULL CHECK (total_cost >= 0),
    request_count       BIGINT       NOT NULL CHECK (request_count >= 0),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, model, day)
);

CREATE INDEX IF NOT EXISTS idx_usage_agg_daily_tenant ON usage_agg_daily (tenant_id, day);
CREATE INDEX IF NOT EXISTS idx_usage_agg_daily_model ON usage_agg_daily (model, day);
