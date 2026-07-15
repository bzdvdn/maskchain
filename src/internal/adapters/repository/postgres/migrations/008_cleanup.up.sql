-- @sk-task cleanup-profile-repository#T4.2: Drop profiles, dictionary_entries tables and profile_id column (AC-011)
DROP TABLE IF EXISTS dictionary_entries;
DROP TABLE IF EXISTS profiles CASCADE;
ALTER TABLE mask_entries DROP COLUMN IF EXISTS profile_id;
