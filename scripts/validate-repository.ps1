<#[
.SYNOPSIS
    Runs repository-safe validation checks that are available before Phase 1 manifests exist.
#>

[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
Set-Location $root

Write-Host "[1/5] Git diff check" -ForegroundColor Cyan
git diff --check
if ($LASTEXITCODE -ne 0) { throw "git diff --check failed" }

Write-Host "[2/5] Ignore policy check" -ForegroundColor Cyan
$ignored = @(
    ".mcp.json",
    ".env",
    "_harvest/raw",
    "build/bin"
)
foreach ($path in $ignored) {
    git check-ignore -q -- $path
    if ($LASTEXITCODE -ne 0) { throw "Expected ignored path is not ignored: $path" }
}

Write-Host "[3/5] Harvest manifest refresh" -ForegroundColor Cyan
& (Join-Path $PSScriptRoot "harvest.ps1") -SkipClone
if ($LASTEXITCODE -ne 0) { throw "Harvest provenance refresh failed" }

Write-Host "[4/5] Secret-pattern review" -ForegroundColor Cyan
$tracked = @(git ls-files)
$scanFiles = @($tracked | Where-Object { $_ -notmatch '(^|/)(\.git|_harvest/raw|node_modules|build/bin)(/|$)' })
$patterns = @('-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----','sk-[A-Za-z0-9]{16,}','xox[baprs]-[A-Za-z0-9-]+','ghp_[A-Za-z0-9]{20,}')
foreach ($pattern in $patterns) {
    $matches = Select-String -Path $scanFiles -Pattern $pattern -ErrorAction SilentlyContinue
    if ($matches) { throw "Possible secret pattern found: $pattern" }
}

Write-Host "[5/5] Phase manifest checks" -ForegroundColor Cyan
if (-not (Test-Path -LiteralPath (Join-Path $root "_harvest/harvest-manifest.json"))) {
    throw "Harvest manifest was not created"
}

Write-Host "Repository validation passed for currently available gates." -ForegroundColor Green
Write-Host "Go/frontend/build gates remain pending until Phase 1 manifests exist." -ForegroundColor Yellow
