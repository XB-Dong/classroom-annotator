-- +goose Up
ALTER TABLE videos DROP CONSTRAINT videos_source_kind_check;
ALTER TABLE videos ADD CONSTRAINT videos_source_kind_check CHECK(source_kind IN ('public_sample','upload','mac','dataset','legacy','local_directory'));
ALTER TABLE videos DROP CONSTRAINT videos_source_provider;
ALTER TABLE videos ADD CONSTRAINT videos_source_provider CHECK(
 (source_kind='public_sample' AND storage_provider='local') OR
 (source_kind IN ('upload','dataset') AND storage_provider IN ('local','mac')) OR
 (source_kind='local_directory' AND storage_provider='local') OR
 (source_kind='mac' AND storage_provider='mac') OR source_kind='legacy');
CREATE TABLE local_import_requests (
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    idempotency_key_hash TEXT NOT NULL CHECK (idempotency_key_hash ~ '^[0-9a-f]{64}$'),
    source_id TEXT NOT NULL CHECK (source_id ~ '^[0-9a-f]{64}$'),
    job_id TEXT NOT NULL REFERENCES jobs(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, idempotency_key_hash)
);
