-- model-aliases-weighted-lb#T1.3: rollback provider weight + routing_aliases (DM-001, DM-002)
DROP TABLE IF EXISTS routing_aliases;

ALTER TABLE routing_providers DROP COLUMN IF EXISTS weight;
