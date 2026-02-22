-- Revert history table improvements.
DROP INDEX IF EXISTS idx_history_created_at;
ALTER TABLE history DROP COLUMN IF EXISTS updated_at;
ALTER TABLE history DROP COLUMN IF EXISTS created_at;
