# CashFlux two-artifact development entry point.
#
# gwc dev currently accepts one app target, so this script lets it own main.wasm
# and live reload while a small background watcher rebuilds services.wasm.
# Use full-page live reload: a hot WASM swap starts a second Go runtime without
# stopping the first, leaving both CashFlux autosave loops writing the same data.

$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $projectRoot "web\bin"
$servicesOut = Join-Path $binDir "services.wasm"
$servicesTmp = Join-Path $binDir "services.wasm.dev-tmp"
. (Join-Path $PSScriptRoot "devwatch.ps1")

New-Item -ItemType Directory -Force -Path $binDir | Out-Null

function Build-ServicesWasm {
    $previousGoos = $env:GOOS
    $previousGoarch = $env:GOARCH
    try {
        $env:GOOS = "js"
        $env:GOARCH = "wasm"
        & go build -o $servicesTmp ./cmd/cashflux-services
        if ($LASTEXITCODE -ne 0) {
            throw "services.wasm build failed"
        }
        Move-Item -Force -LiteralPath $servicesTmp -Destination $servicesOut
        $compressed = "$servicesOut.gz"
        if (Test-Path -LiteralPath $compressed) {
            Remove-Item -LiteralPath $compressed
        }
    }
    finally {
        $env:GOOS = $previousGoos
        $env:GOARCH = $previousGoarch
    }
}

Set-Location -LiteralPath $projectRoot
$initialSourceStamp = Get-CashFluxDevSourceSnapshot -Root $projectRoot
Build-ServicesWasm

$watchJob = Start-Job -ArgumentList $projectRoot, $initialSourceStamp.LastWriteTimeUtc.Ticks -ScriptBlock {
    param($root, $initialStampTicks)
    Set-Location -LiteralPath $root
    . (Join-Path $root "scripts\devwatch.ps1")
    $lastStamp = [DateTime]::new([long]$initialStampTicks, [DateTimeKind]::Utc)
    while ($true) {
        $stamp = (Get-CashFluxDevSourceSnapshot -Root $root).LastWriteTimeUtc
        if ($stamp -gt $lastStamp) {
            $lastStamp = $stamp
            $env:GOOS = "js"
            $env:GOARCH = "wasm"
            $tmp = Join-Path $root "web\bin\services.wasm.dev-tmp"
            $out = Join-Path $root "web\bin\services.wasm"
            & go build -o $tmp ./cmd/cashflux-services
            if ($LASTEXITCODE -eq 0) {
                Move-Item -Force -LiteralPath $tmp -Destination $out
                $gz = "$out.gz"
                if (Test-Path -LiteralPath $gz) {
                    Remove-Item -LiteralPath $gz
                }
                Write-Output "services.wasm rebuilt"
            }
        }
        Start-Sleep -Milliseconds 750
    }
}

try {
    & (Join-Path $projectRoot ".tools\gwc.exe") dev `
        -app ".\main.go" `
        -root ".\web" `
        -html ".\web\index.html" `
        -wasm "web\bin\main.wasm" `
        -hot=false
}
finally {
    Stop-Job -Job $watchJob -ErrorAction SilentlyContinue
    Remove-Job -Job $watchJob -Force -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $servicesTmp) {
        Remove-Item -LiteralPath $servicesTmp
    }
}
