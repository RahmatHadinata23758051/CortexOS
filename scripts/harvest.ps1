<#
.SYNOPSIS
    Phase 0: clone dan audit repository referensi CortexOS.

.DESCRIPTION
    Semua clone dan hasil ekstraksi berada di dalam Big/_harvest.
    Script tidak menghapus file, tidak mengubah repository upstream, dan
    mencatat lisensi serta commit yang dipakai ke harvest-manifest.json.

    Catatan penting:
    - Kode/aset pihak ketiga tidak otomatis dianggap boleh dipakai.
    - Review LICENSE, NOTICE, attribution, dependency, dan hak asset sebelum porting.
    - Folder _harvest adalah gudang referensi, bukan source of truth produk.
#>

[CmdletBinding()]
param(
    [switch]$SkipClone,
    [switch]$Refresh,
    [switch]$Extract
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$harvestRoot = Join-Path $root "_harvest"
$rawRoot = Join-Path $harvestRoot "raw"
$extractedRoot = Join-Path $harvestRoot "extracted"
$manifestPath = Join-Path $harvestRoot "harvest-manifest.json"

function Assert-InRoot([string]$Path) {
    $full = [IO.Path]::GetFullPath($Path)
    if (-not ($full.StartsWith($root, [StringComparison]::OrdinalIgnoreCase))) {
        throw "Path berada di luar root project Big: $full"
    }
}

function Ensure-Directory([string]$Path) {
    Assert-InRoot $Path
    if (-not (Test-Path -LiteralPath $Path)) {
        New-Item -ItemType Directory -Path $Path -Force | Out-Null
    }
}

function Copy-IfExists([string]$Source, [string]$Destination) {
    if (-not (Test-Path -LiteralPath $Source)) {
        Write-Host "  warning missing: $Source" -ForegroundColor Yellow
        return $false
    }
    Ensure-Directory (Split-Path -Parent $Destination)
    Copy-Item -LiteralPath $Source -Destination $Destination -Recurse -Force
    return $true
}

function Test-GitRepository([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return $false }
    $result = ((git -C $Path rev-parse --is-inside-work-tree 2>$null) -join "").Trim()
    return $result -eq "true"
}

function Clone-Repository($Repo) {
    $destination = Join-Path $rawRoot $Repo.Name
    Assert-InRoot $destination

    if (Test-Path -LiteralPath $destination) {
        if (-not (Test-GitRepository $destination)) {
            throw "Harvest destination exists but is not a valid Git repository: $destination. Review it manually before retrying."
        }
        if ($Refresh) {
            Write-Host "  updating $($Repo.Name)" -ForegroundColor Cyan
            git -C $destination fetch --depth 1 origin $Repo.Ref
            git -C $destination reset --hard "origin/$($Repo.Ref)"
            git -C $destination clean -fdx
        } else {
            Write-Host "  exists  $($Repo.Name)" -ForegroundColor DarkGray
        }
        return
    }

    Write-Host "  cloning $($Repo.Name)" -ForegroundColor Green
    git clone --depth 1 --branch $Repo.Ref $Repo.Url $destination
}

Ensure-Directory $rawRoot
Ensure-Directory $extractedRoot

# Referensi disimpan sebagai source material. Engine utama tetap memakai adapter CortexOS.
# Pi resmi berada di badlogic/pi-mono; package coding-agent bukan repository GitHub terpisah.
$repositories = @(
    @{ Name = "pi"; Url = "https://github.com/badlogic/pi-mono.git"; Ref = "main"; AdoptionStatus = "reference-only" },
    @{ Name = "oh-my-pi"; Url = "https://github.com/can1357/oh-my-pi.git"; Ref = "main"; AdoptionStatus = "reference-only-nested-license-review" },
    @{ Name = "agency-agents"; Url = "https://github.com/msitarzewski/agency-agents.git"; Ref = "main"; AdoptionStatus = "reference-only" },
    @{ Name = "agent-teams-ai"; Url = "https://github.com/777genius/agent-teams-ai.git"; Ref = "main"; AdoptionStatus = "blocked-pending-legal-review" },
    @{ Name = "ai-town"; Url = "https://github.com/a16z-infra/ai-town.git"; Ref = "main"; AdoptionStatus = "reference-only" },
    @{ Name = "ai-office"; Url = "https://github.com/ChristianFJung/AIOffice.git"; Ref = "main"; AdoptionStatus = "reference-only-asset-review" },
    @{ Name = "pixel-agents"; Url = "https://github.com/pixel-agents-hq/pixel-agents.git"; Ref = "main"; AdoptionStatus = "reference-only-asset-review" },
    @{ Name = "chatdev"; Url = "https://github.com/OpenBMB/ChatDev.git"; Ref = "main"; AdoptionStatus = "reference-only" },
    @{ Name = "opencode-harness"; Url = "https://github.com/Awaiswilll/opencode-harness.git"; Ref = "main"; AdoptionStatus = "blocked-ambiguous-license" }
)

if (-not $SkipClone) {
    Write-Host "`n[1/4] Cloning reference repositories" -ForegroundColor Cyan
    foreach ($repo in $repositories) { Clone-Repository $repo }
} else {
    Write-Host "`n[1/4] Clone skipped" -ForegroundColor Yellow
}

Write-Host "`n[2/4] Recording provenance and licenses" -ForegroundColor Cyan
$existingManifest = @{}
if (Test-Path -LiteralPath $manifestPath) {
    try {
        foreach ($record in @(Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json)) {
            $existingManifest[$record.name] = $record
        }
    } catch {
        Write-Warning "Existing harvest manifest could not be parsed; review status will start fresh."
    }
}

$manifest = foreach ($repo in $repositories) {
    $path = Join-Path $rawRoot $repo.Name
    if (-not (Test-Path -LiteralPath $path)) {
        [PSCustomObject]@{
            name = $repo.Name
            url = $repo.Url
            ref = $repo.Ref
            commit = $null
            licenseFiles = @()
            repositoryStatus = "missing"
            licenseStatus = "missing-review-blocked"
            adoptionStatus = $repo.AdoptionStatus
            reviewed = $false
        }
        continue
    }

    if (-not (Test-GitRepository $path)) {
        [PSCustomObject]@{
            name = $repo.Name
            url = $repo.Url
            ref = $repo.Ref
            commit = $null
            licenseFiles = @()
            repositoryStatus = "invalid"
            licenseStatus = "missing-review-blocked"
            adoptionStatus = $repo.AdoptionStatus
            reviewed = $false
        }
        continue
    }

    $sha = (git -C $path rev-parse HEAD).Trim()
    $licenseFiles = @(Get-ChildItem -LiteralPath $path -File -Recurse -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match '^(LICENSE|LICENCE|NOTICE|COPYING)(\..*)?$' } |
        ForEach-Object { $_.FullName.Substring($path.Length + 1) })
    $licenseStatus = if ($licenseFiles.Count -gt 0) { "found-review-required" } else { "missing-review-blocked" }

    $previous = $existingManifest[$repo.Name]
    [PSCustomObject]@{
        name = $repo.Name
        url = $repo.Url
        ref = $repo.Ref
        commit = $sha
        licenseFiles = $licenseFiles
        repositoryStatus = "cloned"
        licenseStatus = $licenseStatus
        adoptionStatus = $repo.AdoptionStatus
        reviewed = if ($null -ne $previous) { [bool]$previous.reviewed } else { $false }
        reviewNotes = if ($null -ne $previous) { $previous.reviewNotes } else { $null }
    }
}
$manifest | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $manifestPath -Encoding UTF8
Write-Host "  manifest: _harvest/harvest-manifest.json" -ForegroundColor Green

