-- model-discovery-endpoint: rollback
ALTER TABLE routing_providers DROP COLUMN IF EXISTS models_endpoint;
