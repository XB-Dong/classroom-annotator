# Backend

Build with Go 1.25: `go build -o backend-server ./cmd/server`. SQLite uses the pinned pure-Go modernc driver. FFmpeg and ffprobe must be installed at runtime. `go test ./...` runs capability tests; real media integration runs when FFmpeg/ffprobe are on PATH and otherwise reports a skip.

Required: `APP_DATA_DIR` is an absolute directory outside a source checkout (`DATA_DIR` is an alias). Data is stored as `app.sqlite`, `videos/`, immutable `frames/<batchId>/`, and private `staging/`.

| Variable | Default |
| --- | --- |
| APP_ADDR | :8080 |
| APP_VERSION | dev (or build `-ldflags -X main.version=<sha>`) |
| APP_FFMPEG / APP_FFPROBE | ffmpeg / ffprobe |
| APP_MAX_UPLOAD_BYTES | 1073741824 |
| APP_QUEUE_SIZE | 8 |
| APP_MAX_FRAMES | 1000 |
| APP_JOB_TIMEOUT_SECONDS | 600 |
| APP_HTTP_TIMEOUT_SECONDS | 30 |
| APP_DATASET_API_URL | unset; origin, e.g. https://company.example |
| APP_DATASET_ID | unset; required for company API |
| APP_DATASET_API_KEY | unset; X-API-Key for API only |
| APP_ALLOWED_PRIVATE_ORIGINS | unset; comma-separated exact origins explicitly permitted for company/fixtures |
| APP_ALLOWED_MEDIA_ORIGINS | unset; optional comma-separated media-origin restriction |

Company list routing is `/api/v1/datasets/{APP_DATASET_ID}/lessons?page=&page_size=`. Playback routing is `/api/v1/catalog-files/{catalogFileId}/play-url`. Private/loopback/link-local destinations are rejected by default, including redirect and resolved connection targets. API redirects must retain the original origin; media requests never receive the API key. Do not put credentials or signed URLs in logs.

One worker runs persisted bounded jobs. On startup unfinished jobs become failed with `INTERRUPTED`; staging and unpublished final directories are reconciled. Cancellation returns `cancelling` until work has exited and cleaned up. Only completed batches publish frames. Annotation/export endpoints return explicit 501 until the next capability is registered via `httpapi.Extension`.

Backup: stop the service before copying the complete data directory (database, WAL/SHM when present, videos and frames). Restore into a separate configured directory and start the same schema-compatible version. The schema is creation-only version 1; downgrade across future migrations requires a matching pre-upgrade backup. Never copy only app.sqlite while the service is running.