$missingRepositories = @($manifest | Where-Object { $_.repositoryStatus -ne "cloned" })
$blockedLicenses = @($manifest | Where-Object { $_.licenseStatus -eq "missing-review-blocked" })
if ($missingRepositories.Count -gt 0) {
    Write-Warning ("Missing repositories: " + (($missingRepositories | ForEach-Object { $_.name }) -join ", "))
}
if ($blockedLicenses.Count -gt 0) {
    Write-Warning ("Repositories without a detected license file are blocked from adoption: " + (($blockedLicenses | ForEach-Object { $_.name }) -join ", "))
}

if ($Extract -or -not $SkipClone) {
    Write-Host "`n[3/4] Extracting reference material" -ForegroundColor Cyan

    $copyRules = @(
        @{ Repo = "agency-agents"; Source = "engineering"; Destination = "prompts/agency-agents/engineering" },
        @{ Repo = "agency-agents"; Source = "design"; Destination = "prompts/agency-agents/design" },
        @{ Repo = "agency-agents"; Source = "marketing"; Destination = "prompts/agency-agents/marketing" },
        @{ Repo = "agency-agents"; Source = "product"; Destination = "prompts/agency-agents/product" },
        @{ Repo = "agency-agents"; Source = "security"; Destination = "prompts/agency-agents/security" },
        @{ Repo = "agency-agents"; Source = "testing"; Destination = "prompts/agency-agents/testing" },
        @{ Repo = "agency-agents"; Source = "divisions.json"; Destination = "skills/registry/agency-divisions.json" },
        @{ Repo = "agency-agents"; Source = "tools.json"; Destination = "skills/registry/agency-tools.json" },
        @{ Repo = "opencode-harness"; Source = "agents"; Destination = "skills/opencode-harness/agents" },
        @{ Repo = "opencode-harness"; Source = "skills"; Destination = "skills/opencode-harness/skills" },
        @{ Repo = "opencode-harness"; Source = "commands"; Destination = "skills/opencode-harness/commands" },
        @{ Repo = "agent-teams-ai"; Source = "docs/screenshots"; Destination = "references/agent-teams-ai/screenshots" },
        @{ Repo = "ai-office"; Source = "audio"; Destination = "assets/audio" },
        @{ Repo = "pixel-agents"; Source = "assets"; Destination = "assets/pixel-agents" },
        @{ Repo = "chatdev"; Source = "ChatDev/prompts"; Destination = "prompts/chatdev" }
    )

    $blockedRepositories = @($manifest | Where-Object { $_.licenseStatus -eq "missing-review-blocked" } | ForEach-Object { $_.name })
    if ($blockedRepositories.Count -gt 0) {
        Write-Warning ("Extraction from repositories without detected license files is blocked: " + ($blockedRepositories -join ", "))
    }

    foreach ($rule in $copyRules) {
        $repoRecord = @($manifest | Where-Object { $_.name -eq $rule.Repo }) | Select-Object -First 1
        if ($null -eq $repoRecord -or $repoRecord.repositoryStatus -ne "cloned") {
            Write-Warning "Skipping extraction from unavailable repository: $($rule.Repo)"
            continue
        }
        $source = Join-Path (Join-Path $rawRoot $rule.Repo) $rule.Source
        $destination = Join-Path (Join-Path $extractedRoot $rule.Destination) (Split-Path $rule.Source -Leaf)
        [void](Copy-IfExists $source $destination)
    }

    # Simpan attribution ringkas agar material harvested selalu terlacak.
    $attribution = @"
# Harvest Attribution

Material di folder ini berasal dari repository pihak ketiga yang tercatat di
`../harvest-manifest.json`. Jangan port atau mendistribusikan material sebelum
LICENSE/NOTICE, attribution, dependency, dan hak asset diverifikasi.

Adoption status: reference-only until explicit review.
Harvest date: $(Get-Date -Format "yyyy-MM-ddTHH:mm:ssK")
"@
    Set-Content -LiteralPath (Join-Path $extractedRoot "ATTRIBUTION-REVIEW.md") -Value $attribution -Encoding UTF8
} else {
    Write-Host "`n[3/4] Extraction skipped; use -Extract after cloning" -ForegroundColor Yellow
}

Write-Host "`n[4/4] Harvest complete" -ForegroundColor Cyan
Write-Host "Raw repositories : _harvest/raw" -ForegroundColor Gray
Write-Host "Extracted review : _harvest/extracted" -ForegroundColor Gray
Write-Host "Manifest         : _harvest/harvest-manifest.json" -ForegroundColor Gray
Write-Host "`nReview licenses before porting any code or asset." -ForegroundColor Yellow
