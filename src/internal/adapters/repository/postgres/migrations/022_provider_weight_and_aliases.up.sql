-- model-aliases-weighted-lb#T1.3: provider weight + routing_aliases (DM-001, DM-002)
ALTER TABLE routing_providers ADD COLUMN IF NOT EXISTS weight INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS routing_aliases (
    tenant     TEXT NOT NULL,
    alias      TEXT NOT NULL,
    target     TEXT NOT NULL,
    source     TEXT NOT NULL DEFAULT 'yaml',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant, alias)
);
