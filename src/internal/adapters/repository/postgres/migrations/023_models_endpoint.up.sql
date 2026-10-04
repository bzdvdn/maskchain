-- model-discovery-endpoint: optional per-provider models endpoint override
ALTER TABLE routing_providers ADD COLUMN IF NOT EXISTS models_endpoint TEXT NOT NULL DEFAULT '';
