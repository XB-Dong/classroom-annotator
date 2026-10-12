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
export P0_BACKEND_BINARY="$test_runtime/backend-server"
export P0_RESTART_DATA_DIR="$test_runtime/restart-data"
export P0_DATASET_DATA_DIR="$test_runtime/dataset-data"
export P0_API_DATA_DIR="$test_runtime/api-data"
export P0_FIXTURE_DIR="${P0_FIXTURE_DIR:-$test_runtime/fixtures}"
test -f frontend/package-lock.json
test -f backend/go.mod
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
  if [[ "${P0_SKIP_RACE:-0}" == 1 ]]; then
    echo 'RACE NOT RUN: explicit P0_SKIP_RACE=1; delivery needs separate race evidence.'
  else
    CGO_ENABLED=1 go test -race ./internal/...
  fi
)
(
  cd tests
  npm ci
  npm run check
  npm test
  npm run list
  npm run fixtures
  npm run dataset
  npm run restart
  npm run local-api
  if [[ "$integration" == 1 ]]; then
    : "${P0_FIXTURE_DIR:?external fixture directory required}"
    npm run contract
  fi
  if [[ "$e2e" == 1 ]]; then npm run e2e; fi
)
