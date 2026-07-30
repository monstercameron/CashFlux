# CashFlux two-artifact development entry point.
#
# gwc dev currently accepts one app target, so this script lets it own main.wasm
# and live reload while a small background watcher rebuilds services.wasm.

$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $projectRoot "web\bin"
$servicesOut = Join-Path $binDir "services.wasm"
$servicesTmp = Join-Path $binDir "services.wasm.dev-tmp"

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
Build-ServicesWasm

$watchJob = Start-Job -ArgumentList $projectRoot -ScriptBlock {
    param($root)
    Set-Location -LiteralPath $root
    $lastStamp = [DateTime]::MinValue
    while ($true) {
        $stamp = Get-ChildItem -Path $root -Recurse -Filter "*.go" |
            Where-Object { $_.FullName -notmatch "[\\/](web|node_modules|\.git)[\\/]" } |
            Sort-Object LastWriteTimeUtc -Descending |
            Select-Object -First 1 -ExpandProperty LastWriteTimeUtc
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
        -wasm "web\bin\main.wasm"
}
finally {
    Stop-Job -Job $watchJob -ErrorAction SilentlyContinue
    Remove-Job -Job $watchJob -Force -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $servicesTmp) {
        Remove-Item -LiteralPath $servicesTmp
    }
}
