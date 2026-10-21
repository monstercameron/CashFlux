param(
    [switch]$Probe
)

# Native PowerShell 5.1 fixture for devwatch.ps1. From the repository root run:
# powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\devwatch.Tests.ps1
# Add -Probe to compare one legacy scan with one pruned scan of the live repo.

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "devwatch.ps1")

$tempRoot = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath()).TrimEnd([char[]]@([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar))
$fixtureRoot = Join-Path $tempRoot ("cashflux-devwatch-" + [guid]::NewGuid().ToString("N"))
$sourceRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$junctionPath = Join-Path (Join-Path $fixtureRoot "source") "linked"

function Set-FixtureTimestamp([string]$Path, [string]$Timestamp) {
    [System.IO.File]::SetLastWriteTimeUtc($Path, [DateTime]::Parse($Timestamp).ToUniversalTime())
}

try {
    $fixtureSource = Join-Path $fixtureRoot "source"
    $fixtureTarget = Join-Path $fixtureRoot "junction-target"
    New-Item -ItemType Directory -Path $fixtureSource, $fixtureTarget | Out-Null

    foreach ($directory in @(
        "internal", "pkg", "cmd", ".deferred", "tools", "e2e",
        ".tools\ignored", "web\ignored", "node_modules\ignored", ".git\ignored",
        "empty"
    )) {
        New-Item -ItemType Directory -Force -Path (Join-Path $fixtureSource $directory) | Out-Null
    }
    New-Item -ItemType Directory -Path (Join-Path $fixtureTarget "linked") | Out-Null

    $included = @(
        @{ Path = "root.go"; Date = "2026-01-01T00:00:00Z" },
        @{ Path = "internal\sample.go"; Date = "2026-01-02T00:00:00Z" },
        @{ Path = "pkg\sample.go"; Date = "2026-01-03T00:00:00Z" },
        @{ Path = "cmd\sample.go"; Date = "2026-01-04T00:00:00Z" },
        @{ Path = ".deferred\sample.go"; Date = "2026-01-05T00:00:00Z" },
        @{ Path = "tools\sample.go"; Date = "2026-01-06T00:00:00Z" },
        @{ Path = "e2e\sample.go"; Date = "2026-01-07T00:00:00Z" }
    )
    foreach ($item in $included) {
        $path = Join-Path $fixtureSource $item.Path
        Set-Content -LiteralPath $path -Value "package fixture"
        Set-FixtureTimestamp $path $item.Date
    }

    foreach ($relativePath in @(
        ".tools\ignored\tool.go", "web\ignored\web.go",
        "node_modules\ignored\node.go", ".git\ignored\git.go"
    )) {
        Set-Content -LiteralPath (Join-Path $fixtureSource $relativePath) -Value "package ignored"
    }

    $goMod = Join-Path $fixtureSource "go.mod"
    $goSum = Join-Path $fixtureSource "go.sum"
    Set-Content -LiteralPath $goMod -Value "module fixture"
    Set-Content -LiteralPath $goSum -Value "fixture checksum"
    Set-FixtureTimestamp $goMod "2029-01-01T00:00:00Z"
    Set-FixtureTimestamp $goSum "2030-01-01T00:00:00Z"

    $junctionTargetGo = Join-Path $fixtureTarget "linked\external.go"
    Set-Content -LiteralPath $junctionTargetGo -Value "package external"
    Set-FixtureTimestamp $junctionTargetGo "2040-01-01T00:00:00Z"
    $junctionPath = Join-Path $fixtureSource "linked"
    New-Item -ItemType Junction -Path $junctionPath -Target (Join-Path $fixtureTarget "linked") | Out-Null

    $fixture = Get-CashFluxDevSourceSnapshot -Root $fixtureSource
    if ($fixture.GoFileCount -ne 7) {
        throw "Fixture counted $($fixture.GoFileCount) Go files; expected 7."
    }
    if ($fixture.LastWriteTimeUtc.Year -ne 2030) {
        throw "Fixture newest timestamp was $($fixture.LastWriteTimeUtc.ToString('o')); expected go.sum in 2030, excluding junction target in 2040."
    }
    $empty = Get-CashFluxDevSourceSnapshot -Root (Join-Path $fixtureSource "empty")
    if ($empty.GoFileCount -ne 0 -or $empty.LastWriteTimeUtc -ne [DateTime]::MinValue) {
        throw "Empty-root contract failed: count=$($empty.GoFileCount), stamp=$($empty.LastWriteTimeUtc)."
    }
    Write-Output "Fixture PASS: included source roots, pruned generated/tool trees, skipped junction, go.mod/go.sum stamps and empty root."

    if ($Probe) {
        $watch = [Diagnostics.Stopwatch]::StartNew()
        $legacyFiles = @(Get-ChildItem -Path $sourceRoot -Recurse -Filter "*.go" |
            Where-Object { $_.FullName -notmatch "[\\/](web|node_modules|\.git)[\\/]" })
        $legacyStamp = $legacyFiles |
            Sort-Object LastWriteTimeUtc -Descending |
            Select-Object -First 1 -ExpandProperty LastWriteTimeUtc
        $watch.Stop()
        $legacyMilliseconds = [Math]::Round($watch.Elapsed.TotalMilliseconds, 1)

        $watch.Restart()
        $pruned = Get-CashFluxDevSourceSnapshot -Root $sourceRoot
        $watch.Stop()
        $prunedMilliseconds = [Math]::Round($watch.Elapsed.TotalMilliseconds, 1)

        $legacyExcluded = @($legacyFiles | Where-Object {
            $_.FullName -match "[\\/](\.tools|web|node_modules|\.git)[\\/]"
        })
        [pscustomobject]@{
            PowerShell          = $PSVersionTable.PSVersion.ToString()
            LegacyFiles         = $legacyFiles.Count
            PrunedFiles         = $pruned.GoFileCount
            ExcludedFiles       = $legacyExcluded.Count
            LegacyMilliseconds  = $legacyMilliseconds
            PrunedMilliseconds  = $prunedMilliseconds
            StampsMatch         = ($legacyStamp.Ticks -eq $pruned.LastWriteTimeUtc.Ticks)
            LegacyStamp         = $legacyStamp.ToString("o")
            PrunedStamp         = $pruned.LastWriteTimeUtc.ToString("o")
        } | Format-List
        Write-Output "Probe is one paired wall-time sample, not a CPU measurement or sustained benchmark. File counts and stamp comparison are observations; legacy scan does not include module-file timestamps."
    }
}
finally {
    $junctionRemoved = $true
    if ([System.IO.Directory]::Exists($junctionPath)) {
        try {
            [System.IO.Directory]::Delete($junctionPath, $false)
        }
        catch {
            # The fixture root is still guarded below; a failed link removal must
            # never make cleanup follow a junction outside the unique temp tree.
            $junctionRemoved = $false
        }
    }
    if ([System.IO.Directory]::Exists($junctionPath)) {
        $junctionRemoved = $false
    }

    $fixtureFullPath = [System.IO.Path]::GetFullPath($fixtureRoot)
    $requiredPrefix = $tempRoot + [System.IO.Path]::DirectorySeparatorChar
    if ($junctionRemoved -and
        $fixtureFullPath.StartsWith($requiredPrefix, [StringComparison]::OrdinalIgnoreCase) -and
        [System.IO.Path]::GetFileName($fixtureFullPath).StartsWith("cashflux-devwatch-", [StringComparison]::Ordinal)) {
        Remove-Item -LiteralPath $fixtureFullPath -Recurse -Force -ErrorAction SilentlyContinue
    }
    elseif (-not $junctionRemoved) {
        Write-Warning "Fixture left in place because junction removal could not be verified: $fixtureFullPath"
    }
}
