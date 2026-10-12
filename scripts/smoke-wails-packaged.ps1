# BAN-132 packaged Wails smoke (P7-07).
# Builds the production Wails binary for this host OS, starts it with a
# bounded lifetime, and requires a clean exit (code 0) within a timeout.
# The app exits itself via CORTEXOS_SMOKE_QUIT_MS, which is unset in normal runs.

param(
  [int]$QuitAfterMs = 2000,
  [int]$TimeoutSeconds = 120,
  [switch]$SkipBuild,
  [string]$Output = 'build\bin\cortexos-smoke.exe'
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
  if (-not $SkipBuild) {
    & wails build -clean -nopackage -o cortexos-smoke.exe
    if ($LASTEXITCODE -ne 0) { throw "wails build failed with exit code $LASTEXITCODE" }
  }
  if (-not (Test-Path $Output)) { throw "packaged binary not found: $Output" }

  $env:CORTEXOS_SMOKE_QUIT_MS = [string]$QuitAfterMs
  try {
    $process = Start-Process -FilePath $Output -PassThru
    if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
      $process.Kill()
      throw "packaged smoke timed out after $TimeoutSeconds seconds; process killed (PID $($process.Id))"
    }
    if ($process.ExitCode -ne 0) {
      throw "packaged smoke exited with code $($process.ExitCode)"
    }
  } finally {
    Remove-Item Env:CORTEXOS_SMOKE_QUIT_MS -ErrorAction SilentlyContinue
  }
  Write-Host "BAN-132 packaged Wails smoke passed on $([System.Runtime.InteropServices.RuntimeInformation]::OSDescription)"
} finally {
  Pop-Location
}
