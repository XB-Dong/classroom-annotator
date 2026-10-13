-- +goose Up
ALTER TABLE videos ADD COLUMN storage_provider TEXT NOT NULL DEFAULT 'local' CHECK(storage_provider IN ('local','mac'));
ALTER TABLE videos ADD COLUMN source_kind TEXT NOT NULL DEFAULT 'legacy' CHECK(source_kind IN ('public_sample','upload','mac','dataset','legacy'));
ALTER TABLE videos ADD COLUMN source_id TEXT;
ALTER TABLE videos ADD COLUMN media_sha256 TEXT NOT NULL DEFAULT '' CHECK(media_sha256='' OR media_sha256 ~ '^[0-9a-f]{64}$');
ALTER TABLE videos ADD COLUMN media_size_bytes BIGINT NOT NULL DEFAULT 0 CHECK(media_size_bytes>=0);
ALTER TABLE videos ADD CONSTRAINT videos_source_provider CHECK((source_kind='public_sample' AND storage_provider='local') OR (source_kind IN ('upload','mac','dataset') AND storage_provider='mac') OR source_kind='legacy');
ALTER TABLE videos DROP CONSTRAINT videos_storage_key_key;
ALTER TABLE videos ADD UNIQUE(storage_provider,storage_key);
ALTER TABLE videos ADD UNIQUE(id,storage_provider);
ALTER TABLE videos ADD UNIQUE(storage_provider,source_kind,source_id,media_sha256);
ALTER TABLE frames ADD COLUMN storage_provider TEXT NOT NULL DEFAULT 'local' CHECK(storage_provider IN ('local','mac'));
ALTER TABLE frames ADD COLUMN media_sha256 TEXT NOT NULL DEFAULT '' CHECK(media_sha256='' OR media_sha256 ~ '^[0-9a-f]{64}$');
ALTER TABLE frames ADD COLUMN media_size_bytes BIGINT NOT NULL DEFAULT 0 CHECK(media_size_bytes>=0);
ALTER TABLE frames DROP CONSTRAINT frames_storage_key_key;
ALTER TABLE frames ADD UNIQUE(storage_provider,storage_key);
ALTER TABLE frames ADD FOREIGN KEY(video_id,storage_provider) REFERENCES videos(id,storage_provider);
CREATE TABLE public_samples(sample_id TEXT NOT NULL,version TEXT NOT NULL,author TEXT NOT NULL,license TEXT NOT NULL,source_page TEXT NOT NULL,media_sha256 TEXT NOT NULL CHECK(media_sha256 ~ '^[0-9a-f]{64}$'),media_size_bytes BIGINT NOT NULL CHECK(media_size_bytes>0),width INTEGER NOT NULL CHECK(width>0),height INTEGER NOT NULL CHECK(height>0),duration_seconds DOUBLE PRECISION NOT NULL CHECK(duration_seconds>0 AND duration_seconds<'Infinity'::float8),cache_status TEXT NOT NULL CHECK(cache_status IN ('unavailable','ready')),video_id TEXT REFERENCES videos(id),PRIMARY KEY(sample_id,version,media_sha256));
CREATE TABLE media_operations(operation_id TEXT PRIMARY KEY,operation_type TEXT NOT NULL CHECK(operation_type IN ('registration','upload','dataset_import','extraction','archive')),job_id TEXT REFERENCES jobs(id),resource_id TEXT,provider TEXT NOT NULL CHECK(provider='mac'),request_fingerprint TEXT NOT NULL CHECK(request_fingerprint ~ '^[0-9a-f]{64}$'),status TEXT NOT NULL CHECK(status IN ('queued','running','completed','failed','cancelled')),publication_status TEXT NOT NULL DEFAULT 'unpublished' CHECK(publication_status IN ('unpublished','committed')),publication_revision BIGINT NOT NULL DEFAULT 0 CHECK(publication_revision>=0),acknowledged BOOLEAN NOT NULL DEFAULT FALSE,error_code TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL,updated_at TEXT NOT NULL,CHECK((publication_status='unpublished' AND publication_revision=0) OR (publication_status='committed' AND publication_revision>0)));
CREATE INDEX media_operations_job ON media_operations(job_id);

-- +goose Down
DROP TABLE media_operations,public_samples;
ALTER TABLE frames DROP CONSTRAINT frames_video_id_storage_provider_fkey;
ALTER TABLE frames DROP CONSTRAINT frames_storage_provider_storage_key_key;
ALTER TABLE frames DROP COLUMN storage_provider,DROP COLUMN media_sha256,DROP COLUMN media_size_bytes;
ALTER TABLE frames ADD UNIQUE(storage_key);
ALTER TABLE videos DROP CONSTRAINT videos_storage_provider_storage_key_key;
ALTER TABLE videos DROP CONSTRAINT videos_id_storage_provider_key;
ALTER TABLE videos DROP CONSTRAINT videos_storage_provider_source_kind_source_id_media_sha256_key;
ALTER TABLE videos DROP CONSTRAINT videos_source_provider;
ALTER TABLE videos DROP COLUMN storage_provider,DROP COLUMN source_kind,DROP COLUMN source_id,DROP COLUMN media_sha256,DROP COLUMN media_size_bytes;
ALTER TABLE videos ADD UNIQUE(storage_key);
