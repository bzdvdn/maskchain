CREATE TABLE IF NOT EXISTS dictionary_entries (
    profile_slug TEXT        PRIMARY KEY,
    name         TEXT        NOT NULL,
    entries      JSONB       NOT NULL DEFAULT '[]'::jsonb,
    match_mode   TEXT        NOT NULL CHECK (match_mode IN ('exact', 'contains', 'regex', 'fuzzy')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
