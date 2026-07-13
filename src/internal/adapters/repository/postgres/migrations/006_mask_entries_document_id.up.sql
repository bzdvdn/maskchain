ALTER TABLE mask_entries ADD COLUMN IF NOT EXISTS document_mask_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_mask_entries_document_mask_id ON mask_entries (document_mask_id);
