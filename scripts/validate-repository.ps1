<#
.SYNOPSIS
    Runs the reproducible repository, Go, frontend, and dependency gates.
#>

[CmdletBinding()]
param(
    [switch]$SkipHarvest
)

$ErrorActionPreference = "Stop"
if ($null -ne (Get-Variable PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue)) {
    $PSNativeCommandUseErrorActionPreference = $false
}
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
Set-Location $root

function Invoke-Checked([string]$Command, [scriptblock]$Action) {
    Write-Host $Command -ForegroundColor DarkGray
    & $Action
    if ($LASTEXITCODE -ne 0) { throw "Command failed: $Command" }
}

function Invoke-NpmChecked([string]$ScriptName) {
    Write-Host "npm run $ScriptName" -ForegroundColor DarkGray
    npm.cmd run $ScriptName
    if ($LASTEXITCODE -ne 0) { throw "Command failed: npm run $ScriptName" }
}

Write-Host "[1/8] Git diff check" -ForegroundColor Cyan
git -c core.safecrlf=false diff --check
if ($LASTEXITCODE -ne 0) { throw "git diff --check failed" }

Write-Host "[2/8] Ignore policy check" -ForegroundColor Cyan
$ignored = @(
    ".mcp.json",
    ".env",
    "RULES.md",
    "opencode.json",
    "_harvest/raw",
    "build/bin",
    "web/node_modules/placeholder",
    "web/dist/placeholder",
    "frontend/dist/placeholder"
)
foreach ($path in $ignored) {
    git check-ignore -q -- $path
    if ($LASTEXITCODE -ne 0) { throw "Expected ignored path is not ignored: $path" }
}

Write-Host "[3/8] Harvest manifest refresh" -ForegroundColor Cyan
if (-not $SkipHarvest) {
    & (Join-Path $PSScriptRoot "harvest.ps1") -SkipClone
    if ($LASTEXITCODE -ne 0) { throw "Harvest provenance refresh failed" }
}
if (-not (Test-Path -LiteralPath (Join-Path $root "_harvest/harvest-manifest.json"))) {
    throw "Harvest manifest was not created"
}

Write-Host "[4/8] Secret-pattern review" -ForegroundColor Cyan
$tracked = @(git ls-files)
$scanFiles = @($tracked | Where-Object { $_ -notmatch '(^|/)(\.git|_harvest/raw|node_modules|build/bin|web/wailsjs)(/|$)' })
$patterns = @('-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----','sk-[A-Za-z0-9]{16,}','xox[baprs]-[A-Za-z0-9-]+','ghp_[A-Za-z0-9]{20,}')
foreach ($pattern in $patterns) {
    $matches = Select-String -Path $scanFiles -Pattern $pattern -ErrorAction SilentlyContinue
    if ($matches) { throw "Possible secret pattern found: $pattern" }
}

Write-Host "[5/8] Go gates" -ForegroundColor Cyan
Invoke-Checked "go mod tidy" { go mod tidy }
Invoke-Checked "go test ./..." { go test ./... }
$formatted = @(gofmt -l .)
if ($formatted.Count -gt 0) { $formatted | Write-Error; throw "Go formatting check failed" }
Invoke-Checked "go vet ./..." { go vet ./... }

Write-Host "[6/8] Frontend gates" -ForegroundColor Cyan
Push-Location (Join-Path $root "web")
try {
    npm.cmd ci --ignore-scripts
    $npmExit = $LASTEXITCODE
    if ($npmExit -ne 0) { throw "Command failed: npm ci ($npmExit)" }
    Invoke-NpmChecked "typecheck"
    Invoke-NpmChecked "lint"
    Invoke-NpmChecked "test"
    Invoke-NpmChecked "build"
    npm.cmd audit
    $auditExit = $LASTEXITCODE
    if ($auditExit -ne 0) { throw "Command failed: npm audit ($auditExit)" }
} finally {
    Pop-Location
}

Write-Host "[7/8] Wails shell build" -ForegroundColor Cyan
Invoke-Checked "wails build -nopackage" { wails build -nopackage }
if (Test-Path -LiteralPath (Join-Path $root "web/package.json.md5")) {
    Remove-Item -LiteralPath (Join-Path $root "web/package.json.md5") -Force
}
if (Test-Path -LiteralPath (Join-Path $root "cortexos.exe")) {
    Remove-Item -LiteralPath (Join-Path $root "cortexos.exe") -Force
}

Write-Host "[8/8] Tracked artifact review" -ForegroundColor Cyan
$forbidden = @(git ls-files | Where-Object { $_ -match '(^|/)(node_modules|dist|build/bin|_harvest/raw|wailsjs)(/|$)|(^|/)(\.env|.*\.key|.*\.pem)$' })
if ($forbidden.Count -gt 0) {
    $forbidden | Write-Error
    throw "Forbidden generated, raw, or secret path is tracked"
}

Write-Host "Repository validation passed." -ForegroundColor Green
