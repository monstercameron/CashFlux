# Stamps one release build identity (SV7a): writes web/build.json and the
# matching -ldflags fragment for the wasm build. Run BEFORE `go build`:
#
#   .\scripts\stamp-build.ps1
#   $stamp = Get-Content web\bin\ldflags.txt
#   $env:GOOS='js'; $env:GOARCH='wasm'
#   go build -ldflags "-s -w $stamp" -o web/bin/main.wasm .
#
# Skipping this step leaves the build ID at "dev" (no update banner, service
# worker activates immediately) — the right behaviour for `gwc dev`.
param(
    [string] $Out = "web/build.json",
    [string] $LDFlagsOut = "web/bin/ldflags.txt"
)
$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $projectRoot

$ldflagsDir = Split-Path -Parent $LDFlagsOut
if ($ldflagsDir) { New-Item -ItemType Directory -Force -Path $ldflagsDir | Out-Null }

& go run ./tools/stampbuild -out $Out -ldflags-out $LDFlagsOut
if ($LASTEXITCODE -ne 0) { throw "stampbuild failed" }
