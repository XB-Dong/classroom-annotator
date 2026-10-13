-- +goose Up
ALTER TABLE public_samples ADD COLUMN name TEXT NOT NULL DEFAULT '';
ALTER TABLE public_samples ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE public_samples ADD COLUMN license_url TEXT NOT NULL DEFAULT '';
ALTER TABLE public_samples ADD COLUMN storage_key TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX public_samples_storage_key ON public_samples(storage_key) WHERE storage_key <> '';

-- +goose Down
DROP INDEX public_samples_storage_key;
ALTER TABLE public_samples DROP COLUMN name, DROP COLUMN description, DROP COLUMN license_url, DROP COLUMN storage_key;
