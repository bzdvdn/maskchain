CREATE TABLE IF NOT EXISTS incidents (
    id              SERIAL PRIMARY KEY,
    slug            TEXT        NOT NULL DEFAULT '',
    profile_slug    TEXT        NOT NULL DEFAULT '',
    request_id      TEXT        NOT NULL DEFAULT '',
    detector_type   TEXT        NOT NULL DEFAULT '',
    entry_value     TEXT,
    severity        TEXT        NOT NULL DEFAULT 'low',
    action          TEXT        NOT NULL DEFAULT 'alert',
    raw_snippet     TEXT,
    prompt_snippet_redacted TEXT,
    response_snippet TEXT,
    tenant          TEXT        NOT NULL DEFAULT '',
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE incidents ADD COLUMN IF NOT EXISTS tenant TEXT NOT NULL DEFAULT '';
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS response_snippet TEXT;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS prompt_snippet_redacted TEXT;
