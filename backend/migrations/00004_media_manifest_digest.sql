-- +goose Up
ALTER TABLE media_operations ADD COLUMN manifest_sha256 TEXT NOT NULL DEFAULT '' CHECK(manifest_sha256='' OR manifest_sha256 ~ '^[0-9a-f]{64}$');

-- +goose Down
ALTER TABLE media_operations DROP COLUMN manifest_sha256;
