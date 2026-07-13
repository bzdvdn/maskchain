CREATE TABLE IF NOT EXISTS tenants (
    slug          TEXT        PRIMARY KEY,
    name          TEXT        NOT NULL,
    auth_header   TEXT        NOT NULL DEFAULT 'Authorization',
    api_keys      JSONB       NOT NULL DEFAULT '[]'::JSONB,
    dictionaries  JSONB       NOT NULL DEFAULT '[]'::JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
