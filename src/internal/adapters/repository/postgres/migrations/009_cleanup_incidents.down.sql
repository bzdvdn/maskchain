-- @sk-task remove-audit-incidents#T3.7: Restore incidents table (AC-015)
CREATE TABLE IF NOT EXISTS incidents (
    id           BIGSERIAL    PRIMARY KEY,
    profile_slug TEXT         NOT NULL DEFAULT '',
    request_id   TEXT         NOT NULL,
    detector_type TEXT        NOT NULL,
    entry_value  TEXT,
    severity     TEXT         NOT NULL,
    action       TEXT         NOT NULL,
    raw_snippet  TEXT,
    timestamp    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_incidents_timestamp ON incidents (timestamp DESC);
