-- @sk-task 150-admin-routing-crud#T1.1: Create routing registry tables seeded from YAML (AC-001)
CREATE TABLE IF NOT EXISTS routing_providers (
    name                 TEXT        PRIMARY KEY,
    api_type             TEXT        NOT NULL DEFAULT 'openai',
    base_url             TEXT        NOT NULL,
    health_endpoint      TEXT        NOT NULL DEFAULT '',
    timeout              TEXT        NOT NULL DEFAULT '',
    priority             INT         NOT NULL DEFAULT 0,
    api_keys             JSONB       NOT NULL DEFAULT '[]'::JSONB,
    auth_scheme          TEXT        NOT NULL DEFAULT '',
    auth_header          TEXT        NOT NULL DEFAULT '',
    auth_prefix          TEXT        NOT NULL DEFAULT '',
    additional_headers   JSONB       NOT NULL DEFAULT '{}'::JSONB,
    proxy_url            TEXT        NOT NULL DEFAULT '',
    aws_region           TEXT        NOT NULL DEFAULT '',
    aws_access_key_id    TEXT        NOT NULL DEFAULT '',
    aws_secret_access_key TEXT       NOT NULL DEFAULT '',
    source               TEXT        NOT NULL DEFAULT 'ui' CHECK (source IN ('yaml', 'ui')),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS routing_model_routes (
    id         BIGSERIAL   PRIMARY KEY,
    model      TEXT        NOT NULL,
    tenant     TEXT        NOT NULL DEFAULT '',
    providers  JSONB       NOT NULL DEFAULT '[]'::JSONB,
    source     TEXT        NOT NULL DEFAULT 'ui' CHECK (source IN ('yaml', 'ui')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (model, tenant)
);

CREATE INDEX IF NOT EXISTS idx_routing_model_routes_model ON routing_model_routes (model);
CREATE INDEX IF NOT EXISTS idx_routing_model_routes_tenant ON routing_model_routes (tenant);

CREATE TABLE IF NOT EXISTS cost_rates (
    model               TEXT        PRIMARY KEY,
    input_price_per_1k  NUMERIC(12,6) NOT NULL DEFAULT 0,
    output_price_per_1k NUMERIC(12,6) NOT NULL DEFAULT 0,
    currency            TEXT        NOT NULL DEFAULT 'USD',
    source              TEXT        NOT NULL DEFAULT 'ui' CHECK (source IN ('yaml', 'ui')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);