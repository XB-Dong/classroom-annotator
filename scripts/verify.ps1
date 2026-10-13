param([switch]$Integration, [switch]$E2E)
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
function Invoke-Checked {
  param([string]$Command, [string[]]$Arguments)
  & $Command @Arguments
  if ($LASTEXITCODE -ne 0) { throw "$Command failed with exit code $LASTEXITCODE" }
}
foreach ($command in @('node', 'npm', 'go')) { Get-Command $command -ErrorAction Stop | Out-Null }
$ffmpegCommand = if ($env:FFMPEG_PATH) { $env:FFMPEG_PATH } else { 'ffmpeg' }
$ffprobeCommand = if ($env:FFPROBE_PATH) { $env:FFPROBE_PATH } else { 'ffprobe' }
Get-Command $ffmpegCommand -ErrorAction Stop | Out-Null
Get-Command $ffprobeCommand -ErrorAction Stop | Out-Null
if ($env:FFMPEG_PATH) { $env:PATH = (Split-Path $env:FFMPEG_PATH -Parent) + [IO.Path]::PathSeparator + $env:PATH }
if ($env:FFPROBE_PATH) { $env:PATH = (Split-Path $env:FFPROBE_PATH -Parent) + [IO.Path]::PathSeparator + $env:PATH }
$testRuntime = Join-Path ([System.IO.Path]::GetTempPath()) ('classroom-acceptance-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testRuntime | Out-Null
foreach ($variable in @('P01_GO_DATABASE_URL_FILE', 'P01_DATABASE_URL_FILE', 'P01_ACCEPTANCE_DATABASE_URL_FILE', 'P01_IMPORT_DATABASE_URL_FILE', 'P01_ROLLBACK_DATABASE_URL_FILE')) {
  $file = [Environment]::GetEnvironmentVariable($variable)
  if (!$file -or !(Test-Path -LiteralPath $file -PathType Leaf)) { throw "Required isolated PostgreSQL DSN file missing: $variable" }
}
$env:P01_TEST_DATABASE_URL = (Get-Content -LiteralPath $env:P01_GO_DATABASE_URL_FILE -Raw).Trim()
$env:GOTOOLCHAIN = 'local'
$env:GOMAXPROCS = '2'
$env:GOFLAGS = '-p=1'
$env:P0_EXPECTED_SHA = (& git rev-parse HEAD).Trim()
$env:P01_EXPECTED_AGENT_SHA = $env:P0_EXPECTED_SHA
$env:P01_MIGRATE_BINARY = Join-Path $testRuntime 'migrate.exe'
$env:P01_IMPORT_BINARY = Join-Path $testRuntime 'import-sqlite.exe'
$env:P01_AGENT_BINARY = Join-Path $testRuntime 'media-agent.exe'
$env:P01_SEED_BINARY = Join-Path $testRuntime 'seed-samples.exe'
$env:P01_API_OUTPUT = Join-Path $testRuntime 'postgres-data'
$env:P01_MAC_OUTPUT = Join-Path $testRuntime 'agent-check'
$env:P01_JOB_OUTPUT = Join-Path $testRuntime 'job-race-check'
$env:P01_MIGRATION_OUTPUT = Join-Path $testRuntime 'migration-check'
$env:P0_TEST_OUTPUT = Join-Path $testRuntime 'playwright-results'
$env:P0_REPORT_OUTPUT = Join-Path $testRuntime 'playwright-report'
$env:P0_RESTORE_BASELINE = Join-Path $env:P01_API_OUTPUT 'restore-baseline.json'
$env:P0_BACKEND_BINARY = Join-Path $testRuntime 'backend-server.exe'
$env:P0_RESTART_DATA_DIR = Join-Path $testRuntime 'restart-data'
$env:P0_DATASET_DATA_DIR = Join-Path $testRuntime 'dataset-data'
$env:P0_API_DATA_DIR = Join-Path $testRuntime 'api-data'
if (!$env:P0_FIXTURE_DIR) { $env:P0_FIXTURE_DIR = Join-Path $testRuntime 'fixtures' }
if (!(Test-Path frontend/package-lock.json) -or !(Test-Path backend/go.mod)) { throw 'Frontend/backend product workspace is missing.' }
Push-Location tests
try {
  Invoke-Checked npm @('ci')
  Invoke-Checked node @('scripts/verify-databases.mjs')
} finally { Pop-Location }
Push-Location frontend
try {
  Invoke-Checked npm @('ci')
  Invoke-Checked npm @('run', 'typecheck')
  Invoke-Checked npm @('run', 'lint')
  Invoke-Checked npm @('test', '--', '--run')
  Invoke-Checked npm @('run', 'build')
} finally { Pop-Location }
Push-Location backend
try {
  $unformatted = & gofmt -l .
  if ($LASTEXITCODE -ne 0 -or $unformatted) { throw "Go files need formatting: $unformatted" }
  Invoke-Checked go @('vet', './...')
  Invoke-Checked go @('test', './...')
  Invoke-Checked go @('build', './...')
  Invoke-Checked go @('build', '-o', $env:P0_BACKEND_BINARY, './cmd/server')
  Invoke-Checked go @('build', '-o', $env:P01_MIGRATE_BINARY, './cmd/migrate')
  Invoke-Checked go @('build', '-o', $env:P01_IMPORT_BINARY, './cmd/import-sqlite')
  Invoke-Checked go @('build', '-o', $env:P01_AGENT_BINARY, './cmd/media-agent')
  Invoke-Checked go @('build', '-o', $env:P01_SEED_BINARY, './cmd/seed-samples')
  if ($env:P0_SKIP_RACE -eq '1') {
    Write-Output 'RACE NOT RUN: explicit P0_SKIP_RACE=1; delivery needs separate race evidence.'
  } else {
    $priorCgo = $env:CGO_ENABLED
    try { $env:CGO_ENABLED = '1'; Invoke-Checked go @('test', '-race', './internal/...') }
    finally { $env:CGO_ENABLED = $priorCgo }
  }
} finally { Pop-Location }
Push-Location tests
try {
  Invoke-Checked npm @('run', 'check')
  Invoke-Checked npm @('test')
  Invoke-Checked npm @('run', 'list')
  Invoke-Checked npm @('run', 'fixtures')
  Invoke-Checked npm @('run', 'migration')
  Invoke-Checked npm @('run', 'postgres')
  Invoke-Checked npm @('run', 'restart')
  Invoke-Checked npm @('run', 'job-race')
  Invoke-Checked npm @('run', 'mac-media')
  if ($E2E) { Invoke-Checked node @('scripts/run-api.mjs', '--e2e') }
  else { Invoke-Checked npm @('run', 'local-api') }
} finally { Pop-Location }
Write-Output "Verification evidence: $testRuntime"
