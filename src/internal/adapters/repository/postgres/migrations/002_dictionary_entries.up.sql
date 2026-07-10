CREATE TABLE IF NOT EXISTS dictionary_entries (
    id           BIGSERIAL    PRIMARY KEY,
    profile_slug TEXT         NOT NULL REFERENCES profiles(slug),
    entry_value  TEXT         NOT NULL,
    match_mode   TEXT         NOT NULL CHECK (match_mode IN ('exact', 'contains', 'regex', 'fuzzy')),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_dictionary_entries_profile_slug ON dictionary_entries (profile_slug);
