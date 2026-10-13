-- +goose Up
CREATE TABLE workspaces (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO workspaces(id,name) VALUES('00000000-0000-4000-8000-000000000001','Shared workspace');
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE CHECK(username ~ '^[a-z0-9_-]{3,32}$'),
    display_name TEXT NOT NULL CHECK(length(display_name) BETWEEN 3 AND 32),
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE workspace_members (
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK(role IN ('admin','annotator')),
    PRIMARY KEY(workspace_id,user_id)
);
CREATE TABLE sessions (
    token_digest BYTEA PRIMARY KEY CHECK(octet_length(token_digest)=32),
    user_id TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    csrf_token TEXT NOT NULL CHECK(length(csrf_token)=43),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    FOREIGN KEY(workspace_id,user_id) REFERENCES workspace_members(workspace_id,user_id) ON DELETE CASCADE
);
CREATE INDEX sessions_expiry ON sessions(expires_at);
ALTER TABLE videos ADD COLUMN workspace_id TEXT REFERENCES workspaces(id);
UPDATE videos SET workspace_id='00000000-0000-4000-8000-000000000001' WHERE workspace_id IS NULL;
ALTER TABLE videos ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE videos ALTER COLUMN workspace_id SET DEFAULT '00000000-0000-4000-8000-000000000001';
CREATE INDEX videos_workspace ON videos(workspace_id);
ALTER TABLE frames ADD COLUMN updated_by TEXT REFERENCES users(id);
ALTER TABLE frames ADD COLUMN updated_at TIMESTAMPTZ;
CREATE TABLE annotation_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    frame_id TEXT NOT NULL REFERENCES frames(id),
    user_id TEXT NOT NULL REFERENCES users(id),
    action TEXT NOT NULL CHECK(action IN ('save','complete','reopen','accept','reject','resolve')),
    revision BIGINT NOT NULL CHECK(revision>0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(frame_id,revision)
);

-- +goose Down
DROP TABLE annotation_events;
ALTER TABLE frames DROP COLUMN updated_at, DROP COLUMN updated_by;
ALTER TABLE videos DROP COLUMN workspace_id;
DROP TABLE sessions,workspace_members,users,workspaces;
