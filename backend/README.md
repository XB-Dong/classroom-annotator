# Backend

The active backend is Go/Gin with database/sql + pgx and PostgreSQL 17. See go.mod for the exact Go toolchain, the migration directory for the current schema, and ../docs/deployment/P1-RUNBOOK.zh.md for deployment. SQLite is supported only as an explicit legacy import source, not the running database.

Run explicit `cmd/migrate` and `cmd/bootstrap-user` commands with external restricted configuration before the app. The application role uses business DML, not automatic schema creation. Runtime data, media, secrets, logs and Go caches stay outside this checkout. `APP_DATA_DIR` is an absolute external media root; `APP_DATABASE_URL` points at PostgreSQL. Configuration keys and limits are declared in `.env.example` and `internal/platform/config`.

`cmd/server` registers auth, uploads/sources, extraction, annotation CAS, preannotation/review, whole-document help, and exports. Go is the sole business/data entry point. Python ONNX is an optional internal service without database access. Retired remote-media records remain explicitly unavailable and are not silently mapped to local files.

Annotation saves use transactional expectedRevision CAS. Stale writes return409 without partial changes. Exports contain only completed frames and formal reviewed labels, including explicitly completed zero-head frames, with original JPEG and COCO JSON in a closed ZIP. Repeatable-read snapshots, provenance/review gates, and media identity checks are independent of advisory quality hints.

Company API routing follows the configured observed adapter contract. HTTP redirects/resolved destinations are validated, and media requests never receive the API key. Do not log credentials or signed URLs. Product help sends only the versioned public manual and a question through a bounded HTTPS client. Its key is external and readable only by Go.

`go test ./...` requires a dedicated PostgreSQL17 test namespace. Set P01_TEST_DATABASE_URL to the external isolated p01_test_go target in process memory; never use the application database. FFmpeg/ffprobe must be available for actual media checks. Record real execution and skips separately. The canonical checks are scripts/verify.sh and tests/README.md.

Backup PostgreSQL and its matching media manifest as a consistent set, then restore into an independent target. Do not copy app.sqlite as a current backup or fall back to SQLite after PostgreSQL writes. Formal production migration is outside the current P2 authorization.
