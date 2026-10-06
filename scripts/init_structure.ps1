<#
.SYNOPSIS
    Membuat struktur awal CortexOS di dalam root project Big.

.DESCRIPTION
    Script ini aman dijalankan berulang kali. Script selalu memaksa root
    berada dua level di atas folder scripts dan menolak target di luar root.
#>

[CmdletBinding()]
param(
    [string]$RootPath = (Resolve-Path (Join-Path $PSScriptRoot ".."))
)

$ErrorActionPreference = "Stop"
$root = [IO.Path]::GetFullPath($RootPath)
$expectedRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))

if ($root -ne $expectedRoot) {
    throw "RootPath harus menunjuk ke root project Big: $expectedRoot"
}

Write-Host "CortexOS - initialize project structure" -ForegroundColor Cyan
Write-Host "Root: $root" -ForegroundColor DarkGray

$folders = @(
    "docs/architecture",
    "docs/adr",
    "docs/specs",
    "scripts",
    "_harvest/raw",
    "_harvest/extracted/go-packages",
    "_harvest/extracted/ts-packages",
    "_harvest/extracted/prompts",
    "_harvest/extracted/skills/registry",
    "_harvest/extracted/assets/pixel-office",
    "_harvest/extracted/assets/audio",
    "internal/cortex/workspace",
    "internal/cortex/orchestra",
    "internal/cortex/harness",
    "internal/cortex/staff",
    "internal/engine/native",
    "internal/engine/pi",
    "internal/engine/omp",
    "internal/platform",
    "internal/shared",
    "cmd/cortexos",
    "web/src/components/cockpit",
    "web/src/components/office",
    "web/src/components/ui",
    "web/src/features/kanban",
    "web/src/features/code-review",
    "web/src/features/terminal",
    "web/src/engine/scenes",
    "web/src/engine/systems",
    "web/src/engine/entities",
    "web/src/hooks",
    "web/src/stores",
    "web/src/types",
    "configs",
    "build/bin",
    "build/installer",
    "build/assets",
    "test/integration",
    "test/e2e",
    "test/fixtures"
)

foreach ($relative in $folders) {
    $path = Join-Path $root $relative
    if (-not (Test-Path -LiteralPath $path)) {
        New-Item -ItemType Directory -Path $path -Force | Out-Null
        Write-Host "  created  $relative" -ForegroundColor Green
    } else {
        Write-Host "  exists   $relative" -ForegroundColor DarkGray
    }
}

$placeholders = @{
    "internal/shared/.gitkeep" = ""
    "internal/platform/.gitkeep" = ""
    "configs/.gitkeep" = ""
    "build/.gitkeep" = ""
    "test/fixtures/.gitkeep" = ""
    "_harvest/.gitkeep" = ""
    "web/src/types/index.ts" = "// Shared TypeScript types will be generated or maintained here."
}

foreach ($item in $placeholders.GetEnumerator()) {
    $path = Join-Path $root $item.Key
    if (-not (Test-Path -LiteralPath $path)) {
        Set-Content -LiteralPath $path -Value $item.Value -Encoding UTF8
        Write-Host "  created  $($item.Key)" -ForegroundColor Green
    }
}

Write-Host "`nStructure ready." -ForegroundColor Cyan
