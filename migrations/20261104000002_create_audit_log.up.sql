-- The log: only ever added to. An UPDATE, DELETE or TRUNCATE is refused by
-- the database itself, whoever asks (NFR-33).
CREATE TABLE IF NOT EXISTS audit_log (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_id   UUID,
    actor_name TEXT NOT NULL DEFAULT '',
    branch     TEXT,
    action     TEXT NOT NULL,
    resource   TEXT NOT NULL,
    target     TEXT NOT NULL DEFAULT '',
    rows       INTEGER,
    changes    JSONB
);

CREATE INDEX IF NOT EXISTS idx_audit_log_at ON audit_log (at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_look ON audit_log (actor_id, action, resource, target, at DESC);

CREATE OR REPLACE FUNCTION audit_log_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only';
END
$$;

CREATE TRIGGER audit_log_no_change BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_append_only();
CREATE TRIGGER audit_log_no_truncate BEFORE TRUNCATE ON audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION audit_log_append_only();
