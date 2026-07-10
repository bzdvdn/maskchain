CREATE TABLE IF NOT EXISTS mask_entries (
    mask_id     TEXT        PRIMARY KEY,
    profile_id  TEXT,
    replacements JSONB     NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mask_entries_created_at ON mask_entries (created_at DESC);
