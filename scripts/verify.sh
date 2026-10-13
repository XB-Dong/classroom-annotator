#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
integration=0
e2e=0
for option in "$@"; do
  case "$option" in
    --integration) integration=1 ;;
    --e2e) integration=1; e2e=1 ;;
    *) echo "Unknown verify option: $option" >&2; exit 2 ;;
  esac
done
command -v node >/dev/null
command -v npm >/dev/null
command -v go >/dev/null
command -v "${FFMPEG_PATH:-ffmpeg}" >/dev/null
command -v "${FFPROBE_PATH:-ffprobe}" >/dev/null
if [[ -n "${FFMPEG_PATH:-}" ]]; then export PATH="$(dirname "$FFMPEG_PATH"):$PATH"; fi
if [[ -n "${FFPROBE_PATH:-}" ]]; then export PATH="$(dirname "$FFPROBE_PATH"):$PATH"; fi
test_runtime="$(mktemp -d "${TMPDIR:-/tmp}/classroom-acceptance.XXXXXXXX")"
for variable in P01_GO_DATABASE_URL_FILE P01_DATABASE_URL_FILE P01_ACCEPTANCE_DATABASE_URL_FILE P01_IMPORT_DATABASE_URL_FILE P01_ROLLBACK_DATABASE_URL_FILE; do
  if [[ -z "${!variable:-}" || ! -f "${!variable}" ]]; then echo "Required isolated PostgreSQL DSN file missing: $variable" >&2; exit 1; fi
done
export P01_TEST_DATABASE_URL="$(< "$P01_GO_DATABASE_URL_FILE")"
export GOTOOLCHAIN=local GOMAXPROCS=2 GOFLAGS=-p=1
export P0_EXPECTED_SHA="$(git rev-parse HEAD)"
export P01_EXPECTED_AGENT_SHA="$P0_EXPECTED_SHA"
export P01_MIGRATE_BINARY="$test_runtime/migrate"
export P01_IMPORT_BINARY="$test_runtime/import-sqlite"
export P01_AGENT_BINARY="$test_runtime/media-agent"
export P01_SEED_BINARY="$test_runtime/seed-samples"
export P01_API_OUTPUT="$test_runtime/postgres-data"
export P01_MAC_OUTPUT="$test_runtime/agent-check"
export P01_JOB_OUTPUT="$test_runtime/job-race-check"
export P01_MIGRATION_OUTPUT="$test_runtime/migration-check"
export P0_TEST_OUTPUT="$test_runtime/playwright-results"
export P0_REPORT_OUTPUT="$test_runtime/playwright-report"
export P0_RESTORE_BASELINE="$P01_API_OUTPUT/restore-baseline.json"
export P0_BACKEND_BINARY="$test_runtime/backend-server"
export P0_RESTART_DATA_DIR="$test_runtime/restart-data"
export P0_DATASET_DATA_DIR="$test_runtime/dataset-data"
export P0_API_DATA_DIR="$test_runtime/api-data"
export P0_FIXTURE_DIR="${P0_FIXTURE_DIR:-$test_runtime/fixtures}"
test -f frontend/package-lock.json
test -f backend/go.mod
(cd tests; npm ci; node scripts/verify-databases.mjs)
(
  cd frontend
  npm ci
  npm run typecheck
  npm run lint
  npm test -- --run
  npm run build
)
(
  cd backend
  unformatted="$(gofmt -l .)"
  if [[ -n "$unformatted" ]]; then printf '%s\n' "$unformatted"; exit 1; fi
  go vet ./...
  go test ./...
  go build ./...
  CGO_ENABLED=0 go build -o "$P0_BACKEND_BINARY" ./cmd/server
  CGO_ENABLED=0 go build -o "$P01_MIGRATE_BINARY" ./cmd/migrate
  CGO_ENABLED=0 go build -o "$P01_IMPORT_BINARY" ./cmd/import-sqlite
  CGO_ENABLED=0 go build -o "$P01_AGENT_BINARY" ./cmd/media-agent
  CGO_ENABLED=0 go build -o "$P01_SEED_BINARY" ./cmd/seed-samples
  if [[ "${P0_SKIP_RACE:-0}" == 1 ]]; then
    echo 'RACE NOT RUN: explicit P0_SKIP_RACE=1; delivery needs separate race evidence.'
  else
    CGO_ENABLED=1 go test -race ./internal/...
  fi
)
(
  cd tests
  npm run check
  npm test
  npm run list
  npm run fixtures
  npm run migration
  npm run postgres
  npm run restart
  npm run job-race
  npm run mac-media
  if [[ "$e2e" == 1 ]]; then node scripts/run-api.mjs --e2e; else npm run local-api; fi
)
echo "Verification evidence: $test_runtime"
