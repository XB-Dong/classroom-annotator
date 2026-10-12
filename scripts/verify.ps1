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
$env:P0_BACKEND_BINARY = Join-Path $testRuntime 'backend-server.exe'
$env:P0_RESTART_DATA_DIR = Join-Path $testRuntime 'restart-data'
$env:P0_DATASET_DATA_DIR = Join-Path $testRuntime 'dataset-data'
$env:P0_API_DATA_DIR = Join-Path $testRuntime 'api-data'
if (!$env:P0_FIXTURE_DIR) { $env:P0_FIXTURE_DIR = Join-Path $testRuntime 'fixtures' }
if (!(Test-Path frontend/package-lock.json) -or !(Test-Path backend/go.mod)) { throw 'Frontend/backend product workspace is missing.' }
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
  Invoke-Checked npm @('ci')
  Invoke-Checked npm @('run', 'check')
  Invoke-Checked npm @('test')
  Invoke-Checked npm @('run', 'list')
  Invoke-Checked npm @('run', 'fixtures')
  Invoke-Checked npm @('run', 'dataset')
  Invoke-Checked npm @('run', 'restart')
  Invoke-Checked npm @('run', 'local-api')
  if ($Integration -or $E2E) {
    if (!$env:P0_FIXTURE_DIR) { throw 'P0_FIXTURE_DIR external fixture directory required.' }
    Invoke-Checked npm @('run', 'contract')
  }
  if ($E2E) { Invoke-Checked npm @('run', 'e2e') }
} finally { Pop-Location }
