-- @sk-task cleanup-profile-repository#T4.2: Restore profiles, dictionary_entries tables and profile_id column (AC-011)
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
CREATE TABLE IF NOT EXISTS dictionary_entries (
    id           BIGSERIAL    PRIMARY KEY,
    profile_slug TEXT         NOT NULL REFERENCES profiles(slug),
    entry_value  TEXT         NOT NULL,
    match_mode   TEXT         NOT NULL CHECK (match_mode IN ('exact', 'contains', 'regex', 'fuzzy')),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_dictionary_entries_profile_slug ON dictionary_entries (profile_slug);
ALTER TABLE mask_entries ADD COLUMN IF NOT EXISTS profile_id TEXT;
