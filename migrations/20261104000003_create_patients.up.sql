-- A clinic's patients: the clinic's, made at a branch (workspace). The code
-- is issued from the counter, KH-000001 on, never reused.
CREATE SEQUENCE IF NOT EXISTS patient_code_seq;

CREATE TABLE IF NOT EXISTS patients (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code       TEXT NOT NULL UNIQUE,
    workspace  TEXT NOT NULL REFERENCES workspaces (id),
    name       TEXT NOT NULL,
    gender     TEXT CHECK (gender IN ('male', 'female')),
    birth      TEXT NOT NULL DEFAULT '',
    phone      TEXT NOT NULL,
    cccd       TEXT NOT NULL DEFAULT '',
    email      TEXT NOT NULL DEFAULT '',
    address    TEXT NOT NULL DEFAULT '',
    occupation TEXT NOT NULL DEFAULT '',
    note       TEXT NOT NULL DEFAULT '',
    tags       TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_patients_created ON patients (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_patients_workspace ON patients (workspace);
