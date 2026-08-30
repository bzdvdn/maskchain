-- @sk-task mask-token-format#T1.1: Add reversible column for redact entries (AC-007)
ALTER TABLE mask_entries ADD COLUMN reversible boolean NOT NULL DEFAULT TRUE;