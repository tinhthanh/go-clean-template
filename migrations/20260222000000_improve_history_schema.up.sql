-- Improve history table: add timestamps and indexes for production use.
-- Note: id serial PRIMARY KEY already exists from initial migration.
ALTER TABLE history ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE history ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
CREATE INDEX idx_history_created_at ON history(created_at DESC);
