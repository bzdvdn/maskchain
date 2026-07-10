CREATE TABLE IF NOT EXISTS profiles (
    id          TEXT        PRIMARY KEY,
    slug        TEXT        NOT NULL UNIQUE,
    name        TEXT        NOT NULL,
    tenant_id   TEXT        NOT NULL,
    preprocessors JSONB,
    status      TEXT        NOT NULL DEFAULT 'active',
    version     INTEGER     NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
