-- +goose Up
ALTER TABLE jobs ADD COLUMN started_at TIMESTAMPTZ;
ALTER TABLE jobs ADD COLUMN finished_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE jobs DROP COLUMN finished_at;
ALTER TABLE jobs DROP COLUMN started_at;
