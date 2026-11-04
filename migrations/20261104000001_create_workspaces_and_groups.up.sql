-- Who may do what, where: a clinic's branches (workspaces), the permission
-- groups whose grants are the contract's action keys (x-permission), and
-- each person's group and branches. A person with no branch listed works in
-- every one, as the owner does.
CREATE TABLE IF NOT EXISTS workspaces (
    id   TEXT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS permission_groups (
    id     TEXT PRIMARY KEY,
    name   TEXT NOT NULL,
    grants TEXT[] NOT NULL DEFAULT '{}',
    doctor BOOLEAN NOT NULL DEFAULT false
);

ALTER TABLE users
    ADD COLUMN name TEXT NOT NULL DEFAULT '',
    ADD COLUMN role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'member', 'viewer')),
    ADD COLUMN group_id TEXT REFERENCES permission_groups (id);

CREATE TABLE IF NOT EXISTS user_workspaces (
    user_id      UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workspace_id TEXT NOT NULL REFERENCES workspaces (id),
    PRIMARY KEY (user_id, workspace_id)
);
